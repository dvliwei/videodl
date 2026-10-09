package analyzer

import (
	"fmt"
	"net/url"
	"strings"
)

var allowedSchemes = map[string]bool{
	"http":  true,
	"https": true,
}

func ValidateURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("analyzer: cannot parse URL: %w", err)
	}

	scheme := strings.ToLower(u.Scheme)
	if !allowedSchemes[scheme] {
		return nil, &schemeError{scheme: u.Scheme}
	}

	if u.User != nil {
		return nil, ErrURLHasUserInfo
	}

	host := u.Hostname()
	if host == "" {
		return nil, ErrInvalidHost
	}

	return u, nil
}

func SanitizeDisplayURL(u *url.URL) string {
	clone := *u
	if clone.RawQuery != "" {
		clone.RawQuery = ""
	}
	if clone.Fragment != "" {
		clone.Fragment = ""
	}
	return clone.String()
}
