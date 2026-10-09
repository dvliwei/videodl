package settings

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type memFS struct {
	files map[string][]byte
	dirs  map[string]bool
}

func newMemFS() *memFS {
	return &memFS{files: make(map[string][]byte), dirs: make(map[string]bool)}
}

func (m *memFS) Stat(path string) (os.FileInfo, error) {
	if m.dirs[path] {
		return &memFileInfo{name: filepath.Base(path), isDir: true}, nil
	}
	if data, ok := m.files[path]; ok {
		return &memFileInfo{name: filepath.Base(path), size: int64(len(data)), isDir: false}, nil
	}
	return nil, os.ErrNotExist
}

func (m *memFS) MkdirAll(path string, perm os.FileMode) error {
	m.dirs[path] = true
	return nil
}

func (m *memFS) WriteFile(filename string, data []byte, perm os.FileMode) error {
	if err := m.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	m.files[filename] = append([]byte{}, data...)
	return nil
}

func (m *memFS) ReadFile(filename string) ([]byte, error) {
	data, ok := m.files[filename]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte{}, data...), nil
}

func (m *memFS) CreateTemp(dir, prefix string) (*os.File, error) {
	if !m.dirs[dir] {
		return nil, os.ErrNotExist
	}
	token, _ := randomHex(4)
	name := filepath.Join(dir, prefix+"_random_"+token)
	m.files[name] = []byte{}
	return &os.File{}, nil
}

func (m *memFS) Remove(path string) error {
	if _, ok := m.files[path]; ok {
		delete(m.files, path)
		return nil
	}
	if m.dirs[path] {
		delete(m.dirs, path)
		return nil
	}
	return os.ErrNotExist
}

type memFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (m *memFileInfo) Name() string       { return m.name }
func (m *memFileInfo) Size() int64        { return m.size }
func (m *memFileInfo) Mode() os.FileMode  { return 0o644 }
func (m *memFileInfo) ModTime() time.Time { return time.Time{} }
func (m *memFileInfo) IsDir() bool        { return m.isDir }
func (m *memFileInfo) Sys() interface{}   { return nil }

type fakePlatform struct {
	configDir string
	homeDir   string
}

func (f fakePlatform) UserConfigDir() (string, error) { return f.configDir, nil }
func (f fakePlatform) UserHomeDir() (string, error)   { return f.homeDir, nil }
func (f fakePlatform) DefaultDownloadDir() string     { return "Downloads" }

func TestSettings_ConfigPath(t *testing.T) {
	fs := newMemFS()
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home"}
	s := NewSettingsWith(fs, platform)

	path, err := s.ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	want := "/cfg/videodl/settings.json"
	if path != want {
		t.Errorf("ConfigPath() = %q, want %q", path, want)
	}
}

func TestSettings_Load_NoFile(t *testing.T) {
	fs := newMemFS()
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home"}
	s := NewSettingsWith(fs, platform)

	cfg, err := s.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DownloadDirectory != "" {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestSettings_SaveAndLoad(t *testing.T) {
	fs := newMemFS()
	fs.MkdirAll("/cfg/videodl", 0o755)
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home"}
	s := NewSettingsWith(fs, platform)

	original := &Config{DownloadDirectory: "/custom/downloads"}
	if err := s.Save(original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.DownloadDirectory != original.DownloadDirectory {
		t.Errorf("DownloadDirectory mismatch: got %q want %q", loaded.DownloadDirectory, original.DownloadDirectory)
	}
}

func TestSettings_Save_CreatesParentDirs(t *testing.T) {
	fs := newMemFS()
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home"}
	s := NewSettingsWith(fs, platform)

	if err := s.Save(&Config{DownloadDirectory: "/x"}); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if !fs.dirs["/cfg/videodl"] {
		t.Error("expected parent dirs to be created")
	}
}

func TestSettings_DirectoryExists_True(t *testing.T) {
	fs := newMemFS()
	fs.MkdirAll("/some/dir", 0o755)
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home"}
	s := NewSettingsWith(fs, platform)

	ok, err := s.DirectoryExists("/some/dir")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected directory to exist")
	}
}

func TestSettings_DirectoryExists_False(t *testing.T) {
	fs := newMemFS()
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home"}
	s := NewSettingsWith(fs, platform)

	ok, err := s.DirectoryExists("/no/such/dir")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected directory to not exist")
	}
}

func TestSettings_DirectoryExists_FileNotDir(t *testing.T) {
	fs := newMemFS()
	fs.files["/some/file"] = []byte("hello")
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home"}
	s := NewSettingsWith(fs, platform)

	ok, err := s.DirectoryExists("/some/file")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("file should not be considered a directory")
	}
}

