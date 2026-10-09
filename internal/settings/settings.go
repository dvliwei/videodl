package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

type FileSystemOps interface {
	Stat(path string) (os.FileInfo, error)
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(filename string, data []byte, perm os.FileMode) error
	ReadFile(filename string) ([]byte, error)
	CreateTemp(dir, prefix string) (*os.File, error)
	Remove(path string) error
}

type realFSOps struct{}

func (realFSOps) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}
func (realFSOps) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
func (realFSOps) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}
func (realFSOps) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}
func (realFSOps) CreateTemp(dir, prefix string) (*os.File, error) {
	return os.CreateTemp(dir, prefix)
}
func (realFSOps) Remove(path string) error {
	return os.Remove(path)
}

var defaultFSOps FileSystemOps = realFSOps{}

type Config struct {
	DownloadDirectory string `json:"downloadDirectory,omitempty"`
}

type PlatformPaths interface {
	UserConfigDir() (string, error)
	UserHomeDir() (string, error)
	DefaultDownloadDir() string
}

type realPlatform struct{}

func (realPlatform) UserConfigDir() (string, error) { return os.UserConfigDir() }
func (realPlatform) UserHomeDir() (string, error)   { return os.UserHomeDir() }
func (realPlatform) DefaultDownloadDir() string {
	if runtime.GOOS == "windows" {
		return "Downloads"
	}
	return "Downloads"
}

var defaultPlatform PlatformPaths = realPlatform{}

type Settings struct {
	fs       FileSystemOps
	platform PlatformPaths
}

func NewSettings() *Settings {
	return &Settings{fs: defaultFSOps, platform: defaultPlatform}
}

func NewSettingsWith(fs FileSystemOps, p PlatformPaths) *Settings {
	return &Settings{fs: fs, platform: p}
}

func (s *Settings) ConfigPath() (string, error) {
	cfgDir, err := s.platform.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "videodl", "settings.json"), nil
}

func (s *Settings) Load() (*Config, error) {
	path, err := s.ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := s.fs.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, &filesystemError{path: path, op: "read", err: err}
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *Settings) Save(cfg *Config) error {
	path, err := s.ConfigPath()
	if err != nil {
		return err
	}

	if err := s.fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return &filesystemError{path: filepath.Dir(path), op: "mkdir", err: err}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := s.fs.WriteFile(path, data, 0o644); err != nil {
		return &filesystemError{path: path, op: "write", err: err}
	}
	return nil
}

func (s *Settings) GetDownloadDirectory() (string, error) {
	cfg, err := s.Load()
	if err != nil {
		return "", err
	}

	if cfg.DownloadDirectory != "" {
		if ok, err := s.DirectoryExists(cfg.DownloadDirectory); ok && err == nil {
			return cfg.DownloadDirectory, nil
		}
	}

	return s.DefaultDownloadDirectory()
}

func (s *Settings) SetDownloadDirectory(path string) error {
	if path == "" {
		return errors.New("settings: download directory path is empty")
	}

	if err := s.EnsureDirectory(path); err != nil {
		return err
	}

	cfg, err := s.Load()
	if err != nil {
		return err
	}
	cfg.DownloadDirectory = path
	return s.Save(cfg)
}

func (s *Settings) DefaultDownloadDirectory() (string, error) {
	home, err := s.platform.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, s.platform.DefaultDownloadDir()), nil
}

func (s *Settings) DirectoryExists(path string) (bool, error) {
	info, err := s.fs.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}
	return true, nil
}

func (s *Settings) IsWritable(path string) error {
	ok, err := s.DirectoryExists(path)
	if err != nil {
		return err
	}
	if !ok {
		return &NotExistError{Path: path}
	}

	tmpFile, err := s.fs.CreateTemp(path, ".videodl_write_test_*")
	if err != nil {
		return &NotWritableError{Path: path, Err: err}
	}

	name := tmpFile.Name()
	tmpFile.Close()

	if err := s.fs.Remove(name); err != nil {
		return &NotWritableError{Path: path, Err: err}
	}

	return nil
}

func (s *Settings) EnsureDirectory(path string) error {
	if path == "" {
		return errors.New("settings: path is empty")
	}

	ok, err := s.DirectoryExists(path)
	if err != nil {
		return err
	}
	if !ok {
		if err := s.fs.MkdirAll(path, 0o755); err != nil {
			return &filesystemError{path: path, op: "mkdir", err: err}
		}
	}

	return s.IsWritable(path)
}
