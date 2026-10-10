package ytdlp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeReleaseClient struct {
	release Release
	payload []byte
	err     error
	block   <-chan struct{}
	mu      sync.Mutex
	calls   int
}

func (c *fakeReleaseClient) LatestStable(context.Context) (Release, error) {
	if c.err != nil {
		return Release{}, c.err
	}
	return c.release, nil
}

func (c *fakeReleaseClient) Download(ctx context.Context, _ Asset, dst io.Writer) error {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	if c.block != nil {
		select {
		case <-c.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	_, err := dst.Write(c.payload)
	return err
}

type updateResolver struct {
	bundled BinaryCandidate
	user    BinaryCandidate
	userErr error
}

func (r updateResolver) ResolveBundledCandidate(string) (BinaryCandidate, error) {
	return r.bundled, nil
}
func (r updateResolver) ResolveUserCandidate(string) (BinaryCandidate, error) {
	if r.userErr != nil {
		return BinaryCandidate{}, r.userErr
	}
	return r.user, nil
}

func TestUpdaterStatusAndUpdate(t *testing.T) {
	asset := mustCurrentAsset(t)
	userDir := t.TempDir()
	bundled := filepath.Join(t.TempDir(), asset.Filename)
	writeExecutable(t, bundled)
	payload := []byte("#!/bin/sh\nprintf '%s\\n' '2026.09.01'\n")
	client := &fakeReleaseClient{release: Release{
		Repository: OfficialRepository,
		Tag:        "2026.09.01",
		Version:    "2026.09.01",
		Assets:     []Asset{{Name: asset.Filename, PlatformKey: asset.Key, URL: asset.URL, SHA256: checksum(payload)}},
	}, payload: payload}
	resolver := updateResolver{bundled: BinaryCandidate{Path: bundled, Version: BundledVersion, PlatformKey: asset.Key, Source: BinarySourceBundled}, userErr: ErrBinaryNotFound}
	updater := NewUpdater(client, resolver, userDir)
	status, err := updater.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Available || status.CurrentVersion != BundledVersion || status.LatestVersion != "2026.09.01" || status.Source != string(BinarySourceBundled) {
		t.Fatalf("status = %#v", status)
	}
	status, err = updater.Update(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Source != string(BinarySourceUser) || status.CurrentVersion != "2026.09.01" || status.Available {
		t.Fatalf("updated status = %#v", status)
	}
	if client.calls != 1 {
		t.Fatalf("download calls = %d", client.calls)
	}
}

func TestUpdaterRejectsUnknownRepositoryAndAsset(t *testing.T) {
	asset := mustCurrentAsset(t)
	resolver := updateResolver{bundled: BinaryCandidate{Version: BundledVersion, PlatformKey: asset.Key}, userErr: ErrBinaryNotFound}
	for name, release := range map[string]Release{
		"repository": {Repository: "evil.example/yt-dlp", Version: "2026.09.01"},
		"asset":      {Repository: OfficialRepository, Version: "2026.09.01", Assets: []Asset{{Name: "unknown", PlatformKey: asset.Key, SHA256: strings.Repeat("a", 64)}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewUpdater(&fakeReleaseClient{release: release}, resolver, t.TempDir()).Update(context.Background())
			if err == nil {
				t.Fatal("expected update rejection")
			}
		})
	}
}

func TestUpdaterChecksumFailureKeepsPreviousBinary(t *testing.T) {
	asset := mustCurrentAsset(t)
	userRoot := filepath.Join(t.TempDir(), "yt-dlp")
	oldPath := filepath.Join(userRoot, "tools", asset.ResourceDir, asset.Filename)
	old := []byte("old-good-binary")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, old, 0o755); err != nil {
		t.Fatal(err)
	}
	client := &fakeReleaseClient{release: Release{Repository: OfficialRepository, Version: "2026.09.01", Assets: []Asset{{Name: asset.Filename, PlatformKey: asset.Key, SHA256: strings.Repeat("0", 64)}}}, payload: []byte("new")}
	resolver := updateResolver{user: BinaryCandidate{Path: oldPath, Version: BundledVersion, PlatformKey: asset.Key, Source: BinarySourceUser}}
	_, err := NewUpdater(client, resolver, filepath.Dir(userRoot)).Update(context.Background())
	if err == nil {
		t.Fatal("expected checksum failure")
	}
	got, readErr := os.ReadFile(oldPath)
	if readErr != nil || string(got) != string(old) {
		t.Fatalf("old binary changed: %q, err=%v", got, readErr)
	}
}

func TestUpdaterCancellationAndMutex(t *testing.T) {
	asset := mustCurrentAsset(t)
	block := make(chan struct{})
	client := &fakeReleaseClient{release: Release{Repository: OfficialRepository, Version: "2026.09.01", Assets: []Asset{{Name: asset.Filename, PlatformKey: asset.Key, URL: asset.URL, SHA256: strings.Repeat("a", 64)}}}, payload: []byte("new"), block: block}
	resolver := updateResolver{bundled: BinaryCandidate{Version: BundledVersion, PlatformKey: asset.Key}, userErr: ErrBinaryNotFound}
	updater := NewUpdater(client, resolver, t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	firstDone := make(chan error, 1)
	go func() { _, err := updater.Update(ctx); firstDone <- err }()
	time.Sleep(5 * time.Millisecond)
	if _, err := updater.Update(context.Background()); !errors.Is(err, ErrUpdateInProgress) {
		t.Fatalf("expected mutex error, got %v", err)
	}
	if err := <-firstDone; !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestUpdaterRollbackOnInvalidExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell health fixture is Unix-only")
	}
	asset := mustCurrentAsset(t)
	userRoot := filepath.Join(t.TempDir(), "yt-dlp")
	oldPath := filepath.Join(userRoot, "tools", asset.ResourceDir, asset.Filename)
	old := []byte("#!/bin/sh\nprintf '%s\\n' '2026.08.19'\n")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, old, 0o755); err != nil {
		t.Fatal(err)
	}
	newPayload := []byte("#!/bin/sh\nprintf '%s\\n' 'not-the-release'\n")
	client := &fakeReleaseClient{release: Release{Repository: OfficialRepository, Version: "2026.09.01", Assets: []Asset{{Name: asset.Filename, PlatformKey: asset.Key, SHA256: checksum(newPayload)}}}, payload: newPayload}
	resolver := updateResolver{user: BinaryCandidate{Path: oldPath, Version: BundledVersion, PlatformKey: asset.Key, Source: BinarySourceUser}}
	_, err := NewUpdater(client, resolver, filepath.Dir(userRoot)).Update(context.Background())
	if err == nil {
		t.Fatal("expected health verification failure")
	}
	got, readErr := os.ReadFile(oldPath)
	if readErr != nil || string(got) != string(old) {
		t.Fatalf("rollback failed: %q, err=%v", got, readErr)
	}
}

func mustCurrentAsset(t *testing.T) PlatformAsset {
	t.Helper()
	asset, err := PlatformAssetFor(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	return asset
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
