package proxy

import (
	"net/url"
	"os"
	"strings"
)

// ResolveSystemProxyURL returns a conventional HTTP proxy URL suitable for
// passing to subprocesses that support the standard HTTPS_PROXY / HTTP_PROXY
// environment or a dedicated -http_proxy flag. It first inspects the process
// environment and then falls back to platform-specific system proxy settings
// (for example, macOS Network Preferences). An empty string is returned when
// no usable proxy is configured.
func ResolveSystemProxyURL() string {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			continue
		}
		if validated := ValidateHTTPProxyURL(raw); validated != "" {
			return validated
		}
	}
	return platformSystemProxyURL()
}

// ValidateHTTPProxyURL accepts any URL and returns it unchanged only when it
// represents an HTTP proxy with no credentials or path/query/fragment
// components. An empty string is returned for everything else.
func ValidateHTTPProxyURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if !strings.EqualFold(u.Scheme, "http") {
		return ""
	}
	if u.Hostname() == "" {
		return ""
	}
	if u.User != nil {
		return ""
	}
	if u.Path != "" && u.Path != "/" {
		return ""
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	return raw
}
