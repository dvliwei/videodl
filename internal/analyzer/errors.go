package analyzer

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidScheme      = errors.New("analyzer: only http and https schemes are allowed")
	ErrURLHasUserInfo     = errors.New("analyzer: URLs with user info are not allowed")
	ErrInvalidHost        = errors.New("analyzer: URL has no valid host")
	ErrBlacklistedAddress = errors.New("analyzer: target address is not a public IP")
	ErrTooManyRedirects   = errors.New("analyzer: exceeded maximum redirect count")
	ErrBodyTooLarge       = errors.New("analyzer: response body exceeds the maximum allowed size")
	ErrRedirectLoopback   = errors.New("analyzer: redirect target resolves to a non-public address")
)

type schemeError struct {
	scheme string
}

func (e *schemeError) Error() string {
	return fmt.Sprintf("analyzer: unsupported scheme %q", e.scheme)
}

func (e *schemeError) Unwrap() error { return ErrInvalidScheme }

type blacklistError struct {
	host string
	ip   string
	kind string
}

func (e *blacklistError) Error() string {
	if e.ip != "" {
		return fmt.Sprintf("analyzer: host %s resolves to blacklisted address %s (%s)", e.host, e.ip, e.kind)
	}
	return fmt.Sprintf("analyzer: host %s is blacklisted (%s)", e.host, e.kind)
}

func (e *blacklistError) Unwrap() error { return ErrBlacklistedAddress }
