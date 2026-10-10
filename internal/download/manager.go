package download

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"videodl/internal/ffmpeg"
	"videodl/internal/media"
	"videodl/internal/settings"
)

const (
	DefaultMaxConcurrent = 3
	DefaultMaxQueueSize  = 50
	DefaultAttemptLimit  = 5
)

// MediaSource is the fully resolved input description for a download pipeline.
// The Manager never trusts frontend-supplied URLs — the Resolver (owned by the
// App layer) pulls the real URL from the analysis session, which is why this
// type lives in download and not in the frontend-facing media package.
type MediaSource struct {
	Inputs []ffmpeg.Input
	// InputURL is retained for native callers during the transition to
	// multi-input sources. New resolvers should fill Inputs.
	InputURL        string
	SourceType      media.SourceType
	DurationSeconds *float64
}

// MediaSourceResolver turns opaque frontend IDs into a MediaSource. App
// implements this using its analysisResults cache; unit tests inject a stub.
type MediaSourceResolver interface {
	Resolve(ctx context.Context, analysisID, mediaID, variantID string) (*MediaSource, error)
}

type ManagerConfig struct {
	MaxConcurrent   int
	MaxQueueSize    int
	MaxAttempts     int
	BaseTempDir     string
	PublisherConfig PublisherConfig

	Invoker  *ffmpeg.Invoker
	Resolver MediaSourceResolver
}

func (c ManagerConfig) withDefaults() ManagerConfig {
	if c.MaxConcurrent <= 0 {
		c.MaxConcurrent = DefaultMaxConcurrent
	}
	if c.MaxQueueSize <= 0 {
		c.MaxQueueSize = DefaultMaxQueueSize
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = DefaultAttemptLimit
	}
	return c
}

type Task struct {
	mu sync.RWMutex

	id      string
	Request media.DownloadRequest
	Title   string
	Profile media.DownloadProfile

	state     media.TaskState
	phase     string
	attempt   int
	createdAt time.Time
	startedAt *time.Time

	ctx    context.Context
	cancel context.CancelFunc

	tempDir     string
	stagingFile string
	outputPath  string

	errorCode    string
	errorMessage string

	progress *float64
	speed    int64
	size     *int64

	completedAt *time.Time

	manager *Manager
}

func (t *Task) ID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.id
}

func (t *Task) State() media.TaskState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

func (t *Task) Attempt() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.attempt
}

func (t *Task) Snapshot() media.DownloadTask {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return media.DownloadTask{
		ID:                  t.id,
		Title:               t.Title,
		AnalysisID:          t.Request.AnalysisID,
		MediaID:             t.Request.MediaID,
		VariantID:           t.Request.VariantID,
		Profile:             t.Profile,
		State:               t.state,
		Phase:               t.phase,
		Attempt:             t.attempt,
		Progress:            t.progress,
		SpeedBytesPerSecond: t.speed,
		SizeBytes:           t.size,
		OutputPath:          t.outputPath,
		ErrorCode:           t.errorCode,
		ErrorMessage:        t.errorMessage,
		CreatedAt:           t.createdAt,
		StartedAt:           t.startedAt,
		CompletedAt:         t.completedAt,
	}
}

func (t *Task) setState(s media.TaskState, phase string) {
	t.mu.Lock()
	if t.state == media.TaskCompleted || t.state == media.TaskCanceled || t.state == media.TaskFailed {
		t.mu.Unlock()
		return
	}
	changed := t.state != s
	t.state = s
	if phase != "" {
		t.phase = phase
	}
	if changed && s == media.TaskPreparing && t.startedAt == nil {
		now := time.Now()
		t.startedAt = &now
	}
	if changed && (s == media.TaskCompleted || s == media.TaskCanceled || s == media.TaskFailed) {
		now := time.Now()
		t.completedAt = &now
	}
	t.mu.Unlock()

	if t.manager != nil && t.manager.onEvent != nil {
		t.manager.onEvent(media.TaskEvent{
			Version: media.EventVersion,
			Task:    t.Snapshot(),
		})
	}
}

func (t *Task) setError(code media.ErrorCode, msg string) {
	t.mu.Lock()
	t.errorCode = string(code)
	t.errorMessage = msg
	t.mu.Unlock()
}

