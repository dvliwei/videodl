package download

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"videodl/internal/media"
	"videodl/internal/settings"
)

type OverwritePolicy int

const (
	OverwriteAutoRename OverwritePolicy = iota
	OverwriteAlways
	OverwriteNever
)

type PublisherConfig struct {
	OverwritePolicy OverwritePolicy
}

func DefaultPublisherConfig() PublisherConfig {
	return PublisherConfig{OverwritePolicy: OverwriteAutoRename}
}

type Publisher struct {
	cfg PublisherConfig

	mu       sync.Mutex
	dirLocks map[string]*sync.Mutex
}

func NewPublisher(cfg PublisherConfig) *Publisher {
	return &Publisher{
		cfg:      cfg,
		dirLocks: make(map[string]*sync.Mutex),
	}
}

func (p *Publisher) dirLock(dir string) func() {
	p.mu.Lock()
	lk, ok := p.dirLocks[dir]
	if !ok {
		lk = &sync.Mutex{}
		p.dirLocks[dir] = lk
	}
	p.mu.Unlock()
	lk.Lock()
	return lk.Unlock
}

func (p *Publisher) ResolveOutput(targetPath string) (string, error) {
	if targetPath == "" {
		return "", newPublishError("download.publish.empty_target", "output path is empty", nil)
	}

	dir := filepath.Dir(targetPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", newPublishError("download.publish.dir_missing",
			fmt.Sprintf("target directory does not exist: %s", dir), err)
	} else if err != nil {
		return "", newPublishError("download.publish.dir_stat",
			fmt.Sprintf("cannot access target directory: %s", dir), err)
	}

	unlock := p.dirLock(dir)
	defer unlock()

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return targetPath, nil
	} else if err != nil {
		return "", newPublishError("download.publish.target_stat",
			fmt.Sprintf("cannot check target file: %s", targetPath), err)
	}

	strategy := settings.ConflictStrategy(p.cfg.OverwritePolicy)
	resolved, err := settings.ResolveConflict(targetPath, strategy, nil)
	if err != nil {
		var ce *settings.ConflictError
		if errors.As(err, &ce) {
			return "", newPublishError("download.publish.conflict",
				fmt.Sprintf("file already exists: %s", targetPath), err)
		}
		return "", newPublishError("download.publish.resolve",
			fmt.Sprintf("failed to resolve filename conflict: %s", targetPath), err)
	}

	return resolved, nil
}

func (p *Publisher) Publish(tempFile, targetPath string) (string, error) {
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		return "", newPublishError("download.publish.source_missing",
			fmt.Sprintf("temp file not found: %s", tempFile), err)
	} else if err != nil {
		return "", newPublishError("download.publish.source_stat",
			fmt.Sprintf("cannot access temp file: %s", tempFile), err)
	}

	finalDir := filepath.Dir(targetPath)

	if err := os.MkdirAll(finalDir, 0o755); err != nil {
		return "", newPublishError("download.publish.mkdir",
			fmt.Sprintf("failed to ensure target directory: %s", finalDir), err)
	}

	stagingInTarget, cleanup, err := prepareInTargetDir(tempFile, finalDir)
	if err != nil {
		return "", err
	}
	defer cleanup()

	if err := verifyPublished(stagingInTarget); err != nil {
		return "", err
	}

	unlock := p.dirLock(finalDir)
	defer unlock()

	resolvedPath, err := settings.ResolveConflict(targetPath,
		settings.ConflictStrategy(p.cfg.OverwritePolicy), nil)
	if err != nil {
		var ce *settings.ConflictError
		if errors.As(err, &ce) {
			return "", newPublishError("download.publish.conflict",
				fmt.Sprintf("file already exists: %s", targetPath), err)
		}
		return "", newPublishError("download.publish.resolve",
			fmt.Sprintf("failed to resolve filename conflict: %s", targetPath), err)
	}

	if err := atomicReplace(stagingInTarget, resolvedPath); err != nil {
		return "", err
	}

	_ = os.Remove(tempFile)

	return resolvedPath, nil
}

