package ytdlp

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"videodl/internal/media"
)

var (
	ErrExtractorUnsupported = errors.New("yt-dlp: extractor unsupported")
	ErrAuthRequired         = errors.New("yt-dlp: browser authorization required")
	ErrNoFormats            = errors.New("yt-dlp: no downloadable formats")
	ErrInvalidBrowser       = errors.New("yt-dlp: invalid browser session")
	ErrPlaylistInput        = errors.New("yt-dlp: playlist input is unsupported")
	ErrInvalidInfo          = errors.New("yt-dlp: incomplete media information")
)

var supportedBrowsers = map[string]struct{}{
	"brave": {}, "chrome": {}, "chromium": {}, "edge": {}, "firefox": {},
	"opera": {}, "safari": {}, "vivaldi": {}, "whale": {},
}

// ValidateBrowserSession validates only the explicit browser selector and
// profile name. It never inspects browser storage or accepts a filesystem
// path, cookie file, or arbitrary yt-dlp option.
func ValidateBrowserSession(session BrowserSession) error {
	if _, ok := supportedBrowsers[session.Browser]; !ok {
		return fmt.Errorf("%w: unsupported browser %q", ErrInvalidBrowser, session.Browser)
	}
	if session.Profile == "" {
		return nil
	}
	if len(session.Profile) > 128 || strings.HasPrefix(session.Profile, "-") ||
		strings.Contains(session.Profile, "..") || strings.ContainsAny(session.Profile, "/\\:\x00\r\n") {
		return fmt.Errorf("%w: unsafe profile name", ErrInvalidBrowser)
	}
	return nil
}

// MapInfoToCandidate converts the limited yt-dlp response into the existing
// frontend-safe media model. URLs and request headers are deliberately not
// copied into the candidate; the task layer will re-resolve them later.
func MapInfoToCandidate(info *Info) (media.MediaCandidate, error) {
	if info == nil || strings.EqualFold(info.Type, "playlist") || len(info.Entries) > 0 {
		return media.MediaCandidate{}, ErrPlaylistInput
	}
	if info.ID == "" {
		return media.MediaCandidate{}, fmt.Errorf("%w: missing media ID", ErrInvalidInfo)
	}
	if len(info.Formats) == 0 {
		return media.MediaCandidate{}, ErrNoFormats
	}

	variants := make([]media.MediaVariant, 0, len(info.Formats))
	var first Format
	hasFirst := false
	for index, format := range info.Formats {
		if format.URL == "" || format.FormatID == "" ||
			(!playableCodec(format.VCodec) && !playableCodec(format.ACodec)) {
			continue
		}
		if !hasFirst {
			first = format
			hasFirst = true
		}
		hasVideo := playableCodec(format.VCodec)
		hasAudio := playableCodec(format.ACodec)
		variants = append(variants, media.MediaVariant{
			ID:                     opaqueID("variant", info.ID, format.FormatID, fmt.Sprintf("%d", index)),
			Label:                  formatLabel(format),
			Width:                  format.Width,
			Height:                 format.Height,
			Bandwidth:              int64(format.TBR * 1000),
			HasVideo:               hasVideo,
			HasAudio:               hasAudio,
			AudioDescription:       audioDescription(format),
			InternalFormatSelector: format.FormatID,
		})
	}
	if !hasFirst || len(variants) == 0 {
		return media.MediaCandidate{}, ErrNoFormats
	}
	selector := defaultFormatSelector(info.Formats)
	if len(info.RequestedFormats) > 0 {
		selector = formatIDs(info.RequestedFormats)
	}

	pageURL := info.WebpageURL
	if pageURL == "" {
		pageURL = info.OriginalURL
	}
	candidate := media.MediaCandidate{
		ID:                  opaqueID("candidate", info.Extractor, info.ID),
		Title:               info.Title,
		DisplayURL:          sanitizeDisplayURL(pageURL),
		SourceType:          media.SourceYTDLP,
		Format:              first.Ext,
		Width:               first.Width,
		Height:              first.Height,
		DurationSeconds:     info.Duration,
		HasVideo:            false,
		HasAudio:            false,
		Variants:            variants,
		InternalYTDLPSource: &media.YTDLPSource{PageURL: pageURL, FormatSelector: selector},
	}
	for _, variant := range variants {
		candidate.HasVideo = candidate.HasVideo || variant.HasVideo
		candidate.HasAudio = candidate.HasAudio || variant.HasAudio
	}
	if len(info.Formats) == 1 {
		candidate.SizeBytes = firstSize(first)
	}
	return candidate, nil
}

func defaultFormatSelector(formats []Format) string {
	combined := ""
	video := ""
	audio := ""
	first := ""
	for _, format := range formats {
		if format.FormatID == "" {
			continue
		}
		if first == "" {
			first = format.FormatID
		}
		if playableCodec(format.VCodec) && playableCodec(format.ACodec) && combined == "" {
			combined = format.FormatID
		}
		if playableCodec(format.VCodec) && video == "" {
			video = format.FormatID
		}
		if playableCodec(format.ACodec) && audio == "" {
			audio = format.FormatID
		}
	}
	if combined != "" {
		return combined
	}
	if video != "" && audio != "" {
		return video + "+" + audio
	}
	if video != "" {
		return video
	}
	if audio != "" {
		return audio
	}
	return first
}

func formatIDs(formats []Format) string {
	ids := make([]string, 0, len(formats))
	for _, format := range formats {
		if format.FormatID != "" {
			ids = append(ids, format.FormatID)
		}
	}
	return strings.Join(ids, "+")
}

func playableCodec(codec string) bool {
	switch strings.ToLower(strings.TrimSpace(codec)) {
	case "", "none", "unknown", "unavailable", "null":
		return false
	default:
		return true
	}
}

func firstSize(format Format) *int64 {
	if format.Filesize != nil {
		value := *format.Filesize
		return &value
	}
	if format.FilesizeApprox != nil {
		value := *format.FilesizeApprox
		return &value
	}
	return nil
}

func formatLabel(format Format) string {
	parts := make([]string, 0, 3)
	if format.Height > 0 {
		parts = append(parts, fmt.Sprintf("%dp", format.Height))
	} else if format.Ext != "" {
		parts = append(parts, strings.ToUpper(format.Ext))
	}
	if playableCodec(format.VCodec) && playableCodec(format.ACodec) {
		parts = append(parts, "video+audio")
	} else if playableCodec(format.VCodec) {
		parts = append(parts, "video")
	} else if playableCodec(format.ACodec) {
		parts = append(parts, "audio")
	}
	if len(parts) == 0 {
		return format.FormatID
	}
	return strings.Join(parts, " • ")
}

func audioDescription(format Format) string {
	if !playableCodec(format.ACodec) {
		return ""
	}
	if format.AudioChannels > 0 {
		return fmt.Sprintf("%s, %dch", format.ACodec, format.AudioChannels)
	}
	return format.ACodec
}

func opaqueID(kind string, parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte(part))
		_, _ = hash.Write([]byte{0})
	}
	return kind + "_" + hex.EncodeToString(hash.Sum(nil))[:20]
}

func sanitizeDisplayURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
