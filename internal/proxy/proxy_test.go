package proxy

import (
	"net/http"
	"testing"
)

func TestResolveSystemProxyURL_EnvOverride(t *testing.T) {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		t.Setenv(key, "")
	}

	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7890")
	if got := ResolveSystemProxyURL(); got != "http://127.0.0.1:7890" {
		t.Fatalf("ResolveSystemProxyURL() = %q, want HTTPS_PROXY value", got)
	}

	t.Setenv("HTTPS_PROXY", "https://127.0.0.1:7890")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:8080")
	if got := ResolveSystemProxyURL(); got != "http://127.0.0.1:8080" {
		t.Fatalf("unsupported HTTPS proxy should fall back to HTTP proxy, got %q", got)
	}
}

func TestValidateHTTPProxyURL(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"http://127.0.0.1:7890", "http://127.0.0.1:7890"},
		{"http://proxy.example.com:8080", "http://proxy.example.com:8080"},
		{"http://user:pass@127.0.0.1:7890", ""},
		{"https://127.0.0.1:7890", ""},
		{"socks5://127.0.0.1:7890", ""},
		{"http://127.0.0.1:7890/path", ""},
		{"http://127.0.0.1:7890?x=1", ""},
		{"http://:7890", ""},
		{"http://127.0.0.1", "http://127.0.0.1"},
		{"not-a-url", ""},
	}
	for _, c := range cases {
		if got := ValidateHTTPProxyURL(c.input); got != c.want {
			t.Errorf("ValidateHTTPProxyURL(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestProxyForRequestUsesEnvironmentAndHonorsNoProxy(t *testing.T) {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "NO_PROXY", "no_proxy"} {
		t.Setenv(key, "")
	}
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:7890")
	t.Setenv("NO_PROXY", "localhost")

	req, err := http.NewRequest(http.MethodGet, "http://public.example/video", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ProxyForRequest(req)
	if err != nil || got == nil || got.String() != "http://127.0.0.1:7890" {
		t.Fatalf("ProxyForRequest() = %v, %v", got, err)
	}

	localhostReq, err := http.NewRequest(http.MethodGet, "http://localhost/video", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err = ProxyForRequest(localhostReq)
	if err != nil || got != nil {
		t.Fatalf("ProxyForRequest() with NO_PROXY = %v, %v; want no proxy", got, err)
	}
}

func TestIsConfiguredAddressRecognizesAllEnvironmentProxyURLs(t *testing.T) {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		t.Setenv(key, "")
	}
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7890")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:8080")

	if !IsConfiguredAddress("127.0.0.1:7890") || !IsConfiguredAddress("127.0.0.1:8080") {
		t.Fatal("configured proxy address was not recognized")
	}
}
