package analyzer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestFetchManifest_HLSFromServer(t *testing.T) {
	body := `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1000000,RESOLUTION=640x360
low.m3u8
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	res, err := client.FetchManifest(context.Background(), srv.URL+("/manifest.m3u8"))
	if err != nil {
		t.Fatalf("FetchManifest() error = %v", err)
	}

	if len(res.Body) == 0 {
		t.Fatal("empty body")
	}
	if _, err := ParseManifest(res.Body, res.FinalURL); err != nil {
		t.Errorf("ParseManifest() error = %v", err)
	}
	if res.FetchedAt.IsZero() {
		t.Error("FetchedAt should be set")
	}
	if len(res.RequestHeader.Get("User-Agent")) == 0 {
		t.Error("User-Agent should be present in request headers")
	}
}

func newTestClient(t *testing.T, srv *httptest.Server) *SafeClient {
	t.Helper()
	u := mustURL(t, srv.URL)
	cfg := DefaultConfig()
	cfg.TrustedHosts = map[string]bool{u.Hostname(): true}
	return NewSafeClient(cfg)
}

func TestFetchManifest_RejectsHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>nope</body></html>"))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	_, err := client.FetchManifest(context.Background(), srv.URL+("/index.html"))
	if err == nil {
		t.Fatal("expected error for HTML response")
	}
	if !strings.Contains(err.Error(), "not a manifest") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFetchManifest_RejectsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	_, err := client.FetchManifest(context.Background(), srv.URL+("/m.m3u8"))
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestFetchManifestWithHeaders_SanitizesCredentials(t *testing.T) {
	var seenAuth, seenReferer, seenCookie, seenOrigin string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenReferer = r.Header.Get("Referer")
		seenCookie = r.Header.Get("Cookie")
		seenOrigin = r.Header.Get("Origin")
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte("#EXTM3U\n#EXT-X-TARGETDURATION:1\nseg.ts\n"))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	extra := http.Header{}
	extra.Set("Authorization", "Bearer SECRET")
	extra.Set("Cookie", "session=xyz")
	extra.Set("Referer", "https://page.example.com/")
	extra.Set("Origin", "https://page.example.com")

	_, err := client.FetchManifestWithHeaders(context.Background(), srv.URL+("/m.m3u8"), extra)
	if err != nil {
		t.Fatalf("FetchManifestWithHeaders() error = %v", err)
	}

	if seenAuth != "" {
		t.Error("Authorization must not be forwarded")
	}
	if seenCookie != "" {
		t.Error("Cookie must not be forwarded")
	}
	if seenReferer != "https://page.example.com/" {
		t.Errorf("Referer = %q, want original referer", seenReferer)
	}
	if seenOrigin != "https://page.example.com" {
		t.Errorf("Origin = %q, want original origin", seenOrigin)
	}
}

func TestFetchManifest_RequestHeaderSafety_DefaultHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte("#EXTM3U\nseg.ts\n"))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	res, err := client.FetchManifest(context.Background(), srv.URL+("/m.m3u8"))
	if err != nil {
		t.Fatalf("FetchManifest() error = %v", err)
	}
	h := res.RequestHeader
	if !strings.Contains(h.Get("User-Agent"), "VideoDL") {
		t.Errorf("User-Agent = %q, want VideoDL product token", h.Get("User-Agent"))
	}
	if strings.HasPrefix(h.Get("Authorization"), "Bearer") {
		t.Error("no default Authorization header should be injected")
	}
}

func TestFetchManifest_FinalURLResolved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte("#EXTM3U\nseg.ts\n"))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	raw := srv.URL + "/m.m3u8?token=abc"
	res, err := client.FetchManifest(context.Background(), raw)
	if err != nil {
		t.Fatalf("FetchManifest() error = %v", err)
	}
	wantBase := mustURL(t, srv.URL+"/m.m3u8?token=abc")
	if res.FinalURL.Path != wantBase.Path {
		t.Errorf("FinalURL.Path = %q, want %q", res.FinalURL.Path, wantBase.Path)
	}
}

func TestFetchManifest_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.TotalTimeout = 50 * time.Millisecond
	client := NewSafeClient(cfg)

	ctx := context.Background()
	_, err := client.FetchManifest(ctx, srv.URL+("/m.m3u8"))
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestSanitizeForwardableHeaders(t *testing.T) {
	extra := http.Header{}
	extra.Set("Authorization", "Bearer x")
	extra.Set("Cookie", "k=v")
	extra.Set("Referer", "https://r.example/")
	extra.Set("Origin", "https://o.example")
	extra.Set("X-Custom-Signature", "abc")

	out := sanitizeForwardableHeaders(extra)

	if got := out.Get("Authorization"); got != "" {
		t.Errorf("Authorization leaked: %q", got)
	}
	if got := out.Get("Cookie"); got != "" {
		t.Errorf("Cookie leaked: %q", got)
	}
	if got := out.Get("X-Custom-Signature"); got != "" {
		t.Errorf("custom header leaked: %q", got)
	}
	if got := out.Get("Referer"); got != "https://r.example/" {
		t.Errorf("Referer = %q", got)
	}
	if got := out.Get("Origin"); got != "https://o.example" {
		t.Errorf("Origin = %q", got)
	}
}

func TestManifestWorkflow_RealParse(t *testing.T) {
	body := sampleMultivariantHLS()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	fetched, err := client.FetchManifest(context.Background(), srv.URL+("/master.m3u8"))
	if err != nil {
		t.Fatalf("FetchManifest() error = %v", err)
	}

	res, err := ParseManifest(fetched.Body, fetched.FinalURL)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if res.SourceType != "hls" {
		t.Errorf("SourceType = %q, want hls", res.SourceType)
	}
	if res.DRM {
		t.Error("clear HLS should not be flagged as DRM")
	}
	if len(res.Variants) != 3 {
		t.Fatalf("len(Variants) = %d, want 3", len(res.Variants))
	}
	for _, v := range res.Variants {
		if v.SegmentURL == "" {
			t.Error("Variant segment URL should be resolved from manifest")
		}
	}

	if fetched.FinalURL.Scheme != "http" && fetched.FinalURL.Scheme != "https" {
		t.Errorf("unexpected FinalURL scheme: %s", fetched.FinalURL.Scheme)
	}
}

func TestResolveReference_SkipsInvalid(t *testing.T) {
	base, _ := url.Parse("https://cdn.example.com/a/b.m3u8")
	cases := []struct{ in, want string }{
		{"seg.ts", "https://cdn.example.com/a/seg.ts"},
		{"//other.com/c.m3u8", "https://other.com/c.m3u8"},
		{"data:application/octet-stream,AA", ""},
		{"javascript:alert(1)", ""},
		{"", ""},
		{"ftp://host/x", ""},
	}
	for _, c := range cases {
		got := resolveReference(c.in, base)
		if got != c.want {
			t.Errorf("resolveReference(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