func (p *Publisher) ValidateAndCopy(tempFile, targetPath string) (string, error) {
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		return "", newPublishError("download.publish.source_missing",
			fmt.Sprintf("temp file not found: %s", tempFile), err)
	} else if err != nil {
		return "", newPublishError("download.publish.source_stat",
			fmt.Sprintf("cannot access temp file: %s", tempFile), err)
	}

	finalDir := filepath.Dir(targetPath)

	if err := os.MkdirAll(finalDir, 0o755); err != nil {
		return "", newPublishError("download.publish.mkdir",
			fmt.Sprintf("failed to ensure target directory: %s", finalDir), err)
	}

	stagingInTarget, cleanup, err := prepareInTargetDir(tempFile, finalDir)
	if err != nil {
		return "", err
	}
	defer cleanup()

	if err := verifyPublished(stagingInTarget); err != nil {
		return "", err
	}

	unlock := p.dirLock(finalDir)
	defer unlock()

	resolvedPath, err := settings.ResolveConflict(targetPath,
		settings.ConflictStrategy(p.cfg.OverwritePolicy), nil)
	if err != nil {
		var ce *settings.ConflictError
		if errors.As(err, &ce) {
			return "", newPublishError("download.publish.conflict",
				fmt.Sprintf("file already exists: %s", targetPath), err)
		}
		return "", newPublishError("download.publish.resolve",
			fmt.Sprintf("failed to resolve filename conflict: %s", targetPath), err)
	}

	if err := atomicReplace(stagingInTarget, resolvedPath); err != nil {
		return "", err
	}

	return resolvedPath, nil
}

func prepareInTargetDir(src, targetDir string) (string, func(), error) {
	staging, err := settings.UniqueTempPath(targetDir, ".videodl_staging_", ".part")
	if err != nil {
		return "", noop, newPublishError("download.publish.staging_path",
			"failed to create staging path in target directory", err)
	}

	if err := copyFile(src, staging); err != nil {
		_ = os.Remove(staging)
		return "", noop, err
	}

	cleanup := func() { _ = os.Remove(staging) }
	return staging, cleanup, nil
}

func noop() {}

func verifyPublished(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return newPublishError("download.publish.verify_stat",
			fmt.Sprintf("cannot verify published file: %s", path), err)
	}
	if info.Size() == 0 {
		_ = os.Remove(path)
		return newPublishError("download.publish.verify_empty",
			fmt.Sprintf("published file is empty: %s", path), nil)
	}
	return nil
}

func copyFile(src, dst string) error {
	srcF, err := os.Open(src)
	if err != nil {
		return newPublishError("download.publish.copy_open",
			fmt.Sprintf("cannot open temp file for reading: %s", src), err)
	}
	defer srcF.Close()

	dstF, err := os.Create(dst)
	if err != nil {
		return newPublishError("download.publish.copy_create",
			fmt.Sprintf("cannot create destination file: %s", dst), err)
	}

	if _, err := io.Copy(dstF, srcF); err != nil {
		_ = dstF.Close()
		_ = os.Remove(dst)
		return newPublishError("download.publish.copy_data",
			fmt.Sprintf("failed during file copy to %s", dst), err)
	}

	if err := dstF.Sync(); err != nil {
		_ = dstF.Close()
		_ = os.Remove(dst)
		return newPublishError("download.publish.copy_sync",
			fmt.Sprintf("failed to sync destination file: %s", dst), err)
	}

	if err := dstF.Close(); err != nil {
		_ = os.Remove(dst)
		return newPublishError("download.publish.copy_close",
			fmt.Sprintf("failed to close destination file: %s", dst), err)
	}

	return nil
}

func newPublishError(code string, msg string, err error) *TaskError {
	return &TaskError{Code: media.ErrorCode(code), Message: msg, Err: err}
}

func atomicReplace(source, dest string) error {
	err := os.Rename(source, dest)
	if err == nil {
		return nil
	}

	if !isCrossDevice(err) {
		return newPublishError("download.publish.rename",
			fmt.Sprintf("failed to finalize publish at %s", dest), err)
	}

	if err := copyFile(source, dest); err != nil {
		return err
	}

	if err := os.Remove(source); err != nil {
		return newPublishError("download.publish.cleanup_temp",
			fmt.Sprintf("publish succeeded but failed to remove staging %s", source), err)
	}

	return nil
}

func isCrossDevice(err error) bool {
	if err == nil {
		return false
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		if errno, ok := linkErr.Err.(syscall.Errno); ok {
			return errno == syscall.EXDEV
		}
		if linkErr.Err.Error() == "invalid cross-device link" {
			return true
		}
	}
	return false
}
