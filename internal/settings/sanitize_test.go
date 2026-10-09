package settings

import (
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeFileName_EmptyString(t *testing.T) {
	got := SanitizeFileName("")
	if got != DefaultFileName {
		t.Errorf("empty input = %q, want %q", got, DefaultFileName)
	}
}

func TestSanitizeFileName_PlainName(t *testing.T) {
	cases := []string{"video.mp4", "My Clip", "clip_01.webm"}
	for _, name := range cases {
		got := SanitizeFileName(name)
		if got != name {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", name, got, name)
		}
	}
}

func TestSanitizeFileName_Unicode(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"日本語の動画.mp4", "日本語の動画.mp4"},
		{"한국어 클립.mkv", "한국어 클립.mkv"},
		{"Русское имя.webm", "Русское имя.webm"},
		{"Emoji🎉Clip👌.mp4", "Emoji🎉Clip👌.mp4"},
		{"中文视频文件.mov", "中文视频文件.mov"},
	}
	for _, c := range cases {
		got := SanitizeFileName(c.input)
		if got != c.want {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestSanitizeFileName_PathSeparators(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"path/to/file.mp4", "pathtofile.mp4"},
		{"path\\to\\file.mp4", "pathtofile.mp4"},
		{"mix/and\\match.mp4", "mixandmatch.mp4"},
	}
	for _, c := range cases {
		got := SanitizeFileName(c.input)
		if got != c.want {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestSanitizeFileName_ControlCharacters(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"file\x00name.mp4", "filename.mp4"},
		{"file\x01\x02name.mp4", "filename.mp4"},
		{"file\x00\x1fname.mp4", "filename.mp4"},
		{"file\x7fname.mp4", "filename.mp4"},
		{"\x00\x01\x02", DefaultFileName},
	}
	for _, c := range cases {
		got := SanitizeFileName(c.input)
		if got != c.want {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestSanitizeFileName_WhitespaceHandling(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"  file.mp4  ", "file.mp4"},
		{"\t\nfile.mp4\n\t", "file.mp4"},
		{"   ", DefaultFileName},
	}
	for _, c := range cases {
		got := SanitizeFileName(c.input)
		if got != c.want {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestSanitizeFileName_LengthLimit(t *testing.T) {
	longName := strings.Repeat("a", 300) + ".mp4"
	got := SanitizeFileName(longName)
	if len([]rune(got)) > MaxFileNameLength {
		t.Errorf("filename rune count = %d, want <= %d", len([]rune(got)), MaxFileNameLength)
	}
	if !utf8.ValidString(got) {
		t.Errorf("result is not valid UTF-8: %q", got)
	}
}

func TestSanitizeFileName_LengthLimitUnicode(t *testing.T) {
	longName := strings.Repeat("日", 300) + ".mp4"
	got := SanitizeFileName(longName)
	runes := []rune(got)
	if len(runes) > MaxFileNameLength {
		t.Errorf("filename rune count = %d, want <= %d", len(runes), MaxFileNameLength)
	}
	if !utf8.ValidString(got) {
		t.Errorf("result is not valid UTF-8: %q", got)
	}
	if !strings.HasSuffix(got, ".mp4") {
		t.Errorf("should preserve suffix, got %q", got)
	}
}

func TestSanitizeFileName_WindowsReservedChars(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	cases := []struct {
		input string
		want  string
	}{
		{"file<name>.mp4", "filename.mp4"},
		{"file:name.mp4", "filename.mp4"},
		{"file\"name.mp4", "filename.mp4"},
		{"file|name.mp4", "filename.mp4"},
		{"file?name.mp4", "filename.mp4"},
		{"file*name.mp4", "filename.mp4"},
		{"<>:\"|?*", DefaultFileName},
	}
	for _, c := range cases {
		got := SanitizeFileName(c.input)
		if got != c.want {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestSanitizeFileName_WindowsReservedNames(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	reserved := []string{
		"CON", "con", "Con",
		"PRN", "prn",
		"AUX", "aux",
		"NUL", "nul",
		"COM1", "com1", "Com1",
		"COM9",
		"LPT1", "lpt1",
		"LPT9",
		"CON.mp4", "con.tar.gz",
	}

	for _, name := range reserved {
		got := SanitizeFileName(name)
		base := strings.ToLower(firstDotSegment(got))
		if _, ok := windowsReservedNames[base]; ok {
			t.Errorf("SanitizeFileName(%q) still produces reserved name %q", name, got)
		}
		if !strings.HasPrefix(got, "_") && got != DefaultFileName {
			t.Errorf("SanitizeFileName(%q) = %q, want underscore prefix", name, got)
		}
	}
}

func TestSanitizeFileName_WindowsTrailingDotAndSpace(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	cases := []struct {
		input string
		want  string
	}{
		{"file.mp4.", "file.mp4"},
		{"file....", "file"},
		{"file   .mp4", "file.mp4"},
	}
	for _, c := range cases {
		got := SanitizeFileName(c.input)
		if got != c.want {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestSanitizeFileName_WindowsColon(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	got := SanitizeFileName("file:name:with:colons.mp4")
	want := "filenamewithcolons.mp4"
	if got != want {
		t.Errorf("SanitizeFileName(%q) = %q, want %q", "file:name:with:colons.mp4", got, want)
	}
}

func TestSanitizeFileName_DarwinColon(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific test")
	}

	got := SanitizeFileName("file:name.mp4")
	if strings.Contains(got, ":") {
		t.Errorf("SanitizeFileName(%q) should strip colons, got %q", "file:name.mp4", got)
	}
}

func TestSanitizeFileName_AllPlatforms_NoSlashOrBackslash(t *testing.T) {
	cases := []string{"a/b/c", "a\\b\\c", "/absolute", "\\absolute"}
	for _, name := range cases {
		got := SanitizeFileName(name)
		if strings.Contains(got, "/") || strings.Contains(got, "\\") {
			t.Errorf("SanitizeFileName(%q) = %q, still contains path separator", name, got)
		}
	}
}

func TestSanitizeFileName_AllPlatforms_ControlCharsStripped(t *testing.T) {
	input := "hello\x00\x01\x1f\x7fworld"
	got := SanitizeFileName(input)
	want := "helloworld"
	if got != want {
		t.Errorf("SanitizeFileName(%q) = %q, want %q", input, got, want)
	}
}

func TestSanitizeFileName_AllPlatforms_NilByte(t *testing.T) {
	got := SanitizeFileName("abc\x00def")
	if strings.Contains(got, "\x00") {
		t.Errorf("result still contains null byte: %q", got)
	}
}
