package settings

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func UniqueTempPath(dir, prefix, suffix string) (string, error) {
	if dir == "" {
		dir = os.TempDir()
	}

	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return "", &NotExistError{Path: dir}
		}
		return "", &filesystemError{path: dir, op: "stat", err: err}
	}

	token, err := randomHex(8)
	if err != nil {
		return "", err
	}

	ts := time.Now().UnixNano()
	name := fmt.Sprintf("%s%d_%s%s", sanitizePrefix(prefix), ts, token, suffix)

	return filepath.Join(dir, name), nil
}

func UniqueTempDir(parent, prefix string) (string, error) {
	if parent == "" {
		parent = os.TempDir()
	}

	if _, err := os.Stat(parent); err != nil {
		if os.IsNotExist(err) {
			return "", &NotExistError{Path: parent}
		}
		return "", &filesystemError{path: parent, op: "stat", err: err}
	}

	token, err := randomHex(8)
	if err != nil {
		return "", err
	}

	ts := time.Now().UnixNano()
	name := fmt.Sprintf("%s%d_%s", sanitizePrefix(prefix), ts, token)
	full := filepath.Join(parent, name)

	if err := os.MkdirAll(full, 0o700); err != nil {
		return "", &filesystemError{path: full, op: "mkdir", err: err}
	}

	return full, nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func sanitizePrefix(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, "/", "_")
	p = strings.ReplaceAll(p, "\\", "_")
	if p == "" {
		p = "videodl_tmp"
	}
	return p
}
