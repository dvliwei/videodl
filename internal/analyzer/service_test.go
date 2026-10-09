package analyzer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"videodl/internal/media"
)

func makeClientWithTrusted(host string) *SafeClient {
	c := NewSafeClient(ClientConfig{
		ConnectTimeout: 3 * time.Second,
		TotalTimeout:   5 * time.Second,
		MaxRedirects:   3,
		MaxBodyBytes:   5 * 1024 * 1024,
	})
	c.config.TrustedHosts[host] = true
	return c
}

func testServerURL(srv *httptest.Server, path string) string {
	u, _ := url.Parse(srv.URL + path)
	u.Host = strings.Replace(u.Host, "127.0.0.1", "localhost", 1)
	u.Host = strings.Replace(u.Host, "::1", "localhost", 1)
	return u.String()
}

func TestService_Analyze_ValidHTML(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Test Page</title></head><body>
		<video src="/videos/sample.mp4"></video>
		<video>
			<source src="/videos/alt.webm" type="video/webm">
		</video>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(html))
		case "/videos/sample.mp4", "/videos/alt.webm":
			w.Header().Set("Content-Type", "video/mp4")
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.ID == "" {
		t.Error("expected result ID to be populated")
	}

	if result.PageTitle != "Test Page" {
		t.Errorf("expected PageTitle 'Test Page', got %q", result.PageTitle)
	}

	if len(result.Candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(result.Candidates))
	}

	for _, c := range result.Candidates {
		if c.ID == "" {
			t.Error("candidate should have ID")
		}
		if c.SourceType != media.SourceDirect {
			t.Errorf("expected SourceDirect, got %s", c.SourceType)
		}
		if c.DisplayURL == "" {
			t.Error("candidate should have DisplayURL")
		}
	}
}

func TestService_Analyze_DirectMediaURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/movie.mp4"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}

	c := result.Candidates[0]
	if c.SourceType != media.SourceDirect {
		t.Errorf("expected SourceDirect, got %s", c.SourceType)
	}
	if c.Format != "mp4" {
		t.Errorf("expected format 'mp4', got %q", c.Format)
	}
	if c.Title != "movie" {
		t.Errorf("expected title 'movie', got %q", c.Title)
	}
	if !c.HasVideo {
		t.Error("expected HasVideo true")
	}
}

func TestService_Analyze_HLSManifest(t *testing.T) {
	manifest := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=1280x720,CODECS="avc1.64001e,mp4a.40.2"
variant-720.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=400000,RESOLUTION=854x480,CODECS="avc1.64001e,mp4a.40.2"
variant-480.m3u8
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/playlist.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Write([]byte(manifest))
		case "/variant-720.m3u8", "/variant-480.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Write([]byte("#EXTM3U\n#EXT-X-TARGETDURATION:10\n#EXTINF:10,\nseg0.ts\n#EXT-X-ENDLIST\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/playlist.m3u8"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}

	c := result.Candidates[0]
	if c.SourceType != media.SourceHLS {
		t.Errorf("expected SourceHLS, got %s", c.SourceType)
	}
	if len(c.Variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(c.Variants))
	}

	v720 := c.Variants[0]
	if v720.Height != 720 {
		t.Errorf("expected variant height 720, got %d", v720.Height)
	}
	if v720.Bandwidth != 800000 {
		t.Errorf("expected bandwidth 800000, got %d", v720.Bandwidth)
	}
}

