package analyzer

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"videodl/internal/media"
)

var (
	extKeyRE           = regexp.MustCompile(`EXT-X-KEY:METHOD=([A-Za-z0-9-]+)`)
	widevineRE         = regexp.MustCompile(`EXT-X-KEY.*?KEYFORMAT="([^"]+)"`)
	widevineDRMFormats = map[string]bool{
		"com.widevine":            true,
		"com.widevine.alpha":      true,
		"com.microsoft.playready": true,
		"com.apple.fairplay":      true,
		"org.w3.clearkey":         true,
	}
)

func parseHLS(body []byte, base *url.URL) (*ManifestResult, error) {
	lines := strings.Split(string(body), "\n")

	res := &ManifestResult{
		SourceType: media.SourceHLS,
	}

	var mediaGroups map[string][]hlsMediaGroup
	var streamInfos []hlsStreamInfo
	isSingleVariant := true
	var firstSegment string

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#EXTINF") {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "#EXT-X-MEDIA") {
				if mediaGroups == nil {
					mediaGroups = map[string][]hlsMediaGroup{}
				}
				g := parseHLSMediaGroup(line)
				if g.GroupID != "" && g.Type != "" {
					mediaGroups[g.GroupID] = append(mediaGroups[g.GroupID], g)
				}
			}
			if strings.HasPrefix(line, "#EXT-X-STREAM-INF") {
				info := parseHLSStreamInfo(line)
				if i+1 < len(lines) {
					info.URI = strings.TrimSpace(lines[i+1])
					i++
				}
				streamInfos = append(streamInfos, info)
				isSingleVariant = false
			}
			if strings.HasPrefix(line, "#EXT-X-KEY") {
				res.DRM = true
				m := extKeyRE.FindStringSubmatch(line)
				if len(m) >= 2 {
					method := m[1]
					if method == "NONE" {
						res.DRM = false
					} else {
						res.DRMReason = "HLS EXT-X-KEY METHOD=" + method
						if fm := widevineRE.FindStringSubmatch(line); len(fm) >= 2 {
							res.DRMReason += ", KEYFORMAT=" + fm[1]
						}
					}
				}
			}
			continue
		}

		if !strings.HasPrefix(line, "#") && isSingleVariant && !res.DRM {
			if firstSegment == "" {
				firstSegment = strings.TrimSpace(line)
			}
		}
	}

	if res.DRM {
		return res, nil
	}

	if isSingleVariant {
		segURL := resolveReference(firstSegment, base)
		if segURL != "" {
			v := media.ManifestVariant{
				Label:      "default",
				HasVideo:   true,
				HasAudio:   true,
				SegmentURL: segURL,
			}
			res.Variants = append(res.Variants, v)
		}
	}

	for _, info := range streamInfos {
		segmentURL := resolveReference(info.URI, base)
		if segmentURL == "" {
			continue
		}
		v := media.ManifestVariant{
			Label:      formatHLSLabel(info),
			Width:      info.Width,
			Height:     info.Height,
			Bandwidth:  info.Bandwidth,
			HasVideo:   info.Video,
			HasAudio:   info.Audio,
			Codecs:     info.Codecs,
			SegmentURL: segmentURL,
		}
		if v.Label == "" {
			v.Label = fmt.Sprintf("variant-%d", len(res.Variants)+1)
		}
		res.Variants = append(res.Variants, v)
	}

	if mediaGroups != nil {
		for _, groups := range mediaGroups {
			for _, g := range groups {
				if g.Type != "AUDIO" {
					continue
				}
				segmentURL := resolveReference(g.URI, base)
				if segmentURL == "" && g.URI != "" {
					continue
				}
				a := media.ManifestAudioTrack{
					Label:      g.Name,
					Language:   g.Language,
					HasAudio:   true,
					Codecs:     g.Codecs,
					SegmentURL: segmentURL,
				}
				if a.Label == "" {
					a.Label = fmt.Sprintf("audio-%s-%d", g.Language, len(res.AudioTracks)+1)
				}
				res.AudioTracks = append(res.AudioTracks, a)
			}
		}
	}

	if len(res.Variants) == 0 && !res.DRM {
		return nil, errors.New("analyzer: HLS manifest contains no usable variants")
	}

	return res, nil
}

type hlsStreamInfo struct {
	Bandwidth int64
	Width     int
	Height    int
	Codecs    string
	Audio     bool
	Video     bool
	URI       string
}

type hlsMediaGroup struct {
	Type       string
	GroupID    string
	Name       string
	URI        string
	Language   string
	Codecs     string
	Default    bool
	Autoselect bool
}

