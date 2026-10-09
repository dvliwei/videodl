package analyzer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// FetchedManifest represents a manifest body that has been pulled over a safe
// HTTP client, along with the request headers that were sent and the final URL
// after redirects. Callers must use these headers when the manifest or its
// segment references require auth context; never construct new headers
// ad-hoc when a fetch has already succeeded.
type FetchedManifest struct {
	Body          []byte
	FinalURL      *url.URL
	RequestHeader http.Header
	FetchedAt     time.Time
}

// FetchManifest downloads the manifest at rawURL using the safe client and
// returns its body along with the headers that were required for the request
// to succeed (Referer, User-Agent, any auth-bearing headers from the incoming
// request). It only fetches documents whose content type matches an HLS or
// DASH indicator; plain HTML or arbitrary binary content is rejected so the
// caller can continue to other candidates without false positives.
func (c *SafeClient) FetchManifest(ctx context.Context, rawURL string) (*FetchedManifest, error) {
	return c.FetchManifestWithHeaders(ctx, rawURL, nil)
}

// FetchManifestWithHeaders is like FetchManifest but merges extra headers into
// the outgoing request. Extra headers are restricted to a known-safer set:
// Referer, Origin, and User-Agent. We strip out Authorization, Cookie, and
// similar credentials-bearing headers because they must not be forwarded
// blindly (this is the "安全传给任务层" constraint of task 9).
func (c *SafeClient) FetchManifestWithHeaders(ctx context.Context, rawURL string, extra http.Header) (*FetchedManifest, error) {
	cleaned := sanitizeForwardableHeaders(extra)

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

	for k, vals := range cleaned {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	if err := c.DoWithHeaders(req); err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("analyzer: manifest fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("analyzer: manifest fetch unexpected status %d", resp.StatusCode)
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if !isManifestContentType(ct) {
		return nil, fmt.Errorf("analyzer: response is not a manifest (content-type: %s)", ct)
	}

	limited := &io.LimitedReader{
		R: resp.Body,
		N: c.config.MaxBodyBytes + 1,
	}
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > c.config.MaxBodyBytes {
		return nil, ErrBodyTooLarge
	}

	finalURL := resp.Request.URL
	if finalURL == nil {
		finalURL = u
	}

	return &FetchedManifest{
		Body:          body,
		FinalURL:      finalURL,
		RequestHeader: req.Header.Clone(),
		FetchedAt:     time.Now(),
	}, nil
}

// DoWithHeaders attaches the standard UA/accept headers but does not fire the
// request; exported so FetchManifestWithHeaders can control the full header
// set before the transport is invoked.
func (c *SafeClient) DoWithHeaders(req *http.Request) error {
	if req == nil {
		return errors.New("analyzer: nil request")
	}
	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	return nil
}

func isManifestContentType(ct string) bool {
	if ct == "" {
		return true
	}
	return strings.Contains(ct, "mpegurl") ||
		strings.Contains(ct, "vnd.apple.mpegurl") ||
		strings.Contains(ct, "mpegurl") ||
		strings.Contains(ct, "application/vnd.apple") ||
		strings.Contains(ct, "application/dash+xml") ||
		strings.Contains(ct, "xml")
}

var forwardableHeaderWhitelist = map[string]bool{
	"referer":         true,
	"origin":          true,
	"user-agent":      true,
	"accept":          true,
	"accept-language": true,
}

func sanitizeForwardableHeaders(h http.Header) http.Header {
	out := http.Header{}
	if h == nil {
		return out
	}
	for k, v := range h {
		if forwardableHeaderWhitelist[strings.ToLower(k)] {
			out[k] = append([]string(nil), v...)
		}
	}
	return out
}
