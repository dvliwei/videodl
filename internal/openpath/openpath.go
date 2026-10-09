package openpath

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func RevealInFinder(target string) error {
	if target == "" {
		return errors.New("path is empty")
	}

	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", target).Start()
	case "windows":
		abs, err := filepath.Abs(target)
		if err != nil {
			abs = target
		}
		return exec.Command("rundll32", "shell32.dll,OpenFolderPathAndSelect", abs).Start()
	default:
		dir := filepath.Dir(target)
		if dir == "" || dir == "." {
			dir = "/"
		}
		return exec.Command("xdg-open", dir).Start()
	}
}

func OpenPath(target string) error {
	if target == "" {
		return errors.New("path is empty")
	}

	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", target).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
	default:
		return exec.Command("xdg-open", target).Start()
	}
}

func OpenDirectory(dir string) error {
	if dir == "" {
		return errors.New("path is empty")
	}

	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", dir).Start()
	case "windows":
		return exec.Command("explorer", strings.ReplaceAll(dir, "/", "\\")).Start()
	default:
		return exec.Command("xdg-open", dir).Start()
	}
}
