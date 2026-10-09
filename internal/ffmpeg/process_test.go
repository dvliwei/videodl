package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writeStubScript(t *testing.T, dir, name, script string, args ...string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	s := script
	if runtime.GOOS == "windows" {
		s = "@echo off\r\n" + strings.ReplaceAll(script, "\n", "\r\n")
	}
	if err := os.WriteFile(path, []byte(s), 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestProcess_RunSuccess(t *testing.T) {
	tmp := t.TempDir()
	script := writeStubScript(t, tmp, "echo_args.sh",
		`#!/bin/sh
echo "hello stdout"
echo "hello stderr" >&2
exit 0
`)

	p := NewProcess(script, nil, ProcessConfig{})
	res, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", res.ExitCode)
	}
	if !strings.Contains(string(res.Stdout), "hello stdout") {
		t.Errorf("stdout = %q, want %q", string(res.Stdout), "hello stdout")
	}
	if !strings.Contains(string(res.Stderr), "hello stderr") {
		t.Errorf("stderr = %q, want %q", string(res.Stderr), "hello stderr")
	}
}

func TestProcess_NonZeroExit(t *testing.T) {
	tmp := t.TempDir()
	script := writeStubScript(t, tmp, "fail.sh",
		`#!/bin/sh
echo "some output" >&2
exit 7
`)

	p := NewProcess(script, nil, ProcessConfig{})
	_, err := p.Run(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	var pe *ProcessError
	if !errors.As(err, &pe) {
		t.Fatalf("want *ProcessError, got %T: %v", err, err)
	}
	if pe.ExitCode() != 7 {
		t.Errorf("exit code = %d, want 7", pe.ExitCode())
	}
	if pe.Cmd != script {
		t.Errorf("Cmd = %q, want %q", pe.Cmd, script)
	}
	if pe.IsTimeout() {
		t.Error("should not be marked timeout")
	}
	if pe.IsCanceled() {
		t.Error("should not be marked canceled")
	}
}

func TestProcess_Timeout(t *testing.T) {
	tmp := t.TempDir()
	script := writeStubScript(t, tmp, "slow.sh",
		`#!/bin/sh
sleep 2
exit 0
`)

	p := NewProcess(script, nil, ProcessConfig{})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := p.Run(ctx)
	if err == nil {
		t.Fatal("expected error")
	}

	var pe *ProcessError
	if !errors.As(err, &pe) {
		t.Fatalf("want *ProcessError, got %T", err)
	}
	if !pe.IsTimeout() {
		t.Errorf("want IsTimeout=true, got false. err=%v", err)
	}
}

func TestProcess_Cancel(t *testing.T) {
	tmp := t.TempDir()
	script := writeStubScript(t, tmp, "sleepy.sh",
		`#!/bin/sh
sleep 5
exit 0
`)

	p := NewProcess(script, nil, ProcessConfig{})
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := p.Run(ctx)
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
		t.Fatal("Run did not return after cancel")
	}
}

func TestProcess_OutputTooLarge(t *testing.T) {
	tmp := t.TempDir()
	script := writeStubScript(t, tmp, "bigout.sh",
		`#!/bin/sh
head -c 200 /dev/zero | tr '\0' 'x'
exit 0
`)

	p := NewProcess(script, nil, ProcessConfig{
		MaxStdoutBytes: 100,
	})
	_, err := p.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for oversized output")
	}

	var pe *ProcessError
	if !errors.As(err, &pe) {
		t.Fatalf("want *ProcessError, got %T", err)
	}
	if !pe.IsOutputTooLarge() {
		t.Errorf("want IsOutputTooLarge=true")
	}
	if !errors.Is(pe.Err, ErrOutputTooLarge) {
		t.Errorf("want wrapped ErrOutputTooLarge")
	}
}

