package analyzer

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"videodl/internal/media"
)

type dashMPD struct {
	XMLName xml.Name     `xml:"MPD"`
	Periods []dashPeriod `xml:"Period"`
}

type dashPeriod struct {
	AdaptationSets     []dashAdaptationSet     `xml:"AdaptationSet"`
	ContentProtections []dashContentProtection `xml:"ContentProtection"`
}

type dashAdaptationSet struct {
	ContentType        string                  `xml:"contentType,attr"`
	MimeType           string                  `xml:"mimeType,attr"`
	Representations    []dashRepresentation    `xml:"Representation"`
	ContentProtections []dashContentProtection `xml:"ContentProtection"`
	BaseURL            string                  `xml:"BaseURL"`
	SegmentList        *dashSegmentList        `xml:"SegmentList"`
	SegmentTemplate    *dashSegmentTemplate    `xml:"SegmentTemplate"`
}

type dashRepresentation struct {
	ID                 string                  `xml:"id,attr"`
	Bandwidth          int64                   `xml:"bandwidth,attr"`
	Width              int                     `xml:"width,attr"`
	Height             int                     `xml:"height,attr"`
	Codecs             string                  `xml:"codecs,attr"`
	ContentType        string                  `xml:"contentType,attr"`
	MimeType           string                  `xml:"mimeType,attr"`
	BaseURL            string                  `xml:"BaseURL"`
	SegmentList        *dashSegmentList        `xml:"SegmentList"`
	SegmentTemplate    *dashSegmentTemplate    `xml:"SegmentTemplate"`
	ContentProtections []dashContentProtection `xml:"ContentProtection"`
}

type dashContentProtection struct {
	SchemeIDURI string `xml:"schemeIdUri,attr"`
	Value       string `xml:"value,attr"`
	Default_KID string `xml:"default_KID,attr"`
}

type dashSegmentList struct {
	SegmentURLs []dashSegmentURL `xml:"SegmentURL"`
	BaseURL     string           `xml:"BaseURL"`
}

type dashSegmentURL struct {
	Media string `xml:"media,attr"`
}

type dashSegmentTemplate struct {
	Media   string `xml:"media,attr"`
	BaseURL string `xml:"BaseURL"`
}

var drmDASHSchemes = map[string]string{
	"urn:mpeg:dash:mp4protection:2011":                       "cenc (Common Encryption)",
	"urn:mpeg:dash:playready:2015":                           "PlayReady",
	"urn:com:microsoft:playready":                            "PlayReady",
	"urn:widevine:dash:mp4protection:2014":                   "Widevine",
	"urn:com:apple:streaming:playback:contentprotection:1.0": "FairPlay",
	"urn:mpeg:dash:clearkey:2015":                            "ClearKey",
}

func parseDASH(body []byte, base *url.URL) (*ManifestResult, error) {
	var mpd dashMPD
	if err := xml.Unmarshal(body, &mpd); err != nil {
		return nil, fmt.Errorf("analyzer: failed to parse DASH XML: %w", err)
	}
	if mpd.XMLName.Local != "MPD" {
		return nil, errors.New("analyzer: root element is not <MPD>")
	}

	res := &ManifestResult{
		SourceType: media.SourceDASH,
	}

	for _, p := range mpd.Periods {
		for _, cp := range p.ContentProtections {
			if reason, ok := classifyDASHDRM(cp.SchemeIDURI, cp.Value); ok {
				res.DRM = true
				if res.DRMReason == "" {
					res.DRMReason = reason
				}
			}
		}
	}

	for _, p := range mpd.Periods {
		for _, a := range p.AdaptationSets {
			for _, cp := range a.ContentProtections {
				if reason, ok := classifyDASHDRM(cp.SchemeIDURI, cp.Value); ok {
					res.DRM = true
					if res.DRMReason == "" {
						res.DRMReason = reason + " (AdaptationSet)"
					}
				}
			}
			contentType := effectiveContentType(a.ContentType, a.MimeType)

			for _, rep := range a.Representations {
				for _, cp := range rep.ContentProtections {
					if reason, ok := classifyDASHDRM(cp.SchemeIDURI, cp.Value); ok {
						res.DRM = true
						if res.DRMReason == "" {
							res.DRMReason = reason + " (Representation)"
						}
					}
				}

				v := media.ManifestVariant{
					ID:         rep.ID,
					Width:      rep.Width,
					Height:     rep.Height,
					Bandwidth:  rep.Bandwidth,
					Codecs:     rep.Codecs,
					SegmentURL: resolveDASHSegmentURL(a, rep, base),
				}

				ct := effectiveContentType(rep.ContentType, rep.MimeType)
				if ct == "" {
					ct = contentType
				}
				v.HasVideo = ct == "video" || rep.Width > 0 || rep.Height > 0
				v.HasAudio = ct == "audio" || effectiveIsAudio(rep.Codecs)

				if v.ID == "" {
					v.ID = fmt.Sprintf("%s-%d", ct, len(res.Variants)+1)
				}
				v.Label = formatDASHLabel(v, ct)
				if v.SegmentURL != "" {
					if ct == "audio" {
						audio := media.ManifestAudioTrack{
							ID:         v.ID,
							Label:      v.Label,
							Bandwidth:  v.Bandwidth,
							HasAudio:   true,
							Codecs:     v.Codecs,
							SegmentURL: v.SegmentURL,
						}
						res.AudioTracks = append(res.AudioTracks, audio)
					} else {
						res.Variants = append(res.Variants, v)
					}
				} else {
					if ct == "audio" {
						a := media.ManifestAudioTrack{
							ID:        v.ID,
							Label:     v.Label,
							Bandwidth: v.Bandwidth,
							HasAudio:  true,
							Codecs:    v.Codecs,
						}
						res.AudioTracks = append(res.AudioTracks, a)
					} else {
						res.Variants = append(res.Variants, v)
					}
				}
			}
		}
	}

	if len(res.Variants) == 0 && len(res.AudioTracks) == 0 && !res.DRM {
		return nil, errors.New("analyzer: DASH manifest contains no usable Representations")
	}

	return res, nil
}

