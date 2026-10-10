package ytdlp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

const OfficialRepository = "yt-dlp/yt-dlp"

var (
	ErrUpdateInProgress = errors.New("yt-dlp: update already in progress")
	ErrUpdateFailed     = errors.New("yt-dlp: update failed")
	ErrReleaseRejected  = errors.New("yt-dlp: release rejected")
)

type UpdateStatus struct {
	Available      bool
	CurrentVersion string
	LatestVersion  string
	Source         string
}

type Asset struct {
	Name        string
	PlatformKey string
	URL         string
	SHA256      string
}

type Release struct {
	Repository string
	Tag        string
	Version    string
	Assets     []Asset
}

type ReleaseClient interface {
	LatestStable(ctx context.Context) (Release, error)
	Download(ctx context.Context, asset Asset, dst io.Writer) error
}

// OfficialReleaseClient talks only to the yt-dlp GitHub repository and its
// release assets. It deliberately does not invoke yt-dlp's self-updater.
type OfficialReleaseClient struct {
	HTTPClient *http.Client
	APIBaseURL string
}

func NewOfficialReleaseClient(client *http.Client) *OfficialReleaseClient {
	if client == nil {
		client = &http.Client{}
	}
	return &OfficialReleaseClient{HTTPClient: client, APIBaseURL: "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest"}
}

