package ytdlp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunnerRun_ArgumentArrayAndFixedEnvironment(t *testing.T) {
	skipShellTestOnWindows(t)
	tmp := t.TempDir()
	argsFile := filepath.Join(tmp, "args")
	script := writeStub(t, `#!/bin/sh
printf '%s\n' "$YTDLP_NO_PLUGINS" > "$YTDLP_TEST_ARGS_FILE"
printf '%s\n' "$@" >> "$YTDLP_TEST_ARGS_FILE"
printf '%s' 'structured-output'`)
	runner := NewRunner(RunConfig{
		BinaryPath: script,
		Env:        []string{"YTDLP_TEST_ARGS_FILE=" + argsFile},
	})
	result, err := runner.Run(context.Background(), []string{"--format", `v&audio --flag`, "https://example.test/watch?v=1"})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Stdout) != "structured-output" {
		t.Fatalf("stdout = %q", result.Stdout)
	}
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	want := []string{"1", "--format", "v&audio --flag", "https://example.test/watch?v=1"}
	if len(lines) != len(want) {
		t.Fatalf("argv = %#v, want %#v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("argv[%d] = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestRunnerRun_BoundsStdoutAndStderr(t *testing.T) {
	skipShellTestOnWindows(t)
	script := writeStub(t, `#!/bin/sh
printf '1234567890'
printf 'abcdefghij' >&2`)
	runner := NewRunner(RunConfig{BinaryPath: script, MaxStdoutBytes: 4, MaxStderrBytes: 3})
	_, err := runner.Run(context.Background(), nil)
	var processErr *ProcessError
	if !errors.As(err, &processErr) || !processErr.IsOutputTooLarge() {
		t.Fatalf("expected bounded output error, got %v", err)
	}
}

func TestRunnerRun_NonZeroExitDoesNotLeakSensitiveOutput(t *testing.T) {
	skipShellTestOnWindows(t)
	script := writeStub(t, `#!/bin/sh
echo 'https://example.test/watch?token=secret' >&2
exit 7`)
	_, err := NewRunner(RunConfig{BinaryPath: script}).Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected process error")
	}
	if strings.Contains(err.Error(), "token=secret") || strings.Contains(err.Error(), "example.test") {
		t.Fatalf("sensitive URL leaked in error: %v", err)
	}
	var processErr *ProcessError
	if !errors.As(err, &processErr) || processErr.ExitCode() != 7 {
		t.Fatalf("unexpected process error: %#v", err)
	}
}

func TestRunnerRun_CancelAndTimeout(t *testing.T) {
	skipShellTestOnWindows(t)
	script := writeStub(t, `#!/bin/sh
sleep 2`)
	for name, ctx := range map[string]context.Context{
		"cancel": func() context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		}(),
		"timeout": func() context.Context {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			t.Cleanup(cancel)
			return ctx
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewRunner(RunConfig{BinaryPath: script}).Run(ctx, nil)
			if err == nil {
				t.Fatal("expected context error")
			}
			var processErr *ProcessError
			if !errors.As(err, &processErr) {
				t.Fatalf("expected ProcessError, got %T: %v", err, err)
			}
			if name == "cancel" && !processErr.IsCanceled() {
				t.Fatalf("expected canceled process: %v", err)
			}
			if name == "timeout" && !processErr.IsTimeout() {
				t.Fatalf("expected timeout process: %v", err)
			}
		})
	}
}

func TestRunnerVersion(t *testing.T) {
	skipShellTestOnWindows(t)
	script := writeStub(t, `#!/bin/sh
if [ "$1" = "--version" ]; then printf '%s\n' '2026.08.19'; else printf '%s' '{}'; fi`)
	version, err := NewRunner(RunConfig{BinaryPath: script}).Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if version != "2026.08.19" {
		t.Fatalf("version = %q", version)
	}
}

func writeStub(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "yt-dlp-stub")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func skipShellTestOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is only used on Unix hosts")
	}
}
