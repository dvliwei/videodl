package analyzer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"videodl/internal/ffmpeg"
	"videodl/internal/media"
)

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

const (
	defaultServiceProbeTimeoutSeconds = 15
	defaultMaxProbeConcurrency        = 4

	htmlClassifyTag = "html"

	unknownProbeBytes = 32 * 1024
)

type ServiceOptions struct {
	FetchMediaInfo bool
	ProbeTimeout   int
	MaxConcurrency int
}

func (o ServiceOptions) withDefaults() ServiceOptions {
	if o.ProbeTimeout <= 0 {
		o.ProbeTimeout = defaultServiceProbeTimeoutSeconds
	}
	if o.MaxConcurrency <= 0 {
		o.MaxConcurrency = defaultMaxProbeConcurrency
	}
	return o
}

type AnalysisService struct {
	client  *SafeClient
	invoker *ffmpeg.Invoker
	opts    ServiceOptions
}

func NewAnalysisService(client *SafeClient, invoker *ffmpeg.Invoker) *AnalysisService {
	return &AnalysisService{
		client:  client,
		invoker: invoker,
		opts: ServiceOptions{
			FetchMediaInfo: invoker != nil,
		},
	}
}

func (s *AnalysisService) WithOptions(opts ServiceOptions) *AnalysisService {
	s.opts = opts.withDefaults()
	return s
}

func (s *AnalysisService) SetInvoker(invoker *ffmpeg.Invoker) {
	s.invoker = invoker
	s.opts.FetchMediaInfo = invoker != nil
}

func (s *AnalysisService) Invoker() *ffmpeg.Invoker {
	return s.invoker
}

func (s *AnalysisService) Analyze(ctx context.Context, rawURL string) (*media.AnalysisResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	u, err := ValidateURL(rawURL)
	if err != nil {
		return nil, err
	}

	if err := s.client.validateAndResolveHost(u.Hostname()); err != nil {
		return nil, err
	}

	result := &media.AnalysisResult{
		ID:         newAnalysisID(),
		Candidates: []media.MediaCandidate{},
	}

	body, finalURL, contentType, srcType, err := s.fetchAndClassify(ctx, u)
	if err != nil {
		return nil, err
	}

	switch srcType {
	case string(media.SourceHLS), string(media.SourceDASH):
		return s.analyzeManifest(ctx, body, finalURL, result)
	case htmlClassifyTag:
		return s.analyzeHTML(ctx, finalURL, body, result)
	case string(media.SourceDirect):
		return s.analyzeDirectMedia(ctx, finalURL, contentType, result)
	default:
		if len(body) > 0 {
			if manifestType := DetectManifestType(string(body)); manifestType != "" {
				return s.analyzeManifest(ctx, body, finalURL, result)
			}
		}
		result.Warnings = append(result.Warnings, "no media resources found at the given URL")
		return result, nil
	}
}

func (s *AnalysisService) fetchAndClassify(ctx context.Context, u *url.URL) (body []byte, finalURL *url.URL, contentType string, srcType string, err error) {
	reqCtx, cancel := context.WithTimeout(ctx, s.client.config.TotalTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, "", "", err
	}

	if err := s.client.DoWithHeaders(req); err != nil {
		return nil, nil, "", "", err
	}

	resp, err := s.client.http.Do(req)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("analyzer: fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, "", "", fmt.Errorf("analyzer: unexpected status %d", resp.StatusCode)
	}

	finalURL = resp.Request.URL
	if finalURL == nil {
		finalURL = u
	}

	contentType = strings.ToLower(resp.Header.Get("Content-Type"))

	pre := preclassifyResponse(contentType, finalURL)

	switch pre {
	case string(media.SourceDirect):
		return nil, finalURL, contentType, string(media.SourceDirect), nil

	case string(media.SourceHLS), string(media.SourceDASH), htmlClassifyTag:
		limited := &io.LimitedReader{
			R: resp.Body,
			N: s.client.config.MaxBodyBytes + 1,
		}
		body, err = io.ReadAll(limited)
		if err != nil {
			return nil, nil, "", "", err
		}
		if int64(len(body)) > s.client.config.MaxBodyBytes {
			return nil, nil, "", "", ErrBodyTooLarge
		}
		return body, finalURL, contentType, pre, nil

	default:
		limited := &io.LimitedReader{
			R: resp.Body,
			N: unknownProbeBytes + 1,
		}
		body, err = io.ReadAll(limited)
		if err != nil {
			return nil, nil, "", "", err
		}
		detected := classifyContent(contentType, body, finalURL)

		switch detected {
		case string(media.SourceHLS), string(media.SourceDASH):
			return body, finalURL, contentType, detected, nil
		case htmlClassifyTag:
			return body, finalURL, contentType, htmlClassifyTag, nil
		case "":
			return nil, finalURL, contentType, "", nil
		default:
			if int64(len(body)) > s.client.config.MaxBodyBytes {
				return nil, nil, "", "", ErrBodyTooLarge
			}
			return body, finalURL, contentType, detected, nil
		}
	}
}