func (c *OfficialReleaseClient) LatestStable(ctx context.Context) (Release, error) {
	if c == nil || c.HTTPClient == nil {
		return Release{}, fmt.Errorf("%w: HTTP client is not configured", ErrUpdateFailed)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.APIBaseURL, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "VideoDL/1.0")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("latest release status %d", resp.StatusCode)
	}
	var payload struct {
		TagName    string `json:"tag_name"`
		Prerelease bool   `json:"prerelease"`
		Draft      bool   `json:"draft"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4*1024*1024)).Decode(&payload); err != nil {
		return Release{}, err
	}
	if payload.Prerelease || payload.Draft {
		return Release{}, fmt.Errorf("%w: release is not stable", ErrReleaseRejected)
	}
	version := strings.TrimPrefix(payload.TagName, "v")
	checksumURL := "https://github.com/yt-dlp/yt-dlp/releases/download/" + payload.TagName + "/SHA2-256SUMS"
	checksums, err := c.fetchChecksums(ctx, checksumURL)
	if err != nil {
		return Release{}, err
	}
	release := Release{Repository: OfficialRepository, Tag: payload.TagName, Version: version}
	for _, item := range payload.Assets {
		if item.Name != "yt-dlp.exe" && item.Name != "yt-dlp_linux" && item.Name != "yt-dlp_macos" {
			continue
		}
		platformKey := ""
		switch item.Name {
		case "yt-dlp.exe":
			platformKey = "windows-x64"
		case "yt-dlp_linux":
			platformKey = "linux-x64"
		case "yt-dlp_macos":
			platformKey = "darwin-universal"
		}
		release.Assets = append(release.Assets, Asset{Name: item.Name, PlatformKey: platformKey, URL: item.URL, SHA256: checksums[item.Name]})
	}
	return release, nil
}

func (c *OfficialReleaseClient) fetchChecksums(ctx context.Context, rawURL string) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksum list status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			result[fields[1]] = fields[0]
		}
	}
	return result, nil
}

func (c *OfficialReleaseClient) Download(ctx context.Context, asset Asset, dst io.Writer) error {
	if asset.URL == "" || !strings.HasPrefix(asset.URL, "https://github.com/yt-dlp/yt-dlp/releases/download/") {
		return fmt.Errorf("%w: asset URL is not official", ErrReleaseRejected)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "VideoDL/1.0")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("asset download status %d", resp.StatusCode)
	}
	_, err = io.Copy(dst, io.LimitReader(resp.Body, 128*1024*1024))
	return err
}

type Updater struct {
	client   ReleaseClient
	resolver BinaryResolver
	userDir  string
	mu       sync.Mutex
}

func NewUpdater(client ReleaseClient, resolver BinaryResolver, userDir string) *Updater {
	return &Updater{client: client, resolver: resolver, userDir: userDir}
}

func (u *Updater) Status(ctx context.Context) (*UpdateStatus, error) {
	if u == nil || u.client == nil || u.resolver == nil {
		return nil, fmt.Errorf("%w: updater is not configured", ErrUpdateFailed)
	}
	current, source, err := u.currentVersion()
	if err != nil {
		return nil, err
	}
	release, err := u.client.LatestStable(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: latest release: %w", ErrUpdateFailed, err)
	}
	version, err := normalizeReleaseVersion(release)
	if err != nil {
		return nil, err
	}
	return &UpdateStatus{
		Available:      current == "" || compareVersions(version, current) > 0,
		CurrentVersion: current,
		LatestVersion:  version,
		Source:         source,
	}, nil
}

func (u *Updater) Update(ctx context.Context) (*UpdateStatus, error) {
	if u == nil {
		return nil, fmt.Errorf("%w: updater is nil", ErrUpdateFailed)
	}
	if !u.mu.TryLock() {
		return nil, ErrUpdateInProgress
	}
	defer u.mu.Unlock()

	if u.client == nil || u.resolver == nil || u.userDir == "" {
		return nil, fmt.Errorf("%w: updater is not configured", ErrUpdateFailed)
	}
	release, err := u.client.LatestStable(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: latest release: %w", ErrUpdateFailed, err)
	}
	version, err := normalizeReleaseVersion(release)
	if err != nil {
		return nil, err
	}
	asset, platform, err := selectReleaseAsset(release)
	if err != nil {
		return nil, err
	}
	current, _, err := u.currentVersion()
	if err != nil {
		return nil, err
	}
	if current != "" && compareVersions(version, current) <= 0 {
		return &UpdateStatus{Available: false, CurrentVersion: current, LatestVersion: version, Source: u.currentSource()}, nil
	}

	root := filepath.Join(u.userDir, "yt-dlp")
	dir := filepath.Join(root, "tools", platform.ResourceDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("%w: create update directory: %w", ErrUpdateFailed, err)
	}
	tmp, err := os.CreateTemp(dir, ".yt-dlp-update-*")
	if err != nil {
		return nil, fmt.Errorf("%w: create temporary file: %w", ErrUpdateFailed, err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	hasher := sha256.New()
	if err := u.client.Download(ctx, asset, io.MultiWriter(tmp, hasher)); err != nil {
		cleanup()
		return nil, fmt.Errorf("%w: download: %w", ErrUpdateFailed, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("%w: close temporary file: %w", ErrUpdateFailed, err)
	}
	got := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(got, asset.SHA256) {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("%w: sha256 mismatch", ErrUpdateFailed)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0o755); err != nil {
			_ = os.Remove(tmpPath)
			return nil, fmt.Errorf("%w: set executable permission: %w", ErrUpdateFailed, err)
		}
	}

	destination := filepath.Join(dir, platform.Filename)
	backup := destination + ".previous"
	if err := os.Remove(backup); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("%w: remove old backup: %w", ErrUpdateFailed, err)
	}
	oldExists := false
	if _, err := os.Stat(destination); err == nil {
		oldExists = true
		if err := os.Rename(destination, backup); err != nil {
			_ = os.Remove(tmpPath)
			return nil, fmt.Errorf("%w: preserve current executable: %w", ErrUpdateFailed, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("%w: inspect current executable: %w", ErrUpdateFailed, err)
	}
	rollback := func() {
		_ = os.Remove(destination)
		if oldExists {
			_ = os.Rename(backup, destination)
		}
	}
	if err := os.Rename(tmpPath, destination); err != nil {
		rollback()
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("%w: replace executable: %w", ErrUpdateFailed, err)
	}
	if err := verifyExecutable(ctx, destination, version); err != nil {
		rollback()
		return nil, fmt.Errorf("%w: health check: %w", ErrUpdateFailed, err)
	}
	if err := writeVersionMarker(root, version); err != nil {
		rollback()
		return nil, fmt.Errorf("%w: write version marker: %w", ErrUpdateFailed, err)
	}
	_ = os.Remove(backup)
	return &UpdateStatus{Available: false, CurrentVersion: version, LatestVersion: version, Source: string(BinarySourceUser)}, nil
}

func (u *Updater) currentVersion() (string, string, error) {
	root := filepath.Join(u.userDir, "yt-dlp")
	if candidate, err := u.resolver.ResolveUserCandidate(root); err == nil && candidate.Path != "" {
		version := readVersionMarker(root)
		if version == "" {
			version = candidate.Version
		}
		return version, string(BinarySourceUser), nil
	} else if err != nil && !errors.Is(err, ErrBinaryNotFound) && !errors.Is(err, ErrPlatformNotSupported) {
		return "", "", err
	}
	candidate, err := u.resolver.ResolveBundledCandidate("")
	if err != nil {
		return "", "", err
	}
	return candidate.Version, string(BinarySourceBundled), nil
}

func (u *Updater) currentSource() string {
	_, source, err := u.currentVersion()
	if err != nil {
		return string(BinarySourceBundled)
	}
	return source
}

func normalizeReleaseVersion(release Release) (string, error) {
	if release.Repository != OfficialRepository {
		return "", fmt.Errorf("%w: repository %q", ErrReleaseRejected, release.Repository)
	}
	version := release.Version
	if version == "" {
		version = strings.TrimPrefix(release.Tag, "v")
	}
	if !versionPattern.MatchString(version) {
		return "", fmt.Errorf("%w: invalid stable version", ErrReleaseRejected)
	}
	return version, nil
}

func selectReleaseAsset(release Release) (Asset, PlatformAsset, error) {
	version, err := normalizeReleaseVersion(release)
	if err != nil {
		return Asset{}, PlatformAsset{}, err
	}
	platform, err := PlatformAssetFor(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return Asset{}, PlatformAsset{}, err
	}
	for _, asset := range release.Assets {
		platformMatch := asset.PlatformKey == platform.Key || (platform.ResourceDir == "darwin-universal" && asset.PlatformKey == "darwin-universal")
		if asset.Name != platform.Filename || !platformMatch || !strings.HasPrefix(asset.URL, "https://github.com/yt-dlp/yt-dlp/releases/download/") {
			continue
		}
		if !shaPattern.MatchString(asset.SHA256) {
			return Asset{}, PlatformAsset{}, fmt.Errorf("%w: invalid checksum for %s", ErrReleaseRejected, asset.Name)
		}
		_ = version
		return asset, platform, nil
	}
	return Asset{}, PlatformAsset{}, fmt.Errorf("%w: known asset %s not found", ErrReleaseRejected, platform.Filename)
}

func verifyExecutable(ctx context.Context, path, expectedVersion string) error {
	cmd := exec.CommandContext(ctx, path, "--version")
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	if !strings.Contains(string(output), expectedVersion) {
		return fmt.Errorf("version mismatch")
	}
	return nil
}

func writeVersionMarker(root, version string) error {
	tmp, err := os.CreateTemp(root, ".yt-dlp-version-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := io.WriteString(tmp, version+"\n"); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, filepath.Join(root, "version")); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func readVersionMarker(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "version"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

var versionPattern = regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}$`)
var shaPattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

func compareVersions(left, right string) int {
	parse := func(value string) [3]int {
		parts := strings.Split(value, ".")
		var out [3]int
		for i := range out {
			out[i], _ = strconv.Atoi(parts[i])
		}
		return out
	}
	l, r := parse(left), parse(right)
	for i := range l {
		if l[i] < r[i] {
			return -1
		}
		if l[i] > r[i] {
			return 1
		}
	}
	return 0
}
