package download

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"videodl/internal/ffmpeg"
	"videodl/internal/media"
)

func writeStubScript(t *testing.T, dir, name, script string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	s := script
	if runtime.GOOS == "windows" {
		s = "@echo off\r\n" + s
	}
	if err := os.WriteFile(path, []byte(s), 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(path, 0o755)
	}
	return path
}

type stubResolver struct{}

func (s *stubResolver) Resolve(ctx context.Context, analysisID, mediaID, variantID string) (*MediaSource, error) {
	_ = ctx
	dur := 10.0
	return &MediaSource{
		Inputs:          []ffmpeg.Input{{URL: "https://example.com/stub/" + mediaID}},
		SourceType:      media.SourceDirect,
		DurationSeconds: &dur,
	}, nil
}

type blockingResolver struct {
	called chan context.Context
}

func (r *blockingResolver) Resolve(ctx context.Context, analysisID, mediaID, variantID string) (*MediaSource, error) {
	select {
	case r.called <- ctx:
	default:
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func stubFFmpegSuccessScript() string {
	return `#!/bin/sh
# Emulates stream-copy download OR local-file transcode.
# Output path is always the last argument.
out="${!#}"
have_file=""
for a in "$@"; do
  if [ -f "$a" ] && [ "$a" != "$out" ]; then
    have_file="$a"
    break
  fi
done

# Emit progress markers and sleep so Snapshot-polling tests observe the phase.
for i in 1000000 2000000 3000000; do
  echo "out_time_us=$i"
  echo "progress=continue"
  sleep 0.01
done
echo "out_time_us=3000000"
echo "progress=end"

if [ -n "$have_file" ]; then
  cat "$have_file" > "$out"
  echo "stub ffmpeg appended transcode header" >> "$out"
else
  echo "stub ffmpeg output data for test (stream copy)" > "$out"
fi
exit 0
`
}

func stubFFmpegFailScript() string {
	return `#!/bin/sh
echo "ffmpeg error: something failed" >&2
exit 2
`
}

func stubFFmpegHangingScript() string {
	return `#!/bin/sh
sleep 30
exit 0
`
}

func stubFFmpegRejectPartScript() string {
	return `#!/bin/sh
out=""
for arg in "$@"; do
  out="$arg"
done
case "$out" in
  *.part)
    echo "output must have a container extension" >&2
    exit 2
    ;;
esac
echo "out_time_us=1000000"
echo "progress=end"
echo "stub output" > "$out"
exit 0
`
}

func newTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	tmp := t.TempDir()
	cfg := fakeManagerConfig(t, tmp)
	return NewManager(cfg), tmp
}

func newTestManagerWithScripts(t *testing.T, ffmpegBody string) (*Manager, string) {
	t.Helper()
	tmp := t.TempDir()
	cfg := fakeManagerConfigWithScript(t, tmp, ffmpegBody)
	return NewManager(cfg), tmp
}

func fakeManagerConfig(t *testing.T, tmp string) ManagerConfig {
	t.Helper()
	return fakeManagerConfigWithScript(t, tmp, stubFFmpegSuccessScript())
}

func fakeManagerConfigWithScript(t *testing.T, tmp, ffmpegBody string) ManagerConfig {
	t.Helper()
	binDir, err := os.MkdirTemp("", "videodl_fakebin_")
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(binDir, 0o755)
	ffmpegScript := writeStubScript(t, binDir, "ffmpeg", ffmpegBody)
	ffprobeScript := writeStubScript(t, binDir, "ffprobe", "#!/bin/sh\nexit 0\n")

	invoker := ffmpeg.NewInvoker(ffmpeg.ToolPaths{
		FFmpeg:  ffmpegScript,
		FFprobe: ffprobeScript,
	})

	return ManagerConfig{
		MaxConcurrent: 2,
		MaxQueueSize:  10,
		MaxAttempts:   3,
		BaseTempDir:   tmp,
		Invoker:       invoker,
		Resolver:      &stubResolver{},
	}
}

func newDownloadRequest(id string) media.DownloadRequest {
	return media.DownloadRequest{
		AnalysisID: "analysis_" + id,
		MediaID:    "media_" + id,
		OutputPath: "/tmp/output_" + id + ".mp4",
		Profile:    media.ProfileOriginal,
	}
}

func TestManager_Create(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	task, err := m.Create(newDownloadRequest("1"), "Test Video")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if task.ID() == "" {
		t.Error("task ID should not be empty")
	}
	if task.State() != media.TaskQueued {
		t.Errorf("initial state = %s, want queued", task.State())
	}
	if task.Attempt() != 1 {
		t.Errorf("initial attempt = %d, want 1", task.Attempt())
	}
	if task.Title != "Test Video" {
		t.Errorf("title = %q, want %q", task.Title, "Test Video")
	}
}

func TestManager_CreateQueueFull(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	for i := 0; i < 10; i++ {
		req := newDownloadRequest(string(rune('a' + i)))
		if _, err := m.Create(req, "Video"); err != nil {
			t.Fatalf("Create %d failed: %v", i, err)
		}
	}

	_, err := m.Create(newDownloadRequest("overflow"), "Overflow")
	if !errors.Is(err, ErrQueueFull) {
		t.Errorf("want ErrQueueFull, got %v", err)
	}
}

func TestManager_Get(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	created, _ := m.Create(newDownloadRequest("1"), "Video")

	got, err := m.Get(created.ID())
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID() != created.ID() {
		t.Errorf("got task ID = %s, want %s", got.ID(), created.ID())
	}

	_, err = m.Get("nonexistent")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("want ErrTaskNotFound, got %v", err)
	}
}