func preclassifyResponse(contentType string, u *url.URL) string {
	ext := strings.ToLower(path.Ext(u.Path))

	switch {
	case ext == ".m3u8" || strings.Contains(contentType, "mpegurl") || strings.Contains(contentType, "vnd.apple.mpegurl"):
		return string(media.SourceHLS)
	case ext == ".mpd" || strings.Contains(contentType, "application/dash+xml"):
		return string(media.SourceDASH)
	case strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml+xml"):
		return htmlClassifyTag
	case isMediaExtension(ext) || isMediaContentType(contentType):
		return string(media.SourceDirect)
	}
	return ""
}

func classifyContent(contentType string, body []byte, u *url.URL) string {
	ext := strings.ToLower(path.Ext(u.Path))

	switch {
	case ext == ".m3u8" || strings.Contains(contentType, "mpegurl") || strings.Contains(contentType, "vnd.apple.mpegurl"):
		if len(body) > 0 {
			if mt := DetectManifestType(string(body)); mt == media.SourceHLS {
				return string(media.SourceHLS)
			}
		}
		return string(media.SourceHLS)

	case ext == ".mpd" || strings.Contains(contentType, "application/dash+xml"):
		if len(body) > 0 {
			if mt := DetectManifestType(string(body)); mt == media.SourceDASH {
				return string(media.SourceDASH)
			}
		}
		return string(media.SourceDASH)

	case strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml+xml"):
		return htmlClassifyTag

	case isMediaExtension(ext) || isMediaContentType(contentType):
		return string(media.SourceDirect)

	default:
		if len(body) > 0 {
			if mt := DetectManifestType(string(body)); mt != "" {
				return string(mt)
			}
		}
		return ""
	}
}

func (s *AnalysisService) analyzeHTML(ctx context.Context, base *url.URL, body []byte, result *media.AnalysisResult) (*media.AnalysisResult, error) {
	info := ExtractMediaFromHTML(string(body), base)
	result.PageTitle = info.PageTitle

	if len(info.Candidates) == 0 {
		result.Warnings = append(result.Warnings, "no media candidates found in HTML")
		return result, nil
	}

	seen := map[string]bool{}
	var all []*media.MediaCandidate

	for _, ref := range info.Candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if seen[ref.DisplayURL] {
			continue
		}
		seen[ref.DisplayURL] = true

		candidate, err := s.buildCandidateFromRef(ctx, ref, base)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to analyze %s: %v", ref.DisplayURL, err))
			continue
		}
		if candidate != nil {
			all = append(all, candidate)
		}
	}

	for _, c := range all {
		result.Candidates = append(result.Candidates, *c)
	}

	return result, nil
}

