package analyzer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"videodl/internal/proxy"
)

const (
	defaultConnectTimeout = 10 * time.Second
	defaultTotalTimeout   = 30 * time.Second
	defaultMaxRedirects   = 5
	defaultMaxBodyBytes   = 10 * 1024 * 1024
)

type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type ClientConfig struct {
	ConnectTimeout time.Duration
	TotalTimeout   time.Duration
	MaxRedirects   int
	MaxBodyBytes   int64
	UserAgent      string
	// ProxyURL overrides system proxy discovery for callers that need an
	// explicit, validated HTTP proxy (for example, deterministic tests).
	ProxyURL     string
	Resolver     Resolver
	TrustedHosts map[string]bool
}

func DefaultConfig() ClientConfig {
	return ClientConfig{
		ConnectTimeout: defaultConnectTimeout,
		TotalTimeout:   defaultTotalTimeout,
		MaxRedirects:   defaultMaxRedirects,
		MaxBodyBytes:   defaultMaxBodyBytes,
		UserAgent:      "VideoDL/1.0",
	}
}

type SafeClient struct {
	config ClientConfig
	http   *http.Client
}

func NewSafeClient(cfg ClientConfig) *SafeClient {
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = defaultConnectTimeout
	}
	if cfg.TotalTimeout == 0 {
		cfg.TotalTimeout = defaultTotalTimeout
	}
	if cfg.MaxRedirects == 0 {
		cfg.MaxRedirects = defaultMaxRedirects
	}
	if cfg.MaxBodyBytes == 0 {
		cfg.MaxBodyBytes = defaultMaxBodyBytes
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "VideoDL/1.0"
	}

	if cfg.Resolver == nil {
		cfg.Resolver = net.DefaultResolver
	}

	if cfg.TrustedHosts == nil {
		cfg.TrustedHosts = map[string]bool{}
	}

	dialer := &net.Dialer{
		Timeout:   cfg.ConnectTimeout,
		KeepAlive: 30 * time.Second,
	}

	safe := &SafeClient{config: cfg}

	proxyFunc := proxy.ProxyForRequest
	if rawProxy := proxy.ValidateHTTPProxyURL(cfg.ProxyURL); rawProxy != "" {
		if proxyURL, err := url.Parse(rawProxy); err == nil {
			proxyFunc = http.ProxyURL(proxyURL)
		}
	}

	httpTransport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if proxy.IsConfiguredAddress(addr) || proxy.IsProxyAddress(cfg.ProxyURL, addr) {
				return dialer.DialContext(ctx, network, addr)
			}
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if err := safe.validateAndResolveHost(host); err != nil {
				return nil, err
			}
			return dialer.DialContext(ctx, network, addr)
		},
		Proxy:                 proxyFunc,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   cfg.ConnectTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Timeout:   cfg.TotalTimeout,
		Transport: httpTransport,
	}

	client.CheckRedirect = safe.checkRedirect
	safe.http = client
	return safe
}

func (c *SafeClient) isTrustedHost(host string) bool {
	if c.config.TrustedHosts == nil {
		return false
	}
	return c.config.TrustedHosts[host]
}

func (c *SafeClient) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= c.config.MaxRedirects {
		return ErrTooManyRedirects
	}

	if req.URL.User != nil {
		return ErrURLHasUserInfo
	}

	hostname := req.URL.Hostname()

	if err := c.validateAndResolveHost(hostname); err != nil {
		return fmt.Errorf("%w: %v", ErrRedirectLoopback, err)
	}

	return nil
}

func (c *SafeClient) validateAndResolveHost(host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return ErrInvalidHost
	}

	if c.isTrustedHost(host) {
		return nil
	}

	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return &blacklistError{host: host, ip: ip.String(), kind: string(classifyIP(ip))}
		}
		return nil
	}

	ips, err := c.config.Resolver.LookupIPAddr(context.Background(), host)
	if err != nil {
		return fmt.Errorf("analyzer: DNS resolution failed for %s: %w", host, err)
	}

	if len(ips) == 0 {
		return fmt.Errorf("analyzer: host %s resolved to no addresses", host)
	}

	for _, addr := range ips {
		if !isPublicIP(addr.IP) {
			return &blacklistError{host: host, ip: addr.IP.String(), kind: string(classifyIP(addr.IP))}
		}
	}

	return nil
}

func (c *SafeClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	return c.http.Do(req)
}

func (c *SafeClient) Get(ctx context.Context, rawURL string) (*http.Response, error) {
	u, err := ValidateURL(rawURL)
	if err != nil {
		return nil, err
	}

	if err := c.validateAndResolveHost(u.Hostname()); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.TotalTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	return c.Do(req)
}

func (c *SafeClient) FetchHTML(ctx context.Context, rawURL string) (string, error) {
	resp, err := c.Get(ctx, rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("analyzer: unexpected status %d", resp.StatusCode)
	}

	if !isHTMLResponse(resp) {
		return "", fmt.Errorf("analyzer: response is not HTML (content-type: %s)", resp.Header.Get("Content-Type"))
	}

	limited := &io.LimitedReader{
		R: resp.Body,
		N: c.config.MaxBodyBytes + 1,
	}

	body, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}

	if int64(len(body)) > c.config.MaxBodyBytes {
		return "", ErrBodyTooLarge
	}

	return string(body), nil
}

func isHTMLResponse(resp *http.Response) bool {
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml+xml")
}

func unwrapBlacklistError(err error) *blacklistError {
	var be *blacklistError
	if errors.As(err, &be) {
		return be
	}
	return nil
}
