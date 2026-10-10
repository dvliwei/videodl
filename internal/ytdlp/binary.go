package ytdlp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// BundledVersion is the release pinned by the first packaged integration.
	BundledVersion = "2026.08.19"

	BinarySourceBundled BinarySource = "bundled"
	BinarySourceUser    BinarySource = "user-update"
)

var (
	ErrPlatformNotSupported = errors.New("yt-dlp: platform is not supported")
	ErrBinaryNotFound       = errors.New("yt-dlp: executable not found")
	ErrBinaryNotExecutable  = errors.New("yt-dlp: executable is not executable")
)

// BinarySource identifies where a verified candidate came from. It is kept as
// a label so callers never need to infer trust from an arbitrary filesystem
// path.
type BinarySource string

// PlatformAsset records the immutable metadata required to fetch and verify a
// release asset. The macOS release is universal, so both macOS runtime keys
// deliberately point to the same resource directory and checksum.
type PlatformAsset struct {
	Key         string
	GOOS        string
	GOARCH      string
	ResourceDir string
	Filename    string
	URL         string
	SHA256      string
	Version     string
}

// BinaryCandidate is the only executable information exposed to later
// runtime layers. Paths must come from a known resource layout.
type BinaryCandidate struct {
	Path        string
	Version     string
	PlatformKey string
	Source      BinarySource
}

// BinaryResolver is used by the runtime and updater to resolve known bundle
// locations. It intentionally has no PATH lookup method.
type BinaryResolver interface {
	ResolveBundledCandidate(root string) (BinaryCandidate, error)
	ResolveUserCandidate(root string) (BinaryCandidate, error)
}

// DefaultBinaryResolver uses the fixed application bundle and per-user update
// layouts. It intentionally exposes no arbitrary executable or PATH lookup.
type DefaultBinaryResolver struct{}

func (DefaultBinaryResolver) ResolveBundledCandidate(root string) (BinaryCandidate, error) {
	return ResolveBundledCandidate(root)
}

func (DefaultBinaryResolver) ResolveUserCandidate(root string) (BinaryCandidate, error) {
	return ResolveUserCandidate(root)
}

// PlatformAssetFor returns the pinned release asset for a supported target.
func PlatformAssetFor(goos, goarch string) (PlatformAsset, error) {
	key := PlatformKey(goos, goarch)
	assets := map[string]PlatformAsset{
		"windows-x64": {
			Key:         "windows-x64",
			GOOS:        "windows",
			GOARCH:      "amd64",
			ResourceDir: "windows-x64",
			Filename:    "yt-dlp.exe",
			URL:         "https://github.com/yt-dlp/yt-dlp/releases/download/2026.08.19/yt-dlp.exe",
			SHA256:      "66674953fe251b89f4d08c5f0e35e0728679bd67ab3d7d05c0562af101dd3e7a",
			Version:     BundledVersion,
		},
		"linux-x64": {
			Key:         "linux-x64",
			GOOS:        "linux",
			GOARCH:      "amd64",
			ResourceDir: "linux-x64",
			Filename:    "yt-dlp_linux",
			URL:         "https://github.com/yt-dlp/yt-dlp/releases/download/2026.08.19/yt-dlp_linux",
			SHA256:      "58162f9bfdc27458ea47bfcb311cf47028f17d8154a8bf7d689861d46399230a",
			Version:     BundledVersion,
		},
		"darwin-x64": {
			Key:         "darwin-x64",
			GOOS:        "darwin",
			GOARCH:      "amd64",
			ResourceDir: "darwin-universal",
			Filename:    "yt-dlp_macos",
			URL:         "https://github.com/yt-dlp/yt-dlp/releases/download/2026.08.19/yt-dlp_macos",
			SHA256:      "0f192b7ec147ab6288885d6351d9ab67367640029b4377576ef46dd79cf7b202",
			Version:     BundledVersion,
		},
		"darwin-arm64": {
			Key:         "darwin-arm64",
			GOOS:        "darwin",
			GOARCH:      "arm64",
			ResourceDir: "darwin-universal",
			Filename:    "yt-dlp_macos",
			URL:         "https://github.com/yt-dlp/yt-dlp/releases/download/2026.08.19/yt-dlp_macos",
			SHA256:      "0f192b7ec147ab6288885d6351d9ab67367640029b4377576ef46dd79cf7b202",
			Version:     BundledVersion,
		},
	}
	asset, ok := assets[key]
	if !ok {
		return PlatformAsset{}, fmt.Errorf("%w: %s/%s", ErrPlatformNotSupported, goos, goarch)
	}
	return asset, nil
}