func (s *AnalysisService) buildCandidateFromRef(ctx context.Context, ref MediaRef, base *url.URL) (*media.MediaCandidate, error) {
	u, err := url.Parse(ref.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	ext := strings.ToLower(path.Ext(u.Path))

	if ext == ".m3u8" {
		manifest, err := s.client.FetchManifest(ctx, ref.URL)
		if err != nil {
			return &media.MediaCandidate{
				ID:                newCandidateID(),
				Title:             ref.Title,
				DisplayURL:        ref.DisplayURL,
				SourceType:        media.SourceHLS,
				Unsupported:       fmt.Sprintf("manifest fetch failed: %v", err),
				InternalSourceURL: ref.URL,
			}, nil
		}
		return s.buildCandidateFromManifest(ctx, manifest.Body, manifest.FinalURL, media.SourceHLS, ref)
	}

	if ext == ".mpd" {
		manifest, err := s.client.FetchManifest(ctx, ref.URL)
		if err != nil {
			return &media.MediaCandidate{
				ID:                newCandidateID(),
				Title:             ref.Title,
				DisplayURL:        ref.DisplayURL,
				SourceType:        media.SourceDASH,
				Unsupported:       fmt.Sprintf("manifest fetch failed: %v", err),
				InternalSourceURL: ref.URL,
			}, nil
		}
		return s.buildCandidateFromManifest(ctx, manifest.Body, manifest.FinalURL, media.SourceDASH, ref)
	}

	if isMediaExtension(ext) {
		c := &media.MediaCandidate{
			ID:                newCandidateID(),
			Title:             firstNonEmpty(ref.Title, deriveTitle(u.Path)),
			DisplayURL:        ref.DisplayURL,
			SourceType:        media.SourceDirect,
			Format:            strings.TrimPrefix(ext, "."),
			InternalSourceURL: ref.URL,
		}
		applyMediaDefaults(c, ext, "")
		s.probeCandidate(ctx, c, ref.URL)
		return c, nil
	}

	return nil, fmt.Errorf("unsupported media reference: %s", ref.DisplayURL)
}

func (s *AnalysisService) analyzeManifest(ctx context.Context, body []byte, base *url.URL, result *media.AnalysisResult) (*media.AnalysisResult, error) {
	parsed, err := ParseManifest(body, base)
	if err != nil {
		return nil, fmt.Errorf("analyzer: manifest parse failed: %w", err)
	}

	if parsed.DRM {
		result.Candidates = append(result.Candidates, media.MediaCandidate{
			ID:          newCandidateID(),
			DisplayURL:  SanitizeDisplayURL(base),
			SourceType:  parsed.SourceType,
			Unsupported: "DRM protected: " + parsed.DRMReason,
		})
		return result, nil
	}

	c := &media.MediaCandidate{
		ID:                newCandidateID(),
		Title:             deriveTitle(base.Path),
		DisplayURL:        SanitizeDisplayURL(base),
		SourceType:        parsed.SourceType,
		InternalSourceURL: base.String(),
	}

	variantManifests := map[string]string{}
	for i, v := range parsed.Variants {
		mv := media.MediaVariant{
			ID:        fmt.Sprintf("%s-v%d", c.ID, i),
			Label:     v.Label,
			Width:     v.Width,
			Height:    v.Height,
			Bandwidth: v.Bandwidth,
			HasVideo:  v.HasVideo,
			HasAudio:  v.HasAudio,
		}
		c.Variants = append(c.Variants, mv)
		addr := v.SubManifestURL
		if addr == "" {
			addr = v.SegmentURL
		}
		if addr != "" {
			variantManifests[mv.ID] = addr
		}
	}
	if len(variantManifests) > 0 {
		c.InternalVariantManifests = variantManifests
	}

	c.HasVideo = len(parsed.Variants) > 0 && parsed.Variants[0].HasVideo
	c.HasAudio = len(parsed.Variants) > 0 && parsed.Variants[0].HasAudio
	if !c.HasAudio {
		for _, at := range parsed.AudioTracks {
			c.HasAudio = true
			_ = at
			break
		}
	}

	if parsed.SourceType == media.SourceHLS && len(parsed.Variants) > 0 && parsed.Variants[0].SegmentURL != "" {
		s.probeCandidate(ctx, c, parsed.Variants[0].SegmentURL)
	}

	result.Candidates = append(result.Candidates, *c)
	return result, nil
}

func (s *AnalysisService) buildCandidateFromManifest(ctx context.Context, body []byte, base *url.URL, srcType media.SourceType, ref MediaRef) (*media.MediaCandidate, error) {
	parsed, err := ParseManifest(body, base)
	if err != nil {
		return &media.MediaCandidate{
			ID:                newCandidateID(),
			Title:             firstNonEmpty(ref.Title, deriveTitle(base.Path)),
			DisplayURL:        ref.DisplayURL,
			SourceType:        srcType,
			Unsupported:       fmt.Sprintf("manifest parse failed: %v", err),
			InternalSourceURL: ref.URL,
		}, nil
	}

	if parsed.DRM {
		return &media.MediaCandidate{
			ID:                newCandidateID(),
			Title:             firstNonEmpty(ref.Title, deriveTitle(base.Path)),
			DisplayURL:        ref.DisplayURL,
			SourceType:        srcType,
			Unsupported:       "DRM protected: " + parsed.DRMReason,
			InternalSourceURL: ref.URL,
		}, nil
	}

	c := &media.MediaCandidate{
		ID:                newCandidateID(),
		Title:             firstNonEmpty(ref.Title, deriveTitle(base.Path)),
		DisplayURL:        ref.DisplayURL,
		SourceType:        srcType,
		InternalSourceURL: ref.URL,
	}

	variantManifests := map[string]string{}
	for i, v := range parsed.Variants {
		mv := media.MediaVariant{
			ID:        fmt.Sprintf("%s-v%d", c.ID, i),
			Label:     v.Label,
			Width:     v.Width,
			Height:    v.Height,
			Bandwidth: v.Bandwidth,
			HasVideo:  v.HasVideo,
			HasAudio:  v.HasAudio,
		}
		c.Variants = append(c.Variants, mv)
		addr := v.SubManifestURL
		if addr == "" {
			addr = v.SegmentURL
		}
		if addr != "" {
			variantManifests[mv.ID] = addr
		}
	}
	if len(variantManifests) > 0 {
		c.InternalVariantManifests = variantManifests
	}

	c.HasVideo = len(parsed.Variants) > 0 && parsed.Variants[0].HasVideo
	c.HasAudio = len(parsed.Variants) > 0 && parsed.Variants[0].HasAudio
	if !c.HasAudio {
		for range parsed.AudioTracks {
			c.HasAudio = true
			break
		}
	}

	return c, nil
}

func (s *AnalysisService) analyzeDirectMedia(ctx context.Context, u *url.URL, contentType string, result *media.AnalysisResult) (*media.AnalysisResult, error) {
	ext := strings.ToLower(path.Ext(u.Path))
	c := &media.MediaCandidate{
		ID:                newCandidateID(),
		Title:             deriveTitle(u.Path),
		DisplayURL:        SanitizeDisplayURL(u),
		SourceType:        media.SourceDirect,
		Format:            strings.TrimPrefix(ext, "."),
		InternalSourceURL: u.String(),
	}

	applyMediaDefaults(c, ext, contentType)

	s.probeCandidate(ctx, c, u.String())

	result.Candidates = append(result.Candidates, *c)
	return result, nil
}

func (s *AnalysisService) probeCandidate(ctx context.Context, c *media.MediaCandidate, probeURL string) {
	if s.invoker == nil || !s.opts.FetchMediaInfo || probeURL == "" {
		return
	}

	u, err := ValidateURL(probeURL)
	if err != nil {
		return
	}

	if err := s.client.validateAndResolveHost(u.Hostname()); err != nil {
		return
	}

	probeCtx, cancel := context.WithTimeout(ctx, time.Duration(s.opts.ProbeTimeout)*time.Second)
	defer cancel()

	info, err := s.invoker.ProbeMedia(probeCtx, probeURL, ffmpeg.ProbeOptions{
		TimeoutSeconds: s.opts.ProbeTimeout,
	})
	if err != nil {
		return
	}

	if info == nil {
		return
	}

	c.HasVideo = info.HasVideo
	c.HasAudio = info.HasAudio
	c.DurationSeconds = info.DurationSeconds
	c.SizeBytes = info.SizeBytes

	if info.FormatName != "" {
		c.Format = info.FormatName
	}

	if len(info.VideoStreams) > 0 {
		c.Width = info.VideoStreams[0].Width
		c.Height = info.VideoStreams[0].Height
	}
}

func (s *AnalysisService) ProbeAll(ctx context.Context, candidates []*media.MediaCandidate, probeURLs []string) {
	if s.invoker == nil || !s.opts.FetchMediaInfo {
		return
	}

	n := len(candidates)
	if len(probeURLs) != n {
		return
	}

	sem := make(chan struct{}, s.opts.MaxConcurrency)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		if err := ctx.Err(); err != nil {
			return
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			s.probeCandidate(ctx, candidates[idx], probeURLs[idx])
		}(i)
	}

	wg.Wait()
}

