package ffmpeg

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"videodl/internal/media"
)

func TestPreset_GetOriginal(t *testing.T) {
	p, ok := GetPreset(media.ProfileOriginal)
	if !ok {
		t.Fatal("want preset for original profile")
	}
	if !p.StreamCopy {
		t.Error("original profile should be stream copy")
	}
	if p.Name == "" {
		t.Error("preset name should not be empty")
	}
	if indexOf(p.Args, "-c") < 0 || indexOf(p.Args, "copy") < 0 {
		t.Errorf("original profile args should contain -c copy, got %v", p.Args)
	}
}

func TestPreset_GetMP4(t *testing.T) {
	p, ok := GetPreset(media.ProfileMP4)
	if !ok {
		t.Fatal("want preset for mp4-h264-aac profile")
	}
	if p.StreamCopy {
		t.Error("mp4 profile should not be stream copy")
	}
	if indexOf(p.Args, "-c:v") < 0 || indexOf(p.Args, "libx264") < 0 {
		t.Error("mp4 profile should use libx264 video codec")
	}
	if indexOf(p.Args, "-c:a") < 0 || indexOf(p.Args, "aac") < 0 {
		t.Error("mp4 profile should use aac audio codec")
	}
	if indexOf(p.Args, "-pix_fmt") < 0 || indexOf(p.Args, "yuv420p") < 0 {
		t.Error("mp4 profile should use yuv420p pixel format")
	}
}

func TestPreset_DefaultToOriginal(t *testing.T) {
	p, ok := GetPreset("")
	if !ok {
		t.Fatal("empty profile should default to original")
	}
	if !p.StreamCopy {
		t.Error("empty profile should be stream copy")
	}
}

func TestPreset_AllPresets(t *testing.T) {
	presets := AllPresets()
	if len(presets) < 2 {
		t.Errorf("want at least 2 presets, got %d", len(presets))
	}

	foundOriginal := false
	foundMP4 := false
	for _, p := range presets {
		if p.Profile == media.ProfileOriginal {
			foundOriginal = true
			if p.Description == "" {
				t.Error("original preset should have description")
			}
			if p.QualityLossHint == "" {
				t.Error("original preset should have quality loss hint")
			}
			if p.TimeCostHint == "" {
				t.Error("original preset should have time cost hint")
			}
		}
		if p.Profile == media.ProfileMP4 {
			foundMP4 = true
			if p.Description == "" {
				t.Error("mp4 preset should have description")
			}
			if p.QualityLossHint == "" {
				t.Error("mp4 preset should have quality loss hint")
			}
			if p.TimeCostHint == "" {
				t.Error("mp4 preset should have time cost hint")
			}
		}
	}
	if !foundOriginal {
		t.Error("missing original preset")
	}
	if !foundMP4 {
		t.Error("missing mp4 preset")
	}
}

func TestPreset_OriginalArgsUseStreamCopy(t *testing.T) {
	p, _ := GetPreset(media.ProfileOriginal)
	hasCopy := false
	for i := 0; i < len(p.Args)-1; i++ {
		if p.Args[i] == "-c" && p.Args[i+1] == "copy" {
			hasCopy = true
			break
		}
	}
	if !hasCopy {
		t.Errorf("original preset args should contain -c copy, got %v", p.Args)
	}
}

func TestIsStreamCopyProfile(t *testing.T) {
	if !IsStreamCopyProfile(media.ProfileOriginal) {
		t.Error("original should be stream copy")
	}
	if IsStreamCopyProfile(media.ProfileMP4) {
		t.Error("mp4 should NOT be stream copy")
	}
	if !IsStreamCopyProfile("") {
		t.Error("unknown profile should default to stream copy (safe fallback)")
	}
}

func TestPresetOutputExtension(t *testing.T) {
	if ext := PresetOutputExtension(media.ProfileOriginal); ext != ".mp4" {
		t.Errorf("original extension = %q, want .mp4", ext)
	}
	if ext := PresetOutputExtension(media.ProfileMP4); ext != ".mp4" {
		t.Errorf("mp4 extension = %q, want .mp4", ext)
	}
}

