package analyzer

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type fakeResolver struct {
	lookup map[string][]net.IPAddr
}

func (r *fakeResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	if addrs, ok := r.lookup[host]; ok {
		return addrs, nil
	}
	return nil, fmt.Errorf("no DNS record for %s", host)
}

func testClientWithTrustedHosts(cfg ClientConfig) *SafeClient {
	if cfg.TrustedHosts == nil {
		cfg.TrustedHosts = map[string]bool{}
	}
	cfg.TrustedHosts["127.0.0.1"] = true
	cfg.TrustedHosts["localhost"] = true
	cfg.TrustedHosts["::1"] = true
	return NewSafeClient(cfg)
}

func TestSafeClient_Get_BlacklistedIP(t *testing.T) {
	client := NewSafeClient(DefaultConfig())

	_, err := client.Get(context.Background(), "http://127.0.0.1:9999/page")
	if err == nil {
		t.Fatal("expected error for loopback IP")
	}
}

func TestSafeClient_Get_InvalidScheme(t *testing.T) {
	client := NewSafeClient(DefaultConfig())

	_, err := client.Get(context.Background(), "ftp://example.com/file")
	if err == nil {
		t.Fatal("expected error for ftp scheme")
	}
}

func TestSafeClient_FetchHTML_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "<html><body>hello</body></html>")
	}))
	defer server.Close()

	client := testClientWithTrustedHosts(DefaultConfig())
	body, err := client.FetchHTML(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "<html>") {
		t.Errorf("body does not contain expected HTML: %s", body)
	}
}

func TestSafeClient_FetchHTML_NonHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{0x00, 0x00, 0x00})
	}))
	defer server.Close()

	client := testClientWithTrustedHosts(DefaultConfig())
	_, err := client.FetchHTML(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for non-HTML content type")
	}
}

func TestSafeClient_FetchHTML_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := testClientWithTrustedHosts(DefaultConfig())
	_, err := client.FetchHTML(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestSafeClient_FetchHTML_BodyTooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		big := strings.Repeat("x", 20*1024*1024)
		w.Write([]byte(big))
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.MaxBodyBytes = 1024
	client := testClientWithTrustedHosts(cfg)
	_, err := client.FetchHTML(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for body too large")
	}
}

func TestSafeClient_Redirect_SameHost(t *testing.T) {
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count > 3 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}
		http.Redirect(w, r, "/next", http.StatusFound)
	}))
	defer server.Close()

	client := testClientWithTrustedHosts(DefaultConfig())
	resp, err := client.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error in successful redirect: %v", err)
	}
	resp.Body.Close()
}

func TestSafeClient_Redirect_PrivateHost(t *testing.T) {
	r := &fakeResolver{
		lookup: map[string][]net.IPAddr{
			"intranet": {{IP: net.ParseIP("192.168.1.1")}},
		},
	}

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://intranet/internal", http.StatusFound)
	}))
	defer redirector.Close()

	cfg := DefaultConfig()
	cfg.Resolver = r
	cfg.TrustedHosts = map[string]bool{"127.0.0.1": true}
	client := NewSafeClient(cfg)

	_, err := client.Get(context.Background(), redirector.URL)
	if err == nil {
		t.Fatal("expected error for redirect to private host")
	}
}

func TestSafeClient_Redirect_LoopbackIP(t *testing.T) {
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.1:9999/internal", http.StatusFound)
	}))
	defer redirector.Close()

	client := testClientWithTrustedHosts(DefaultConfig())
	_, err := client.Get(context.Background(), redirector.URL)
	if err == nil {
		t.Fatal("expected error for redirect to loopback IP on different port")
	}
}

func TestSafeClient_Redirect_MaxRedirects(t *testing.T) {
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count > 7 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}
		http.Redirect(w, r, "/next", http.StatusFound)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.MaxRedirects = 3
	client := testClientWithTrustedHosts(cfg)
	_, err := client.Get(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected redirect limit error")
	}
}

func TestSafeClient_FetchHTML_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.TotalTimeout = 100 * time.Millisecond
	client := testClientWithTrustedHosts(cfg)
	_, err := client.FetchHTML(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestSafeClient_UserAgent(t *testing.T) {
	var receivedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html></html>"))
	}))
	defer server.Close()

	client := testClientWithTrustedHosts(DefaultConfig())
	client.FetchHTML(context.Background(), server.URL)

	if receivedUA == "" {
		t.Error("User-Agent header not sent")
	}
}

func TestSafeClient_RequestHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if !strings.Contains(accept, "text/html") {
			t.Error("Accept header not set properly, got:", accept)
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html></html>"))
	}))
	defer server.Close()

	client := testClientWithTrustedHosts(DefaultConfig())
	client.FetchHTML(context.Background(), server.URL)
}

func TestDialContext_RejectsConnectionBeforeDial(t *testing.T) {
	var dialed bool

	dialer := &net.Dialer{Timeout: time.Second}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if ip := net.ParseIP(host); ip != nil {
				if !isPublicIP(ip) {
					return nil, &blacklistError{host: host, ip: ip.String(), kind: string(classifyIP(ip))}
				}
			}
			dialed = true
			return dialer.DialContext(ctx, network, addr)
		},
	}

	client := &http.Client{Transport: transport}
	req, _ := http.NewRequest("GET", "http://127.0.0.1:9999/", nil)

	_, err := client.Do(req)
	if err == nil {
		t.Fatal("expected error")
	}
	if dialed {
		t.Fatal("should not have dialed when target is blacklisted")
	}
}