func isMediaExtension(ext string) bool {
	switch ext {
	case ".mp4", ".m4v", ".mov", ".avi", ".mkv", ".webm", ".flv", ".wmv", ".ts", ".mts", ".m2ts",
		".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma", ".m4a":
		return true
	}
	return false
}

func isMediaContentType(ct string) bool {
	if ct == "" {
		return false
	}
	return strings.HasPrefix(ct, "video/") || strings.HasPrefix(ct, "audio/")
}

func applyMediaDefaults(c *media.MediaCandidate, ext string, contentType string) {
	if c.HasVideo || c.HasAudio {
		return
	}
	videoExts := map[string]bool{
		".mp4": true, ".m4v": true, ".mov": true, ".avi": true, ".mkv": true,
		".webm": true, ".flv": true, ".wmv": true, ".ts": true, ".mts": true, ".m2ts": true,
	}
	audioExts := map[string]bool{
		".mp3": true, ".wav": true, ".flac": true, ".aac": true, ".ogg": true, ".wma": true, ".m4a": true,
	}
	switch {
	case videoExts[ext] || strings.HasPrefix(contentType, "video/"):
		c.HasVideo = true
		c.HasAudio = true
	case audioExts[ext] || strings.HasPrefix(contentType, "audio/"):
		c.HasAudio = true
	}
}

func deriveTitle(p string) string {
	name := path.Base(p)
	if name == "/" || name == "." || name == "" {
		return "untitled"
	}
	ext := path.Ext(name)
	if ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	if name == "" {
		return "untitled"
	}
	return name
}

func newAnalysisID() string {
	return randHex(12)
}

func newCandidateID() string {
	return randHex(8)
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))[:n*2]
	}
	return hex.EncodeToString(b)
}
