package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var (
	ErrOutputTooLarge = fmt.Errorf("ffmpeg: process output exceeds configured limit")
	ErrNoShellAllowed = fmt.Errorf("ffmpeg: commands must not run via a shell")
	ErrCommandFailed  = fmt.Errorf("ffmpeg: command exited with non-zero code")
)

type ProcessError struct {
	Cmd         string
	Args        []string
	ExitCodeVal int
	StdoutTail  string
	StderrTail  string
	Err         error
	isTimeout   bool
	isCanceled  bool
	isOutputBig bool
}

func (e *ProcessError) Error() string {
	if e == nil {
		return ""
	}

	if e.isTimeout {
		return fmt.Sprintf("ffmpeg: command timed out: %s %s", e.Cmd, strings.Join(e.Args, " "))
	}
	if e.isCanceled {
		return fmt.Sprintf("ffmpeg: command canceled: %s %s", e.Cmd, strings.Join(e.Args, " "))
	}
	if e.isOutputBig {
		return fmt.Sprintf("ffmpeg: output too large for %s", e.Cmd)
	}
	if e.Err != nil {
		return fmt.Sprintf("ffmpeg: failed to start %s: %v", e.Cmd, e.Err)
	}
	return fmt.Sprintf("ffmpeg: %s %s exited with code %d: %s",
		e.Cmd, strings.Join(e.Args, " "), e.ExitCodeVal, strings.TrimSpace(e.StderrTail))
}

func (e *ProcessError) ExitCode() int {
	if e == nil {
		return -1
	}
	return e.ExitCodeVal
}

func (e *ProcessError) IsTimeout() bool  { return e != nil && e.isTimeout }
func (e *ProcessError) IsCanceled() bool { return e != nil && e.isCanceled }
func (e *ProcessError) IsOutputTooLarge() bool {
	return e != nil && e.isOutputBig
}

func (e *ProcessError) Unwrap() error { return e.Err }

func tailOf(s string) string {
	const max = 4096
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}

func classifyError(cmd string, args []string, stdout, stderr string, execErr error) *ProcessError {
	pe := &ProcessError{
		Cmd:         cmd,
		Args:        args,
		StdoutTail:  tailOf(stdout),
		StderrTail:  tailOf(stderr),
		ExitCodeVal: -1,
	}

	if execErr == nil {
		return pe
	}

	if errors.Is(execErr, context.DeadlineExceeded) {
		pe.isTimeout = true
		pe.Err = execErr
		return pe
	}
	if errors.Is(execErr, context.Canceled) {
		pe.isCanceled = true
		pe.Err = execErr
		return pe
	}

	var exitErr *exec.ExitError
	if errors.As(execErr, &exitErr) {
		pe.ExitCodeVal = exitErr.ExitCode()
		if isKilledExit(exitErr) {
			pe.isCanceled = true
		}
	} else {
		pe.Err = execErr
	}

	return pe
}