// PlatformKey returns the stable resource key for a target platform.
func PlatformKey(goos, goarch string) string {
	switch {
	case goos == "windows" && goarch == "amd64":
		return "windows-x64"
	case goos == "linux" && goarch == "amd64":
		return "linux-x64"
	case goos == "darwin" && goarch == "amd64":
		return "darwin-x64"
	case goos == "darwin" && goarch == "arm64":
		return "darwin-arm64"
	default:
		return ""
	}
}

// ResolveBundledCandidate resolves the current runtime's bundled executable.
func ResolveBundledCandidate(root string) (BinaryCandidate, error) {
	if root == "" {
		var err error
		root, err = ResolveFromExecutable()
		if err != nil {
			return BinaryCandidate{}, err
		}
	}
	return ResolveBundledCandidateForPlatform(root, runtime.GOOS, runtime.GOARCH)
}

// ResolveFromExecutable locates the read-only resource directory next to the
// application executable, matching the FFmpeg bundle layout.
func ResolveFromExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("yt-dlp: cannot determine executable path: %w", err)
	}
	exeDir := filepath.Dir(exe)
	if runtime.GOOS == "darwin" && filepath.Base(exeDir) == "MacOS" {
		contents := filepath.Dir(exeDir)
		candidate := filepath.Join(contents, "Resources", "yt-dlp")
		if dirExists(candidate) {
			return candidate, nil
		}
	}
	candidate := filepath.Join(exeDir, "resources", "yt-dlp")
	if dirExists(candidate) {
		return candidate, nil
	}
	if wd, err := os.Getwd(); err == nil {
		candidate = filepath.Join(wd, "build", "resources", "yt-dlp")
		if dirExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%w: bundled resources not found", ErrBinaryNotFound)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ResolveBundledCandidateForPlatform exists so packaging and unit tests can
// validate all supported layouts without pretending to change runtime.GOOS.
func ResolveBundledCandidateForPlatform(root, goos, goarch string) (BinaryCandidate, error) {
	asset, err := PlatformAssetFor(goos, goarch)
	if err != nil {
		return BinaryCandidate{}, err
	}
	return resolveCandidate(root, asset, BinarySourceBundled)
}

// ResolveUserCandidate resolves a previously verified update stored in the
// same platform layout below root. No user-provided path or PATH search is
// accepted here.
func ResolveUserCandidate(root string) (BinaryCandidate, error) {
	asset, err := PlatformAssetFor(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return BinaryCandidate{}, err
	}
	return resolveCandidate(root, asset, BinarySourceUser)
}

func resolveCandidate(root string, asset PlatformAsset, source BinarySource) (BinaryCandidate, error) {
	if root == "" {
		return BinaryCandidate{}, fmt.Errorf("%w: empty resources root", ErrBinaryNotFound)
	}
	path := filepath.Join(root, "tools", asset.ResourceDir, asset.Filename)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		if err == nil {
			err = fmt.Errorf("path is a directory")
		}
		return BinaryCandidate{}, fmt.Errorf("%w: %s: %v", ErrBinaryNotFound, path, err)
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		return BinaryCandidate{}, fmt.Errorf("%w: %s", ErrBinaryNotExecutable, path)
	}
	return BinaryCandidate{
		Path:        path,
		Version:     asset.Version,
		PlatformKey: asset.Key,
		Source:      source,
	}, nil
}
