package ffmpeg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

const DefaultOutputLimit = 16 * 1024 * 1024

type ProcessConfig struct {
	MaxStdoutBytes int64
	MaxStderrBytes int64
	WorkDir        string
	Env            []string
}

func (c ProcessConfig) withDefaults() ProcessConfig {
	if c.MaxStdoutBytes == 0 {
		c.MaxStdoutBytes = DefaultOutputLimit
	}
	if c.MaxStderrBytes == 0 {
		c.MaxStderrBytes = DefaultOutputLimit
	}
	return c
}

type Process struct {
	binPath string
	args    []string
	cfg     ProcessConfig
	cmd     *exec.Cmd
}

type RunResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

func NewProcess(binPath string, args []string, cfg ProcessConfig) *Process {
	return &Process{
		binPath: binPath,
		args:    args,
		cfg:     cfg.withDefaults(),
	}
}

func (p *Process) Run(ctx context.Context) (*RunResult, error) {
	if p.binPath == "" {
		return nil, &ProcessError{
			Cmd:         p.binPath,
			Args:        p.args,
			ExitCodeVal: -1,
			Err:         fmt.Errorf("ffmpeg: empty binary path"),
		}
	}

	if ctx == nil {
		ctx = context.Background()
	}

	cmd := exec.CommandContext(ctx, p.binPath, p.args...)
	if p.cfg.WorkDir != "" {
		cmd.Dir = p.cfg.WorkDir
	}
	if p.cfg.Env != nil {
		cmd.Env = p.cfg.Env
	}

	cmd.SysProcAttr = newSysProcAttr()

	stdoutBuf := &limitedBuffer{limit: p.cfg.MaxStdoutBytes}
	stderrBuf := &limitedBuffer{limit: p.cfg.MaxStderrBytes}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	p.cmd = cmd

	if err := cmd.Start(); err != nil {
		pe := &ProcessError{
			Cmd:         p.binPath,
			Args:        p.args,
			ExitCodeVal: -1,
			Err:         err,
		}
		return nil, pe
	}

	execErr := cmd.Wait()

	stdoutBytes := stdoutBuf.Bytes()
	stderrBytes := stderrBuf.Bytes()

	if stdoutBuf.wasLimited || stderrBuf.wasLimited {
		pe := &ProcessError{
			Cmd:         p.binPath,
			Args:        p.args,
			StdoutTail:  tailOf(string(stdoutBytes)),
			StderrTail:  tailOf(string(stderrBytes)),
			ExitCodeVal: -1,
			isOutputBig: true,
			Err:         ErrOutputTooLarge,
		}
		return nil, pe
	}

	if ctx.Err() != nil {
		pe := classifyError(p.binPath, p.args, string(stdoutBytes), string(stderrBytes), ctx.Err())
		if execErr != nil {
			var exitErr *exec.ExitError
			if errors.As(execErr, &exitErr) {
				pe.ExitCodeVal = exitErr.ExitCode()
			}
		}
		return nil, pe
	}

	if execErr != nil {
		pe := classifyError(p.binPath, p.args, string(stdoutBytes), string(stderrBytes), execErr)
		return nil, pe
	}

	return &RunResult{
		Stdout:   stdoutBytes,
		Stderr:   stderrBytes,
		ExitCode: 0,
	}, nil
}

type Invoker struct {
	paths ToolPaths
}

func NewInvoker(paths ToolPaths) *Invoker {
	return &Invoker{paths: paths}
}

func (inv *Invoker) Paths() ToolPaths { return inv.paths }

func (inv *Invoker) RunFFmpeg(ctx context.Context, args []string) (*RunResult, error) {
	if inv.paths.FFmpeg == "" {
		return nil, fmt.Errorf("%w: FFmpeg path not configured", ErrFFmpegNotFound)
	}
	return NewProcess(inv.paths.FFmpeg, args, ProcessConfig{}).Run(ctx)
}

func (inv *Invoker) RunFFmpegWith(ctx context.Context, args []string, cfg ProcessConfig) (*RunResult, error) {
	if inv.paths.FFmpeg == "" {
		return nil, fmt.Errorf("%w: FFmpeg path not configured", ErrFFmpegNotFound)
	}
	return NewProcess(inv.paths.FFmpeg, args, cfg).Run(ctx)
}

func (inv *Invoker) RunFFprobe(ctx context.Context, args []string) (*RunResult, error) {
	if inv.paths.FFprobe == "" {
		return nil, fmt.Errorf("%w: FFprobe path not configured", ErrFFprobeNotFound)
	}
	return NewProcess(inv.paths.FFprobe, args, ProcessConfig{}).Run(ctx)
}

func (inv *Invoker) RunFFprobeWith(ctx context.Context, args []string, cfg ProcessConfig) (*RunResult, error) {
	if inv.paths.FFprobe == "" {
		return nil, fmt.Errorf("%w: FFprobe path not configured", ErrFFprobeNotFound)
	}
	return NewProcess(inv.paths.FFprobe, args, cfg).Run(ctx)
}

func (inv *Invoker) VerifyFFmpegVersion(ctx context.Context) error {
	res, err := inv.RunFFmpeg(ctx, []string{"-version"})
	if err != nil {
		return err
	}
	return ValidateVersion(string(res.Stdout))
}

func (inv *Invoker) VerifyFFprobeVersion(ctx context.Context) error {
	res, err := inv.RunFFprobe(ctx, []string{"-version"})
	if err != nil {
		return err
	}
	return ValidateVersion(string(res.Stdout))
}

type limitedBuffer struct {
	buf        bytes.Buffer
	limit      int64
	written    int64
	wasLimited bool
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if l.wasLimited {
		return len(p), nil
	}

	remaining := l.limit - l.written
	if remaining <= 0 {
		l.wasLimited = true
		return len(p), nil
	}

	if int64(len(p)) <= remaining {
		n, _ := l.buf.Write(p)
		l.written += int64(n)
		return len(p), nil
	}

	n, _ := l.buf.Write(p[:remaining])
	l.written += int64(n)
	l.wasLimited = true
	return len(p), nil
}

func (l *limitedBuffer) Bytes() []byte  { return l.buf.Bytes() }
func (l *limitedBuffer) String() string { return l.buf.String() }

var _ io.Writer = (*limitedBuffer)(nil)
