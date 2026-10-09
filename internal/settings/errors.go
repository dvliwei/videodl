package settings

import (
	"errors"
	"fmt"
)

var (
	errTooManyAttempts = errors.New("settings: too many auto-rename attempts")
)

type filesystemError struct {
	path string
	op   string
	err  error
}

func (e *filesystemError) Error() string {
	return fmt.Sprintf("settings: %s %s: %v", e.op, e.path, e.err)
}

func (e *filesystemError) Unwrap() error { return e.err }

type ConflictError struct {
	Path string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("settings: file already exists at %s", e.Path)
}

type NotExistError struct {
	Path string
}

func (e *NotExistError) Error() string {
	return fmt.Sprintf("settings: directory does not exist: %s", e.Path)
}

type NotWritableError struct {
	Path string
	Err  error
}

func (e *NotWritableError) Error() string {
	return fmt.Sprintf("settings: directory not writable: %s: %v", e.Path, e.Err)
}

func (e *NotWritableError) Unwrap() error { return e.Err }
