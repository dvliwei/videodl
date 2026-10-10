package netguard

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type AddressValidator interface {
	ValidateHost(ctx context.Context, host string) error
}

type ProxyConfig struct {
	MaxConnections int
	ConnectTimeout time.Duration
	TotalTimeout   time.Duration
	// UpstreamProxyURL optionally routes outbound traffic through an HTTP
	// proxy. The target host is still validated before this proxy is used.
	UpstreamProxyURL string
}

type Proxy struct {
	validator AddressValidator
	cfg       ProxyConfig
	listener  net.Listener
	server    *http.Server
	transport *http.Transport
	upstream  *url.URL
	semaphore chan struct{}
	url       string
	closeOnce sync.Once
}

func NewProxy(ctx context.Context, validator AddressValidator, cfg ProxyConfig) (*Proxy, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if validator == nil {
		validator = NewResolver(nil)
	}
	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = 4
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 10 * time.Second
	}
	if cfg.TotalTimeout <= 0 {
		cfg.TotalTimeout = 30 * time.Second
	}
	upstream, err := parseUpstreamProxyURL(cfg.UpstreamProxyURL)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("netguard: listen on loopback: %w", err)
	}
	p := &Proxy{
		validator: validator,
		cfg:       cfg,
		upstream:  upstream,
		listener:  listener,
		url:       "http://" + listener.Addr().String(),
		semaphore: make(chan struct{}, cfg.MaxConnections),
	}
	p.transport = &http.Transport{
		Proxy:                 http.ProxyURL(upstream),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.MaxConnections,
		MaxIdleConnsPerHost:   cfg.MaxConnections,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   cfg.ConnectTimeout,
		ResponseHeaderTimeout: cfg.TotalTimeout,
		DialContext: func(dialCtx context.Context, network, address string) (net.Conn, error) {
			if p.isUpstreamAddress(address) {
				return p.dialUpstream(dialCtx, network, address)
			}
			return p.dial(dialCtx, network, address)
		},
	}
	p.server = &http.Server{
		Handler:           p,
		ReadHeaderTimeout: cfg.ConnectTimeout,
	}
	go func() {
		_ = p.server.Serve(listener)
	}()
	if done := ctx.Done(); done != nil {
		go func() {
			<-done
			_ = p.Close()
		}()
	}
	return p, nil
}

func (p *Proxy) URL() string {
	if p == nil {
		return ""
	}
	return p.url
}

func (p *Proxy) Close() error {
	if p == nil {
		return nil
	}
	var err error
	p.closeOnce.Do(func() {
		err = p.server.Close()
		_ = p.listener.Close()
	})
	return err
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if !p.acquire(req.Context()) {
		http.Error(w, "proxy connection limit reached", http.StatusServiceUnavailable)
		return
	}
	defer p.release()

	if req.Method == http.MethodConnect {
		p.handleConnect(w, req)
		return
	}
	if req.URL == nil || req.URL.Hostname() == "" || req.URL.User != nil {
		http.Error(w, "invalid proxy request", http.StatusBadRequest)
		return
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		http.Error(w, "unsupported proxy scheme", http.StatusBadRequest)
		return
	}
	if err := p.validator.ValidateHost(req.Context(), req.URL.Hostname()); err != nil {
		http.Error(w, "upstream address blocked", http.StatusForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), p.cfg.TotalTimeout)
	defer cancel()
	outReq := req.Clone(ctx)
	outReq.RequestURI = ""
	removeHopByHopHeaders(outReq.Header)
	resp, err := p.transport.RoundTrip(outReq)
	if err != nil {
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	removeHopByHopHeaders(resp.Header)
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (p *Proxy) handleConnect(w http.ResponseWriter, req *http.Request) {
	target := req.Host
	if target == "" {
		target = req.URL.Host
	}
	host, port, err := net.SplitHostPort(target)
	if err != nil || host == "" || port == "" {
		http.Error(w, "invalid CONNECT target", http.StatusBadRequest)
		return
	}
	if err := p.validator.ValidateHost(req.Context(), host); err != nil {
		http.Error(w, "upstream address blocked", http.StatusForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), p.cfg.ConnectTimeout)
	defer cancel()
	target = net.JoinHostPort(host, port)
	var upstream net.Conn
	if p.upstream != nil {
		upstream, err = p.dialUpstream(ctx, "tcp", p.upstream.Host)
		if err == nil {
			upstream, err = p.establishUpstreamConnect(upstream, target)
		}
	} else {
		upstream, err = p.dial(ctx, "tcp", target)
	}
	if err != nil {
		http.Error(w, "upstream connect failed", http.StatusBadGateway)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = upstream.Close()
		http.Error(w, "CONNECT unsupported", http.StatusNotImplemented)
		return
	}
	client, _, err := hijacker.Hijack()
	if err != nil {
		_ = upstream.Close()
		return
	}
	_, _ = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	go func() {
		_, _ = io.Copy(upstream, client)
		_ = upstream.Close()
	}()
	_, _ = io.Copy(client, upstream)
	_ = client.Close()
}

func (p *Proxy) establishUpstreamConnect(conn net.Conn, target string) (net.Conn, error) {
	reader := bufio.NewReader(conn)
	if _, err := fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Connection: Keep-Alive\r\n\r\n", target, target); err != nil {
		_ = conn.Close()
		return nil, err
	}
	response, err := http.ReadResponse(reader, nil)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_ = conn.Close()
		return nil, fmt.Errorf("upstream proxy CONNECT returned %s", response.Status)
	}
	// ReadResponse may have buffered bytes that belong to the tunneled
	// connection. Keep those bytes visible after the HTTP handshake.
	return &bufferedConn{Conn: conn, reader: reader}, nil
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.reader.Read(p) }

func (p *Proxy) dial(ctx context.Context, network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if err := p.validator.ValidateHost(ctx, host); err != nil {
		return nil, err
	}
	return (&net.Dialer{Timeout: p.cfg.ConnectTimeout, KeepAlive: 30 * time.Second}).DialContext(ctx, network, address)
}

func (p *Proxy) dialUpstream(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{Timeout: p.cfg.ConnectTimeout, KeepAlive: 30 * time.Second}).DialContext(ctx, network, address)
}

func (p *Proxy) isUpstreamAddress(address string) bool {
	return p.upstream != nil && address == p.upstream.Host
}

func parseUpstreamProxyURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, "http") || u.Hostname() == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("netguard: upstream proxy must be an HTTP URL without credentials or a non-root path")
	}
	u.Scheme = "http"
	if u.Port() == "" {
		u.Host = net.JoinHostPort(u.Hostname(), "80")
	}
	return u, nil
}

func (p *Proxy) acquire(ctx context.Context) bool {
	select {
	case p.semaphore <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (p *Proxy) release() { <-p.semaphore }

func removeHopByHopHeaders(header http.Header) {
	for _, key := range []string{
		"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
		"TE", "Trailer", "Transfer-Encoding", "Upgrade", "Proxy-Connection",
	} {
		header.Del(key)
	}
	if connection := header.Values("Connection"); len(connection) > 0 {
		for _, value := range connection {
			for _, key := range strings.Split(value, ",") {
				header.Del(strings.TrimSpace(key))
			}
		}
	}
}

var _ AddressValidator = (*Resolver)(nil)
var _ http.Handler = (*Proxy)(nil)
