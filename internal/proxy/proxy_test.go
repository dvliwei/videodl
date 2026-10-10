package proxy

import (
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