func TestManager_List(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	m.Create(newDownloadRequest("1"), "Video 1")
	m.Create(newDownloadRequest("2"), "Video 2")

	tasks := m.List()
	if len(tasks) != 2 {
		t.Errorf("List len = %d, want 2", len(tasks))
	}
}

func TestManager_Snapshots(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	m.Create(newDownloadRequest("1"), "Video 1")

	snaps := m.Snapshots()
	if len(snaps) != 1 {
		t.Fatalf("Snapshots len = %d, want 1", len(snaps))
	}
	if snaps[0].State != media.TaskQueued {
		t.Errorf("state = %s, want queued", snaps[0].State)
	}
	if snaps[0].Attempt != 1 {
		t.Errorf("attempt = %d, want 1", snaps[0].Attempt)
	}
}

func TestManager_CancelQueued(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	task, _ := m.Create(newDownloadRequest("1"), "Video")

	err := m.Cancel(task.ID())
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	if task.State() != media.TaskCanceled {
		t.Errorf("state after cancel = %s, want canceled", task.State())
	}
}

func TestManager_ResolverReceivesTaskContextCancellation(t *testing.T) {
	tmp := t.TempDir()
	resolver := &blockingResolver{called: make(chan context.Context, 1)}
	cfg := fakeManagerConfig(t, tmp)
	cfg.Resolver = resolver
	m := NewManager(cfg)
	defer m.Close()

	task, err := m.Create(newDownloadRequest("context"), "Context")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case ctx := <-resolver.called:
		if ctx == nil {
			t.Fatal("resolver received nil context")
		}
		if err := m.Cancel(task.ID()); err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("resolver was not called")
	}
	deadline := time.After(2 * time.Second)
	for task.State() != media.TaskCanceled {
		select {
		case <-deadline:
			t.Fatalf("task state = %s, want canceled", task.State())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestManager_CancelRepeat(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	task, _ := m.Create(newDownloadRequest("1"), "Video")

	err := m.Cancel(task.ID())
	if err != nil {
		t.Fatalf("first Cancel failed: %v", err)
	}

	err = m.Cancel(task.ID())
	if !errors.Is(err, ErrTaskNotCancelable) {
		t.Errorf("second cancel want ErrTaskNotCancelable, got %v", err)
	}
}

func TestManager_CancelNotFound(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	err := m.Cancel("nonexistent")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("want ErrTaskNotFound, got %v", err)
	}
}

func TestManager_RetrySuccess(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	task, _ := m.Create(newDownloadRequest("1"), "Video")
	m.Cancel(task.ID())

	time.Sleep(50 * time.Millisecond)

	retried, err := m.Retry(task.ID())
	if err != nil {
		t.Fatalf("Retry failed: %v", err)
	}
	if retried.Attempt() != 2 {
		t.Errorf("attempt after retry = %d, want 2", retried.Attempt())
	}
	if retried.State() != media.TaskQueued {
		t.Errorf("state after retry = %s, want queued", retried.State())
	}
}

func TestManager_RetryLimit(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	task, _ := m.Create(newDownloadRequest("1"), "Video")
	m.Cancel(task.ID())
	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 2; i++ {
		m.Retry(task.ID())
		time.Sleep(50 * time.Millisecond)
		m.Cancel(task.ID())
		time.Sleep(50 * time.Millisecond)
	}

	_, err := m.Retry(task.ID())
	if err == nil {
		t.Error("expected error for exceeding retry limit")
	}
}

func TestManager_RetryWrongState(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	task, _ := m.Create(newDownloadRequest("1"), "Video")

	time.Sleep(200 * time.Millisecond)

	_, err := m.Retry(task.ID())
	if !errors.Is(err, ErrTaskNotRetryable) {
		t.Errorf("want ErrTaskNotRetryable, got %v", err)
	}

	m.Cancel(task.ID())
}

func TestManager_RetryRepeat(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	task, _ := m.Create(newDownloadRequest("1"), "Video")
	m.Cancel(task.ID())
	time.Sleep(50 * time.Millisecond)

	m.Retry(task.ID())
	time.Sleep(200 * time.Millisecond)

	_, err := m.Retry(task.ID())
	if !errors.Is(err, ErrTaskNotRetryable) {
		t.Errorf("want ErrTaskNotRetryable during active, got %v", err)
	}

	m.Cancel(task.ID())
	time.Sleep(50 * time.Millisecond)

	m.Retry(task.ID())
	time.Sleep(50 * time.Millisecond)

	_, err = m.Retry(task.ID())
	if !errors.Is(err, ErrTaskNotRetryable) {
		t.Errorf("want ErrTaskNotRetryable when queued, got %v", err)
	}
}

func TestManager_ClassifyFFmpegHTTPSCapabilityError(t *testing.T) {
	manager := &Manager{}
	err := manager.classifyFFmpegError(&ffmpeg.ProcessError{
		Cmd:         "ffmpeg",
		Args:        []string{"-i", "https://example.test/video"},
		ExitCodeVal: 8,
		StderrTail:  "[https] Protocol not found",
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "HTTPS/TLS") {
		t.Fatalf("error = %v, want HTTPS/TLS guidance", err)
	}
}

func TestManager_ConcurrencyLimit(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	var maxRunning atomic.Int64

	for i := 0; i < 8; i++ {
		task, _ := m.Create(newDownloadRequest(string(rune('a'+i))), "Video")
		go func(task *Task) {
			for {
				running := m.RunningCount()
				if int64(running) > maxRunning.Load() {
					maxRunning.Store(int64(running))
				}
				if task.State() == media.TaskCompleted || task.State() == media.TaskFailed || task.State() == media.TaskCanceled {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(task)
	}

	deadline := time.After(5 * time.Second)
	for {
		if m.PendingCount() == 0 && m.RunningCount() == 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for tasks to complete")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	if maxRunning.Load() > int64(m.cfg.MaxConcurrent) {
		t.Errorf("max concurrent running = %d, want <= %d", maxRunning.Load(), m.cfg.MaxConcurrent)
	}
}

func TestManager_Close(t *testing.T) {
	m, _ := newTestManager(t)

	for i := 0; i < 4; i++ {
		m.Create(newDownloadRequest(string(rune('a'+i))), "Video")
	}

	err := m.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !m.IsClosed() {
		t.Error("manager should be closed")
	}

	tasks := m.List()
	for _, task := range tasks {
		if task.State() != media.TaskCanceled {
			t.Errorf("task %s state = %s, want canceled after close", task.ID(), task.State())
		}
	}

	_, err = m.Create(newDownloadRequest("after"), "Video")
	if !errors.Is(err, ErrManagerClosed) {
		t.Errorf("want ErrManagerClosed, got %v", err)
	}

	err = m.Close()
	if !errors.Is(err, ErrManagerClosed) {
		t.Errorf("second close want ErrManagerClosed, got %v", err)
	}
}

func TestManager_CancelOneDoesNotAffectOthers(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	task1, _ := m.Create(newDownloadRequest("1"), "Video 1")
	task2, _ := m.Create(newDownloadRequest("2"), "Video 2")
	task3, _ := m.Create(newDownloadRequest("3"), "Video 3")

	time.Sleep(100 * time.Millisecond)

	m.Cancel(task1.ID())
	m.Cancel(task2.ID())

	done := make(chan struct{})
	go func() {
		for task3.State() != media.TaskCompleted {
			time.Sleep(50 * time.Millisecond)
			if m.IsClosed() {
				return
			}
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for task3 to complete")
	}
	if task3.State() != media.TaskCompleted {
		t.Fatalf("task3 state = %s, want completed", task3.State())
	}

	if task1.State() != media.TaskCanceled {
		t.Errorf("task1 state = %s, want canceled", task1.State())
	}
	if task2.State() != media.TaskCanceled {
		t.Errorf("task2 state = %s, want canceled", task2.State())
	}
}

func TestManager_TaskLifecycle(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	var events []media.TaskEvent
	var eventsMu sync.Mutex
	m.SetEventCallback(func(e media.TaskEvent) {
		eventsMu.Lock()
		events = append(events, e)
		eventsMu.Unlock()
	})

	task, _ := m.Create(newDownloadRequest("1"), "Video")

	deadline := time.After(5 * time.Second)
	for {
		if task.State() == media.TaskCompleted {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for task completion")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	eventsMu.Lock()
	defer eventsMu.Unlock()

	if len(events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(events))
	}

	states := make([]media.TaskState, 0, len(events))
	for _, e := range events {
		states = append(states, e.Task.State)
	}

	foundQueued := false
	foundPreparing := false
	foundCompleted := false
	for _, s := range states {
		if s == media.TaskQueued {
			foundQueued = true
		}
		if s == media.TaskPreparing {
			foundPreparing = true
		}
		if s == media.TaskCompleted {
			foundCompleted = true
		}
	}

	if !foundQueued {
		t.Error("missing queued state event")
	}
	if !foundPreparing {
		t.Error("missing preparing state event")
	}
	if !foundCompleted {
		t.Error("missing completed state event")
	}
}

func TestManager_TempDirCreated(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	task, _ := m.Create(newDownloadRequest("1"), "Video")

	task.mu.RLock()
	tempDir := task.tempDir
	task.mu.RUnlock()

	if tempDir == "" {
		t.Fatal("tempDir should not be empty")
	}

	info, err := os.Stat(tempDir)
	if err != nil {
		t.Fatalf("tempDir stat failed: %v", err)
	}
	if !info.IsDir() {
		t.Error("tempDir should be a directory")
	}
}

func TestManager_TempDirCleanedAfterCancel(t *testing.T) {
	m, _ := newTestManager(t)

	task, _ := m.Create(newDownloadRequest("1"), "Video")
	task.mu.RLock()
	tempDir := task.tempDir
	task.mu.RUnlock()

	m.Cancel(task.ID())
	time.Sleep(100 * time.Millisecond)

	m.Close()

	_, err := os.Stat(tempDir)
	if !os.IsNotExist(err) {
		t.Errorf("tempDir %s should be cleaned after cancel, stat err = %v", tempDir, err)
	}
}

func TestManager_DefaultConfig(t *testing.T) {
	m := NewManager(ManagerConfig{})
	defer m.Close()

	cfg := m.Config()
	if cfg.MaxConcurrent != DefaultMaxConcurrent {
		t.Errorf("MaxConcurrent = %d, want %d", cfg.MaxConcurrent, DefaultMaxConcurrent)
	}
	if cfg.MaxQueueSize != DefaultMaxQueueSize {
		t.Errorf("MaxQueueSize = %d, want %d", cfg.MaxQueueSize, DefaultMaxQueueSize)
	}
	if cfg.MaxAttempts != DefaultAttemptLimit {
		t.Errorf("MaxAttempts = %d, want %d", cfg.MaxAttempts, DefaultAttemptLimit)
	}
}

func TestManager_RunningAndPendingCounts(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	if m.RunningCount() != 0 {
		t.Errorf("initial RunningCount = %d, want 0", m.RunningCount())
	}
	if m.PendingCount() != 0 {
		t.Errorf("initial PendingCount = %d, want 0", m.PendingCount())
	}

	for i := 0; i < 5; i++ {
		m.Create(newDownloadRequest(string(rune('a'+i))), "Video")
	}

	deadline := time.After(5 * time.Second)
	for {
		if m.PendingCount() == 0 && m.RunningCount() == 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestTask_Snapshot(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	task, _ := m.Create(newDownloadRequest("1"), "Video Snapshot Test")
	snap := task.Snapshot()

	if snap.ID != task.ID() {
		t.Errorf("snapshot ID mismatch")
	}
	if snap.Title != "Video Snapshot Test" {
		t.Errorf("snapshot title = %q, want %q", snap.Title, "Video Snapshot Test")
	}
	if snap.Profile != media.ProfileOriginal {
		t.Errorf("snapshot profile = %s, want %s", snap.Profile, media.ProfileOriginal)
	}
	if snap.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestManager_RemoveQueued(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	task, _ := m.Create(newDownloadRequest("1"), "Video")

	m.Remove(task.ID())

	_, err := m.Get(task.ID())
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("want ErrTaskNotFound after Remove, got %v", err)
	}
}

func TestManager_RemoveActive(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	task, _ := m.Create(newDownloadRequest("1"), "Video")
	time.Sleep(200 * time.Millisecond)

	err := m.Remove(task.ID())
	if err == nil {
		t.Error("expected error removing active task")
	}

	m.Cancel(task.ID())
	time.Sleep(100 * time.Millisecond)

	err = m.Remove(task.ID())
	if err != nil {
		t.Errorf("Remove after cancel failed: %v", err)
	}
}

func TestManager_ConcurrentCreate(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	var wg sync.WaitGroup
	n := 10
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			m.Create(newDownloadRequest(string(rune('a'+idx))), "Video")
		}(i)
	}

	wg.Wait()

	tasks := m.List()
	if len(tasks) != n {
		t.Errorf("got %d tasks, want %d", len(tasks), n)
	}

	deadline := time.After(5 * time.Second)
	for {
		if m.PendingCount() == 0 && m.RunningCount() == 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestManager_CancelWhileRunning(t *testing.T) {
	m, _ := newTestManager(t)

	task, _ := m.Create(newDownloadRequest("1"), "Video")

	time.Sleep(100 * time.Millisecond)

	err := m.Cancel(task.ID())
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	deadline := time.After(3 * time.Second)
	for {
		state := task.State()
		if state == media.TaskCanceled || state == media.TaskFailed {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("task not canceled, state = %s", state)
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	m.Close()
}

func TestManager_TranscodeProfile(t *testing.T) {
	m, _ := newTestManager(t)
	defer m.Close()

	req := newDownloadRequest("1")
	req.Profile = media.ProfileMP4

	task, err := m.Create(req, "Video")
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.After(5 * time.Second)
	foundTranscoding := false
	for {
		snap := task.Snapshot()
		if snap.State == media.TaskCompleted {
			break
		}
		if snap.State == media.TaskTranscoding {
			foundTranscoding = true
		}
		select {
		case <-deadline:
			t.Fatal("timeout")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	if !foundTranscoding {
		t.Error("expected transcoding state for MP4 profile")
	}
}

func TestManager_OriginalDownloadUsesContainerExtensionForStaging(t *testing.T) {
	tmp := t.TempDir()
	manager, _ := newTestManagerWithScripts(t, stubFFmpegRejectPartScript())
	defer manager.Close()

	request := newDownloadRequest("staging-extension")
	request.OutputPath = filepath.Join(tmp, "video.mp4")
	task, err := manager.Create(request, "Video")
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.After(5 * time.Second)
	for task.State() != media.TaskCompleted {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for task completion; state = %s", task.State())
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	if task.Snapshot().OutputPath == "" {
		t.Fatal("completed task should have an output path")
	}
}

func TestStagingSuffixForOutput(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		profile media.DownloadProfile
		want    string
	}{
		{name: "target extension", target: "/tmp/video.webm", profile: media.ProfileOriginal, want: ".webm"},
		{name: "default original", target: "", profile: media.ProfileOriginal, want: ".mp4"},
		{name: "transcode preset", target: "/tmp/video.webm", profile: media.ProfileMP4, want: ".mp4"},
		{name: "part target falls back", target: "/tmp/video.part", profile: media.ProfileOriginal, want: ".mp4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stagingSuffixForOutput(tt.target, tt.profile); got != tt.want {
				t.Fatalf("stagingSuffixForOutput(%q, %q) = %q, want %q", tt.target, tt.profile, got, tt.want)
			}
		})
	}
}

func TestError_ErrorMethod(t *testing.T) {
	e := newTaskError(ErrCodeTaskNotFound, "task missing", nil)
	msg := e.Error()
	if msg == "" {
		t.Error("TaskError.Error() should not be empty")
	}

	inner := errors.New("inner error")
	e2 := newTaskError(ErrCodeTaskRunning, "active", inner)
	if !errors.Is(e2.Unwrap(), inner) {
		t.Error("Unwrap should return inner error")
	}
}