func TestProcess_StderrTooLarge(t *testing.T) {
	tmp := t.TempDir()
	script := writeStubScript(t, tmp, "biggerr.sh",
		`#!/bin/sh
head -c 200 /dev/zero | tr '\0' 'y' >&2
exit 0
`)

	p := NewProcess(script, nil, ProcessConfig{
		MaxStderrBytes: 50,
	})
	_, err := p.Run(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	var pe *ProcessError
	if !errors.As(err, &pe) {
		t.Fatalf("want *ProcessError, got %T", err)
	}
	if !pe.IsOutputTooLarge() {
		t.Errorf("want IsOutputTooLarge=true")
	}
}

func TestProcess_ExecutableNotFound(t *testing.T) {
	p := NewProcess("/nonexistent/bin", nil, ProcessConfig{})
	_, err := p.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for missing binary")
	}

	var pe *ProcessError
	if !errors.As(err, &pe) {
		t.Fatalf("want *ProcessError, got %T", err)
	}
	if pe.ExitCode() != -1 {
		t.Errorf("want exit code -1, got %d", pe.ExitCode())
	}
	if pe.Err == nil {
		t.Error("want underlying Err set")
	}
}

func TestProcess_EmptyBinPath(t *testing.T) {
	p := NewProcess("", nil, ProcessConfig{})
	_, err := p.Run(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	var pe *ProcessError
	if !errors.As(err, &pe) {
		t.Fatalf("want *ProcessError, got %T", err)
	}
	if pe.ExitCode() != -1 {
		t.Errorf("want exit code -1, got %d", pe.ExitCode())
	}
}

func TestProcess_SeparateStdoutStderr(t *testing.T) {
	tmp := t.TempDir()
	script := writeStubScript(t, tmp, "sep.sh",
		`#!/bin/sh
echo "only stdout"
echo "only stderr" >&2
exit 0
`)

	p := NewProcess(script, nil, ProcessConfig{})
	res, err := p.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(res.Stdout), "only stdout") {
		t.Errorf("stdout missing its content: %q", res.Stdout)
	}
	if strings.Contains(string(res.Stdout), "only stderr") {
		t.Errorf("stdout should not contain stderr")
	}
	if !strings.Contains(string(res.Stderr), "only stderr") {
		t.Errorf("stderr missing its content: %q", res.Stderr)
	}
}

func TestProcess_NoShellInvocation(t *testing.T) {
	tmp := t.TempDir()
	script := writeStubScript(t, tmp, "noop.sh",
		`#!/bin/sh
exit 0
`)

	p := NewProcess(script, []string{"$HOME", "`pwd`", "$(date)"}, ProcessConfig{})
	_, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var pe *ProcessError
	if errors.As(err, &pe) {
		t.Fatalf("no ProcessError expected: %v", err)
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("where", "cmd.exe")
		_ = cmd.Run()
	} else {
		cmd := exec.Command("ps", "-o", "comm=")
		out, _ := cmd.Output()
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "/bin/sh" || line == "sh" || line == "bash" {
			}
		}
	}
}

func TestProcess_ArgsPassedAsArray(t *testing.T) {
	tmp := t.TempDir()
	script := writeStubScript(t, tmp, "echo_args.sh",
		`#!/bin/sh
printf '%s\n' "$@"
exit 0
`)

	p := NewProcess(script, []string{"one", "two with spaces", "three"}, ProcessConfig{})
	res, err := p.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(res.Stdout)), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "one" || lines[1] != "two with spaces" || lines[2] != "three" {
		t.Errorf("args not preserved: %v", lines)
	}
}

func TestProcess_WorkingDirectory(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	script := writeStubScript(t, tmp, "pwd.sh",
		`#!/bin/sh
pwd
exit 0
`)

	p := NewProcess(script, nil, ProcessConfig{WorkDir: sub})
	res, err := p.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(res.Stdout))
	want, _ := filepath.EvalSymlinks(sub)
	got, _ = filepath.EvalSymlinks(got)
	if got != want {
		t.Errorf("pwd = %q, want %q", got, want)
	}
}

