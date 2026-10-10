package proxy

import (
	"net"
	"net/http"
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
	if envProxy := environmentProxyURL(); envProxy != "" {
		return envProxy
	}
	return platformSystemProxyURL()
}

// ProxyForRequest follows the process proxy settings and honors NO_PROXY. If
// no proxy environment is configured, it falls back to the platform setting.
func ProxyForRequest(req *http.Request) (*url.URL, error) {
	if req == nil || req.URL == nil {
		return nil, nil
	}
	proxyURL, err := http.ProxyFromEnvironment(req)
	if err != nil || proxyURL != nil || hasProxyEnvironment() {
		return proxyURL, err
	}
	return parseProxyURL(platformSystemProxyURL())
}

// IsConfiguredAddress reports whether address is the configured upstream
// proxy. The safe analyzer transport may dial this private/local endpoint;
// target hosts are validated before requests and redirects are followed.
func IsConfiguredAddress(address string) bool {
	for _, raw := range configuredProxyURLs() {
		if IsProxyAddress(raw, address) {
			return true
		}
	}
	return false
}

// IsProxyAddress reports whether address is the host:port from rawProxyURL.
func IsProxyAddress(rawProxyURL, address string) bool {
	proxyURL, err := parseProxyURL(rawProxyURL)
	if err != nil || proxyURL == nil {
		return false
	}
	port := proxyURL.Port()
	if port == "" {
		port = "80"
	}
	return net.JoinHostPort(proxyURL.Hostname(), port) == address
}

func configuredProxyURLs() []string {
	values := make([]string, 0, 5)
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if validated := ValidateHTTPProxyURL(strings.TrimSpace(os.Getenv(key))); validated != "" {
			values = append(values, validated)
		}
	}
	if len(values) == 0 {
		if platform := platformSystemProxyURL(); platform != "" {
			values = append(values, platform)
		}
	}
	return values
}

func environmentProxyURL() string {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if validated := ValidateHTTPProxyURL(strings.TrimSpace(os.Getenv(key))); validated != "" {
			return validated
		}
	}
	return ""
}

func hasProxyEnvironment() bool {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

func parseProxyURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, nil
	}
	u, err := url.Parse(raw)
	if err != nil || ValidateHTTPProxyURL(raw) == "" {
		if err == nil {
			err = &url.Error{Op: "parse", URL: raw, Err: errInvalidProxyURL{}}
		}
		return nil, err
	}
	return u, nil
}

type errInvalidProxyURL struct{}

func (errInvalidProxyURL) Error() string { return "invalid HTTP proxy URL" }

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
