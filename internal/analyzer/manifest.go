package analyzer

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"videodl/internal/media"
)

// ManifestResult is the structured outcome of parsing an HLS or DASH manifest.
// When DRM is detected, DRM is true and DRMReason explains what triggered the
// flag so the UI can present an actionable reason instead of a generic failure.
type ManifestResult struct {
	SourceType  media.SourceType
	Variants    []media.ManifestVariant
	AudioTracks []media.ManifestAudioTrack
	DRM         bool
	DRMReason   string
}

// ManifestVariant is a single selectable rendition from a manifest. It embeds
// MediaVariant fields so callers can convert directly to MediaVariant for the
// AnalysisResult without an adapter, while keeping the original segment URL
// and codecs visible to the analyzer layer.
func ParseManifest(body []byte, base *url.URL) (*ManifestResult, error) {
	if base == nil {
		return nil, errors.New("analyzer: manifest base URL is nil")
	}
	if len(body) == 0 {
		return nil, errors.New("analyzer: empty manifest body")
	}

	srcType := DetectManifestType(string(body))
	switch srcType {
	case media.SourceHLS:
		return parseHLS(body, base)
	case media.SourceDASH:
		return parseDASH(body, base)
	default:
		return nil, fmt.Errorf("analyzer: unrecognised manifest body")
	}
}

// resolveReference resolves a relative or absolute manifest reference against
// the given base. It mirrors URL resolve semantics but also rejects the
// javascript: and data: schemes we refuse to fetch downstream.
func resolveReference(ref string, base *url.URL) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	lower := strings.ToLower(ref)
	if strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "javascript:") {
		return ""
	}
	if ref == "." || ref == ".." {
		return ""
	}

	var absolute *url.URL
	var err error
	if base != nil {
		absolute, err = base.Parse(ref)
	} else {
		absolute, err = url.Parse(ref)
	}
	if err != nil || absolute == nil {
		return ""
	}
	if absolute.Scheme != "http" && absolute.Scheme != "https" {
		return ""
	}
	return absolute.String()
}
