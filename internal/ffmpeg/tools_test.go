package ffmpeg

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPlatformDirName(t *testing.T) {
	tests := []struct {
		goos   string
		goarch string
		want   string
	}{
		{"windows", "amd64", "windows-x64"},
		{"linux", "amd64", "linux-x64"},
		{"darwin", "amd64", "darwin-x64"},
		{"darwin", "arm64", "darwin-arm64"},
		{"windows", "arm64", ""},
		{"freebsd", "amd64", ""},
	}
	for _, tt := range tests {
		got := platformDirName(tt.goos, tt.goarch)
		if got != tt.want {
			t.Errorf("platformDirName(%q, %q) = %q, want %q", tt.goos, tt.goarch, got, tt.want)
		}
	}
}

func TestAllPlatformDirs(t *testing.T) {
	dirs := AllPlatformDirs()
	if len(dirs) != 4 {
		t.Errorf("AllPlatformDirs() len = %d, want 4", len(dirs))
	}
	seen := map[string]bool{}
	for _, d := range dirs {
		if seen[d] {
			t.Errorf("duplicate dir: %s", d)
		}
		seen[d] = true
	}
	for _, expected := range []string{"windows-x64", "linux-x64", "darwin-x64", "darwin-arm64"} {
		if !seen[expected] {
			t.Errorf("missing expected platform dir: %s", expected)
		}
	}
}

func TestCurrentPlatform(t *testing.T) {
	spec := CurrentPlatform()
	if spec.OS != runtime.GOOS {
		t.Errorf("OS mismatch: got %q, want %q", spec.OS, runtime.GOOS)
	}
	if spec.Arch != runtime.GOARCH {
		t.Errorf("Arch mismatch: got %q, want %q", spec.Arch, runtime.GOARCH)
	}
	if spec.DirName == "" {
		t.Skip("current platform not in target matrix")
	}
	if runtime.GOOS == "windows" {
		if spec.FFmpegBin != "ffmpeg.exe" {
			t.Errorf("windows ffmpeg bin: got %q, want ffmpeg.exe", spec.FFmpegBin)
		}
	} else {
		if spec.FFmpegBin != "ffmpeg" {
			t.Errorf("unix ffmpeg bin: got %q, want ffmpeg", spec.FFmpegBin)
		}
	}
}

func TestResolvePaths_MissingResources(t *testing.T) {
	_, err := ResolvePaths("/nonexistent/resources")
	if err == nil {
		t.Fatal("expected error for nonexistent resources root")
	}
}

func TestResolvePaths_PlatformNotSupported(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "tools", "freebsd-x64"), 0o755)
	_, err := ResolvePaths(tmp)
	if !errors.Is(err, ErrResourcesNotFound) && !errors.Is(err, ErrFFmpegNotFound) {
		t.Logf("got expected error: %v", err)
	}
}

func TestResolvePaths_MissingBinary(t *testing.T) {
	dirName := platformDirName(runtime.GOOS, runtime.GOARCH)
	if dirName == "" {
		t.Skip("platform not in target matrix")
	}

	tmp := t.TempDir()
	toolsDir := filepath.Join(tmp, "tools", dirName)
	os.MkdirAll(toolsDir, 0o755)

	_, err := ResolvePaths(tmp)
	if !errors.Is(err, ErrFFmpegNotFound) {
		t.Errorf("expected ErrFFmpegNotFound, got: %v", err)
	}
}

