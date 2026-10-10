package netguard

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordingValidator struct {
	mu          sync.Mutex
	hosts       []string
	err         error
	blockedHost string
}

func (v *recordingValidator) ValidateHost(_ context.Context, host string) error {
	v.mu.Lock()
	v.hosts = append(v.hosts, host)
	v.mu.Unlock()
	if v.blockedHost == host {
		return errors.New("blocked host")
	}
	return v.err
}

func TestProxyForwardsHTTPAndValidatesHost(t *testing.T) {
	upstream := newIPv4Server(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()
	validator := &recordingValidator{}
	proxy, err := NewProxy(context.Background(), validator, ProxyConfig{ConnectTimeout: time.Second, TotalTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()

	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(mustURL(t, proxy.URL()))}}
	resp, err := client.Get(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}
	validator.mu.Lock()
	defer validator.mu.Unlock()
	if len(validator.hosts) == 0 {
		t.Fatal("proxy did not validate upstream host")
	}
}

func TestProxyRejectsBeforeDialAndRedirectTarget(t *testing.T) {
	validator := &recordingValidator{err: errors.New("blocked address")}
	proxy, err := NewProxy(context.Background(), validator, ProxyConfig{ConnectTimeout: time.Second, TotalTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(mustURL(t, proxy.URL()))}}
	resp, err := client.Get("http://127.0.0.1:9/not-dialed")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		resp.Body.Close()
		t.Fatalf("blocked response status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	redirectServer := newIPv4Server(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://blocked.example/next", http.StatusFound)
	}))
	defer redirectServer.Close()
	validator.err = nil
	validator.blockedHost = "blocked.example"
	client.CheckRedirect = nil
	resp, err = client.Get(redirectServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("redirect target response status = %d", resp.StatusCode)
	}
}

func TestProxyRejectsUnsupportedSchemeAndCONNECTTarget(t *testing.T) {
	validator := &recordingValidator{err: errors.New("blocked")}
	proxy, err := NewProxy(context.Background(), validator, ProxyConfig{ConnectTimeout: time.Second, TotalTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(mustURL(t, proxy.URL()))}}
	if _, err := client.Get("ftp://example.test/file"); err == nil {
		t.Fatal("expected unsupported scheme error")
	}

	conn, err := net.Dial("tcp", strings.TrimPrefix(proxy.URL(), "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = fmt.Fprintf(conn, "CONNECT 127.0.0.1:443 HTTP/1.1\r\nHost: 127.0.0.1:443\r\n\r\n")
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "403") {
		t.Fatalf("CONNECT response = %q", line)
	}
}

func TestProxyCloseIsIdempotent(t *testing.T) {
	proxy, err := NewProxy(context.Background(), &recordingValidator{}, ProxyConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if err := proxy.Close(); err != nil {
		t.Fatal(err)
	}
	if err := proxy.Close(); err != nil {
		t.Fatal(err)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func newIPv4Server(handler http.Handler) *httptest.Server {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: handler}}
	server.Start()
	return server
}