func TestService_Analyze_DASHManifest(t *testing.T) {
	mpd := `<?xml version="1.0"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static">
  <Period>
    <AdaptationSet mimeType="video/mp4">
      <Representation id="1" bandwidth="800000" width="1280" height="720" codecs="avc1.64001e">
        <SegmentTemplate media="video-seg/segment-$RepresentationID$-0.m4s" />
      </Representation>
      <Representation id="2" bandwidth="400000" width="854" height="480" codecs="avc1.64001e">
        <SegmentTemplate media="video-seg/segment-$RepresentationID$-0.m4s" />
      </Representation>
    </AdaptationSet>
    <AdaptationSet mimeType="audio/mp4">
      <Representation id="3" bandwidth="128000" codecs="mp4a.40.2">
        <SegmentTemplate media="audio-seg/segment-$RepresentationID$-0.m4s" />
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dash+xml")
		w.Write([]byte(mpd))
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/manifest.mpd"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}

	c := result.Candidates[0]
	if c.SourceType != media.SourceDASH {
		t.Errorf("expected SourceDASH, got %s", c.SourceType)
	}
	if len(c.Variants) < 1 {
		t.Errorf("expected at least 1 variant, got %d", len(c.Variants))
	}
	if !c.HasAudio {
		t.Error("expected HasAudio true")
	}
}

func TestService_Analyze_EmptyResult(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Empty</title></head><body><p>No media here.</p></body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(result.Candidates))
	}

	if len(result.Warnings) == 0 {
		t.Error("expected a warning for empty result")
	}
}

func TestService_Analyze_NetworkFailure(t *testing.T) {
	// httptest server is not started - use a URL that will fail TCP connection
	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := svc.Analyze(ctx, "http://127.0.0.1:1/nope.html")
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}

func TestService_Analyze_HTTPErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.Analyze(ctx, testServerURL(srv, "/secret.html"))
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
}

func TestService_Analyze_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	var gotErr error
	go func() {
		_, gotErr = svc.Analyze(ctx, testServerURL(srv, "/"))
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
		if !errors.Is(gotErr, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", gotErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Analyze did not respect cancel")
	}
}

func TestService_Analyze_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := svc.Analyze(ctx, testServerURL(srv, "/"))
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestService_Analyze_URLValidation(t *testing.T) {
	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	cases := []struct {
		name string
		url  string
	}{
		{"ftp scheme", "ftp://example.com/file.mp4"},
		{"file scheme", "file:///etc/passwd"},
		{"javascript", "javascript:alert(1)"},
		{"user info", "http://admin:pass@example.com/video.mp4"},
		{"loopback", "http://127.0.0.1/video.mp4"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := svc.Analyze(ctx, c.url)
			if err == nil {
				t.Errorf("expected error for %s, got nil", c.url)
			}
		})
	}
}

func TestService_Analyze_DeduplicatesCandidates(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Dupes</title></head><body>
		<video src="https://example.com/video.mp4"></video>
		<video><source src="https://example.com/video.mp4"></video>
		<source src="https://example.com/video.mp4" type="video/mp4">
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Candidates) != 1 {
		t.Errorf("expected 1 deduplicated candidate, got %d", len(result.Candidates))
	}
}

func TestService_Analyze_PartialFailure(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Partial</title></head><body>
		<video src="http://localhost:1/noexist/movie.mp4"></video>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/"))
	if err != nil {
		t.Fatalf("Analyze should succeed even with broken candidate: %v", err)
	}

	if len(result.Candidates) == 0 && len(result.Warnings) == 0 {
		t.Error("expected either candidates or warnings for broken candidate URL")
	}
}

func TestService_Analyze_DRMDetection(t *testing.T) {
	manifest := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-KEY:METHOD=AES-128,URI="key.bin"
#EXTINF:10,
seg0.ts
#EXT-X-ENDLIST
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte(manifest))
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/protected.m3u8"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}

	c := result.Candidates[0]
	if c.Unsupported == "" {
		t.Error("expected DRM reason in Unsupported field")
	}
}

func TestService_Analyze_NoDuplicateID(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>IDs</title></head><body>
		<video src="/a.mp4"></video>
		<video src="/b.webm"></video>
		<video src="/c.mov"></video>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(html))
		default:
			w.Header().Set("Content-Type", "video/mp4")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	seen := map[string]bool{}
	for _, c := range result.Candidates {
		if seen[c.ID] {
			t.Errorf("duplicate candidate ID: %s", c.ID)
		}
		seen[c.ID] = true
	}

	if len(seen) != 3 {
		t.Errorf("expected 3 unique IDs, got %d", len(seen))
	}
}

func TestService_Analyze_ResultFieldsStable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/webm")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/clip.webm"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.ID == "" {
		t.Error("AnalysisResult.ID missing")
	}
	if result.PageTitle != "" {
		t.Error("direct media should have empty PageTitle")
	}
	if len(result.Candidates) != 1 {
		t.Fatal("expected exactly 1 candidate")
	}

	c := result.Candidates[0]
	checkStableFields(t, &c)
}

func checkStableFields(t *testing.T, c *media.MediaCandidate) {
	t.Helper()
	if c.ID == "" {
		t.Error("MediaCandidate.ID missing")
	}
	if c.SourceType == "" {
		t.Error("MediaCandidate.SourceType missing")
	}
	if c.DisplayURL == "" {
		t.Error("MediaCandidate.DisplayURL missing")
	}
	if c.Title == "" {
		t.Error("MediaCandidate.Title missing")
	}
	if c.DisplayURL != "" {
		u, err := url.Parse(c.DisplayURL)
		if err != nil {
			t.Errorf("DisplayURL not parseable: %v", err)
		} else {
			if u.RawQuery != "" || u.Fragment != "" {
				t.Errorf("DisplayURL should not contain query or fragment: %s", c.DisplayURL)
			}
		}
	}
}

func TestService_Analyze_ManifestViaHTML(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Manifest In Page</title></head><body>
		<video>
			<source src="/stream.m3u8" type="application/vnd.apple.mpegurl">
		</video>
	</body></html>`

	manifest := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=500000,RESOLUTION=640x360,CODECS="avc1,mp4a"
low.m3u8
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(html))
		case "/stream.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Write([]byte(manifest))
		default:
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Write([]byte("#EXTM3U\n#EXT-X-TARGETDURATION:10\n#EXTINF:10,\nseg0.ts\n#EXT-X-ENDLIST\n"))
		}
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}

	c := result.Candidates[0]
	if c.SourceType != media.SourceHLS {
		t.Errorf("expected SourceHLS, got %s", c.SourceType)
	}
	if len(c.Variants) != 1 {
		t.Errorf("expected 1 variant, got %d", len(c.Variants))
	}
}

