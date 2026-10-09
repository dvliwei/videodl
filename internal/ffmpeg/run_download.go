package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

type DownloadResult struct {
	ExitCode      int
	StderrTail    string
	FinalProgress DownloadProgress
}

type trackingSink struct {
	mu            sync.Mutex
	sink          ProgressSink
	last          DownloadProgress
	totalDuration *float64
}

func (t *trackingSink) SetTotalDurationSeconds(seconds float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalDuration = &seconds
}

func (t *trackingSink) OnProgress(p DownloadProgress) {
	t.mu.Lock()
	if t.totalDuration != nil && *t.totalDuration > 0 {
		p.SetTotalDurationSeconds(*t.totalDuration)
	}
	t.last = p
	t.mu.Unlock()
	if t.sink != nil {
		t.sink.OnProgress(p)
	}
}

func (t *trackingSink) Last() DownloadProgress {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.last
}

func RunFFmpegDownload(ctx context.Context, inv *Invoker, opts DownloadOptions, sink ProgressSink) (*DownloadResult, error) {
	if inv == nil {
		return nil, fmt.Errorf("ffmpeg: RunFFmpegDownload: nil invoker")
	}

	args, err := BuildDownloadArgs(opts)
	if err != nil {
		return nil, err
	}

	if ctx == nil {
		ctx = context.Background()
	}

	cmd := exec.CommandContext(ctx, inv.Paths().FFmpeg, args...)
	cmd.SysProcAttr = newSysProcAttr()

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, &ProcessError{
			Cmd:         inv.Paths().FFmpeg,
			Args:        args,
			ExitCodeVal: -1,
			Err:         fmt.Errorf("ffmpeg: stdout pipe: %w", err),
		}
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, &ProcessError{
			Cmd:         inv.Paths().FFmpeg,
			Args:        args,
			ExitCodeVal: -1,
			Err:         fmt.Errorf("ffmpeg: stderr pipe: %w", err),
		}
	}

	if err := cmd.Start(); err != nil {
		return nil, &ProcessError{
			Cmd:         inv.Paths().FFmpeg,
			Args:        args,
			ExitCodeVal: -1,
			Err:         err,
		}
	}

	stderrBuf := &limitedBuffer{limit: DefaultOutputLimit}

	tracker := &trackingSink{sink: sink}
	if opts.DurationSeconds != nil && *opts.DurationSeconds > 0 {
		tracker.SetTotalDurationSeconds(*opts.DurationSeconds)
	}

	var streamErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		streamErr = StreamProgress(stdoutPipe, tracker)
	}()

	go func() {
		_, _ = io.Copy(stderrBuf, stderrPipe)
	}()

	waitErr := cmd.Wait()

	<-done

	stderr := tailOf(stderrBuf.String())

	if err := ctx.Err(); err != nil {
		pe := classifyError(inv.Paths().FFmpeg, args, "", stderr, err)
		if waitErr != nil {
			var exitErr *exec.ExitError
			if errors.As(waitErr, &exitErr) {
				pe.ExitCodeVal = exitErr.ExitCode()
			}
		}
		return nil, pe
	}

	if waitErr != nil {
		pe := classifyError(inv.Paths().FFmpeg, args, "", stderr, waitErr)
		return nil, pe
	}

	if streamErr != nil && streamErr != io.EOF {
		return nil, &ProcessError{
			Cmd:         inv.Paths().FFmpeg,
			Args:        args,
			ExitCodeVal: 0,
			StderrTail:  stderr,
			Err:         fmt.Errorf("ffmpeg: progress stream: %w", streamErr),
		}
	}

	return &DownloadResult{
		ExitCode:      0,
		StderrTail:    stderr,
		FinalProgress: tracker.Last(),
	}, nil
}
