package ytdlp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const defaultOutputLimit int64 = 16 * 1024 * 1024

var (
	ErrOutputTooLarge = errors.New("yt-dlp: process output exceeds configured limit")
	ErrInvalidRequest = errors.New("yt-dlp: invalid request")
)

type Runner struct {
	cfg RunConfig
}

func NewRunner(cfg RunConfig) *Runner {
	if cfg.MaxStdoutBytes <= 0 {
		cfg.MaxStdoutBytes = defaultOutputLimit
	}
	if cfg.MaxStderrBytes <= 0 {
		cfg.MaxStderrBytes = defaultOutputLimit
	}
	return &Runner{cfg: cfg}
}

type ProcessError struct {
	ExitCodeVal int
	err         error
	timeout     bool
	canceled    bool
	outputBig   bool
}

func (e *ProcessError) Error() string {
	if e == nil {
		return ""
	}
	switch {
	case e.timeout:
		return "yt-dlp: process timed out"
	case e.canceled:
		return "yt-dlp: process canceled"
	case e.outputBig:
		return "yt-dlp: process output too large"
	case e.err != nil:
		return fmt.Sprintf("yt-dlp: process failed: %v", e.err)
	default:
		return fmt.Sprintf("yt-dlp: process exited with code %d", e.ExitCodeVal)
	}
}

func (e *ProcessError) Unwrap() error { return e.err }
func (e *ProcessError) ExitCode() int {
	if e == nil {
		return -1
	}
	return e.ExitCodeVal
}
func (e *ProcessError) IsTimeout() bool        { return e != nil && e.timeout }
func (e *ProcessError) IsCanceled() bool       { return e != nil && e.canceled }
func (e *ProcessError) IsOutputTooLarge() bool { return e != nil && e.outputBig }

func (r *Runner) Run(ctx context.Context, args []string) (RunResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.cfg.BinaryPath == "" {
		return RunResult{}, &ProcessError{ExitCodeVal: -1, err: ErrInvalidRequest}
	}

	cmd := exec.CommandContext(ctx, r.cfg.BinaryPath, args...)
	cmd.Env = buildEnv(r.cfg.Env)
	cmd.SysProcAttr = newYTDLPSysProcAttr()
	stdout := &limitedBuffer{limit: r.cfg.MaxStdoutBytes}
	stderr := &limitedBuffer{limit: r.cfg.MaxStderrBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		pe := &ProcessError{ExitCodeVal: -1, err: err}
		if ctx.Err() != nil {
			pe.err = ctx.Err()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				pe.timeout = true
			} else {
				pe.canceled = true
			}
		}
		return RunResult{}, pe
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	var waitErr error
	select {
	case waitErr = <-waitCh:
	case <-ctx.Done():
		killYTDLPProcess(cmd)
		waitErr = <-waitCh
	}
	result := RunResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: 0}
	if stdout.wasLimited || stderr.wasLimited {
		return RunResult{}, &ProcessError{ExitCodeVal: -1, outputBig: true, err: ErrOutputTooLarge}
	}
	if ctx.Err() != nil {
		pe := &ProcessError{ExitCodeVal: -1, err: ctx.Err()}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			pe.timeout = true
		} else {
			pe.canceled = true
		}
		return RunResult{}, pe
	}
	if waitErr != nil {
		pe := &ProcessError{ExitCodeVal: -1}
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			pe.ExitCodeVal = exitErr.ExitCode()
		} else {
			pe.err = waitErr
		}
		return RunResult{}, pe
	}
	return result, nil
}

func (r *Runner) Version(ctx context.Context) (string, error) {
	result, err := r.Run(ctx, []string{"--version"})
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(result.Stdout), "\n") {
		if version := strings.TrimSpace(line); version != "" {
			return version, nil
		}
	}
	return "", fmt.Errorf("%w: empty version output", ErrInvalidRequest)
}

func buildEnv(extra []string) []string {
	values := make(map[string]string)
	order := make([]string, 0)
	for _, item := range append(os.Environ(), extra...) {
		key, value, ok := strings.Cut(item, "=")
		if !ok || key == "" {
			continue
		}
		if _, exists := values[key]; !exists {
			order = append(order, key)
		}
		values[key] = value
	}
	if _, exists := values["YTDLP_NO_PLUGINS"]; !exists {
		order = append(order, "YTDLP_NO_PLUGINS")
	}
	values["YTDLP_NO_PLUGINS"] = "1"
	result := make([]string, 0, len(order))
	for _, key := range order {
		result = append(result, key+"="+values[key])
	}
	return result
}

type limitedBuffer struct {
	buffer     bytes.Buffer
	limit      int64
	written    int64
	wasLimited bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.wasLimited {
		return len(p), nil
	}
	remaining := b.limit - b.written
	if remaining <= 0 {
		b.wasLimited = true
		return len(p), nil
	}
	if int64(len(p)) > remaining {
		_, _ = b.buffer.Write(p[:remaining])
		b.written += remaining
		b.wasLimited = true
		return len(p), nil
	}
	_, _ = b.buffer.Write(p)
	b.written += int64(len(p))
	return len(p), nil
}

func (b *limitedBuffer) Bytes() []byte { return b.buffer.Bytes() }
