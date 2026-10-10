package ffmpeg

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

	toolsSubdir  = "tools"
	licensesDir  = "licenses"
	manifestFile = "manifest.yaml"
	creditsFile  = "CREDITS.md"
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

func AllPlatformDirs() []string {
	return []string{
		"windows-x64",
		"linux-x64",
		"darwin-x64",
		"darwin-arm64",
	}
}

type ToolPaths struct {
	FFmpeg  string
	FFprobe string
}

var ErrPlatformNotSupported = fmt.Errorf("ffmpeg: unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
var ErrResourcesNotFound = fmt.Errorf("ffmpeg: tools directory not found")
var ErrFFmpegNotFound = fmt.Errorf("ffmpeg: ffmpeg binary not found")
var ErrFFprobeNotFound = fmt.Errorf("ffmpeg: ffprobe binary not found")
var ErrSHA256Mismatch = fmt.Errorf("ffmpeg: sha256 mismatch")
var ErrArchMismatch = fmt.Errorf("ffmpeg: binary architecture does not match runtime")

func ResolvePaths(resourcesRoot string) (ToolPaths, error) {
	spec := CurrentPlatform()
	if spec.DirName == "" {
		return ToolPaths{}, ErrPlatformNotSupported
	}

	toolsDir := filepath.Join(resourcesRoot, toolsSubdir, spec.DirName)

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

	if err := checkBinaryArch(ffmpegPath); err != nil {
		return ToolPaths{}, fmt.Errorf("%w (ffmpeg): %v", ErrArchMismatch, err)
	}
	if err := checkBinaryArch(ffprobePath); err != nil {
		return ToolPaths{}, fmt.Errorf("%w (ffprobe): %v", ErrArchMismatch, err)
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

func checkBinaryArch(path string) error {
	archStr, err := readBinaryArch(path)
	if err != nil {
		return nil
	}

	runtimeArch := runtime.GOARCH
	if runtimeArch == "amd64" {
		if !strings.Contains(archStr, "x86_64") && !strings.Contains(archStr, "AMD64") && !strings.Contains(archStr, "arm64") && !strings.Contains(archStr, "aarch64") {
			return nil
		}
		if strings.Contains(archStr, "arm64") || strings.Contains(archStr, "aarch64") {
			return fmt.Errorf("runtime is amd64, binary reports %q", archStr)
		}
	} else if runtimeArch == "arm64" {
		if !strings.Contains(archStr, "arm64") && !strings.Contains(archStr, "aarch64") && !strings.Contains(archStr, "x86_64") {
			return nil
		}
		if strings.Contains(archStr, "x86_64") {
			return fmt.Errorf("runtime is arm64, binary reports %q", archStr)
		}
	}
	return nil
}

func readBinaryArch(path string) (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("file", path)
	case "linux":
		cmd = exec.Command("file", path)
	case "windows":
		return "", fmt.Errorf("windows arch check not implemented via file")
	default:
		return "", fmt.Errorf("unsupported OS for arch check")
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func VerifySHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, expected) {
		return fmt.Errorf("%w: got %s, want %s", ErrSHA256Mismatch, got, expected)
	}
	return nil
}

func ResolveFromExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("ffmpeg: cannot determine executable path: %w", err)
	}
	exeDir := filepath.Dir(exe)

	if runtime.GOOS == "darwin" {
		if strings.Contains(exeDir, "Contents"+string(os.PathSeparator)+"MacOS") {
			contentsDir := filepath.Dir(exeDir)
			candidate := filepath.Join(contentsDir, "Resources", "ffmpeg")
			if dirExists(candidate) {
				return candidate, nil
			}
		}
	}

	candidate := filepath.Join(exeDir, "resources", "ffmpeg")
	if dirExists(candidate) {
		return candidate, nil
	}

	wd, _ := os.Getwd()
	if wd != "" {
		candidate = filepath.Join(wd, "build", "resources", "ffmpeg")
		if dirExists(candidate) {
			return candidate, nil
		}
	}

	return "", ErrResourcesNotFound
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

var versionRe = regexp.MustCompile(`(\d+)\.(\d+)(?:\.\d+)?`)

func ValidateVersion(versionOutput string) error {
	m := versionRe.FindStringSubmatch(versionOutput)
	if m == nil {
		return fmt.Errorf("ffmpeg: unable to parse version output: %q", versionOutput)
	}

	var major, minor int
	if _, err := fmt.Sscanf(m[1]+"."+m[2], "%d.%d", &major, &minor); err != nil {
		return fmt.Errorf("ffmpeg: unable to parse version number from %q: %w", m[0], err)
	}

	if major != ExpectedVersionMajor || (minor != 0 && minor != 1) {
		return fmt.Errorf("ffmpeg: version mismatch: got %d.%d, want 8.0.x or 8.1.x",
			major, minor)
	}

	return nil
}
