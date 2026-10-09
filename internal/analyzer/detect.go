package analyzer

import (
	"strings"

	"videodl/internal/media"
)

// DetectManifestType inspects the first bytes of a manifest and returns which
// kind of manifest it is, or an empty SourceType if the body does not look
// like a recognized manifest. This is a heuristic and intentionally cheap so
// it can be called from any code path.
func DetectManifestType(body string) media.SourceType {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, "#") {
		if strings.HasPrefix(trimmed, "#EXTM3U") || strings.Contains(trimmed, "\n#EXTM3U") {
			return media.SourceHLS
		}
	}

	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "<?xml") || strings.HasPrefix(lower, "<mpd") {
		if strings.Contains(lower, "<mpd") && strings.Contains(lower, "urn:mpeg:dash") {
			return media.SourceDASH
		}
		if strings.Contains(lower, "<mpd") {
			return media.SourceDASH
		}
	}

	if strings.Contains(lower, "<mpd") && strings.Contains(lower, "<representation") {
		return media.SourceDASH
	}

	if strings.Contains(trimmed, "#EXT-X-") || strings.Contains(trimmed, "#EXTINF") {
		return media.SourceHLS
	}

	return ""
}