func (t *Task) updateProgress(progress *float64, speed int64, size *int64) {
	t.mu.Lock()
	if progress != nil {
		p := *progress
		if p > 100 {
			p = 100
		}
		if p < 0 {
			p = 0
		}
		t.progress = &p
	}
	t.speed = speed
	if size != nil {
		t.size = size
	}
	t.mu.Unlock()
}

func (t *Task) tempDirectory() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.tempDir
}

func (t *Task) StagingFile() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.stagingFile
}

type Manager struct {
	cfg ManagerConfig

	publisher *Publisher

	mu      sync.RWMutex
	tasks   map[string]*Task
	taskIDs []string

	sem     chan struct{}
	queue   []*Task
	queueMu sync.Mutex

	running atomic.Int64
	closed  atomic.Bool

	onEvent func(media.TaskEvent)
	onLog   func(format string, args ...interface{})
}

func NewManager(cfg ManagerConfig) *Manager {
	cfg = cfg.withDefaults()
	return &Manager{
		cfg:       cfg,
		publisher: NewPublisher(cfg.PublisherConfig),
		tasks:     make(map[string]*Task),
		sem:       make(chan struct{}, cfg.MaxConcurrent),
		queue:     make([]*Task, 0),
	}
}

func (m *Manager) SetEventCallback(fn func(media.TaskEvent)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onEvent = fn
}

func (m *Manager) SetLogCallback(fn func(format string, args ...interface{})) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onLog = fn
}

func (m *Manager) log(format string, args ...interface{}) {
	m.mu.RLock()
	fn := m.onLog
	m.mu.RUnlock()
	if fn != nil {
		fn(format, args...)
	}
}

func (m *Manager) Create(req media.DownloadRequest, title string) (*Task, error) {
	if m.closed.Load() {
		return nil, ErrManagerClosed
	}

	profile := req.Profile
	if profile == "" {
		profile = media.ProfileOriginal
	}
	if _, ok := ffmpeg.GetPreset(profile); !ok {
		return nil, newTaskError(media.ErrorCode("download.unknown_profile"),
			fmt.Sprintf("unknown profile: %s", profile), nil)
	}

	id, err := newTaskID()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	baseDir := m.cfg.BaseTempDir
	if baseDir == "" {
		baseDir = os.TempDir()
	}

	taskTemp, err := settings.UniqueTempDir(baseDir, "videodl_task_")
	if err != nil {
		cancel()
		return nil, newTaskError("download.temp_create", "failed to create temp directory", err)
	}

	task := &Task{
		id:        id,
		Request:   req,
		Title:     title,
		Profile:   profile,
		state:     media.TaskQueued,
		attempt:   1,
		createdAt: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
		tempDir:   taskTemp,
		manager:   m,
	}

	m.mu.Lock()
	if len(m.tasks) >= m.cfg.MaxQueueSize {
		m.mu.Unlock()
		_ = os.RemoveAll(taskTemp)
		cancel()
		return nil, ErrQueueFull
	}
	m.tasks[id] = task
	m.taskIDs = append(m.taskIDs, id)
	m.mu.Unlock()

	m.queueMu.Lock()
	m.queue = append(m.queue, task)
	m.queueMu.Unlock()

	m.setState(task, media.TaskQueued, "")
	m.log("task %s created and queued (attempt 1)", id)

	m.dispatch()

	return task, nil
}

func (m *Manager) Get(id string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return t, nil
}

