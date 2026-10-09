package ffmpeg

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func runFfmpegOrSkip(t *testing.T, args ...string) error {
	t.Helper()
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func TestParseFFprobeJSON_FullVideoAudio(t *testing.T) {
	raw := `{
		"streams": [
			{
				"index": 0,
				"codec_name": "h264",
				"codec_type": "video",
				"width": 1920,
				"height": 1080,
				"r_frame_rate": "30000/1001",
				"avg_frame_rate": "30000/1001",
				"duration": "120.500000",
				"bit_rate": "4500000",
				"pix_fmt": "yuv420p"
			},
			{
				"index": 1,
				"codec_name": "aac",
				"codec_type": "audio",
				"sample_rate": "48000",
				"channels": 2,
				"channel_layout": "stereo",
				"duration": "120.500000",
				"bit_rate": "128000"
			}
		],
		"format": {
			"format_name": "mov,mp4,m4a,3gp,3g2,mj2",
			"format_long_name": "QuickTime / MOV",
			"duration": "120.500000",
			"size": "72400000",
			"bit_rate": "4800000"
		}
	}`

	info, err := parseFFprobeJSON([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !info.HasVideo {
		t.Error("HasVideo should be true")
	}
	if !info.HasAudio {
		t.Error("HasAudio should be true")
	}
	if info.FormatName != "mov,mp4,m4a,3gp,3g2,mj2" {
		t.Errorf("FormatName = %q", info.FormatName)
	}
	if info.FormatLongName != "QuickTime / MOV" {
		t.Errorf("FormatLongName = %q", info.FormatLongName)
	}
	if info.DurationSeconds == nil || *info.DurationSeconds != 120.5 {
		t.Errorf("DurationSeconds = %v, want 120.5", info.DurationSeconds)
	}
	if info.SizeBytes == nil || *info.SizeBytes != 72400000 {
		t.Errorf("SizeBytes = %v", info.SizeBytes)
	}
	if info.BitRate == nil || *info.BitRate != 4800000 {
		t.Errorf("BitRate = %v", info.BitRate)
	}
	if len(info.VideoStreams) != 1 {
		t.Fatalf("want 1 video stream, got %d", len(info.VideoStreams))
	}
	v := info.VideoStreams[0]
	if v.CodecName != "h264" {
		t.Errorf("video codec = %q", v.CodecName)
	}
	if v.Width != 1920 || v.Height != 1080 {
		t.Errorf("resolution = %dx%d", v.Width, v.Height)
	}
	if v.FrameRate < 29.97 || v.FrameRate > 30.0 {
		t.Errorf("FrameRate = %f, want ~29.97", v.FrameRate)
	}
	if v.PixFmt != "yuv420p" {
		t.Errorf("PixFmt = %q", v.PixFmt)
	}
	if len(info.AudioStreams) != 1 {
		t.Fatalf("want 1 audio stream, got %d", len(info.AudioStreams))
	}
	a := info.AudioStreams[0]
	if a.CodecName != "aac" {
		t.Errorf("audio codec = %q", a.CodecName)
	}
	if a.SampleRate != 48000 {
		t.Errorf("SampleRate = %d", a.SampleRate)
	}
	if a.Channels != 2 {
		t.Errorf("Channels = %d", a.Channels)
	}
	if a.ChannelLayout != "stereo" {
		t.Errorf("ChannelLayout = %q", a.ChannelLayout)
	}
}

func TestParseFFprobeJSON_VideoOnly(t *testing.T) {
	raw := `{
		"streams": [
			{
				"codec_name": "vp9",
				"codec_type": "video",
				"width": 1280,
				"height": 720,
				"r_frame_rate": "60/1",
				"avg_frame_rate": "60/1",
				"pix_fmt": "yuv420p"
			}
		],
		"format": {
			"format_name": "webm",
			"format_long_name": "WebM"
		}
	}`

	info, err := parseFFprobeJSON([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !info.HasVideo {
		t.Error("HasVideo should be true")
	}
	if info.HasAudio {
		t.Error("HasAudio should be false")
	}
	if len(info.AudioStreams) != 0 {
		t.Errorf("want 0 audio streams, got %d", len(info.AudioStreams))
	}
	if info.VideoStreams[0].FrameRate != 60.0 {
		t.Errorf("FrameRate = %f, want 60.0", info.VideoStreams[0].FrameRate)
	}
	if info.DurationSeconds != nil {
		t.Errorf("DurationSeconds should be nil when absent, got %v", *info.DurationSeconds)
	}
	if info.SizeBytes != nil {
		t.Errorf("SizeBytes should be nil when absent")
	}
}

func TestParseFFprobeJSON_AudioOnly(t *testing.T) {
	raw := `{
		"streams": [
			{
				"codec_name": "mp3",
				"codec_type": "audio",
				"sample_rate": "44100",
				"channels": 2,
				"channel_layout": "stereo",
				"duration": "180.000000",
				"bit_rate": "320000"
			}
		],
		"format": {
			"format_name": "mp3",
			"format_long_name": "MP2/3 (MPEG audio layer 2/3)",
			"duration": "180.000000",
			"size": "7200000"
		}
	}`

	info, err := parseFFprobeJSON([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if info.HasVideo {
		t.Error("HasVideo should be false")
	}
	if !info.HasAudio {
		t.Error("HasAudio should be true")
	}
	if info.DurationSeconds == nil || *info.DurationSeconds != 180.0 {
		t.Errorf("DurationSeconds = %v", info.DurationSeconds)
	}
	if len(info.VideoStreams) != 0 {
		t.Errorf("want 0 video streams")
	}
	if info.AudioStreams[0].CodecName != "mp3" {
		t.Errorf("codec = %q", info.AudioStreams[0].CodecName)
	}
}

func TestParseFFprobeJSON_MultipleStreams(t *testing.T) {
	raw := `{
		"streams": [
			{"codec_name": "h264", "codec_type": "video", "width": 1920, "height": 1080, "r_frame_rate": "24/1"},
			{"codec_name": "h264", "codec_type": "video", "width": 1280, "height": 720, "r_frame_rate": "24/1"},
			{"codec_name": "aac", "codec_type": "audio", "sample_rate": "48000", "channels": 2, "channel_layout": "stereo"},
			{"codec_name": "aac", "codec_type": "audio", "sample_rate": "48000", "channels": 6, "channel_layout": "5.1"}
		],
		"format": {
			"format_name": "mpegts",
			"format_long_name": "MPEG-TS (MPEG-2 Transport Stream)"
		}
	}`

	info, err := parseFFprobeJSON([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(info.VideoStreams) != 2 {
		t.Errorf("want 2 video streams, got %d", len(info.VideoStreams))
	}
	if len(info.AudioStreams) != 2 {
		t.Errorf("want 2 audio streams, got %d", len(info.AudioStreams))
	}
	if info.VideoStreams[0].Width != 1920 || info.VideoStreams[1].Width != 1280 {
		t.Errorf("unexpected widths")
	}
	if info.AudioStreams[1].ChannelLayout != "5.1" {
		t.Errorf("channel layout = %q", info.AudioStreams[1].ChannelLayout)
	}
}

func TestParseFFprobeJSON_NoFormatDuration_FallbackToStream(t *testing.T) {
	raw := `{
		"streams": [
			{
				"codec_name": "h264",
				"codec_type": "video",
				"width": 640,
				"height": 480,
				"r_frame_rate": "30/1",
				"duration": "45.000000"
			}
		],
		"format": {
			"format_name": "mp4"
		}
	}`

	info, err := parseFFprobeJSON([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if info.DurationSeconds == nil {
		t.Fatal("DurationSeconds should fallback to video stream")
	}
	if *info.DurationSeconds != 45.0 {
		t.Errorf("DurationSeconds = %f, want 45.0", *info.DurationSeconds)
	}
}

func TestParseFFprobeJSON_EmptyStreams(t *testing.T) {
	raw := `{
		"streams": [],
		"format": {
			"format_name": "data",
			"format_long_name": "raw data"
		}
	}`

	info, err := parseFFprobeJSON([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if info.HasVideo || info.HasAudio {
		t.Error("should not have video or audio")
	}
	if len(info.VideoStreams) != 0 || len(info.AudioStreams) != 0 {
		t.Error("stream slices should be empty")
	}
}

func TestParseFFprobeJSON_EmptyInput(t *testing.T) {
	info, err := parseFFprobeJSON([]byte{})
	if err != nil {
		t.Fatalf("empty input should not error, got: %v", err)
	}
	if info == nil {
		t.Fatal("should return non-nil MediaInfo")
	}
}

func TestParseFFprobeJSON_InvalidJSON(t *testing.T) {
	_, err := parseFFprobeJSON([]byte("not json {{{"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseFFprobeJSON_CorruptJSON(t *testing.T) {
	_, err := parseFFprobeJSON([]byte(`{"streams": [broken]}`))
	if err == nil {
		t.Fatal("expected error for corrupt JSON")
	}
}

func TestParseFFprobeJSON_MinimalFields(t *testing.T) {
	raw := `{"streams": [{"codec_type": "video"}]}`

	info, err := parseFFprobeJSON([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !info.HasVideo {
		t.Error("should detect video stream")
	}
	if len(info.VideoStreams) != 1 {
		t.Fatalf("want 1 video stream")
	}
	if info.VideoStreams[0].CodecName != "unknown" {
		t.Errorf("empty codec_name should become 'unknown', got %q", info.VideoStreams[0].CodecName)
	}
	if info.VideoStreams[0].Width != 0 || info.VideoStreams[0].Height != 0 {
		t.Errorf("missing resolution should default to 0")
	}
	if info.VideoStreams[0].FrameRate != 0 {
		t.Errorf("missing frame rate should be 0")
	}
}

func TestParseFFprobeJSON_FrameRateVariants(t *testing.T) {
	tests := []struct {
		name   string
		rFrame string
		avg    string
		want   float64
	}{
		{"exact", "30/1", "30/1", 30.0},
		{"fractional", "30000/1001", "30000/1001", 30000.0 / 1001.0},
		{"r_frame preferred", "24/1", "60/1", 24.0},
		{"fallback to avg", "", "50/1", 50.0},
		{"both zero", "0/0", "0/0", 0},
		{"r_frame zero avg valid", "0/0", "25/1", 25.0},
		{"all missing", "", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := `{"streams": [{"codec_type": "video", "codec_name": "h264", "width": 1, "height": 1, "r_frame_rate": "` + tt.rFrame + `", "avg_frame_rate": "` + tt.avg + `"}]}`
			info, err := parseFFprobeJSON([]byte(raw))
			if err != nil {
				t.Fatal(err)
			}
			got := info.VideoStreams[0].FrameRate
			if tt.want == 0 {
				if got != 0 {
					t.Errorf("FrameRate = %f, want 0", got)
				}
			} else {
				diff := got - tt.want
				if diff < 0 {
					diff = -diff
				}
				if diff > 0.01 {
					t.Errorf("FrameRate = %f, want %f", got, tt.want)
				}
			}
		})
	}
}

func TestParseFFprobeJSON_ZeroValuesOmitted(t *testing.T) {
	raw := `{
		"streams": [
			{"codec_name": "h264", "codec_type": "video", "width": 1, "height": 1, "r_frame_rate": "1/1", "duration": "0.000000", "bit_rate": "0"},
			{"codec_name": "aac", "codec_type": "audio", "sample_rate": "0", "channels": 0, "duration": "0.000000", "bit_rate": "0"}
		],
		"format": {
			"format_name": "x",
			"duration": "0",
			"size": "0",
			"bit_rate": "0"
		}
	}`

	info, err := parseFFprobeJSON([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if info.DurationSeconds != nil {
		t.Errorf("zero format duration should be nil")
	}
	if info.SizeBytes != nil {
		t.Errorf("zero size should be nil")
	}
	if info.BitRate != nil {
		t.Errorf("zero bit rate should be nil")
	}
	if info.VideoStreams[0].Duration != nil {
		t.Errorf("zero stream duration should be nil")
	}
	if info.VideoStreams[0].BitRate != nil {
		t.Errorf("zero stream bit_rate should be nil")
	}
	if info.AudioStreams[0].SampleRate != 0 {
		t.Errorf("zero sample rate should stay 0, got %d", info.AudioStreams[0].SampleRate)
	}
}

func TestParseFFprobeJSON_JSONTags(t *testing.T) {
	raw := `{
		"streams": [
			{"codec_name": "h264", "codec_type": "video", "width": 640, "height": 480, "r_frame_rate": "30/1", "pix_fmt": "yuv420p"},
			{"codec_name": "aac", "codec_type": "audio", "sample_rate": "44100", "channels": 2, "channel_layout": "stereo"}
		],
		"format": {"format_name": "mp4", "format_long_name": "MP4", "duration": "10.0", "size": "102400"}
	}`

	info, err := parseFFprobeJSON([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)

	expectedKeys := []string{
		`"formatName":"mp4"`,
		`"formatLongName":"MP4"`,
		`"hasVideo":true`,
		`"hasAudio":true`,
		`"codecName":"h264"`,
		`"codecName":"aac"`,
		`"width":640`,
		`"height":480`,
		`"durationSeconds":10`,
		`"sizeBytes":102400`,
	}
	for _, key := range expectedKeys {
		if !strings.Contains(s, key) {
			t.Errorf("JSON missing %q in %s", key, s)
		}
	}
}

func TestParseFFprobeJSON_IgnoresUnknownStreamTypes(t *testing.T) {
	raw := `{
		"streams": [
			{"codec_type": "subtitle", "codec_name": "srt"},
			{"codec_type": "data"},
			{"codec_name": "h264", "codec_type": "video", "width": 100, "height": 100, "r_frame_rate": "1/1"},
			{"codec_name": "aac", "codec_type": "audio", "sample_rate": "44100", "channels": 1}
		],
		"format": {"format_name": "mkv"}
	}`

	info, err := parseFFprobeJSON([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(info.VideoStreams) != 1 {
		t.Errorf("subtitle/data streams should be ignored, video count = %d", len(info.VideoStreams))
	}
	if len(info.AudioStreams) != 1 {
		t.Errorf("audio count = %d", len(info.AudioStreams))
	}
}

func TestParseFraction(t *testing.T) {
	tests := []struct {
		s    string
		want float64
		ok   bool
	}{
		{"30/1", 30.0, true},
		{"30000/1001", 30000.0 / 1001.0, true},
		{"0/0", 0, false},
		{"0/1", 0, false},
		{"", 0, false},
		{"abc", 0, false},
		{"120", 120.0, true},
		{"-5", 0, false},
		{"0", 0, false},
	}

	for _, tt := range tests {
		v, ok := parseFraction(tt.s)
		if ok != tt.ok {
			t.Errorf("parseFraction(%q) ok = %v, want %v", tt.s, ok, tt.ok)
			continue
		}
		if ok {
			diff := v - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.01 {
				t.Errorf("parseFraction(%q) = %f, want %f", tt.s, v, tt.want)
			}
		}
	}
}

func TestInvoker_ProbeMedia_StubScript(t *testing.T) {
	tmp := t.TempDir()

	jsonPayload := `{
		"streams": [
			{"codec_name": "h264", "codec_type": "video", "width": 1920, "height": 1080, "r_frame_rate": "30/1", "duration": "60.0"},
			{"codec_name": "aac", "codec_type": "audio", "sample_rate": "48000", "channels": 2, "channel_layout": "stereo", "duration": "60.0"}
		],
		"format": {"format_name": "mp4", "duration": "60.0", "size": "50000000"}
	}`

	writeStubScript(t, tmp, "ffprobe",
		`#!/bin/sh
echo '`+jsonPayload+`'
exit 0
`)

	writeStubScript(t, tmp, "ffmpeg",
		`#!/bin/sh
exit 0
`)

	inv := NewInvoker(ToolPaths{
		FFmpeg:  filepath.Join(tmp, "ffmpeg"),
		FFprobe: filepath.Join(tmp, "ffprobe"),
	})

	info, err := inv.ProbeMedia(context.Background(), "/fake/video.mp4", ProbeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.HasVideo || !info.HasAudio {
		t.Error("should have both video and audio")
	}
	if info.DurationSeconds == nil || *info.DurationSeconds != 60.0 {
		t.Errorf("duration = %v", info.DurationSeconds)
	}
	if len(info.VideoStreams) != 1 || info.VideoStreams[0].Width != 1920 {
		t.Error("video stream info mismatch")
	}
}

func TestInvoker_ProbeMedia_EmptyInput(t *testing.T) {
	inv := NewInvoker(ToolPaths{})
	_, err := inv.ProbeMedia(context.Background(), "", ProbeOptions{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestInvoker_ProbeMedia_ProbeTimeout(t *testing.T) {
	tmp := t.TempDir()

	writeStubScript(t, tmp, "ffprobe",
		`#!/bin/sh
sleep 5
exit 0
`)

	writeStubScript(t, tmp, "ffmpeg",
		`#!/bin/sh
exit 0
`)

	inv := NewInvoker(ToolPaths{
		FFmpeg:  filepath.Join(tmp, "ffmpeg"),
		FFprobe: filepath.Join(tmp, "ffprobe"),
	})

	ctx := context.Background()
	_, err := inv.ProbeMedia(ctx, "/fake/slow.mp4", ProbeOptions{TimeoutSeconds: 1})
	if err == nil {
		t.Fatal("expected timeout error")
	}

	var pe *ProcessError
	if !errors.As(err, &pe) {
		t.Fatalf("want *ProcessError wrapped, got %T: %v", err, err)
	}
	if !pe.IsTimeout() {
		t.Errorf("want IsTimeout=true, got false")
	}
}

func TestInvoker_ProbeMedia_BinaryNotFound(t *testing.T) {
	inv := NewInvoker(ToolPaths{FFprobe: "/nonexistent/ffprobe"})
	_, err := inv.ProbeMedia(context.Background(), "/anything.mp4", ProbeOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInvoker_ProbeMedia_CorruptMedia(t *testing.T) {
	tmp := t.TempDir()

	writeStubScript(t, tmp, "ffprobe",
		`#!/bin/sh
echo "Invalid data found when processing input" >&2
exit 1
`)

	writeStubScript(t, tmp, "ffmpeg",
		`#!/bin/sh
exit 0
`)

	inv := NewInvoker(ToolPaths{
		FFmpeg:  filepath.Join(tmp, "ffmpeg"),
		FFprobe: filepath.Join(tmp, "ffprobe"),
	})

	_, err := inv.ProbeMedia(context.Background(), "/tmp/corrupt.mp4", ProbeOptions{})
	if err == nil {
		t.Fatal("expected error for corrupt media")
	}
}

func TestInvoker_ProbeMedia_RealMP4(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real binary test skipped on windows")
	}

	_, err := os.Stat("/opt/homebrew/bin/ffprobe")
	if os.IsNotExist(err) {
		t.Skip("ffprobe not installed, skipping real binary test")
	}

	tmp := t.TempDir()
	mp4Path := filepath.Join(tmp, "sample.mp4")

	ffmpegErr := runFfmpegOrSkip(t, "-y", "-f", "lavfi",
		"-i", "testsrc=duration=1:size=320x240:rate=10",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-c:v", "libx264", "-c:a", "aac", "-shortest",
		mp4Path)
	if ffmpegErr != nil {
		t.Skipf("ffmpeg not available for sample generation: %v", ffmpegErr)
	}

	inv := NewInvoker(ToolPaths{FFprobe: "/opt/homebrew/bin/ffprobe"})
	info, err := inv.ProbeMedia(context.Background(), mp4Path, ProbeOptions{TimeoutSeconds: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.HasVideo {
		t.Error("should have video")
	}
	if !info.HasAudio {
		t.Error("should have audio")
	}
	if len(info.VideoStreams) != 1 {
		t.Fatalf("want 1 video stream")
	}
	if info.VideoStreams[0].Width != 320 || info.VideoStreams[0].Height != 240 {
		t.Errorf("resolution = %dx%d", info.VideoStreams[0].Width, info.VideoStreams[0].Height)
	}
	if info.DurationSeconds == nil || *info.DurationSeconds <= 0 {
		t.Errorf("duration should be > 0, got %v", info.DurationSeconds)
	}
}

func TestInvoker_ProbeMedia_RealWebM(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real binary test skipped on windows")
	}
	_, err := os.Stat("/opt/homebrew/bin/ffprobe")
	if os.IsNotExist(err) {
		t.Skip("ffprobe not installed")
	}

	tmp := t.TempDir()
	webmPath := filepath.Join(tmp, "sample.webm")

	ffmpegErr := runFfmpegOrSkip(t, "-y", "-f", "lavfi",
		"-i", "testsrc=duration=1:size=320x240:rate=10",
		"-c:v", "libvpx", webmPath)
	if ffmpegErr != nil {
		t.Skipf("ffmpeg/VP8 not available: %v", ffmpegErr)
	}

	inv := NewInvoker(ToolPaths{FFprobe: "/opt/homebrew/bin/ffprobe"})
	info, err := inv.ProbeMedia(context.Background(), webmPath, ProbeOptions{TimeoutSeconds: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.HasVideo {
		t.Error("should have video")
	}
	if !strings.Contains(strings.ToLower(info.FormatName), "webm") &&
		!strings.Contains(strings.ToLower(info.FormatName), "matroska") {
		t.Logf("format name = %q (webm is a matroska container)", info.FormatName)
	}
}

func TestInvoker_ProbeMedia_PureAudioMP3(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real binary test skipped on windows")
	}
	_, err := os.Stat("/opt/homebrew/bin/ffprobe")
	if os.IsNotExist(err) {
		t.Skip("ffprobe not installed")
	}

	tmp := t.TempDir()
	mp3Path := filepath.Join(tmp, "sample.mp3")

	ffmpegErr := runFfmpegOrSkip(t, "-y", "-f", "lavfi",
		"-i", "sine=frequency=220:duration=1",
		"-c:a", "libmp3lame", mp3Path)
	if ffmpegErr != nil {
		t.Skipf("ffmpeg/mp3lame not available: %v", ffmpegErr)
	}

	inv := NewInvoker(ToolPaths{FFprobe: "/opt/homebrew/bin/ffprobe"})
	info, err := inv.ProbeMedia(context.Background(), mp3Path, ProbeOptions{TimeoutSeconds: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.HasVideo {
		t.Error("should NOT have video")
	}
	if !info.HasAudio {
		t.Error("should have audio")
	}
	if len(info.AudioStreams) != 1 {
		t.Fatalf("want 1 audio stream")
	}
}

func TestInvoker_ProbeMedia_CorruptRealFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real binary test skipped on windows")
	}
	_, err := os.Stat("/opt/homebrew/bin/ffprobe")
	if os.IsNotExist(err) {
		t.Skip("ffprobe not installed")
	}

	tmp := t.TempDir()
	corruptPath := filepath.Join(tmp, "corrupt.mp4")
	if err := os.WriteFile(corruptPath, []byte("this is not a real mp4 file just garbage data"), 0o644); err != nil {
		t.Fatal(err)
	}

	inv := NewInvoker(ToolPaths{FFprobe: "/opt/homebrew/bin/ffprobe"})
	_, err = inv.ProbeMedia(context.Background(), corruptPath, ProbeOptions{TimeoutSeconds: 10})
	if err == nil {
		t.Fatal("expected error for corrupt real file")
	}
}

func TestProbeOptions_Defaults(t *testing.T) {
	var opts ProbeOptions
	opts = opts.withDefaults()
	if opts.TimeoutSeconds != ProbeDefaultTimeoutSeconds {
		t.Errorf("default timeout = %d, want %d", opts.TimeoutSeconds, ProbeDefaultTimeoutSeconds)
	}

	custom := ProbeOptions{TimeoutSeconds: 30}.withDefaults()
	if custom.TimeoutSeconds != 30 {
		t.Errorf("custom timeout should be preserved, got %d", custom.TimeoutSeconds)
	}

	zero := ProbeOptions{TimeoutSeconds: 0}.withDefaults()
	if zero.TimeoutSeconds != ProbeDefaultTimeoutSeconds {
		t.Errorf("zero timeout should use default, got %d", zero.TimeoutSeconds)
	}

	neg := ProbeOptions{TimeoutSeconds: -5}.withDefaults()
	if neg.TimeoutSeconds != ProbeDefaultTimeoutSeconds {
		t.Errorf("negative timeout should use default, got %d", neg.TimeoutSeconds)
	}
}

func TestMediaInfo_JSONOmitEmpty(t *testing.T) {
	info := &MediaInfo{
		FormatName: "mp4",
		HasVideo:   false,
		HasAudio:   false,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)

	if strings.Contains(s, `"videoStreams"`) {
		t.Errorf("empty VideoStreams should be omitted: %s", s)
	}
	if strings.Contains(s, `"audioStreams"`) {
		t.Errorf("empty AudioStreams should be omitted: %s", s)
	}
}

func TestInvoker_ProbeMedia_ContextCanceled(t *testing.T) {
	tmp := t.TempDir()

	writeStubScript(t, tmp, "ffprobe",
		`#!/bin/sh
sleep 10
exit 0
`)

	writeStubScript(t, tmp, "ffmpeg",
		`#!/bin/sh
exit 0
`)

	inv := NewInvoker(ToolPaths{
		FFmpeg:  filepath.Join(tmp, "ffmpeg"),
		FFprobe: filepath.Join(tmp, "ffprobe"),
	})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := inv.ProbeMedia(ctx, "/fake/long.mp4", ProbeOptions{TimeoutSeconds: 30})
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error after cancel")
		}
		var pe *ProcessError
		if !errors.As(err, &pe) {
			t.Fatalf("want *ProcessError, got %T", err)
		}
		if !pe.IsCanceled() {
			t.Errorf("want IsCanceled=true")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ProbeMedia did not return after cancel")
	}
}