func TestService_Analyze_FetchErrorOnBodyTooLarge(t *testing.T) {
	big := make([]byte, 10*1024*1024+1)
	for i := range big {
		big[i] = 'x'
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(big)
	}))
	defer srv.Close()

	client := NewSafeClient(ClientConfig{
		ConnectTimeout: 3 * time.Second,
		TotalTimeout:   5 * time.Second,
		MaxBodyBytes:   1024,
	})
	client.config.TrustedHosts["localhost"] = true
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.Analyze(ctx, testServerURL(srv, "/big.html"))
	if err == nil {
		t.Fatal("expected body too large error, got nil")
	}
}

func TestService_Analyze_InvalidManifest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte("this is not a valid manifest at all"))
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.Analyze(ctx, testServerURL(srv, "/broken.m3u8"))
	if err == nil {
		t.Fatal("expected manifest parse error")
	}
}

func TestService_DetectDirectMediaByContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/track"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}

	c := result.Candidates[0]
	if c.SourceType != media.SourceDirect {
		t.Errorf("expected SourceDirect for audio content, got %s", c.SourceType)
	}
	if !c.HasAudio {
		t.Error("audio content should set HasAudio")
	}
}

func TestService_Analyze_MultipleVariantsHLS(t *testing.T) {
	manifest := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=2000000,RESOLUTION=1920x1080,CODECS="avc1.640028,mp4a.40.2"
1080p.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=1280x720,CODECS="avc1.64001e,mp4a.40.2"
720p.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=400000,RESOLUTION=854x480,CODECS="avc1.64001e,mp4a.40.2"
480p.m3u8
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="English",LANGUAGE="en",URI="audio-en.m3u8",DEFAULT=YES
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/master.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Write([]byte(manifest))
		default:
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Write([]byte("#EXTM3U\n#EXT-X-TARGETDURATION:10\n#EXTINF:10,\nseg0.ts\n#EXT-X-ENDLIST\n"))
		}
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/master.m3u8"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}

	c := result.Candidates[0]
	if len(c.Variants) != 3 {
		t.Fatalf("expected 3 variants, got %d", len(c.Variants))
	}

	heights := map[int]bool{}
	for _, v := range c.Variants {
		heights[v.Height] = true
	}
	if !heights[1080] || !heights[720] || !heights[480] {
		t.Errorf("expected heights 1080, 720, 480; got %v", heights)
	}
}

