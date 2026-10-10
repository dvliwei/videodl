package netguard

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
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
}

type Proxy struct {
	validator AddressValidator
	cfg       ProxyConfig
	listener  net.Listener
	server    *http.Server
	transport *http.Transport
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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("netguard: listen on loopback: %w", err)
	}
	p := &Proxy{
		validator: validator,
		cfg:       cfg,
		listener:  listener,
		url:       "http://" + listener.Addr().String(),
		semaphore: make(chan struct{}, cfg.MaxConnections),
	}
	p.transport = &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.MaxConnections,
		MaxIdleConnsPerHost:   cfg.MaxConnections,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   cfg.ConnectTimeout,
		ResponseHeaderTimeout: cfg.TotalTimeout,
		DialContext: func(dialCtx context.Context, network, address string) (net.Conn, error) {
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
	upstream, err := p.dial(ctx, "tcp", net.JoinHostPort(host, port))
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
