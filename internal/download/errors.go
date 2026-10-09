package download

import (
	"errors"
	"fmt"

	"videodl/internal/media"
)

const (
	ErrCodeTaskNotFound        media.ErrorCode = "download.task_not_found"
	ErrCodeTaskNotCanceled     media.ErrorCode = "download.task_not_cancelable"
	ErrCodeTaskNotRetry        media.ErrorCode = "download.task_not_retryable"
	ErrCodeTaskRunning         media.ErrorCode = "download.task_running"
	ErrCodeQueueFull           media.ErrorCode = "download.queue_full"
	ErrCodeConcurrencyExceeded media.ErrorCode = "download.concurrency_exceeded"
	ErrCodeInvalidState        media.ErrorCode = "download.invalid_state"
)

var (
	ErrTaskNotFound        = errors.New("download: task not found")
	ErrTaskNotCancelable   = errors.New("download: task cannot be canceled in current state")
	ErrTaskNotRetryable    = errors.New("download: task cannot be retried in current state")
	ErrTaskRunning         = errors.New("download: task is already running")
	ErrQueueFull           = errors.New("download: queue is full")
	ErrConcurrencyExceeded = errors.New("download: maximum concurrent tasks reached")
	ErrInvalidState        = errors.New("download: invalid task state transition")
	ErrManagerClosed       = errors.New("download: manager is closed")
)

type TaskError struct {
	Code    media.ErrorCode
	Message string
	Err     error
}

func (e *TaskError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("download [%s]: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("download [%s]: %s", e.Code, e.Message)
}

func (e *TaskError) Unwrap() error { return e.Err }

func newTaskError(code media.ErrorCode, msg string, err error) *TaskError {
	return &TaskError{Code: code, Message: msg, Err: err}
}