func TestService_Analyze_RelativeURLsInHTML(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Relative</title></head><body>
		<video src="media/video.mp4"></video>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page/index.html":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(html))
		default:
			w.Header().Set("Content-Type", "video/mp4")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/page/index.html"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}

	c := result.Candidates[0]
	if !strings.Contains(c.DisplayURL, "/page/media/video.mp4") {
		t.Errorf("expected DisplayURL to be resolved relative to page, got %s", c.DisplayURL)
	}
}

func TestService_Analyze_AllCandidatesFailIndependently(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Independence</title></head><body>
		<video src="http://localhost:1/z.mp4"></video>
		<video src="http://localhost:1/y.mp4"></video>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/indep.html"))
	if err != nil {
		t.Fatalf("Analyze should succeed even when all candidates are broken: %v", err)
	}

	if len(result.Candidates) == 0 && len(result.Warnings) == 0 {
		t.Error("expected either candidates or warnings")
	}
}

func TestService_Analyze_CoreFunctions(t *testing.T) {
	t.Run("deriveTitle from path", func(t *testing.T) {
		cases := []struct {
			path string
			want string
		}{
			{"/videos/my-file.mp4", "my-file"},
			{"/", "untitled"},
			{"", "untitled"},
			{"movie.mkv", "movie"},
		}
		for _, c := range cases {
			got := deriveTitle(c.path)
			if got != c.want {
				t.Errorf("deriveTitle(%q) = %q, want %q", c.path, got, c.want)
			}
		}
	})

	t.Run("isMediaExtension", func(t *testing.T) {
		if !isMediaExtension(".mp4") {
			t.Error(".mp4 should be media extension")
		}
		if !isMediaExtension(".mp3") {
			t.Error(".mp3 should be media extension")
		}
		if isMediaExtension(".html") {
			t.Error(".html should not be media extension")
		}
		if isMediaExtension(".txt") {
			t.Error(".txt should not be media extension")
		}
	})

	t.Run("isMediaContentType", func(t *testing.T) {
		if !isMediaContentType("video/mp4") {
			t.Error("video/mp4 should be media content type")
		}
		if !isMediaContentType("audio/mpeg") {
			t.Error("audio/mpeg should be media content type")
		}
		if isMediaContentType("text/html") {
			t.Error("text/html should not be media content type")
		}
	})

	t.Run("firstNonEmpty", func(t *testing.T) {
		if firstNonEmpty("", " ", "ok", "other") != "ok" {
			t.Error("should pick first non-empty")
		}
		if firstNonEmpty("", "", "") != "" {
			t.Error("should return empty when all empty")
		}
	})
}

func TestService_Analyze_ConcurrentProbe(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Concurrent</title></head><body>
		<video src="/a.mp4"></video>
		<video src="/b.webm"></video>
	</body></html>`

	var accessCount int
	var mu chan struct{}
	mu = make(chan struct{}, 8)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case mu <- struct{}{}:
		default:
		}
		defer func() { <-mu }()
		accessCount++

		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(html))
		default:
			w.Header().Set("Content-Type", "video/mp4")
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	client := makeClientWithTrusted("localhost")
	svc := NewAnalysisService(client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.Analyze(ctx, testServerURL(srv, "/"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(result.Candidates))
	}

	t.Logf("accessCount: %d", accessCount)
}