func TestValidateURL_PublicIP(t *testing.T) {
	u, err := ValidateURL("http://8.8.8.8/page")
	if err != nil {
		t.Fatalf("public IP should be accepted by ValidateURL, got %v", err)
	}
	_ = u
}

func TestCheckRedirect_RejectsRedirectToLoopback(t *testing.T) {
	safe := NewSafeClient(DefaultConfig())

	origReq, _ := http.NewRequest("GET", "http://example.com/initial", nil)

	redirectURL, _ := url.Parse("http://127.0.0.1/internal")
	redirectReq := &http.Request{URL: redirectURL, Method: http.MethodGet}

	err := safe.checkRedirect(redirectReq, []*http.Request{origReq})
	if err == nil {
		t.Fatal("expected error for redirect to loopback")
	}
}

func TestCheckRedirect_UserInfoInRedirect(t *testing.T) {
	safe := NewSafeClient(DefaultConfig())

	req, _ := http.NewRequest("GET", "http://example.com/", nil)
	req.URL, _ = url.Parse("http://admin:secret@evil.com/")

	err := safe.checkRedirect(req, []*http.Request{})
	if err == nil {
		t.Fatal("expected error for user info in redirect")
	}
}

func TestCheckRedirect_TooManyRedirects(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxRedirects = 2
	safe := NewSafeClient(cfg)

	req, _ := http.NewRequest("GET", "http://example.com/", nil)

	err := safe.checkRedirect(req, []*http.Request{req, req, req})
	if err == nil {
		t.Fatal("expected too many redirects error")
	}
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxBodyBytes == 0 {
		t.Error("MaxBodyBytes should have a default")
	}
	if cfg.MaxRedirects == 0 {
		t.Error("MaxRedirects should have a default")
	}
	if cfg.ConnectTimeout == 0 {
		t.Error("ConnectTimeout should have a default")
	}
	if cfg.TotalTimeout == 0 {
		t.Error("TotalTimeout should have a default")
	}
	if cfg.UserAgent == "" {
		t.Error("UserAgent should have a default")
	}
}

func TestValidateAndResolveHost_NonPublicHostname(t *testing.T) {
	safe := NewSafeClient(DefaultConfig())

	err := safe.validateAndResolveHost("localhost")
	if err == nil {
		t.Fatal("expected localhost to fail DNS validation (it resolves to ::1 or 127.0.0.1)")
	}
}

func TestValidateAndResolveHost_PrivateIPDirect(t *testing.T) {
	safe := NewSafeClient(DefaultConfig())

	err := safe.validateAndResolveHost("192.168.1.1")
	if err == nil {
		t.Fatal("expected private IP to fail")
	}
}

func TestValidateAndResolveHost_LinkLocalDirect(t *testing.T) {
	safe := NewSafeClient(DefaultConfig())

	err := safe.validateAndResolveHost("169.254.1.1")
	if err == nil {
		t.Fatal("expected link-local IP to fail")
	}
}

func TestValidateAndResolveHost_IPv6Loopback(t *testing.T) {
	safe := NewSafeClient(DefaultConfig())

	err := safe.validateAndResolveHost("::1")
	if err == nil {
		t.Fatal("expected IPv6 loopback to fail")
	}
}

func TestValidateAndResolveHost_MultipleIPsAllPublic(t *testing.T) {
	safe := NewSafeClient(DefaultConfig())

	err := safe.validateAndResolveHost("example.com")
	if err != nil {
		t.Skipf("example.com resolution unexpectedly failed: %v (may be offline)", err)
	}
}

func TestValidateAndResolveHost_RejectsOneNonPublicInMultiple(t *testing.T) {
	r := &fakeResolver{
		lookup: map[string][]net.IPAddr{
			"mixed.local": {
				{IP: net.ParseIP("8.8.8.8")},
				{IP: net.ParseIP("127.0.0.1")},
			},
		},
	}

	cfg := DefaultConfig()
	cfg.Resolver = r
	safe := NewSafeClient(cfg)

	err := safe.validateAndResolveHost("mixed.local")
	if err == nil {
		t.Fatal("expected error when at least one resolved IP is non-public")
	}
}

func TestTrustedHost_SkipsValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TrustedHosts = map[string]bool{"127.0.0.1": true}
	safe := NewSafeClient(cfg)

	err := safe.validateAndResolveHost("127.0.0.1")
	if err != nil {
		t.Fatalf("trusted host should pass validation, got %v", err)
	}
}

func TestValidateAndResolveHost_MixedIPv6Result(t *testing.T) {
	r := &fakeResolver{
		lookup: map[string][]net.IPAddr{
			"multi.local": {
				{IP: net.ParseIP("2606:4700:4700::1111")},
				{IP: net.ParseIP("::1")},
			},
		},
	}

	cfg := DefaultConfig()
	cfg.Resolver = r
	safe := NewSafeClient(cfg)

	err := safe.validateAndResolveHost("multi.local")
	if err == nil {
		t.Fatal("expected error when one resolved IPv6 is loopback")
	}
}
