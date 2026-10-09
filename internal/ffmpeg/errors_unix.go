//go:build darwin || linux || freebsd

package ffmpeg

import (
	"os/exec"
	"syscall"
)

func isKilledExit(exitErr *exec.ExitError) bool {
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
		return status.Signaled()
	}
	return false
}
