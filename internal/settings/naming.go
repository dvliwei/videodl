package settings

import (
	"os"
	"path/filepath"
	"strings"
)

type ConflictStrategy int

const (
	ConflictAutoRename ConflictStrategy = iota
	ConflictOverwrite
	ConflictSkip
)

type FileSystem interface {
	Stat(path string) (os.FileInfo, error)
}

type realFS struct{}

func (realFS) Stat(path string) (os.FileInfo, error) { return os.Stat(path) }

var defaultFS FileSystem = realFS{}

func ResolveConflict(targetPath string, strategy ConflictStrategy, fs FileSystem) (string, error) {
	if fs == nil {
		fs = defaultFS
	}

	if _, err := fs.Stat(targetPath); os.IsNotExist(err) {
		return targetPath, nil
	} else if err != nil {
		return "", &filesystemError{path: targetPath, op: "stat", err: err}
	}

	switch strategy {
	case ConflictOverwrite:
		return targetPath, nil
	case ConflictSkip:
		return "", &ConflictError{Path: targetPath}
	case ConflictAutoRename:
		return autoRename(targetPath, fs)
	default:
		return autoRename(targetPath, fs)
	}
}

func autoRename(targetPath string, fs FileSystem) (string, error) {
	dir := filepath.Dir(targetPath)
	ext := filepath.Ext(targetPath)
	base := strings.TrimSuffix(filepath.Base(targetPath), ext)

	for i := 1; i < 10000; i++ {
		candidate := filepath.Join(dir, base+" ("+itoa(i)+")"+ext)
		if _, err := fs.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", &filesystemError{path: candidate, op: "stat", err: err}
		}
	}

	return "", &filesystemError{path: targetPath, op: "resolve", err: errTooManyAttempts}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