func TestResolvePaths_RealBinary(t *testing.T) {
	dirName := platformDirName(runtime.GOOS, runtime.GOARCH)
	if dirName == "" {
		t.Skip("platform not in target matrix")
	}

	tmp := t.TempDir()
	toolsDir := filepath.Join(tmp, "tools", dirName)
	os.MkdirAll(toolsDir, 0o755)

	ffBin := filepath.Join(toolsDir, CurrentPlatform().FFmpegBin)
	fpBin := filepath.Join(toolsDir, CurrentPlatform().FFprobeBin)

	script := "#!/bin/sh\necho ffmpeg version 8.0.3\n"
	if runtime.GOOS == "windows" {
		script = "@echo off\r\necho ffmpeg version 8.0.3\r\n"
	}
	if err := os.WriteFile(ffBin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fpBin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	paths, err := ResolvePaths(tmp)
	if err != nil {
		t.Fatalf("ResolvePaths: %v", err)
	}
	if paths.FFmpeg != ffBin {
		t.Errorf("FFmpeg path mismatch: got %q, want %q", paths.FFmpeg, ffBin)
	}
	if paths.FFprobe != fpBin {
		t.Errorf("FFprobe path mismatch: got %q, want %q", paths.FFprobe, fpBin)
	}
}

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{
			name:    "correct version",
			output:  "ffmpeg version 8.0.3 Copyright (c) 2000-2026 the FFmpeg developers",
			wantErr: false,
		},
		{
			name:    "git-tag n prefix accepted",
			output:  "ffmpeg version n8.0.3 Copyright (c) 2000-2026 the FFmpeg developers",
			wantErr: false,
		},
		{
			name:    "8.0 branch accepted",
			output:  "ffmpeg version 8.0.1-dev Copyright",
			wantErr: false,
		},
		{
			name:    "8.1 branch accepted",
			output:  "ffmpeg version 8.1.2 Copyright",
			wantErr: false,
		},
		{
			name:    "wrong major version",
			output:  "ffmpeg version 7.1.4 Copyright",
			wantErr: true,
		},
		{
			name:    "too new major",
			output:  "ffmpeg version 9.0.0 Copyright",
			wantErr: true,
		},
		{
			name:    "garbage output",
			output:  "something completely different",
			wantErr: true,
		},
		{
			name:    "empty",
			output:  "",
			wantErr: true,
		},
		{
			name:    "version prefix with letter",
			output:  "ffmpeg version N-127054-g9d3f0f2c58 Copyright",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVersion(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBinaryExt(t *testing.T) {
	if runtime.GOOS == "windows" {
		if binaryExt() != ".exe" {
			t.Errorf("windows binaryExt = %q, want .exe", binaryExt())
		}
	} else {
		if binaryExt() != "" {
			t.Errorf("unix binaryExt = %q, want empty", binaryExt())
		}
	}
}

func TestVerifySHA256(t *testing.T) {
	tmp := t.TempDir()
	fpath := filepath.Join(tmp, "data.bin")
	content := []byte("hello world ffmpeg bundled tools verification")
	if err := os.WriteFile(fpath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	want := "4f44d15a5f47f45d1d7c5e8e5d1d6f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e"

	t.Run("correct sha256", func(t *testing.T) {
		got := mustComputeSHA256(t, fpath)
		if err := VerifySHA256(fpath, got); err != nil {
			t.Errorf("VerifySHA256 with correct hash failed: %v", err)
		}
	})

	t.Run("wrong sha256", func(t *testing.T) {
		err := VerifySHA256(fpath, want)
		if err == nil {
			t.Fatal("expected error for wrong sha256")
		}
		if !errors.Is(err, ErrSHA256Mismatch) {
			t.Errorf("expected ErrSHA256Mismatch, got %v", err)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		err := VerifySHA256(filepath.Join(tmp, "nope.bin"), want)
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})
}

func mustComputeSHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestResolveFromExecutable_FallbackToWD(t *testing.T) {
	dirName := platformDirName(runtime.GOOS, runtime.GOARCH)
	if dirName == "" {
		t.Skip("platform not in target matrix")
	}

	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	mockRoot := filepath.Join(tmp, "build", "resources", "ffmpeg")
	toolsDir := filepath.Join(mockRoot, "tools", dirName)
	os.MkdirAll(toolsDir, 0o755)

	script := "#!/bin/sh\necho ffmpeg version 8.0.3\n"
	if runtime.GOOS == "windows" {
		script = "@echo off\r\necho ffmpeg version 8.0.3\r\n"
	}
	target := filepath.Join(toolsDir, CurrentPlatform().FFmpegBin)
	if err := os.WriteFile(target, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	target2 := filepath.Join(toolsDir, CurrentPlatform().FFprobeBin)
	if err := os.WriteFile(target2, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	origWD, _ := os.Getwd()
	defer os.Chdir(origWD)
	os.Chdir(tmp)

	root, err := ResolveFromExecutable()
	if err != nil {
		t.Fatalf("ResolveFromExecutable failed: %v", err)
	}
	if root != mockRoot {
		t.Errorf("ResolveFromExecutable = %q, want %q", root, mockRoot)
	}
}

func TestResolveFromExecutable_NoResourcesFound(t *testing.T) {
	tmp := t.TempDir()
	origWD, _ := os.Getwd()
	defer os.Chdir(origWD)
	os.Chdir(tmp)

	_, err := ResolveFromExecutable()
	if err == nil {
		t.Fatal("expected error when no resources found")
	}
	if !errors.Is(err, ErrResourcesNotFound) {
		t.Errorf("expected ErrResourcesNotFound, got: %v", err)
	}
}

func TestDirExists(t *testing.T) {
	tmp := t.TempDir()
	if !dirExists(tmp) {
		t.Errorf("dirExists(%s) should be true", tmp)
	}
	if dirExists(filepath.Join(tmp, "nope")) {
		t.Error("dirExists of nonexistent should be false")
	}

	fpath := filepath.Join(tmp, "file.txt")
	os.WriteFile(fpath, []byte("x"), 0o644)
	if dirExists(fpath) {
		t.Error("dirExists of file should be false")
	}
}

func TestErrPlatformNotSupported(t *testing.T) {
	if ErrPlatformNotSupported == nil {
		t.Error("ErrPlatformNotSupported should be non-nil")
	}
	if ErrResourcesNotFound == nil {
		t.Error("ErrResourcesNotFound should be non-nil")
	}
	if ErrFFmpegNotFound == nil {
		t.Error("ErrFFmpegNotFound should be non-nil")
	}
	if ErrFFprobeNotFound == nil {
		t.Error("ErrFFprobeNotFound should be non-nil")
	}
	if ErrSHA256Mismatch == nil {
		t.Error("ErrSHA256Mismatch should be non-nil")
	}
	if ErrArchMismatch == nil {
		t.Error("ErrArchMismatch should be non-nil")
	}
}
