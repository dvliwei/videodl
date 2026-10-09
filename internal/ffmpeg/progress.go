package ffmpeg

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type DownloadProgress struct {
	OutTimeUs  int64
	OutTimeMs  int64
	SpeedQ     string
	SizeBytes  int64
	TotalUs    int64
	Percentage *float64
	IsEnd      bool
}

type ProgressSink interface {
	OnProgress(p DownloadProgress)
}

type ProgressFunc func(p DownloadProgress)

func (f ProgressFunc) OnProgress(p DownloadProgress) { f(p) }

func ParseProgressKV(line string, prior DownloadProgress) DownloadProgress {
	p := prior

	trimmed := strings.TrimSpace(line)
	if trimmed == "" || !strings.Contains(trimmed, "=") {
		return p
	}

	idx := strings.Index(trimmed, "=")
	key := strings.TrimSpace(strings.ToLower(trimmed[:idx]))
	val := strings.TrimSpace(trimmed[idx+1:])

	switch key {
	case "out_time_us":
		if v, err := strconv.ParseInt(val, 10, 64); err == nil {
			p.OutTimeUs = v
		}
	case "out_time_ms":
		if v, err := strconv.ParseInt(val, 10, 64); err == nil {
			p.OutTimeMs = v
		}
	case "total_size", "size":
		if v, err := strconv.ParseInt(val, 10, 64); err == nil {
			p.SizeBytes = v
		}
	case "speed":
		p.SpeedQ = val
	case "progress":
		if strings.EqualFold(val, "end") {
			p.IsEnd = true
		}
	}

	p.updatePercentage()
	return p
}

func (p *DownloadProgress) updatePercentage() {
	if p.TotalUs <= 0 || p.OutTimeUs <= 0 {
		p.Percentage = nil
		return
	}
	pct := float64(p.OutTimeUs) / float64(p.TotalUs) * 100
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}
	p.Percentage = &pct
}

func (p *DownloadProgress) SetTotalDurationSeconds(seconds float64) {
	if seconds > 0 {
		p.TotalUs = int64(seconds * 1_000_000)
		p.updatePercentage()
	}
}

func StreamProgress(r io.Reader, sink ProgressSink) error {
	if r == nil {
		return fmt.Errorf("ffmpeg: progress stream reader is nil")
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var current DownloadProgress
	lastEmit := time.Now()

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "=") {
			current = ParseProgressKV(line, current)

			if sink != nil && shouldEmit(current, &lastEmit) {
				sink.OnProgress(current)
				lastEmit = time.Now()
			}
		}
	}

	if current.IsEnd && sink != nil {
		sink.OnProgress(current)
	}

	if err := scanner.Err(); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	return nil
}

func shouldEmit(current DownloadProgress, last *time.Time) bool {
	if current.IsEnd {
		return true
	}
	if time.Since(*last) >= 150*time.Millisecond {
		return true
	}
	return false
}
