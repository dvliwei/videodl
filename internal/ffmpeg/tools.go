package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ExpectedVersionMajor = 8
	ExpectedVersionMinor = 0

	ConfiguredVersion = "8.0.3"
	ConfiguredBranch  = "8.0"

	ffmpegTool  = "ffmpeg"
	ffprobeTool = "ffprobe"
)

var (
	ffmpegBin  = ffmpegTool + binaryExt()
	ffprobeBin = ffprobeTool + binaryExt()
)

func binaryExt() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

type PlatformSpec struct {
	OS         string
	Arch       string
	DirName    string
	FFmpegBin  string
	FFprobeBin string
}

func CurrentPlatform() PlatformSpec {
	return PlatformSpec{
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		DirName:    platformDirName(runtime.GOOS, runtime.GOARCH),
		FFmpegBin:  ffmpegBin,
		FFprobeBin: ffprobeBin,
	}
}

func platformDirName(goos, goarch string) string {
	switch goos {
	case "windows":
		if goarch == "amd64" {
			return "windows-x64"
		}
	case "linux":
		return "linux-x64"
	case "darwin":
		switch goarch {
		case "arm64":
			return "darwin-arm64"
		case "amd64":
			return "darwin-x64"
		}
	}
	return ""
}

type ToolPaths struct {
	FFmpeg  string
	FFprobe string
}

var ErrPlatformNotSupported = fmt.Errorf("ffmpeg: unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
var ErrResourcesNotFound = fmt.Errorf("ffmpeg: tools directory not found")
var ErrFFmpegNotFound = fmt.Errorf("ffmpeg: ffmpeg binary not found")
var ErrFFprobeNotFound = fmt.Errorf("ffmpeg: ffprobe binary not found")

func ResolvePaths(resourcesRoot string) (ToolPaths, error) {
	spec := CurrentPlatform()
	if spec.DirName == "" {
		return ToolPaths{}, ErrPlatformNotSupported
	}

	toolsDir := filepath.Join(resourcesRoot, "tools", spec.DirName)

	info, err := os.Stat(toolsDir)
	if err != nil || !info.IsDir() {
		return ToolPaths{}, fmt.Errorf("%w: %s", ErrResourcesNotFound, toolsDir)
	}

	ffmpegPath := filepath.Join(toolsDir, spec.FFmpegBin)
	ffprobePath := filepath.Join(toolsDir, spec.FFprobeBin)

	if err := checkExecutable(ffmpegPath); err != nil {
		return ToolPaths{}, fmt.Errorf("%w: %s: %w", ErrFFmpegNotFound, ffmpegPath, err)
	}
	if err := checkExecutable(ffprobePath); err != nil {
		return ToolPaths{}, fmt.Errorf("%w: %s: %w", ErrFFprobeNotFound, ffprobePath, err)
	}

	return ToolPaths{
		FFmpeg:  ffmpegPath,
		FFprobe: ffprobePath,
	}, nil
}

func checkExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if info.Mode()&0111 == 0 {
			return fmt.Errorf("not executable")
		}
	}
	return nil
}

func ValidateVersion(versionOutput string) error {
	parts := strings.Fields(versionOutput)
	if len(parts) < 3 {
		return fmt.Errorf("ffmpeg: unable to parse version output: %q", versionOutput)
	}

	var major, minor int
	_, err := fmt.Sscanf(parts[2], "%d.%d", &major, &minor)
	if err != nil {
		return fmt.Errorf("ffmpeg: unable to parse version number from %q: %w", parts[2], err)
	}

	if major != ExpectedVersionMajor || minor != ExpectedVersionMinor {
		return fmt.Errorf("ffmpeg: version mismatch: got %d.%d, want %d.%d",
			major, minor, ExpectedVersionMajor, ExpectedVersionMinor)
	}

	return nil
}
