package analyzer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"videodl/internal/media"
	"videodl/internal/ytdlp"
)

type fakeYTDLPExtractor struct {
	info *ytdlp.Info
	err  error
	req  ytdlp.ExtractRequest
}

func (f *fakeYTDLPExtractor) Extract(_ context.Context, req ytdlp.ExtractRequest) (*ytdlp.Info, error) {
	f.req = req
	return f.info, f.err
}

type fakeYTDLPProxy struct {
	url    string
	closed bool
}

func (p *fakeYTDLPProxy) URL() string { return p.url }
func (p *fakeYTDLPProxy) Close() error {
	p.closed = true
	return nil
}

func TestAnalyzeYTDLPSuccess(t *testing.T) {
	extractor := &fakeYTDLPExtractor{info: &ytdlp.Info{
		ID: "video-1", Title: "yt-dlp result", Extractor: "example",
		WebpageURL: "https://video.example/watch?token=secret",
		Formats:    []ytdlp.Format{{FormatID: "18", URL: "https://cdn.example/v?sig=secret", VCodec: "avc1", ACodec: "mp4a", Ext: "mp4"}},
	}}
	proxy := &fakeYTDLPProxy{url: "http://127.0.0.1:12345"}
	svc := newYTDLPService(extractor, proxy, nil)
	result, err := svc.Analyze(context.Background(), "https://video.example/watch")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].SourceType != media.SourceYTDLP {
		t.Fatalf("result = %#v", result)
	}
	if extractor.req.ProxyURL != proxy.url || !proxy.closed {
		t.Fatalf("proxy lifecycle/request = %#v, closed=%v", extractor.req, proxy.closed)
	}
	if extractor.req.Browser != nil {
		t.Fatal("default analysis unexpectedly requested a browser session")
	}
}

func TestYTDLPFallbackToNativeOnUnsupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><video src="/video.mp4"></video></body></html>`))
	}))
	defer srv.Close()
	extractor := &fakeYTDLPExtractor{err: ytdlp.ErrExtractorUnsupported}
	proxy := &fakeYTDLPProxy{url: "http://127.0.0.1:12346"}
	svc := newYTDLPService(extractor, proxy, nil)
	result, err := svc.Analyze(context.Background(), testServerURL(srv, "/"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].SourceType != media.SourceDirect {
		t.Fatalf("fallback result = %#v", result)
	}
}

func TestYTDLPAuthRequiredWithoutBrowserAuthorization(t *testing.T) {
	extractor := &fakeYTDLPExtractor{err: ytdlp.ErrAuthRequired}
	svc := newYTDLPService(extractor, &fakeYTDLPProxy{url: "http://127.0.0.1:12347"}, nil)
	_, err := svc.Analyze(context.Background(), "https://video.example/watch")
	if !errors.Is(err, ytdlp.ErrAuthRequired) {
		t.Fatalf("expected auth required, got %v", err)
	}
}

func TestAnalyzeWithBrowserSessionIsExplicitAndValidated(t *testing.T) {
	extractor := &fakeYTDLPExtractor{info: &ytdlp.Info{
		ID: "video-1", Title: "authorized", Formats: []ytdlp.Format{{FormatID: "1", URL: "https://cdn.example/v", VCodec: "avc1"}},
	}}
	proxy := &fakeYTDLPProxy{url: "http://127.0.0.1:12348"}
	svc := newYTDLPService(extractor, proxy, nil)
	_, err := svc.AnalyzeWithOptions(context.Background(), "https://video.example/watch", AnalyzeOptions{
		BrowserSession: &ytdlp.BrowserSession{Browser: "chrome", Profile: "Profile 1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if extractor.req.Browser == nil || extractor.req.Browser.Browser != "chrome" || extractor.req.Browser.Profile != "Profile 1" {
		t.Fatalf("browser request = %#v", extractor.req)
	}
}

func TestYTDLPBlockedTimeoutAndCancellationAreNotNativeFallbacks(t *testing.T) {
	for name, factoryErr := range map[string]error{
		"blocked":  errors.New("netguard: upstream address blocked"),
		"timeout":  context.DeadlineExceeded,
		"canceled": context.Canceled,
	} {
		t.Run(name, func(t *testing.T) {
			extractor := &fakeYTDLPExtractor{err: factoryErr}
			proxy := &fakeYTDLPProxy{url: "http://127.0.0.1:12349"}
			svc := newYTDLPService(extractor, proxy, factoryErr)
			_, err := svc.Analyze(context.Background(), "https://video.example/watch")
			if err == nil {
				t.Fatal("expected non-fallback error")
			}
		})
	}
}

func TestResolveYTDLPInputs_MultipleStreamsAndHeaders(t *testing.T) {
	extractor := &fakeYTDLPExtractor{info: &ytdlp.Info{
		RequestedFormats: []ytdlp.Format{
			{FormatID: "137", URL: "https://cdn.example/video", HTTPHeaders: map[string]string{"Referer": "https://video.example/"}},
			{FormatID: "140", URL: "https://cdn.example/audio", HTTPHeaders: map[string]string{"User-Agent": "VideoDL/1.0"}},
		},
	}}
	proxy := &fakeYTDLPProxy{url: "http://127.0.0.1:12350"}
	svc := newYTDLPService(extractor, proxy, nil)
	inputs, err := svc.ResolveYTDLPInputs(context.Background(), &media.YTDLPSource{PageURL: "https://video.example/watch"}, "137+140")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 2 || inputs[0].URL == inputs[1].URL || inputs[0].Headers["Referer"] == "" || inputs[1].Headers["User-Agent"] == "" {
		t.Fatalf("inputs = %#v", inputs)
	}
	if extractor.req.FormatSelector != "137+140" || !proxy.closed {
		t.Fatalf("re-resolution request/lifecycle = %#v, closed=%v", extractor.req, proxy.closed)
	}
}

func TestResolveYTDLPInputs_PropagatesReresolveFailure(t *testing.T) {
	extractor := &fakeYTDLPExtractor{err: errors.New("temporary extractor failure")}
	svc := newYTDLPService(extractor, &fakeYTDLPProxy{url: "http://127.0.0.1:12351"}, nil)
	_, err := svc.ResolveYTDLPInputs(context.Background(), &media.YTDLPSource{PageURL: "https://video.example/watch"}, "137")
	if err == nil || !strings.Contains(err.Error(), "temporary extractor failure") {
		t.Fatalf("expected re-resolution failure, got %v", err)
	}
}

func newYTDLPService(extractor *fakeYTDLPExtractor, proxy *fakeYTDLPProxy, factoryErr error) *AnalysisService {
	client := makeClientWithTrusted("video.example")
	client.config.TrustedHosts["localhost"] = true
	client.config.TrustedHosts["cdn.example"] = true
	return NewAnalysisServiceWithYTDLP(client, nil, extractor, func(context.Context) (YTDLPProxy, error) {
		if factoryErr != nil {
			return nil, factoryErr
		}
		return proxy, nil
	}).WithOptions(ServiceOptions{ProbeTimeout: int((5 * time.Second).Seconds())})
}
