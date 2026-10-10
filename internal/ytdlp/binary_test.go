package ytdlp

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPlatformAsset(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		goarch   string
		key      string
		resource string
		filename string
		sha256   string
	}{
		{"windows amd64", "windows", "amd64", "windows-x64", "windows-x64", "yt-dlp.exe", "66674953fe251b89f4d08c5f0e35e0728679bd67ab3d7d05c0562af101dd3e7a"},
		{"linux amd64", "linux", "amd64", "linux-x64", "linux-x64", "yt-dlp_linux", "58162f9bfdc27458ea47bfcb311cf47028f17d8154a8bf7d689861d46399230a"},
		{"darwin amd64", "darwin", "amd64", "darwin-x64", "darwin-universal", "yt-dlp_macos", "0f192b7ec147ab6288885d6351d9ab67367640029b4377576ef46dd79cf7b202"},
		{"darwin arm64", "darwin", "arm64", "darwin-arm64", "darwin-universal", "yt-dlp_macos", "0f192b7ec147ab6288885d6351d9ab67367640029b4377576ef46dd79cf7b202"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset, err := PlatformAssetFor(tt.goos, tt.goarch)
			if err != nil {
				t.Fatal(err)
			}
			if asset.Key != tt.key || asset.ResourceDir != tt.resource || asset.Filename != tt.filename || asset.SHA256 != tt.sha256 {
				t.Fatalf("asset = %#v", asset)
			}
			if asset.Version != BundledVersion || asset.URL == "" {
				t.Fatalf("asset metadata incomplete: %#v", asset)
			}
		})
	}
}

func TestPlatformAsset_Unsupported(t *testing.T) {
	if _, err := PlatformAssetFor("freebsd", "amd64"); !errors.Is(err, ErrPlatformNotSupported) {
		t.Fatalf("expected ErrPlatformNotSupported, got %v", err)
	}
}

func TestResolveBundledCandidateForPlatform(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		goarch   string
		resource string
		filename string
	}{
		{"windows resources", "windows", "amd64", "resources/yt-dlp", "yt-dlp.exe"},
		{"linux resources", "linux", "amd64", "resources/yt-dlp", "yt-dlp_linux"},
		{"macOS app bundle x64", "darwin", "amd64", "Contents/Resources/yt-dlp", "yt-dlp_macos"},
		{"macOS app bundle arm64", "darwin", "arm64", "Contents/Resources/yt-dlp", "yt-dlp_macos"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), filepath.FromSlash(tt.resource))
			asset, err := PlatformAssetFor(tt.goos, tt.goarch)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "tools", asset.ResourceDir, tt.filename)
			writeExecutable(t, path)
			candidate, err := ResolveBundledCandidateForPlatform(root, tt.goos, tt.goarch)
			if err != nil {
				t.Fatal(err)
			}
			if candidate.Path != path || candidate.Source != BinarySourceBundled || candidate.PlatformKey != PlatformKey(tt.goos, tt.goarch) {
				t.Fatalf("candidate = %#v", candidate)
			}
			if candidate.Version != BundledVersion {
				t.Fatalf("candidate version = %q", candidate.Version)
			}
		})
	}
}

func TestResolveBundledCandidate_RejectsMissingWrongNameAndNonExecutable(t *testing.T) {
	root := filepath.Join(t.TempDir(), "resources", "yt-dlp")
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
	}{
		{"missing", func(t *testing.T, dir string) {}},
		{"wrong filename", func(t *testing.T, dir string) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			writeExecutable(t, filepath.Join(dir, "yt-dlp-wrong"))
		}},
		{"non executable", func(t *testing.T, dir string) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "yt-dlp_linux")
			if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(root, "tools", "linux-x64")
			tt.setup(t, dir)
			_, err := ResolveBundledCandidateForPlatform(root, "linux", "amd64")
			if err == nil {
				t.Fatal("expected resolution error")
			}
			if !errors.Is(err, ErrBinaryNotFound) && !errors.Is(err, ErrBinaryNotExecutable) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveBundledCandidate_UsesRuntimePlatform(t *testing.T) {
	asset, err := PlatformAssetFor(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skipf("host platform not in release matrix: %v", err)
	}
	root := filepath.Join(t.TempDir(), "yt-dlp")
	path := filepath.Join(root, "tools", asset.ResourceDir, asset.Filename)
	writeExecutable(t, path)
	candidate, err := ResolveBundledCandidate(root)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Path != path {
		t.Fatalf("candidate path = %q, want %q", candidate.Path, path)
	}
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}
}