func (m *Manager) List() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*Task, 0, len(m.taskIDs))
	for _, id := range m.taskIDs {
		if t, ok := m.tasks[id]; ok {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func (m *Manager) Snapshots() []media.DownloadTask {
	tasks := m.List()
	snaps := make([]media.DownloadTask, len(tasks))
	for i, t := range tasks {
		snaps[i] = t.Snapshot()
	}
	return snaps
}

func (m *Manager) Cancel(id string) error {
	m.mu.RLock()
	t, ok := m.tasks[id]
	m.mu.RUnlock()
	if !ok {
		return ErrTaskNotFound
	}

	t.mu.Lock()
	state := t.state
	t.mu.Unlock()

	switch state {
	case media.TaskCompleted, media.TaskCanceled, media.TaskFailed:
		return ErrTaskNotCancelable
	case media.TaskQueued:
		m.removeFromQueue(t)
		t.mu.Lock()
		if t.cancel != nil {
			t.cancel()
		}
		t.mu.Unlock()
		m.setState(t, media.TaskCanceled, "")
		m.log("task %s canceled (was queued)", id)
		return nil
	default:
		t.mu.Lock()
		cancel := t.cancel
		t.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		m.log("task %s cancel requested", id)
		return nil
	}
}

func (m *Manager) Retry(id string) (*Task, error) {
	m.mu.RLock()
	t, ok := m.tasks[id]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrTaskNotFound
	}

	t.mu.Lock()
	state := t.state
	currentAttempt := t.attempt
	t.mu.Unlock()

	if state != media.TaskCanceled && state != media.TaskFailed {
		return nil, ErrTaskNotRetryable
	}

	if currentAttempt >= m.cfg.MaxAttempts {
		return nil, newTaskError(media.ErrorCode("download.max_attempts"),
			"maximum retry attempts reached", nil)
	}

	t.mu.Lock()
	t.errorCode = ""
	t.errorMessage = ""
	t.progress = nil
	t.speed = 0
	t.size = nil
	t.startedAt = nil
	t.completedAt = nil
	t.attempt = currentAttempt + 1
	t.state = media.TaskQueued
	t.phase = ""
	t.mu.Unlock()

	t.mu.Lock()
	if t.cancel != nil {
		t.cancel()
	}
	t.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	t.mu.Lock()
	t.ctx = ctx
	t.cancel = cancel
	t.mu.Unlock()

	baseDir := m.cfg.BaseTempDir
	if baseDir == "" {
		baseDir = os.TempDir()
	}
	if taskTemp, err := settings.UniqueTempDir(baseDir, "videodl_task_"); err == nil {
		t.mu.Lock()
		oldTemp := t.tempDir
		t.tempDir = taskTemp
		t.mu.Unlock()
		_ = os.RemoveAll(oldTemp)
	}

	m.queueMu.Lock()
	m.queue = append(m.queue, t)
	m.queueMu.Unlock()

	m.setState(t, media.TaskQueued, "")
	m.log("task %s retried (attempt %d)", id, t.Attempt())

	m.dispatch()

	return t, nil
}

func (m *Manager) dispatch() {
	for {
		m.queueMu.Lock()
		if len(m.queue) == 0 {
			m.queueMu.Unlock()
			return
		}
		task := m.queue[0]
		m.queue = m.queue[1:]
		m.queueMu.Unlock()

		if m.closed.Load() {
			m.cancelCollectedTask(task)
			continue
		}

		task.mu.RLock()
		alreadyTerminal := task.state == media.TaskCanceled || task.state == media.TaskFailed || task.state == media.TaskCompleted
		ctxErr := task.ctx.Err()
		task.mu.RUnlock()

		if alreadyTerminal || ctxErr != nil {
			if ctxErr != nil && !alreadyTerminal {
				m.cancelCollectedTask(task)
			}
			continue
		}

		select {
		case m.sem <- struct{}{}:
			go m.execute(task)
		default:
			m.queueMu.Lock()
			m.queue = append([]*Task{task}, m.queue...)
			m.queueMu.Unlock()
			return
		}
	}
}

func (m *Manager) cancelCollectedTask(task *Task) {
	task.mu.Lock()
	wasTerminal := task.state == media.TaskCanceled || task.state == media.TaskFailed || task.state == media.TaskCompleted
	if task.cancel != nil {
		task.cancel()
	}
	if !wasTerminal {
		task.state = media.TaskCanceled
		if task.completedAt == nil {
			now := time.Now()
			task.completedAt = &now
		}
	}
	task.mu.Unlock()
	if !wasTerminal {
		m.setState(task, media.TaskCanceled, "")
	}
}

func (m *Manager) execute(task *Task) {
	m.running.Add(1)
	defer func() {
		m.running.Add(-1)
		<-m.sem
		m.dispatch()
	}()

	task.mu.RLock()
	state := task.state
	ctx := task.ctx
	task.mu.RUnlock()

	if state == media.TaskCanceled || state == media.TaskFailed || state == media.TaskCompleted {
		return
	}
	if err := ctx.Err(); err != nil {
		m.setState(task, media.TaskCanceled, "")
		return
	}

	m.setState(task, media.TaskPreparing, "preparing")

	task.mu.RLock()
	ctx = task.ctx
	task.mu.RUnlock()

	if err := ctx.Err(); err != nil {
		m.cleanupTaskArtifacts(task)
		m.setState(task, media.TaskCanceled, "")
		return
	}

	if err := m.runPipeline(ctx, task); err != nil {
		task.mu.RLock()
		ctx = task.ctx
		task.mu.RUnlock()

		if ctx.Err() != nil {
			task.setError(media.ErrCodeCanceled, "task canceled")
			m.setState(task, media.TaskCanceled, "")
		} else {
			m.cleanupTaskArtifacts(task)
			task.setError(media.ErrCodeUnknown, err.Error())
			m.setState(task, media.TaskFailed, "")
		}
		m.log("task %s ended: %v", task.ID(), err)
		return
	}

	task.mu.RLock()
	ctx = task.ctx
	task.mu.RUnlock()

	if ctx.Err() != nil {
		m.cleanupTaskArtifacts(task)
		task.setError(media.ErrCodeCanceled, "task canceled")
		m.setState(task, media.TaskCanceled, "")
		return
	}

	m.setState(task, media.TaskCompleted, "")
	m.log("task %s completed successfully", task.ID())
}

func (m *Manager) cleanupTaskArtifacts(task *Task) {
	task.mu.Lock()
	staging := task.stagingFile
	tempDir := task.tempDir
	task.mu.Unlock()

	if staging != "" {
		_ = os.Remove(staging)
	}

	if tempDir != "" {
		_ = os.RemoveAll(tempDir)
	}
}

func (m *Manager) runPipeline(ctx context.Context, task *Task) error {
	m.setState(task, media.TaskDownloading, "downloading")
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.runDownloadPhase(ctx, task); err != nil {
		return err
	}

	task.mu.RLock()
	profile := task.Profile
	task.mu.RUnlock()

	if profile != media.ProfileOriginal {
		m.setState(task, media.TaskTranscoding, "transcoding")
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := m.runTranscodePhase(ctx, task); err != nil {
			return err
		}
	} else {
		m.setState(task, media.TaskMerging, "merging")
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := m.runMergePhase(ctx, task); err != nil {
			return err
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	m.setState(task, media.TaskPreparing, "finishing")
	return m.runFinishPhase(ctx, task)
}

func (m *Manager) ensureStagingFile(task *Task, suffix string) error {
	task.mu.RLock()
	existing := task.stagingFile
	task.mu.RUnlock()

	if existing != "" {
		return nil
	}

	task.mu.RLock()
	tempDir := task.tempDir
	task.mu.RUnlock()

	if tempDir == "" {
		baseDir := m.cfg.BaseTempDir
		if baseDir == "" {
			baseDir = os.TempDir()
		}
		dir, err := settings.UniqueTempDir(baseDir, "videodl_task_")
		if err != nil {
			return newTaskError("download.staging_dir", "failed to create temp dir", err)
		}
		task.mu.Lock()
		task.tempDir = dir
		task.mu.Unlock()
		tempDir = dir
	}

	stagingPath, err := settings.UniqueTempPath(tempDir, "staging_", suffix)
	if err != nil {
		return newTaskError("download.staging_path", "failed to create staging path", err)
	}

	task.mu.Lock()
	task.stagingFile = stagingPath
	task.mu.Unlock()

	return nil
}

func (m *Manager) runDownloadPhase(ctx context.Context, task *Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	invoker := m.cfg.Invoker
	if invoker == nil {
		return newTaskError("download.invoker_missing",
			"ffmpeg invoker not configured", nil)
	}

	resolver := m.cfg.Resolver
	if resolver == nil {
		return newTaskError("download.resolver_missing",
			"media source resolver not configured", nil)
	}

	task.mu.RLock()
	req := task.Request
	profile := task.Profile
	task.mu.RUnlock()

	src, err := resolver.Resolve(ctx, req.AnalysisID, req.MediaID, req.VariantID)
	if err != nil {
		return newTaskError("download.resolve_failed",
			fmt.Sprintf("failed to resolve media source: %v", err), err)
	}
	if src == nil {
		return newTaskError("download.resolve_empty", "resolver returned no media source", nil)
	}
	inputs := src.Inputs
	if len(inputs) == 0 && src.InputURL != "" {
		inputs = []ffmpeg.Input{{URL: src.InputURL}}
	}
	if len(inputs) == 0 {
		return newTaskError("download.resolve_empty", "resolver returned no input URLs", nil)
	}

	if err := m.ensureStagingFile(task, stagingSuffixForOutput(req.OutputPath, profile)); err != nil {
		return err
	}

	task.mu.RLock()
	outputPath := task.stagingFile
	task.mu.RUnlock()

	opts := ffmpeg.DownloadOptions{
		Inputs:          inputs,
		SourceType:      src.SourceType,
		OutputPath:      outputPath,
		Profile:         media.ProfileOriginal,
		DurationSeconds: src.DurationSeconds,
	}

	m.log("task %s: downloading %d input(s) -> %s",
		task.ID(), len(inputs), outputPath)

	sink := &taskProgressSink{task: task}

	result, runErr := ffmpeg.RunFFmpegDownload(ctx, invoker, opts, sink)
	if runErr != nil {
		_ = os.Remove(outputPath)
		return m.classifyFFmpegError(runErr, result)
	}

	if result == nil {
		_ = os.Remove(outputPath)
		return newTaskError("download.ffmpeg_nil_result",
			"ffmpeg returned nil result without error", nil)
	}

	if _, statErr := os.Stat(outputPath); statErr != nil {
		return newTaskError("download.output_missing",
			fmt.Sprintf("ffmpeg finished but output file missing: %v", statErr), statErr)
	}

	zeroProgress := 0.0
	speedZero := int64(0)
	task.updateProgress(&zeroProgress, speedZero, nil)

	return nil
}

func (m *Manager) runMergePhase(ctx context.Context, task *Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	task.mu.RLock()
	staging := task.stagingFile
	task.mu.RUnlock()
	if staging == "" {
		return newTaskError("download.merge.no_staging", "no staging file before merge", nil)
	}
	if _, err := os.Stat(staging); err != nil {
		return newTaskError("download.merge.staging_missing",
			fmt.Sprintf("staging file missing before merge: %v", err), err)
	}
	return nil
}

func (m *Manager) runTranscodePhase(ctx context.Context, task *Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	invoker := m.cfg.Invoker
	if invoker == nil {
		return newTaskError("download.invoker_missing",
			"ffmpeg invoker not configured", nil)
	}

	task.mu.RLock()
	profile := task.Profile
	srcStaging := task.stagingFile
	task.mu.RUnlock()

	if srcStaging == "" {
		return newTaskError("download.transcode.no_staging", "no staging file before transcode", nil)
	}
	if _, statErr := os.Stat(srcStaging); statErr != nil {
		return newTaskError("download.transcode.staging_missing",
			fmt.Sprintf("staging file missing before transcode: %v", statErr), statErr)
	}

	outSuffix := outputSuffixForProfile(profile)
	transcodePath, err := settings.UniqueTempPath(task.tempDirectory(), "staging_tc_", outSuffix)
	if err != nil {
		return newTaskError("download.transcode.path", "failed to create transcode staging path", err)
	}

	tcOpts := ffmpeg.TranscodeOptions{
		InputPath:  srcStaging,
		OutputPath: transcodePath,
		Profile:    profile,
	}

	sink := &taskProgressSink{task: task}
	result, runErr := ffmpeg.RunFFmpegTranscode(ctx, invoker, tcOpts, sink)
	if runErr != nil {
		_ = os.Remove(transcodePath)
		return m.classifyFFmpegError(runErr, result)
	}

	if _, statErr := os.Stat(transcodePath); statErr != nil {
		return newTaskError("download.transcode.output_missing",
			fmt.Sprintf("transcode finished but output missing: %v", statErr), statErr)
	}

	if err := os.Remove(srcStaging); err != nil {
		m.log("task %s: could not remove pre-transcode staging %s: %v",
			task.ID(), srcStaging, err)
	}

	task.mu.Lock()
	task.stagingFile = transcodePath
	task.mu.Unlock()

	zeroProgress := 0.0
	speedZero := int64(0)
	task.updateProgress(&zeroProgress, speedZero, nil)

	return nil
}

func (m *Manager) classifyFFmpegError(runErr error, result *ffmpeg.DownloadResult) error {
	if errors.Is(runErr, context.Canceled) {
		return runErr
	}
	if errors.Is(runErr, context.DeadlineExceeded) {
		return runErr
	}
	pe, ok := runErr.(*ffmpeg.ProcessError)
	if ok {
		msg := pe.Error()
		stderr := strings.ToLower(pe.StderrTail)
		if strings.Contains(stderr, "protocol not found") && strings.Contains(stderr, "https") {
			msg = "bundled FFmpeg lacks HTTPS/TLS protocol support; update the app's bundled FFmpeg resources"
		} else if pe.StderrTail != "" {
			msg = msg + " | " + pe.StderrTail
		}
		return newTaskError("download.ffmpeg_failed", msg, runErr)
	}
	if runErr != nil {
		return newTaskError("download.ffmpeg_failed", runErr.Error(), runErr)
	}
	return newTaskError("download.ffmpeg_unknown", "ffmpeg exited with failure", nil)
}

type taskProgressSink struct {
	task *Task
}

func (s *taskProgressSink) OnProgress(p ffmpeg.DownloadProgress) {
	var progress *float64
	if p.Percentage != nil {
		progress = p.Percentage
	} else if p.TotalUs > 0 && p.OutTimeUs > 0 {
		pct := float64(p.OutTimeUs) / float64(p.TotalUs) * 100
		if pct > 100 {
			pct = 100
		}
		progress = &pct
	}

	var speed int64
	if p.SpeedQ != "" {
		speed = parseSpeedQ(p.SpeedQ)
	}

	var size *int64
	if p.SizeBytes > 0 {
		sizeVal := p.SizeBytes
		size = &sizeVal
	}

	s.task.updateProgress(progress, speed, size)
}

func parseSpeedQ(q string) int64 {
	q = strings.TrimSpace(q)
	if q == "" || strings.EqualFold(q, "N/A") {
		return 0
	}
	var numStr string
	for i := 0; i < len(q); i++ {
		c := q[i]
		if (c >= '0' && c <= '9') || c == '.' || c == '-' {
			numStr += string(c)
		} else {
			break
		}
	}
	if numStr == "" {
		return 0
	}
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}
	suffix := strings.ToLower(strings.TrimSpace(q[len(numStr):]))
	switch suffix {
	case "k", "kb", "k/s":
		return int64(val * 1024)
	case "m", "mb", "m/s":
		return int64(val * 1024 * 1024)
	case "g", "gb", "g/s":
		return int64(val * 1024 * 1024 * 1024)
	case "b", "b/s", "":
		return int64(val)
	default:
		return int64(val)
	}
}

func outputSuffixForProfile(profile media.DownloadProfile) string {
	if profile == "" {
		profile = media.ProfileOriginal
	}
	suffix := ffmpeg.PresetOutputExtension(profile)
	if suffix == "" {
		suffix = ".mp4"
	}
	return suffix
}

func stagingSuffixForOutput(targetPath string, profile media.DownloadProfile) string {
	if profile != media.ProfileOriginal {
		return outputSuffixForProfile(profile)
	}

	ext := strings.ToLower(filepath.Ext(targetPath))
	switch ext {
	case ".avi", ".flv", ".m2ts", ".m4a", ".m4v", ".mkv", ".mov", ".mp3", ".mp4", ".mpeg", ".mpg", ".ogg", ".ogv", ".ts", ".wav", ".webm":
		return ext
	default:
		return outputSuffixForProfile(profile)
	}
}

func (m *Manager) runFinishPhase(ctx context.Context, task *Task) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	task.mu.RLock()
	stagingFile := task.stagingFile
	targetPath := task.Request.OutputPath
	tempDir := task.tempDir
	task.mu.RUnlock()

	if stagingFile == "" {
		return newTaskError("download.publish.no_staging",
			"no staging file produced by pipeline", nil)
	}

	if targetPath == "" {
		targetPath = defaultOutputPath(task.Title, ".mp4")
	}

	resolvedPath, err := m.publisher.Publish(stagingFile, targetPath)
	if err != nil {
		return err
	}

	task.mu.Lock()
	task.outputPath = resolvedPath
	task.stagingFile = ""
	task.mu.Unlock()

	if tempDir != "" {
		_ = os.RemoveAll(tempDir)
		task.mu.Lock()
		task.tempDir = ""
		task.mu.Unlock()
	}

	return nil
}

func (m *Manager) setState(task *Task, s media.TaskState, phase string) {
	task.setState(s, phase)
}

func (m *Manager) removeFromQueue(t *Task) {
	m.queueMu.Lock()
	defer m.queueMu.Unlock()
	for i, qt := range m.queue {
		if qt == t {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			return
		}
	}
}

func (m *Manager) RunningCount() int {
	return int(m.running.Load())
}

func (m *Manager) PendingCount() int {
	m.queueMu.Lock()
	defer m.queueMu.Unlock()
	return len(m.queue)
}

func (m *Manager) Close() error {
	if m.closed.Swap(true) {
		return ErrManagerClosed
	}

	m.log("manager closing, canceling all tasks")

	m.queueMu.Lock()
	for _, t := range m.queue {
		t.mu.Lock()
		if t.cancel != nil {
			t.cancel()
		}
		if t.state == media.TaskQueued {
			t.state = media.TaskCanceled
			if t.completedAt == nil {
				now := time.Now()
				t.completedAt = &now
			}
		}
		t.mu.Unlock()
		m.setState(t, media.TaskCanceled, "")
	}
	m.queue = nil
	m.queueMu.Unlock()

	m.mu.RLock()
	for _, id := range m.taskIDs {
		if t, ok := m.tasks[id]; ok {
			t.mu.Lock()
			if t.cancel != nil {
				t.cancel()
			}
			state := t.state
			t.mu.Unlock()

			if state != media.TaskCompleted && state != media.TaskCanceled && state != media.TaskFailed {
				m.setState(t, media.TaskCanceled, "")
			}
		}
	}
	m.mu.RUnlock()

	deadline := time.After(10 * time.Second)
	for m.RunningCount() > 0 {
		select {
		case <-deadline:
			m.log("close timed out waiting for tasks")
			goto cleanup
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

cleanup:
	m.cleanupTempDirs()

	m.log("manager closed")
	return nil
}

func (m *Manager) cleanupTempDirs() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, id := range m.taskIDs {
		if t, ok := m.tasks[id]; ok {
			t.mu.RLock()
			tempDir := t.tempDir
			state := t.state
			t.mu.RUnlock()

			if (state == media.TaskCanceled || state == media.TaskFailed) && tempDir != "" {
				_ = os.RemoveAll(tempDir)
			}
		}
	}
}

func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return ErrTaskNotFound
	}

	t.mu.Lock()
	state := t.state
	tempDir := t.tempDir
	t.mu.Unlock()

	switch state {
	case media.TaskQueued:
		m.removeFromQueue(t)
		t.mu.Lock()
		if t.cancel != nil {
			t.cancel()
		}
		t.mu.Unlock()
	case media.TaskPreparing, media.TaskDownloading, media.TaskMerging,
		media.TaskTranscoding:
		m.mu.Unlock()
		return newTaskError(ErrCodeTaskRunning, "task is still active, cancel first", nil)
	}

	delete(m.tasks, id)
	for i, tid := range m.taskIDs {
		if tid == id {
			m.taskIDs = append(m.taskIDs[:i], m.taskIDs[i+1:]...)
			break
		}
	}
	m.mu.Unlock()

	if tempDir != "" {
		_ = os.RemoveAll(tempDir)
	}

	return nil
}

func newTaskID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (m *Manager) Config() ManagerConfig {
	return m.cfg
}

func (m *Manager) IsClosed() bool {
	return m.closed.Load()
}

func defaultOutputPath(title, ext string) string {
	if title == "" {
		title = "video"
	}
	return title + ext
}