func parseHLSStreamInfo(line string) hlsStreamInfo {
	info := hlsStreamInfo{Video: true, Audio: true}
	attrs := splitHLSAttrs(line)

	if v, ok := attrs["BANDWIDTH"]; ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			info.Bandwidth = n
		}
	}
	if res, ok := attrs["RESOLUTION"]; ok {
		parts := strings.SplitN(res, "x", 2)
		if len(parts) == 2 {
			if w, err := strconv.Atoi(parts[0]); err == nil {
				info.Width = w
			}
			if h, err := strconv.Atoi(parts[1]); err == nil {
				info.Height = h
			}
		}
	}
	if c, ok := attrs["CODECS"]; ok {
		info.Codecs = stripQuotes(c)
		lower := strings.ToLower(info.Codecs)
		if isAudioOnlyCodec(lower) {
			info.Video = false
			info.Audio = true
		} else if isVideoCodec(lower) {
			info.Video = true
			info.Audio = strings.Contains(lower, "mp4a") || strings.Contains(lower, "ac-3") ||
				strings.Contains(lower, "ec-3") || strings.Contains(lower, "opus") ||
				strings.Contains(lower, "mp3")
		}
	} else {
		if _, hasA := attrs["AUDIO"]; hasA {
			info.Audio = true
		}
	}
	return info
}

func parseHLSMediaGroup(line string) hlsMediaGroup {
	g := hlsMediaGroup{}
	attrs := splitHLSAttrs(line)

	for k, v := range attrs {
		switch strings.ToUpper(k) {
		case "TYPE":
			g.Type = strings.ToUpper(v)
		case "GROUP-ID":
			g.GroupID = stripQuotes(v)
		case "NAME":
			g.Name = stripQuotes(v)
		case "URI":
			g.URI = stripQuotes(v)
		case "LANGUAGE":
			g.Language = stripQuotes(v)
		case "CODECS":
			g.Codecs = stripQuotes(v)
		case "DEFAULT":
			g.Default = strings.ToUpper(v) == "YES"
		case "AUTOSELECT":
			g.Autoselect = strings.ToUpper(v) == "YES"
		}
	}
	return g
}

func splitHLSAttrs(line string) map[string]string {
	m := map[string]string{}
	rest := line
	if idx := strings.Index(rest, ":"); idx >= 0 {
		rest = rest[idx+1:]
	}
	var cur strings.Builder
	inQuote := false
	key := ""
	for i := 0; i < len(rest); i++ {
		ch := rest[i]
		switch {
		case ch == '"':
			inQuote = !inQuote
			cur.WriteByte(ch)
		case ch == ',' && !inQuote:
			tok := cur.String()
			if eq := strings.Index(tok, "="); eq >= 0 {
				key = tok[:eq]
				val := tok[eq+1:]
				m[strings.ToUpper(strings.TrimSpace(key))] = strings.TrimSpace(val)
			}
			cur.Reset()
		default:
			cur.WriteByte(ch)
		}
	}
	if cur.Len() > 0 {
		tok := cur.String()
		if eq := strings.Index(tok, "="); eq >= 0 {
			m[strings.ToUpper(strings.TrimSpace(tok[:eq]))] = strings.TrimSpace(tok[eq+1:])
		}
	}
	return m
}

func stripQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func formatHLSLabel(info hlsStreamInfo) string {
	if info.Width > 0 && info.Height > 0 {
		return strconv.Itoa(info.Height) + "p"
	}
	if info.Bandwidth > 0 {
		kb := info.Bandwidth / 1000
		return strconv.FormatInt(kb, 10) + "k"
	}
	return ""
}

func isVideoCodec(lower string) bool {
	tokens := strings.Split(lower, ",")
	for _, t := range tokens {
		t = strings.TrimSpace(t)
		if strings.HasPrefix(t, "avc") || strings.HasPrefix(t, "hev") ||
			strings.HasPrefix(t, "hvc") || strings.HasPrefix(t, "av1") ||
			strings.HasPrefix(t, "vp8") || strings.HasPrefix(t, "vp9") {
			return true
		}
	}
	return false
}

func isAudioOnlyCodec(lower string) bool {
	if lower == "" {
		return false
	}
	audioPrefixes := []string{"mp4a", "ac-3", "ec-3", "opus", "mp3", "aac", "vorbis"}
	for _, p := range audioPrefixes {
		if strings.HasPrefix(lower, p) {
			return !isVideoCodec(lower)
		}
	}
	return false
}
