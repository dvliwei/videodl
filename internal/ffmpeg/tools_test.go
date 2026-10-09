package ffmpeg

import (
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
			name:    "8.0 branch accepted",
			output:  "ffmpeg version 8.0.1-dev Copyright",
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