func classifyDASHDRM(scheme, value string) (string, bool) {
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if scheme == "" {
		return "", false
	}
	if reason, ok := drmDASHSchemes[scheme]; ok {
		return "DASH ContentProtection " + reason + " (" + scheme + ")", true
	}
	if strings.Contains(scheme, "protection") || strings.Contains(scheme, "playready") ||
		strings.Contains(scheme, "widevine") || strings.Contains(scheme, "fairplay") {
		return "DASH ContentProtection scheme=" + scheme, true
	}
	return "", false
}

func effectiveContentType(ct, mime string) string {
	if ct = strings.TrimSpace(strings.ToLower(ct)); ct != "" {
		return ct
	}
	mime = strings.ToLower(mime)
	switch {
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	case strings.HasPrefix(mime, "image/"):
		return "image"
	}
	return ""
}

func effectiveIsAudio(codecs string) bool {
	lower := strings.ToLower(codecs)
	return strings.HasPrefix(lower, "mp4a") || strings.HasPrefix(lower, "ac-3") ||
		strings.HasPrefix(lower, "ec-3") || strings.HasPrefix(lower, "opus") ||
		strings.HasPrefix(lower, "mp3") || strings.HasPrefix(lower, "vorbis") ||
		strings.HasPrefix(lower, "aac")
}

func resolveDASHSegmentURL(a dashAdaptationSet, rep dashRepresentation, base *url.URL) string {
	var rel string

	if rep.SegmentList != nil {
		if len(rep.SegmentList.SegmentURLs) > 0 {
			rel = rep.SegmentList.SegmentURLs[0].Media
		}
		if rel == "" && rep.SegmentList.BaseURL != "" {
			rel = rep.SegmentList.BaseURL
		}
	}
	if rel == "" && rep.SegmentTemplate != nil {
		rel = rep.SegmentTemplate.Media
	}
	if rel == "" && rep.BaseURL != "" {
		rel = rep.BaseURL
	}

	if rel == "" && a.SegmentList != nil {
		if len(a.SegmentList.SegmentURLs) > 0 {
			rel = a.SegmentList.SegmentURLs[0].Media
		}
		if rel == "" && a.SegmentList.BaseURL != "" {
			rel = a.SegmentList.BaseURL
		}
	}
	if rel == "" && a.SegmentTemplate != nil {
		rel = a.SegmentTemplate.Media
	}
	if rel == "" && a.BaseURL != "" {
		rel = a.BaseURL
	}

	if rel == "" {
		return ""
	}

	rel = strings.ReplaceAll(rel, "$RepresentationID$", rep.ID)
	rel = strings.ReplaceAll(rel, "$Bandwidth$", strconv.FormatInt(rep.Bandwidth, 10))

	return resolveReference(rel, base)
}

func formatDASHLabel(v media.ManifestVariant, contentType string) string {
	if contentType == "video" && v.Height > 0 {
		return strconv.Itoa(v.Height) + "p"
	}
	if v.Bandwidth > 0 {
		kb := v.Bandwidth / 1000
		return strconv.FormatInt(kb, 10) + "k"
	}
	if v.ID != "" {
		return v.ID
	}
	return "variant"
}
