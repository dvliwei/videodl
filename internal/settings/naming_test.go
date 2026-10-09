package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type mockFS struct {
	exists map[string]bool
}

func newMockFS() *mockFS {
	return &mockFS{exists: make(map[string]bool)}
}

func (m *mockFS) Exists(p string) { m.exists[p] = true }

func (m *mockFS) Stat(path string) (os.FileInfo, error) {
	if m.exists[path] {
		return &mockFileInfo{name: filepath.Base(path)}, nil
	}
	return nil, os.ErrNotExist
}

type mockFileInfo struct{ name string }

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 0 }
func (m *mockFileInfo) Mode() os.FileMode  { return 0 }
func (m *mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() interface{}   { return nil }

func TestResolveConflict_NotExist(t *testing.T) {
	mock := newMockFS()
	got, err := ResolveConflict("/some/dir/file.mp4", ConflictAutoRename, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/some/dir/file.mp4" {
		t.Errorf("got %q, want original path", got)
	}
}

func TestResolveConflict_Overwrite(t *testing.T) {
	mock := newMockFS()
	mock.Exists("/some/dir/file.mp4")

	got, err := ResolveConflict("/some/dir/file.mp4", ConflictOverwrite, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/some/dir/file.mp4" {
		t.Errorf("got %q, want original path (overwrite)", got)
	}
}

func TestResolveConflict_Skip(t *testing.T) {
	mock := newMockFS()
	mock.Exists("/some/dir/file.mp4")

	_, err := ResolveConflict("/some/dir/file.mp4", ConflictSkip, mock)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if _, ok := err.(*ConflictError); !ok {
		t.Errorf("expected *ConflictError, got %T: %v", err, err)
	}
}

func TestResolveConflict_AutoRename_NoExt(t *testing.T) {
	mock := newMockFS()
	mock.Exists("/dir/readme")
	mock.Exists("/dir/readme (1)")

	got, err := ResolveConflict("/dir/readme", ConflictAutoRename, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/dir/readme (2)" {
		t.Errorf("got %q, want %q", got, "/dir/readme (2)")
	}
}

func TestResolveConflict_AutoRename_WithExt(t *testing.T) {
	mock := newMockFS()
	mock.Exists("/dir/video.mp4")

	got, err := ResolveConflict("/dir/video.mp4", ConflictAutoRename, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/dir/video (1).mp4" {
		t.Errorf("got %q, want %q", got, "/dir/video (1).mp4")
	}
}

func TestResolveConflict_AutoRename_MultipleExisting(t *testing.T) {
	mock := newMockFS()
	mock.Exists("/dir/video.mp4")
	mock.Exists("/dir/video (1).mp4")
	mock.Exists("/dir/video (2).mp4")
	mock.Exists("/dir/video (3).mp4")

	got, err := ResolveConflict("/dir/video.mp4", ConflictAutoRename, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/dir/video (4).mp4" {
		t.Errorf("got %q, want %q", got, "/dir/video (4).mp4")
	}
}

func TestResolveConflict_AutoRename_MultiDotExt(t *testing.T) {
	mock := newMockFS()
	mock.Exists("/dir/archive.tar.gz")

	got, err := ResolveConflict("/dir/archive.tar.gz", ConflictAutoRename, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/dir/archive.tar (1).gz" {
		t.Errorf("got %q, want %q", got, "/dir/archive.tar (1).gz")
	}
}

func TestResolveConflict_DefaultNilFS(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "file.txt")

	got, err := ResolveConflict(target, ConflictAutoRename, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != target {
		t.Errorf("got %q, want %q", got, target)
	}
}

func TestResolveConflict_RealFilesystem(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveConflict(target, ConflictAutoRename, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(got, "/video (1).mp4") && !strings.HasSuffix(got, `\video (1).mp4`) {
		t.Errorf("unexpected renamed path: %q", got)
	}

	if err := os.WriteFile(got, []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	got2, err := ResolveConflict(target, ConflictAutoRename, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(got2, "video (2).mp4") {
		t.Errorf("unexpected second rename: %q", got2)
	}
}

func TestResolveConflict_StatError(t *testing.T) {
	mock := &errorMockFS{statErr: os.ErrPermission}
	_, err := ResolveConflict("/dir/file.mp4", ConflictAutoRename, mock)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestItota(t *testing.T) {
	cases := map[int]string{
		0:    "0",
		1:    "1",
		5:    "5",
		9:    "9",
		10:   "10",
		123:  "123",
		9999: "9999",
	}
	for in, want := range cases {
		got := itoa(in)
		if got != want {
			t.Errorf("itoa(%d) = %q, want %q", in, got, want)
		}
	}
}

type errorMockFS struct {
	statErr error
}

func (m *errorMockFS) Stat(path string) (os.FileInfo, error) {
	return nil, m.statErr
}
