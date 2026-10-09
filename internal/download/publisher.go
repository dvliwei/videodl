package download

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
}

func NewPublisher(cfg PublisherConfig) *Publisher {
	return &Publisher{cfg: cfg}
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

	resolvedPath, err := p.ResolveOutput(targetPath)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		return "", newPublishError("download.publish.mkdir",
			fmt.Sprintf("failed to ensure target directory: %s", resolvedPath), err)
	}

	if err := os.Rename(tempFile, resolvedPath); err != nil {
		if !isCrossDevice(err) {
			return "", newPublishError("download.publish.rename",
				fmt.Sprintf("failed to move temp file to %s", resolvedPath), err)
		}

		if err := copyFile(tempFile, resolvedPath); err != nil {
			return "", err
		}

		if err := os.Remove(tempFile); err != nil {
			return "", newPublishError("download.publish.cleanup_temp",
				fmt.Sprintf("publish succeeded but failed to remove temp file %s", tempFile), err)
		}
	}

	if err := verifyPublished(resolvedPath); err != nil {
		_ = os.Remove(resolvedPath)
		return "", err
	}

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

	resolvedPath, err := p.ResolveOutput(targetPath)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		return "", newPublishError("download.publish.mkdir",
			fmt.Sprintf("failed to ensure target directory: %s", resolvedPath), err)
	}

	destTemp, err := settings.UniqueTempPath(filepath.Dir(resolvedPath), "publish_", ".tmp")
	if err != nil {
		return "", newPublishError("download.publish.temp_create",
			"failed to create staging temp file", err)
	}

	if err := copyFile(tempFile, destTemp); err != nil {
		_ = os.Remove(destTemp)
		return "", err
	}

	if err := verifyPublished(destTemp); err != nil {
		_ = os.Remove(destTemp)
		return "", err
	}

	if err := os.Rename(destTemp, resolvedPath); err != nil {
		_ = os.Remove(destTemp)
		return "", newPublishError("download.publish.rename",
			fmt.Sprintf("failed to finalize publish at %s", resolvedPath), err)
	}

	return resolvedPath, nil
}

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

func isCrossDevice(err error) bool {
	if err == nil {
		return false
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return linkErr.Err.Error() == "invalid cross-device link"
	}
	return false
}

func newPublishError(code string, msg string, err error) *TaskError {
	return &TaskError{Code: media.ErrorCode(code), Message: msg, Err: err}
}
