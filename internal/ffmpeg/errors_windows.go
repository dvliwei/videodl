//go:build windows

package ffmpeg

import "os/exec"

func isKilledExit(exitErr *exec.ExitError) bool {
	return false
}
