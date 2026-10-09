package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUniqueTempPath_Basic(t *testing.T) {
	dir := t.TempDir()

	p1, err := UniqueTempPath(dir, "video", ".mp4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p2, err := UniqueTempPath(dir, "video", ".mp4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p1 == p2 {
		t.Errorf("two calls produced identical paths: %q", p1)
	}

	if filepath.Dir(p1) != dir {
		t.Errorf("unexpected directory: got %q want %q", filepath.Dir(p1), dir)
	}

	if !filepath.IsAbs(p1) {
		t.Errorf("path should be absolute: %q", p1)
	}
}

func TestUniqueTempPath_CustomPrefixSuffix(t *testing.T) {
	dir := t.TempDir()

	p, err := UniqueTempPath(dir, "myprefix_", ".part")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	base := filepath.Base(p)
	if !startsWith(base, "myprefix_") {
		t.Errorf("prefix not found in %q", base)
	}
	if !endsWith(base, ".part") {
		t.Errorf("suffix not found in %q", base)
	}
}

func TestUniqueTempPath_EmptyDefaults(t *testing.T) {
	dir := t.TempDir()

	p, err := UniqueTempPath(dir, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	base := filepath.Base(p)
	if !startsWith(base, "videodl_tmp") {
		t.Errorf("default prefix not found in %q", base)
	}
}

func TestUniqueTempPath_NonExistentDir(t *testing.T) {
	_, err := UniqueTempPath("/nonexistent/path/xyz123", "t", ".tmp")
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
	if _, ok := err.(*NotExistError); !ok {
		t.Errorf("expected *NotExistError, got %T: %v", err, err)
	}
}

func TestUniqueTempPath_UsesDefaultTempDir(t *testing.T) {
	p, err := UniqueTempPath("", "test", ".tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Clean(filepath.Dir(p)) != filepath.Clean(os.TempDir()) {
		t.Errorf("should use os.TempDir(), got %q want %q", filepath.Dir(p), os.TempDir())
	}
}

func TestUniqueTempPath_1000Unique(t *testing.T) {
	dir := t.TempDir()

	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		p, err := UniqueTempPath(dir, "batch", ".tmp")
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if seen[p] {
			t.Errorf("duplicate path at iteration %d: %q", i, p)
		}
		seen[p] = true
	}
}

func TestUniqueTempDir_Basic(t *testing.T) {
	parent := t.TempDir()

	d1, err := UniqueTempDir(parent, "work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d2, err := UniqueTempDir(parent, "work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if d1 == d2 {
		t.Errorf("two calls produced identical dirs: %q", d1)
	}

	info, err := os.Stat(d1)
	if err != nil {
		t.Fatalf("temp dir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("temp path should be a directory")
	}
}

func TestUniqueTempDir_Cleanup(t *testing.T) {
	parent := t.TempDir()

	d, err := UniqueTempDir(parent, "cleanup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := os.RemoveAll(d); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(d); !os.IsNotExist(err) {
		t.Error("directory should be removed")
	}
}

func TestUniqueTempDir_DefaultParent(t *testing.T) {
	d, err := UniqueTempDir("", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.RemoveAll(d)

	if filepath.Clean(filepath.Dir(d)) != filepath.Clean(os.TempDir()) {
		t.Errorf("should use os.TempDir(), got %q want %q", filepath.Dir(d), os.TempDir())
	}
}

func TestUniqueTempDir_NonExistentParent(t *testing.T) {
	_, err := UniqueTempDir("/nonexistent/xyz123", "t")
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*NotExistError); !ok {
		t.Errorf("expected *NotExistError, got %T: %v", err, err)
	}
}

func TestSanitizePrefix(t *testing.T) {
	cases := map[string]string{
		"hello":         "hello",
		"  hello  ":     "hello",
		"a/b":           "a_b",
		"a\\b":          "a_b",
		"   ":           "videodl_tmp",
		"":              "videodl_tmp",
		"my/video\\job": "my_video_job",
	}
	for in, want := range cases {
		got := sanitizePrefix(in)
		if got != want {
			t.Errorf("sanitizePrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRandomHex_Length(t *testing.T) {
	hex, err := randomHex(4)
	if err != nil {
		t.Fatal(err)
	}
	if len(hex) != 8 {
		t.Errorf("4 bytes hex should be 8 chars, got %d", len(hex))
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
