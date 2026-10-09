package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	ProbeDefaultTimeoutSeconds = 15
	ProbeMaxOutputBytes        = 1 * 1024 * 1024
)

type ProbeOptions struct {
	TimeoutSeconds int
}

func (o ProbeOptions) withDefaults() ProbeOptions {
	if o.TimeoutSeconds <= 0 {
		o.TimeoutSeconds = ProbeDefaultTimeoutSeconds
	}
	return o
}

type MediaInfo struct {
	FormatName      string            `json:"formatName"`
	FormatLongName  string            `json:"formatLongName,omitempty"`
	DurationSeconds *float64          `json:"durationSeconds,omitempty"`
	SizeBytes       *int64            `json:"sizeBytes,omitempty"`
	BitRate         *int64            `json:"bitRate,omitempty"`
	HasVideo        bool              `json:"hasVideo"`
	HasAudio        bool              `json:"hasAudio"`
	VideoStreams    []VideoStreamInfo `json:"videoStreams,omitempty"`
	AudioStreams    []AudioStreamInfo `json:"audioStreams,omitempty"`
}

type VideoStreamInfo struct {
	CodecName string   `json:"codecName"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	FrameRate float64  `json:"frameRate"`
	PixFmt    string   `json:"pixFmt,omitempty"`
	Duration  *float64 `json:"duration,omitempty"`
	BitRate   *int64   `json:"bitRate,omitempty"`
}

type AudioStreamInfo struct {
	CodecName     string   `json:"codecName"`
	SampleRate    int      `json:"sampleRate"`
	Channels      int      `json:"channels"`
	ChannelLayout string   `json:"channelLayout,omitempty"`
	Duration      *float64 `json:"duration,omitempty"`
	BitRate       *int64   `json:"bitRate,omitempty"`
}

type ffprobeJSON struct {
	Streams []ffStreamRaw `json:"streams"`
	Format  ffFormatRaw   `json:"format"`
}

type ffStreamRaw struct {
	Index         int    `json:"index"`
	CodecName     string `json:"codec_name"`
	CodecType     string `json:"codec_type"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	RFrameRate    string `json:"r_frame_rate"`
	AvgFrameRate  string `json:"avg_frame_rate"`
	Duration      string `json:"duration"`
	BitRate       string `json:"bit_rate"`
	PixFmt        string `json:"pix_fmt"`
	SampleRate    string `json:"sample_rate"`
	Channels      int    `json:"channels"`
	ChannelLayout string `json:"channel_layout"`
}

type ffFormatRaw struct {
	FormatName     string `json:"format_name"`
	FormatLongName string `json:"format_long_name"`
	Duration       string `json:"duration"`
	Size           string `json:"size"`
	BitRate        string `json:"bit_rate"`
}

func (inv *Invoker) ProbeMedia(ctx context.Context, input string, opts ProbeOptions) (*MediaInfo, error) {
	if input == "" {
		return nil, fmt.Errorf("ffmpeg: ProbeMedia: empty input")
	}

	if err := validateProbeInput(input); err != nil {
		return nil, err
	}

	opts = opts.withDefaults()

	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		input,
	}

	probeCtx, cancel := context.WithTimeout(ctx, secondsToDuration(opts.TimeoutSeconds))
	defer cancel()

	res, err := inv.RunFFprobeWith(probeCtx, args, ProcessConfig{
		MaxStdoutBytes: ProbeMaxOutputBytes,
		MaxStderrBytes: 64 * 1024,
	})
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: probe failed for %q: %w", input, err)
	}

	return parseFFprobeJSON(res.Stdout)
}

func parseFFprobeJSON(raw []byte) (*MediaInfo, error) {
	if len(raw) == 0 {
		return &MediaInfo{}, nil
	}

	var parsed ffprobeJSON
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("ffmpeg: failed to parse ffprobe JSON: %w", err)
	}

	info := &MediaInfo{
		FormatName:     parsed.Format.FormatName,
		FormatLongName: parsed.Format.FormatLongName,
	}

	if dur, ok := parseFloat(parsed.Format.Duration); ok && dur > 0 {
		info.DurationSeconds = &dur
	}
	if sz, ok := parseInt64(parsed.Format.Size); ok && sz > 0 {
		info.SizeBytes = &sz
	}
	if br, ok := parseInt64(parsed.Format.BitRate); ok && br > 0 {
		info.BitRate = &br
	}

	for _, s := range parsed.Streams {
		switch s.CodecType {
		case "video":
			info.HasVideo = true
			info.VideoStreams = append(info.VideoStreams, VideoStreamInfo{
				CodecName: firstNonEmpty(s.CodecName, "unknown"),
				Width:     s.Width,
				Height:    s.Height,
				FrameRate: resolveFrameRate(s.RFrameRate, s.AvgFrameRate),
				PixFmt:    s.PixFmt,
				Duration:  optionalFloat(s.Duration),
				BitRate:   optionalInt64(s.BitRate),
			})
		case "audio":
			info.HasAudio = true
			info.AudioStreams = append(info.AudioStreams, AudioStreamInfo{
				CodecName:     firstNonEmpty(s.CodecName, "unknown"),
				SampleRate:    parseIntOrDefault(s.SampleRate, 0),
				Channels:      s.Channels,
				ChannelLayout: s.ChannelLayout,
				Duration:      optionalFloat(s.Duration),
				BitRate:       optionalInt64(s.BitRate),
			})
		}
	}

	if info.DurationSeconds == nil {
		info.DurationSeconds = pickStreamDuration(&parsed)
	}

	return info, nil
}

func pickStreamDuration(parsed *ffprobeJSON) *float64 {
	var firstVideoDur, firstAudioDur *float64
	for _, s := range parsed.Streams {
		if s.CodecType == "video" && firstVideoDur == nil {
			firstVideoDur = optionalFloat(s.Duration)
		}
		if s.CodecType == "audio" && firstAudioDur == nil {
			firstAudioDur = optionalFloat(s.Duration)
		}
	}
	if firstVideoDur != nil {
		return firstVideoDur
	}
	return firstAudioDur
}

func resolveFrameRate(rFrame, avgFrame string) float64 {
	if rFrame != "" {
		if v, ok := parseFraction(rFrame); ok {
			return v
		}
	}
	if avgFrame != "" {
		if v, ok := parseFraction(avgFrame); ok {
			return v
		}
	}
	return 0
}

func parseFraction(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0/0" {
		return 0, false
	}
	parts := strings.SplitN(s, "/", 2)
	if len(parts) == 1 {
		v, err := strconv.ParseFloat(parts[0], 64)
		if err != nil || v <= 0 {
			return 0, false
		}
		return v, true
	}
	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 || num == 0 {
		return 0, false
	}
	return num / den, true
}

func parseFloat(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func parseInt64(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func parseIntOrDefault(s string, def int) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func optionalFloat(s string) *float64 {
	if v, ok := parseFloat(s); ok && v > 0 {
		return &v
	}
	return nil
}

func optionalInt64(s string) *int64 {
	if v, ok := parseInt64(s); ok && v > 0 {
		return &v
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func secondsToDuration(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}

func validateProbeInput(input string) error {
	u, err := url.Parse(input)
	if err != nil || u.Scheme == "" {
		return nil
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		return nil
	}
	if len(u.Scheme) == 1 {
		return nil
	}
	return fmt.Errorf("ffmpeg: ProbeMedia: scheme %q is not allowed", u.Scheme)
}
