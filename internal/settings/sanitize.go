package settings

import (
	"runtime"
	"strings"
	"unicode"
)

const (
	DefaultFileName   = "untitled"
	MaxFileNameLength = 255
)

var windowsReservedChars = map[rune]bool{
	'<': true, '>': true, ':': true, '"': true, '/': true,
	'\\': true, '|': true, '?': true, '*': true,
}

var windowsReservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true,
	"com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true,
	"lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

type platform struct {
	os string
}

func currentPlatform() platform {
	return platform{os: runtime.GOOS}
}

func (p platform) isWindows() bool {
	return p.os == "windows"
}

func (p platform) isDarwin() bool {
	return p.os == "darwin"
}

func (p platform) isLinux() bool {
	return p.os == "linux"
}

func SanitizeFileName(name string) string {
	return sanitizeForPlatform(name, currentPlatform())
}

func sanitizeForPlatform(name string, p platform) string {
	if name == "" {
		return DefaultFileName
	}

	var b strings.Builder
	b.Grow(len(name))

	for _, r := range name {
		if unicode.IsControl(r) {
			continue
		}
		if r == 0 {
			continue
		}
		if p.isWindows() {
			if windowsReservedChars[r] {
				continue
			}
		}
		if p.isDarwin() {
			if r == ':' {
				continue
			}
		}
		if r == '/' {
			continue
		}
		if r == '\\' {
			continue
		}
		b.WriteRune(r)
	}

	result := b.String()

	result = strings.TrimSpace(result)

	if p.isWindows() {
		result = strings.TrimRight(result, ".")
	}

	if result == "" {
		return DefaultFileName
	}

	if p.isWindows() {
		base := strings.ToLower(firstDotSegment(result))
		if windowsReservedNames[base] {
			result = "_" + result
		}
	}

	runes := []rune(result)
	if len(runes) > MaxFileNameLength {
		runes = runes[:MaxFileNameLength]
		result = string(runes)
	}

	if result == "" {
		return DefaultFileName
	}

	return result
}

func firstDotSegment(name string) string {
	runes := []rune(name)
	for i, r := range runes {
		if r == '.' {
			return string(runes[:i])
		}
	}
	return name
}