func TestProcessError_ErrorMethod(t *testing.T) {
	tests := []struct {
		name string
		pe   *ProcessError
		want string
	}{
		{
			name: "timeout message",
			pe: &ProcessError{
				Cmd:         "/x/ffmpeg",
				Args:        []string{"-i", "url"},
				ExitCodeVal: -1,
				isTimeout:   true,
			},
			want: "timed out",
		},
		{
			name: "canceled message",
			pe: &ProcessError{
				Cmd:         "/x/ffmpeg",
				ExitCodeVal: -1,
				isCanceled:  true,
			},
			want: "canceled",
		},
		{
			name: "output too large",
			pe: &ProcessError{
				Cmd:         "/x/ffmpeg",
				ExitCodeVal: -1,
				isOutputBig: true,
				Err:         ErrOutputTooLarge,
			},
			want: "too large",
		},
		{
			name: "non-zero exit",
			pe: &ProcessError{
				Cmd:         "/x/ffmpeg",
				Args:        []string{"-i", "bad"},
				ExitCodeVal: 1,
				StderrTail:  "some stderr",
			},
			want: "exited with code 1",
		},
		{
			name: "start error",
			pe: &ProcessError{
				Cmd:         "/x/ffmpeg",
				ExitCodeVal: -1,
				Err:         fmt.Errorf("no such file"),
			},
			want: "failed to start",
		},
		{
			name: "nil receiver",
			pe:   nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.pe.Error()
			if !strings.Contains(msg, tt.want) {
				t.Errorf("error message %q does not contain %q", msg, tt.want)
			}
		})
	}
}

func TestProcessError_ExitCodeAccessor(t *testing.T) {
	var nilPE *ProcessError
	if nilPE.ExitCode() != -1 {
		t.Errorf("nil ProcessError ExitCode() = %d, want -1", nilPE.ExitCode())
	}

	pe := &ProcessError{ExitCodeVal: 42}
	if pe.ExitCode() != 42 {
		t.Errorf("ExitCode() = %d, want 42", pe.ExitCode())
	}
}

func TestProcessError_Flags(t *testing.T) {
	flags := []struct {
		set  func(pe *ProcessError)
		get  func(pe *ProcessError) bool
		name string
	}{
		{func(pe *ProcessError) { pe.isTimeout = true }, func(pe *ProcessError) bool { return pe.IsTimeout() }, "IsTimeout"},
		{func(pe *ProcessError) { pe.isCanceled = true }, func(pe *ProcessError) bool { return pe.IsCanceled() }, "IsCanceled"},
		{func(pe *ProcessError) { pe.isOutputBig = true }, func(pe *ProcessError) bool { return pe.IsOutputTooLarge() }, "IsOutputTooLarge"},
	}

	var nilPE *ProcessError
	for _, f := range flags {
		if f.get(nilPE) {
			t.Errorf("nil ProcessError %s should be false", f.name)
		}
	}

	for _, f := range flags {
		pe := &ProcessError{}
		f.set(pe)
		if !f.get(pe) {
			t.Errorf("want %s=true after set", f.name)
		}
	}
}

func TestLimitedBuffer_Truncates(t *testing.T) {
	lb := &limitedBuffer{limit: 10}
	n, _ := lb.Write([]byte("abcdefghijklmnop"))
	if n != 16 {
		t.Errorf("Write returned %d, want 16", n)
	}
	if string(lb.Bytes()) != "abcdefghij" {
		t.Errorf("buffer = %q, want %q", lb.Bytes(), "abcdefghij")
	}
	if !lb.wasLimited {
		t.Error("wasLimited should be true")
	}

	lb.Write([]byte("more"))
	if string(lb.Bytes()) != "abcdefghij" {
		t.Error("additional writes should not grow buffer after limit")
	}
}

func TestLimitedBuffer_UnderLimit(t *testing.T) {
	lb := &limitedBuffer{limit: 100}
	lb.Write([]byte("hello"))
	lb.Write([]byte(" "))
	lb.Write([]byte("world"))
	if string(lb.Bytes()) != "hello world" {
		t.Errorf("buffer = %q, want %q", lb.Bytes(), "hello world")
	}
	if lb.wasLimited {
		t.Error("wasLimited should be false")
	}
}