func TestBuildTranscodeArgs_Original(t *testing.T) {
	args, err := BuildTranscodeArgs(TranscodeOptions{
		InputPath:  "/tmp/input.mkv",
		OutputPath: "/tmp/output.mp4",
		Profile:    media.ProfileOriginal,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertContainsAll(t, args,
		"-nostdin", "-y", "-progress", "pipe:1",
		"-i", "/tmp/input.mkv",
		"-c", "copy",
		"/tmp/output.mp4",
	)

	assertNotContains(t, args, "-c:v")
	assertNotContains(t, args, "libx264")
	assertNotContains(t, args, "aac")
}

func TestBuildTranscodeArgs_MP4Profile(t *testing.T) {
	args, err := BuildTranscodeArgs(TranscodeOptions{
		InputPath:  "/tmp/input.webm",
		OutputPath: "/tmp/output.mp4",
		Profile:    media.ProfileMP4,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertContainsAll(t, args,
		"-nostdin", "-y", "-progress", "pipe:1",
		"-i", "/tmp/input.webm",
		"-c:v", "libx264",
		"-c:a", "aac",
		"/tmp/output.mp4",
	)

	assertNotContains(t, args, "copy")
	assertNotContains(t, args, "-c")
}

func TestBuildTranscodeArgs_ValidationErrors(t *testing.T) {
	_, err := BuildTranscodeArgs(TranscodeOptions{OutputPath: "/tmp/out"})
	if err == nil || !strings.Contains(err.Error(), "empty input") {
		t.Errorf("want empty input path error, got %v", err)
	}

	_, err = BuildTranscodeArgs(TranscodeOptions{InputPath: "/tmp/in"})
	if err == nil || !strings.Contains(err.Error(), "empty output") {
		t.Errorf("want empty output path error, got %v", err)
	}

	_, err = BuildTranscodeArgs(TranscodeOptions{
		InputPath:  "/tmp/in",
		OutputPath: "/tmp/out",
		Profile:    "nonexistent-profile",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown profile") {
		t.Errorf("want unknown profile error, got %v", err)
	}
}

func TestRunFFmpegTranscode_Success(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "transcoded.mp4")

	script := writeStubScript(t, tmp, "ffmpeg",
		`#!/bin/sh
echo "out_time_us=1000000"
echo "progress=continue"
echo "out_time_us=2000000"
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

	opts := TranscodeOptions{
		InputPath:       "/tmp/source.mkv",
		OutputPath:      outPath,
		Profile:         media.ProfileMP4,
		DurationSeconds: floatPtr(3.0),
	}

	res, err := RunFFmpegTranscode(context.Background(), inv, opts, sink)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}

	mu.Lock()
	defer mu.Unlock()
	if lastProgress.OutTimeUs != 2_000_000 {
		t.Errorf("last OutTimeUs = %d, want 2000000", lastProgress.OutTimeUs)
	}
	if !lastProgress.IsEnd {
		t.Error("want IsEnd=true")
	}
}

func TestRunFFmpegTranscode_NonZeroExit(t *testing.T) {
	tmp := t.TempDir()

	script := writeStubScript(t, tmp, "ffmpeg",
		`#!/bin/sh
echo "[error] encoding failed" >&2
exit 1
`)

	writeStubScript(t, tmp, "ffprobe",
		`#!/bin/sh
exit 0
`)

	inv := NewInvoker(ToolPaths{FFmpeg: script, FFprobe: filepath.Join(tmp, "ffprobe")})

	_, err := RunFFmpegTranscode(context.Background(), inv, TranscodeOptions{
		InputPath:  "/tmp/bad_input.mkv",
		OutputPath: filepath.Join(tmp, "out.mp4"),
		Profile:    media.ProfileMP4,
	}, nil)

	if err == nil {
		t.Fatal("want error for non-zero exit")
	}

	var pe *ProcessError
	if !errors.As(err, &pe) {
		t.Fatalf("want *ProcessError, got %T: %v", err, err)
	}
	if pe.ExitCode() == 0 {
		t.Error("want non-zero exit code")
	}
}

func TestRunFFmpegTranscode_Cancel(t *testing.T) {
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
		_, err := RunFFmpegTranscode(ctx, inv, TranscodeOptions{
			InputPath:  "/tmp/slow.mkv",
			OutputPath: filepath.Join(tmp, "out.mp4"),
			Profile:    media.ProfileMP4,
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
			t.Errorf("want IsCanceled=true, got %v", pe)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunFFmpegTranscode did not return after cancel")
	}
}

func TestRunFFmpegTranscode_NilInvoker(t *testing.T) {
	_, err := RunFFmpegTranscode(context.Background(), nil, TranscodeOptions{
		InputPath:  "/tmp/in",
		OutputPath: "/tmp/out",
	}, nil)
	if err == nil {
		t.Fatal("want error for nil invoker")
	}
}

func TestRunFFmpegTranscode_StreamCopyProfile(t *testing.T) {
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "output.mp4")

	script := writeStubScript(t, tmp, "ffmpeg",
		`#!/bin/sh
echo "progress=end"
touch "`+outPath+`"
exit 0
`)

	writeStubScript(t, tmp, "ffprobe",
		`#!/bin/sh
exit 0
`)

	inv := NewInvoker(ToolPaths{FFmpeg: script, FFprobe: filepath.Join(tmp, "ffprobe")})

	opts := TranscodeOptions{
		InputPath:  "/tmp/source.mp4",
		OutputPath: outPath,
		Profile:    media.ProfileOriginal,
	}

	res, err := RunFFmpegTranscode(context.Background(), inv, opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
}

func TestRunFFmpegTranscode_ArgsContainProgressPipe(t *testing.T) {
	args, err := BuildTranscodeArgs(TranscodeOptions{
		InputPath:  "/tmp/in.mkv",
		OutputPath: "/tmp/out.mp4",
		Profile:    media.ProfileMP4,
	})
	if err != nil {
		t.Fatal(err)
	}

	hasProgressPipe := false
	for i, a := range args {
		if a == "-progress" && i+1 < len(args) && args[i+1] == "pipe:1" {
			hasProgressPipe = true
			break
		}
	}
	if !hasProgressPipe {
		t.Errorf("args missing `-progress pipe:1`: %v", args)
	}
}

func TestBuildTranscodeArgs_NoShellInjection(t *testing.T) {
	args, err := BuildTranscodeArgs(TranscodeOptions{
		InputPath:  "/tmp/input.mkv",
		OutputPath: "/tmp/output.mp4",
		Profile:    media.ProfileMP4,
	})
	if err != nil {
		t.Fatal(err)
	}

	if indexOf(args, "-nostdin") < 0 {
		t.Error("args should start with safety flags")
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