func TestSettings_IsWritable_NotExist(t *testing.T) {
	fs := newMemFS()
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home"}
	s := NewSettingsWith(fs, platform)

	err := s.IsWritable("/nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	var notExist *NotExistError
	if !errors.As(err, &notExist) {
		t.Errorf("expected *NotExistError, got %T: %v", err, err)
	}
}

func TestSettings_EnsureDirectory_Creates(t *testing.T) {
	fs := newMemFS()
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home"}
	s := NewSettingsWith(fs, platform)

	target := "/home/Downloads"
	if err := s.EnsureDirectory(target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fs.dirs[target] {
		t.Error("directory should be created")
	}
}

func TestSettings_EnsureDirectory_AlreadyExists(t *testing.T) {
	fs := newMemFS()
	fs.MkdirAll("/home/Downloads", 0o755)
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home"}
	s := NewSettingsWith(fs, platform)

	if err := s.EnsureDirectory("/home/Downloads"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSettings_EnsureDirectory_EmptyPath(t *testing.T) {
	fs := newMemFS()
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home"}
	s := NewSettingsWith(fs, platform)

	if err := s.EnsureDirectory(""); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestSettings_DefaultDownloadDirectory(t *testing.T) {
	fs := newMemFS()
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home/user"}
	s := NewSettingsWith(fs, platform)

	dir, err := s.DefaultDownloadDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if dir != "/home/user/Downloads" {
		t.Errorf("got %q, want %q", dir, "/home/user/Downloads")
	}
}

func TestSettings_SetDownloadDirectory_Saves(t *testing.T) {
	fs := newMemFS()
	fs.MkdirAll("/custom/dl", 0o755)
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home"}
	s := NewSettingsWith(fs, platform)

	if err := s.SetDownloadDirectory("/custom/dl"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DownloadDirectory != "/custom/dl" {
		t.Errorf("DownloadDirectory = %q, want %q", cfg.DownloadDirectory, "/custom/dl")
	}
}

func TestSettings_SetDownloadDirectory_EmptyPath(t *testing.T) {
	fs := newMemFS()
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home"}
	s := NewSettingsWith(fs, platform)

	if err := s.SetDownloadDirectory(""); err == nil {
		t.Error("expected error")
	}
}

func TestSettings_SetDownloadDirectory_CreatesDir(t *testing.T) {
	fs := newMemFS()
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home"}
	s := NewSettingsWith(fs, platform)

	if err := s.SetDownloadDirectory("/new/downloads"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fs.dirs["/new/downloads"] {
		t.Error("directory should be created")
	}
}

func TestSettings_GetDownloadDirectory_NoSaved(t *testing.T) {
	fs := newMemFS()
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home/user"}
	s := NewSettingsWith(fs, platform)

	// No saved config, returns default
	dir, err := s.GetDownloadDirectory()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir != "/home/user/Downloads" {
		t.Errorf("got %q, want %q", dir, "/home/user/Downloads")
	}
}

func TestSettings_GetDownloadDirectory_SavedValid(t *testing.T) {
	fs := newMemFS()
	fs.MkdirAll("/custom/dl", 0o755)
	fs.MkdirAll("/cfg/videodl", 0o755)
	fs.files["/cfg/videodl/settings.json"] = []byte(`{"downloadDirectory":"/custom/dl"}`)
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home/user"}
	s := NewSettingsWith(fs, platform)

	dir, err := s.GetDownloadDirectory()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir != "/custom/dl" {
		t.Errorf("got %q, want saved %q", dir, "/custom/dl")
	}
}

func TestSettings_GetDownloadDirectory_SavedInvalidFallsBack(t *testing.T) {
	fs := newMemFS()
	// Saved dir doesn't exist, should fall back to default
	fs.MkdirAll("/cfg/videodl", 0o755)
	fs.files["/cfg/videodl/settings.json"] = []byte(`{"downloadDirectory":"/gone/missing"}`)
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home/user"}
	s := NewSettingsWith(fs, platform)

	dir, err := s.GetDownloadDirectory()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir != "/home/user/Downloads" {
		t.Errorf("got %q, want default %q", dir, "/home/user/Downloads")
	}
}

func TestSettings_Save_InvalidJSONRoundTrip(t *testing.T) {
	fs := newMemFS()
	platform := fakePlatform{configDir: "/cfg", homeDir: "/home"}
	s := NewSettingsWith(fs, platform)

	if err := s.Save(&Config{DownloadDirectory: "/x/y/z"}); err != nil {
		t.Fatal(err)
	}

	data, err := fs.ReadFile("/cfg/videodl/settings.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("saved data should not be empty")
	}
}

func TestNotExistError_Implements(t *testing.T) {
	e := &NotExistError{Path: "/nope"}
	msg := e.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}

func TestNotWritableError_Implements(t *testing.T) {
	inner := errors.New("permission denied")
	e := &NotWritableError{Path: "/locked", Err: inner}
	msg := e.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
	if e.Unwrap() != inner {
		t.Error("Unwrap should return inner error")
	}
}