func TestLimitedBuffer_ZeroLimit(t *testing.T) {
	lb := &limitedBuffer{limit: 0}
	lb.Write([]byte("anything"))
	if !lb.wasLimited {
		t.Error("limit 0 should immediately trigger wasLimited")
	}
	if len(lb.Bytes()) != 0 {
		t.Error("limit 0 should not buffer anything")
	}
}

func TestInvoker_RunFFmpegSuccess(t *testing.T) {
	tmp := t.TempDir()
	script := writeStubScript(t, tmp, "ffmpeg",
		`#!/bin/sh
echo "ffmpeg version 8.0.3"
exit 0
`)
	writeStubScript(t, tmp, "ffprobe",
		`#!/bin/sh
echo "ffprobe version 8.0.3"
exit 0
`)

	inv := NewInvoker(ToolPaths{FFmpeg: script, FFprobe: filepath.Join(tmp, "ffprobe")})
	res, err := inv.RunFFmpeg(context.Background(), []string{"-version"})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit code = %d", res.ExitCode)
	}
}

func TestInvoker_RunFFprobeSuccess(t *testing.T) {
	tmp := t.TempDir()
	writeStubScript(t, tmp, "ffmpeg",
		`#!/bin/sh
exit 0
`)
	script := writeStubScript(t, tmp, "ffprobe",
		`#!/bin/sh
echo "ffprobe version 8.0.3"
exit 0
`)

	inv := NewInvoker(ToolPaths{FFmpeg: filepath.Join(tmp, "ffmpeg"), FFprobe: script})
	res, err := inv.RunFFprobe(context.Background(), []string{"-version"})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit code = %d", res.ExitCode)
	}
}

