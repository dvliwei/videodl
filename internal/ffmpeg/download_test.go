package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"videodl/internal/media"
)

func TestBuildDownloadArgs_DirectOriginal(t *testing.T) {
	args, err := BuildDownloadArgs(DownloadOptions{
		InputURL:   "https://example.com/video.mp4",
		SourceType: media.SourceDirect,
		OutputPath: "/tmp/out.mp4",
		Profile:    media.ProfileOriginal,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContainsAll(t, args,
		"-nostdin", "-y", "-progress", "pipe:1",
		"-protocol_whitelist", "-c", "copy",
		"-movflags", "+faststart",
	)
	assertContains(t, args, "https://example.com/video.mp4")
	assertContains(t, args, "/tmp/out.mp4")
	assertNotContains(t, args, "-map")
	assertNotContains(t, args, "-c:v")
}

func TestBuildDownloadArgs_RefererAndUserAgent(t *testing.T) {
	args, err := BuildDownloadArgs(DownloadOptions{
		InputURL:   "https://example.com/seg.ts",
		SourceType: media.SourceHLS,
		OutputPath: "/tmp/out.ts",
		Referer:    "https://example.com/page.html",
		UserAgent:  "VideoDL/1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	headerIdx := indexOf(args, "-headers")
	if headerIdx < 0 {
		t.Fatal("want -headers arg")
	}
	headerVal := args[headerIdx+1]
	if !strings.Contains(headerVal, "Referer: https://example.com/page.html") {
		t.Errorf("headers missing Referer, got: %s", headerVal)
	}
	if !strings.Contains(headerVal, "User-Agent: VideoDL/1.0") {
		t.Errorf("headers missing User-Agent, got: %s", headerVal)
	}
}

func TestBuildDownloadArgs_ExtraHeaders(t *testing.T) {
	args, err := BuildDownloadArgs(DownloadOptions{
		InputURL:   "https://example.com/video",
		OutputPath: "/tmp/out.mkv",
		Headers:    map[string]string{"Authorization": "Bearer token", "X-Custom": "value"},
	})
	if err != nil {
		t.Fatal(err)
	}
	headerIdx := indexOf(args, "-headers")
	if headerIdx < 0 {
		t.Fatal("want -headers arg")
	}
	headerVal := args[headerIdx+1]
	if !strings.Contains(headerVal, "Authorization: Bearer token") {
		t.Error("Authorization header missing")
	}
	if !strings.Contains(headerVal, "X-Custom: value") {
		t.Error("X-Custom header missing")
	}
}

func TestBuildDownloadArgs_DASHMapStreams(t *testing.T) {
	args, err := BuildDownloadArgs(DownloadOptions{
		InputURL:   "https://example.com/manifest.mpd",
		SourceType: media.SourceDASH,
		OutputPath: "/tmp/out.mp4",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContainsAll(t, args, "-map", "0:v?", "-map", "0:a?")
}

func TestBuildDownloadArgs_MP4ProfileTranscode(t *testing.T) {
	args, err := BuildDownloadArgs(DownloadOptions{
		InputURL:   "https://example.com/video.webm",
		OutputPath: "/tmp/out.mp4",
		Profile:    media.ProfileMP4,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContainsAll(t, args,
		"-c:v", "libx264", "-c:a", "aac",
		"-preset", "veryfast", "-crf", "23",
		"-pix_fmt", "yuv420p",
	)
	assertNotContains(t, args, "-c")
}

func TestBuildDownloadArgs_ValidationErrors(t *testing.T) {
	_, err := BuildDownloadArgs(DownloadOptions{OutputPath: "/tmp/out"})
	if err == nil || !strings.Contains(err.Error(), "empty input URL") {
		t.Errorf("want empty input URL error, got %v", err)
	}
	_, err = BuildDownloadArgs(DownloadOptions{InputURL: "https://x"})
	if err == nil || !strings.Contains(err.Error(), "empty output") {
		t.Errorf("want empty output error, got %v", err)
	}
}

func TestBuildDownloadArgs_MKVNoFaststart(t *testing.T) {
	args, err := BuildDownloadArgs(DownloadOptions{
		InputURL:   "https://example.com/video.mkv",
		OutputPath: "/tmp/out.mkv",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertNotContains(t, args, "-movflags")
}

func TestParseProgressKV_OutTimeUs(t *testing.T) {
	p := ParseProgressKV("out_time_us=500000", DownloadProgress{TotalUs: 1_000_000})
	if p.OutTimeUs != 500000 {
		t.Errorf("OutTimeUs = %d, want 500000", p.OutTimeUs)
	}
	if p.Percentage == nil {
		t.Fatal("want percentage")
	}
	if *p.Percentage < 49 || *p.Percentage > 51 {
		t.Errorf("percentage = %v, want ~50", *p.Percentage)
	}
}

func TestParseProgressKV_NoTotalNoPercentage(t *testing.T) {
	p := ParseProgressKV("out_time_us=500000", DownloadProgress{})
	if p.Percentage != nil {
		t.Errorf("want nil percentage when TotalUs is zero, got %v", *p.Percentage)
	}
}

func TestParseProgressKV_ProgressEnd(t *testing.T) {
	p := ParseProgressKV("progress=end", DownloadProgress{TotalUs: 1_000_000, OutTimeUs: 999_999})
	if !p.IsEnd {
		t.Error("want IsEnd=true")
	}
	if p.Percentage == nil {
		t.Fatal("want percentage")
	}
	if *p.Percentage > 100 {
		t.Errorf("percentage capped at 100, got %v", *p.Percentage)
	}
}

func TestParseProgressKV_SizeAndSpeed(t *testing.T) {
	p := DownloadProgress{}
	p = ParseProgressKV("total_size=1234567", p)
	p = ParseProgressKV("speed=2.5x", p)
	if p.SizeBytes != 1234567 {
		t.Errorf("SizeBytes = %d, want 1234567", p.SizeBytes)
	}
	if p.SpeedQ != "2.5x" {
		t.Errorf("SpeedQ = %q, want 2.5x", p.SpeedQ)
	}
}

func TestParseProgressKV_IgnoresEmptyAndBad(t *testing.T) {
	p := DownloadProgress{OutTimeUs: 100}
	p = ParseProgressKV("", p)
	p = ParseProgressKV("noequalsign", p)
	p = ParseProgressKV("out_time_us=notanumber", p)
	if p.OutTimeUs != 100 {
		t.Errorf("OutTimeUs mutated, got %d", p.OutTimeUs)
	}
}

func TestDownloadProgress_SetTotalDuration(t *testing.T) {
	p := DownloadProgress{}
	p.SetTotalDurationSeconds(60.0)
	if p.TotalUs != 60_000_000 {
		t.Errorf("TotalUs = %d, want 60_000_000", p.TotalUs)
	}

	p2 := DownloadProgress{OutTimeUs: 30_000_000}
	p2.SetTotalDurationSeconds(60.0)
	if p2.Percentage == nil || *p2.Percentage < 49 || *p2.Percentage > 51 {
		t.Errorf("percentage = %v, want ~50", p2.Percentage)
	}

	p3 := DownloadProgress{}
	p3.SetTotalDurationSeconds(-1)
	if p3.TotalUs != 0 {
		t.Errorf("negative duration should not set TotalUs, got %d", p3.TotalUs)
	}
}

func TestStreamProgress_ParsesMultipleLines(t *testing.T) {
	input := "out_time_us=100000\nout_time_us=200000\nspeed=1.5x\nprogress=end\n"

	var mu sync.Mutex
	var received []DownloadProgress

	sink := ProgressFunc(func(p DownloadProgress) {
		mu.Lock()
		received = append(received, p)
		mu.Unlock()
	})

	err := StreamProgress(strings.NewReader(input), sink)
	if err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(received) == 0 {
		t.Fatal("want at least one progress update")
	}

	last := received[len(received)-1]
	if !last.IsEnd {
		t.Error("last progress should be end")
	}
	if last.OutTimeUs != 200000 {
		t.Errorf("last OutTimeUs = %d, want 200000", last.OutTimeUs)
	}
}

func TestStreamProgress_NilSink(t *testing.T) {
	input := "out_time_us=100000\nprogress=end\n"
	err := StreamProgress(strings.NewReader(input), nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestStreamProgress_NilReader(t *testing.T) {
	err := StreamProgress(nil, nil)
	if err == nil {
		t.Fatal("want error for nil reader")
	}
}

func TestRunFFmpegDownload_Success(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "out.mp4")

	script := writeStubScript(t, tmp, "ffmpeg",
		`#!/bin/sh
echo "out_time_us=1000000"
echo "progress=continue"
echo "out_time_us=2000000"
echo "progress=continue"
echo "out_time_us=3000000"
echo "progress=end"
touch "`+outPath+`"
exit 0
`)

	writeStubScript(t, tmp, "ffprobe",
		`#!/bin/sh
exit 0
`)

	inv := NewInvoker(ToolPaths{FFmpeg: script, FFprobe: filepath.Join(tmp, "ffprobe")})

	var mu sync.Mutex
	var lastProgress DownloadProgress
	var callCount int32

	sink := ProgressFunc(func(p DownloadProgress) {
		mu.Lock()
		lastProgress = p
		atomic.AddInt32(&callCount, 1)
		mu.Unlock()
	})

	opts := DownloadOptions{
		InputURL:        "https://example.com/video.m3u8",
		SourceType:      media.SourceHLS,
		OutputPath:      outPath,
		Profile:         media.ProfileOriginal,
		DurationSeconds: floatPtr(3.0),
	}

	res, err := RunFFmpegDownload(context.Background(), inv, opts, sink)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}

	mu.Lock()
	defer mu.Unlock()
	if lastProgress.OutTimeUs != 3_000_000 {
		t.Errorf("last OutTimeUs = %d, want 3000000", lastProgress.OutTimeUs)
	}
	if !lastProgress.IsEnd {
		t.Error("want IsEnd=true")
	}
	if lastProgress.Percentage == nil || *lastProgress.Percentage != 100.0 {
		t.Errorf("percentage = %v, want 100", lastProgress.Percentage)
	}
}

func TestRunFFmpegDownload_NonZeroExit(t *testing.T) {
	tmp := t.TempDir()

	script := writeStubScript(t, tmp, "ffmpeg",
		`#!/bin/sh
echo "some error" >&2
exit 2
`)

	writeStubScript(t, tmp, "ffprobe",
		`#!/bin/sh
exit 0
`)

	inv := NewInvoker(ToolPaths{FFmpeg: script, FFprobe: filepath.Join(tmp, "ffprobe")})

	_, err := RunFFmpegDownload(context.Background(), inv, DownloadOptions{
		InputURL:   "https://example.com/bad",
		OutputPath: filepath.Join(tmp, "out.mp4"),
	}, nil)

	if err == nil {
		t.Fatal("want error for non-zero exit")
	}

	var pe *ProcessError
	if !errors.As(err, &pe) {
		t.Fatalf("want *ProcessError, got %T: %v", err, err)
	}
	if pe.ExitCode() != 2 {
		t.Errorf("ExitCode = %d, want 2", pe.ExitCode())
	}
}

func TestRunFFmpegDownload_Cancel(t *testing.T) {
	tmp := t.TempDir()

	script := writeStubScript(t, tmp, "ffmpeg",
		`#!/bin/sh
echo "out_time_us=100000"
echo "progress=continue"
sleep 5
exit 0
`)

	writeStubScript(t, tmp, "ffprobe",
		`#!/bin/sh
exit 0
`)

	inv := NewInvoker(ToolPaths{FFmpeg: script, FFprobe: filepath.Join(tmp, "ffprobe")})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := RunFFmpegDownload(ctx, inv, DownloadOptions{
			InputURL:   "https://example.com/slow.m3u8",
			OutputPath: filepath.Join(tmp, "out.mp4"),
		}, nil)
		done <- err
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("want error after cancel")
		}
		var pe *ProcessError
		if !errors.As(err, &pe) {
			t.Fatalf("want *ProcessError, got %T", err)
		}
		if !pe.IsCanceled() {
			t.Errorf("want IsCanceled=true, got %v (err=%v)", pe, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunFFmpegDownload did not return after cancel")
	}
}

func TestRunFFmpegDownload_MissingBinary(t *testing.T) {
	inv := NewInvoker(ToolPaths{FFmpeg: "/nonexistent/ffmpeg", FFprobe: ""})
	_, err := RunFFmpegDownload(context.Background(), inv, DownloadOptions{
		InputURL:   "https://x",
		OutputPath: "/tmp/out",
	}, nil)
	if err == nil {
		t.Fatal("want error")
	}
	var pe *ProcessError
	if !errors.As(err, &pe) {
		t.Fatalf("want *ProcessError, got %T", err)
	}
}

func TestRunFFmpegDownload_NilInvoker(t *testing.T) {
	_, err := RunFFmpegDownload(context.Background(), nil, DownloadOptions{
		InputURL:   "https://x",
		OutputPath: "/tmp/out",
	}, nil)
	if err == nil {
		t.Fatal("want error for nil invoker")
	}
}

func TestRunFFmpegDownload_ArgsContainProgressPipe(t *testing.T) {
	args, err := BuildDownloadArgs(DownloadOptions{
		InputURL:   "https://example.com/test",
		OutputPath: "/tmp/out.mp4",
	})
	if err != nil {
		t.Fatal(err)
	}

	hasProgress := false
	hasPipe := false
	for i, a := range args {
		if a == "-progress" && i+1 < len(args) && args[i+1] == "pipe:1" {
			hasProgress = true
		}
		if a == "-progress" && i+1 < len(args) && args[i+1] != "pipe:1" {
			hasPipe = true
		}
	}
	if !hasProgress {
		t.Errorf("args missing `-progress pipe:1`: %v", args)
	}
	if hasPipe && !hasProgress {
		t.Error("progress should go to pipe:1")
	}
}

func TestRunFFmpegDownload_DASHProducesArgs(t *testing.T) {
	args, err := BuildDownloadArgs(DownloadOptions{
		InputURL:   "https://example.com/manifest.mpd",
		SourceType: media.SourceDASH,
		OutputPath: "/tmp/out.mp4",
		Profile:    media.ProfileOriginal,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, args, "-map")
	assertContains(t, args, "0:v?")
	assertContains(t, args, "0:a?")
}

func floatPtr(v float64) *float64 { return &v }

func indexOf(s []string, target string) int {
	for i, v := range s {
		if v == target {
			return i
		}
	}
	return -1
}

func assertContains(t *testing.T, args []string, needle string) {
	t.Helper()
	for _, a := range args {
		if a == needle {
			return
		}
	}
	t.Errorf("args missing %q in %v", needle, args)
}

func assertNotContains(t *testing.T, args []string, needle string) {
	t.Helper()
	for _, a := range args {
		if a == needle {
			t.Errorf("args unexpectedly contains %q", needle)
			return
		}
	}
}

func assertContainsAll(t *testing.T, args []string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		assertContains(t, args, n)
	}
}

func TestRunFFmpegDownload_NetworkErrorReported(t *testing.T) {
	tmp := t.TempDir()

	script := writeStubScript(t, tmp, "ffmpeg",
		`#!/bin/sh
echo "[error] Connection refused" >&2
echo "out_time_us=500000"
echo "progress=continue"
exit 1
`)

	writeStubScript(t, tmp, "ffprobe",
		`#!/bin/sh
exit 0
`)

	inv := NewInvoker(ToolPaths{FFmpeg: script, FFprobe: filepath.Join(tmp, "ffprobe")})

	_, err := RunFFmpegDownload(context.Background(), inv, DownloadOptions{
		InputURL:   "https://example.com/video",
		OutputPath: filepath.Join(tmp, "out.mp4"),
	}, nil)

	if err == nil {
		t.Fatal("want error for network failure")
	}

	var pe *ProcessError
	if !errors.As(err, &pe) {
		t.Fatalf("want *ProcessError, got %T", err)
	}
	if pe.ExitCode() == 0 {
		t.Error("want non-zero exit code")
	}
}

func TestBuildDownloadArgs_NoShellInjection(t *testing.T) {
	args, err := BuildDownloadArgs(DownloadOptions{
		InputURL:   "https://example.com/video.mp4",
		OutputPath: "/tmp/out.mp4",
	})
	if err != nil {
		t.Fatal(err)
	}

	if indexOf(args, "-nostdin") < 0 {
		t.Error("args should start with safety flags")
	}

	if indexOf(args, "-progress") < 0 || indexOf(args, "pipe:1") < 0 {
		t.Error("args should contain -progress pipe:1")
	}
}

func protocolWhitelistValue(t *testing.T, args []string) string {
	t.Helper()
	for i, a := range args {
		if a == "-protocol_whitelist" && i+1 < len(args) {
			return args[i+1]
		}
	}
	t.Fatal("args missing -protocol_whitelist")
	return ""
}

func assertWhitelistContains(t *testing.T, whitelist, needle string) {
	t.Helper()
	if !strings.Contains(whitelist, needle) {
		t.Errorf("whitelist %q should contain %q", whitelist, needle)
	}
}

func assertWhitelistNotContains(t *testing.T, whitelist, needle string) {
	t.Helper()
	if strings.Contains(whitelist, needle) {
		t.Errorf("whitelist %q should NOT contain %q", whitelist, needle)
	}
}

func TestBuildDownloadArgs_HttpsInput_NoFileProtocol(t *testing.T) {
	args, err := BuildDownloadArgs(DownloadOptions{
		InputURL:   "https://example.com/video.mp4",
		OutputPath: "/tmp/out.mp4",
	})
	if err != nil {
		t.Fatal(err)
	}
	wl := protocolWhitelistValue(t, args)
	assertWhitelistContains(t, wl, "http")
	assertWhitelistContains(t, wl, "https")
	assertWhitelistNotContains(t, wl, "file")
}

func TestBuildDownloadArgs_HttpInput_NoFileProtocol(t *testing.T) {
	args, err := BuildDownloadArgs(DownloadOptions{
		InputURL:   "http://example.com/seg.ts",
		OutputPath: "/tmp/out.ts",
	})
	if err != nil {
		t.Fatal(err)
	}
	wl := protocolWhitelistValue(t, args)
	assertWhitelistContains(t, wl, "http")
	assertWhitelistNotContains(t, wl, "file")
}

func TestBuildDownloadArgs_LocalPath_HasFileProtocol(t *testing.T) {
	args, err := BuildDownloadArgs(DownloadOptions{
		InputURL:   "/tmp/local/video.mp4",
		OutputPath: "/tmp/out.mp4",
	})
	if err != nil {
		t.Fatal(err)
	}
	wl := protocolWhitelistValue(t, args)
	assertWhitelistContains(t, wl, "file")
	assertWhitelistContains(t, wl, "http")
	assertWhitelistContains(t, wl, "https")
}

func TestBuildDownloadArgs_RelativePath_HasFileProtocol(t *testing.T) {
	args, err := BuildDownloadArgs(DownloadOptions{
		InputURL:   "./downloads/file.mkv",
		OutputPath: "/tmp/out.mkv",
	})
	if err != nil {
		t.Fatal(err)
	}
	wl := protocolWhitelistValue(t, args)
	assertWhitelistContains(t, wl, "file")
}

func TestBuildDownloadArgs_FilePathScheme_NoFileProtocol(t *testing.T) {
	args, err := BuildDownloadArgs(DownloadOptions{
		InputURL:   "file:///etc/passwd",
		OutputPath: "/tmp/out.bin",
	})
	if err != nil {
		t.Fatal(err)
	}
	wl := protocolWhitelistValue(t, args)
	assertWhitelistNotContains(t, wl, "file")
}

func TestBuildDownloadArgs_FtpScheme_NoFileProtocol(t *testing.T) {
	args, err := BuildDownloadArgs(DownloadOptions{
		InputURL:   "ftp://evil.com/secret",
		OutputPath: "/tmp/out.bin",
	})
	if err != nil {
		t.Fatal(err)
	}
	wl := protocolWhitelistValue(t, args)
	assertWhitelistNotContains(t, wl, "file")
	assertWhitelistNotContains(t, wl, "ftp")
}

func TestProtocolWhitelistFor(t *testing.T) {
	tests := []struct {
		input    string
		wantFile bool
		wantHTTP bool
	}{
		{"https://example.com/v.mp4", false, true},
		{"http://example.com/v.mp4", false, true},
		{"/tmp/local.mp4", true, true},
		{"./local.mp4", true, true},
		{"file:///etc/passwd", false, true},
		{"ftp://evil.com/x", false, true},
		{"rtmp://live/x", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			wl := protocolWhitelistFor(tt.input)
			hasFile := strings.Contains(wl, "file")
			hasHTTP := strings.Contains(wl, "http")
			if hasFile != tt.wantFile {
				t.Errorf("protocolWhitelistFor(%q) has file=%v, want %v (wl=%q)",
					tt.input, hasFile, tt.wantFile, wl)
			}
			if hasHTTP != tt.wantHTTP {
				t.Errorf("protocolWhitelistFor(%q) has http=%v, want %v (wl=%q)",
					tt.input, hasHTTP, tt.wantHTTP, wl)
			}
		})
	}
}

var _ = fmt.Sprintf