func TestInvoker_VerifyFFmpegVersion(t *testing.T) {
	tmp := t.TempDir()
	writeStubScript(t, tmp, "ffprobe",
		`#!/bin/sh
exit 0
`)
	script := writeStubScript(t, tmp, "ffmpeg",
		`#!/bin/sh
echo "ffmpeg version 8.0.3 Copyright (c) 2000-2026 the FFmpeg developers"
exit 0
`)

	inv := NewInvoker(ToolPaths{FFmpeg: script, FFprobe: filepath.Join(tmp, "ffprobe")})
	if err := inv.VerifyFFmpegVersion(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInvoker_VerifyWrongVersion(t *testing.T) {
	tmp := t.TempDir()
	writeStubScript(t, tmp, "ffprobe",
		`#!/bin/sh
exit 0
`)
	script := writeStubScript(t, tmp, "ffmpeg",
		`#!/bin/sh
echo "ffmpeg version 7.1.4 Copyright"
exit 0
`)

	inv := NewInvoker(ToolPaths{FFmpeg: script, FFprobe: filepath.Join(tmp, "ffprobe")})
	err := inv.VerifyFFmpegVersion(context.Background())
	if err == nil {
		t.Fatal("expected version mismatch error")
	}
}

func TestInvoker_EmptyPath(t *testing.T) {
	inv := NewInvoker(ToolPaths{FFmpeg: "", FFprobe: ""})

	_, err := inv.RunFFmpeg(context.Background(), nil)
	if !errors.Is(err, ErrFFmpegNotFound) {
		t.Errorf("RunFFmpeg: want ErrFFmpegNotFound, got %v", err)
	}

	_, err = inv.RunFFprobe(context.Background(), nil)
	if !errors.Is(err, ErrFFprobeNotFound) {
		t.Errorf("RunFFprobe: want ErrFFprobeNotFound, got %v", err)
	}

	err = inv.VerifyFFmpegVersion(context.Background())
	if !errors.Is(err, ErrFFmpegNotFound) {
		t.Errorf("VerifyFFmpegVersion: want ErrFFmpegNotFound, got %v", err)
	}

	err = inv.VerifyFFprobeVersion(context.Background())
	if !errors.Is(err, ErrFFprobeNotFound) {
		t.Errorf("VerifyFFprobeVersion: want ErrFFprobeNotFound, got %v", err)
	}
}

func TestResolvedInvoker_FullFlow(t *testing.T) {
	dirName := platformDirName(runtime.GOOS, runtime.GOARCH)
	if dirName == "" {
		t.Skip("platform not in target matrix")
	}

	tmp := t.TempDir()
	toolsDir := filepath.Join(tmp, "tools", dirName)
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	script := `#!/bin/sh
echo "ffmpeg version 8.0.3 Copyright (c) 2000-2026"
exit 0
`
	if runtime.GOOS == "windows" {
		script = `@echo off\r\n` + `echo ffmpeg version 8.0.3 Copyright (c) 2000-2026\r\n` + `exit /b 0\r\n`
	}

	ffPath := filepath.Join(toolsDir, CurrentPlatform().FFmpegBin)
	fpPath := filepath.Join(toolsDir, CurrentPlatform().FFprobeBin)
	if err := os.WriteFile(ffPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fpPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	paths, err := ResolvePaths(tmp)
	if err != nil {
		t.Fatalf("ResolvePaths: %v", err)
	}

	inv := NewInvoker(paths)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := inv.VerifyFFmpegVersion(ctx); err != nil {
		t.Errorf("VerifyFFmpegVersion: %v", err)
	}
	if err := inv.VerifyFFprobeVersion(ctx); err != nil {
		t.Errorf("VerifyFFprobeVersion: %v", err)
	}
}

func TestProcessConfig_Defaults(t *testing.T) {
	c := ProcessConfig{}
	c = c.withDefaults()
	if c.MaxStdoutBytes != DefaultOutputLimit {
		t.Errorf("MaxStdoutBytes = %d, want %d", c.MaxStdoutBytes, DefaultOutputLimit)
	}
	if c.MaxStderrBytes != DefaultOutputLimit {
		t.Errorf("MaxStderrBytes = %d, want %d", c.MaxStderrBytes, DefaultOutputLimit)
	}
}

func TestProcessConfig_CustomValues(t *testing.T) {
	c := ProcessConfig{MaxStdoutBytes: 1024, MaxStderrBytes: 2048}
	c = c.withDefaults()
	if c.MaxStdoutBytes != 1024 {
		t.Errorf("MaxStdoutBytes should not be overwritten")
	}
	if c.MaxStderrBytes != 2048 {
		t.Errorf("MaxStderrBytes should not be overwritten")
	}
}

func TestProcessError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("inner")
	pe := &ProcessError{Err: inner}
	if pe.Unwrap() != inner {
		t.Error("Unwrap() should return inner Err")
	}
}

func TestTailOf(t *testing.T) {
	longInput := strings.Repeat("x", 10000)
	got := tailOf(longInput)
	if len(got) != 4096 {
		t.Errorf("tailOf long input length = %d, want 4096", len(got))
	}
	if got != longInput[len(longInput)-4096:] {
		t.Error("tailOf should keep the end of string")
	}

	short := "hello"
	if tailOf(short) != short {
		t.Error("tailOf short input should be unchanged")
	}
}

func TestProcess_MultipleArgs(t *testing.T) {
	tmp := t.TempDir()
	script := writeStubScript(t, tmp, "args.sh",
		`#!/bin/sh
echo "first=$1"
echo "second=$2"
echo "third=$3"
exit 0
`)

	p := NewProcess(script, []string{"alpha", "beta gamma", "delta"}, ProcessConfig{})
	res, err := p.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	out := string(res.Stdout)
	if !strings.Contains(out, "first=alpha") {
		t.Error("first arg wrong")
	}
	if !strings.Contains(out, "second=beta gamma") {
		t.Error("second arg with spaces wrong")
	}
	if !strings.Contains(out, "third=delta") {
		t.Error("third arg wrong")
	}
}
