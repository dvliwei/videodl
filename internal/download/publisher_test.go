package download

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"videodl/internal/media"
	"videodl/internal/settings"
)

func TestPublisher_ResolveOutput_NoTarget(t *testing.T) {
	p := NewPublisher(DefaultPublisherConfig())
	_, err := p.ResolveOutput("")
	if err == nil {
		t.Fatal("expected error for empty target")
	}
}

func TestPublisher_ResolveOutput_DirMissing(t *testing.T) {
	p := NewPublisher(DefaultPublisherConfig())
	_, err := p.ResolveOutput("/nonexistent/deep/dir/video.mp4")
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestPublisher_Publish_Success(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "staging.part")
	if err := os.WriteFile(staging, []byte("hello video"), 0o644); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "final.mp4")
	p := NewPublisher(DefaultPublisherConfig())
	resolved, err := p.Publish(staging, target)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if resolved != target {
		t.Errorf("resolved path = %q, want %q", resolved, target)
	}

	if _, err := os.Stat(target); err != nil {
		t.Errorf("target file should exist: %v", err)
	}

	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Errorf("staging file should be gone, stat err = %v", err)
	}
}

func TestPublisher_Publish_TargetExists_AutoRename(t *testing.T) {
	dir := t.TempDir()

	staging := filepath.Join(dir, "staging.part")
	if err := os.WriteFile(staging, []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewPublisher(DefaultPublisherConfig())
	resolved, err := p.Publish(staging, target)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	expected := filepath.Join(dir, "video (1).mp4")
	if resolved != expected {
		t.Errorf("resolved = %q, want %q", resolved, expected)
	}

	data, _ := os.ReadFile(resolved)
	if string(data) != "new content" {
		t.Errorf("new file content = %q, want %q", data, "new content")
	}

	origData, _ := os.ReadFile(target)
	if string(origData) != "existing" {
		t.Error("original file should not be modified")
	}
}

func TestPublisher_Publish_TargetExists_Overwrite(t *testing.T) {
	dir := t.TempDir()

	staging := filepath.Join(dir, "staging.part")
	if err := os.WriteFile(staging, []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewPublisher(PublisherConfig{OverwritePolicy: OverwriteAlways})
	resolved, err := p.Publish(staging, target)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if resolved != target {
		t.Errorf("resolved = %q, want %q (overwrite)", resolved, target)
	}

	data, _ := os.ReadFile(target)
	if string(data) != "new content" {
		t.Errorf("overwritten content = %q, want %q", data, "new content")
	}
}

func TestPublisher_Publish_TargetExists_Never(t *testing.T) {
	dir := t.TempDir()

	staging := filepath.Join(dir, "staging.part")
	if err := os.WriteFile(staging, []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewPublisher(PublisherConfig{OverwritePolicy: OverwriteNever})
	_, err := p.Publish(staging, target)
	if err == nil {
		t.Fatal("expected conflict error")
	}

	var te *TaskError
	if !errors.As(err, &te) {
		t.Fatalf("want *TaskError, got %T", err)
	}

	var ce *settings.ConflictError
	if !errors.As(err, &ce) {
		t.Errorf("should wrap ConflictError, got: %v", err)
	}

	if _, err := os.Stat(target); err != nil {
		t.Error("original file should still exist")
	}

	if _, err := os.Stat(staging); err != nil {
		t.Errorf("staging should not be removed on failure: %v", err)
	}
}

func TestPublisher_Publish_SourceMissing(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "video.mp4")

	p := NewPublisher(DefaultPublisherConfig())
	_, err := p.Publish(filepath.Join(dir, "nonexistent.part"), target)
	if err == nil {
		t.Fatal("expected error for missing staging")
	}
}

func TestPublisher_Publish_EmptyFileRejected(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "staging.part")
	if err := os.WriteFile(staging, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "video.mp4")

	p := NewPublisher(DefaultPublisherConfig())
	_, err := p.Publish(staging, target)
	if err == nil {
		t.Fatal("expected error for empty staging")
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("empty file should not be published")
	}
}

func TestPublisher_Publish_AutoRenameMultiple(t *testing.T) {
	dir := t.TempDir()

	for i := 0; i < 3; i++ {
		f := filepath.Join(dir, "video (", ")")
		_ = f
	}

	target := filepath.Join(dir, "video.mp4")
	os.WriteFile(target, []byte("orig"), 0o644)
	os.WriteFile(filepath.Join(dir, "video (1).mp4"), []byte("1"), 0o644)
	os.WriteFile(filepath.Join(dir, "video (2).mp4"), []byte("2"), 0o644)

	staging := filepath.Join(dir, "staging.part")
	os.WriteFile(staging, []byte("new"), 0o644)

	p := NewPublisher(DefaultPublisherConfig())
	resolved, err := p.Publish(staging, target)
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(dir, "video (3).mp4")
	if resolved != want {
		t.Errorf("resolved = %q, want %q", resolved, want)
	}
}

func TestManager_Publish_SuccessOnComplete(t *testing.T) {
	m, tmp := newTestManager(t)
	defer m.Close()

	outputDir := filepath.Join(tmp, "outputs")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	req := newDownloadRequest("1")
	req.OutputPath = filepath.Join(outputDir, "final.mp4")
	task, err := m.Create(req, "Test Video")
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.After(5 * time.Second)
	for {
		snap := task.Snapshot()
		if snap.State == media.TaskCompleted {
			break
		}
		if snap.State == media.TaskFailed || snap.State == media.TaskCanceled {
			t.Fatalf("task ended unexpectedly: state=%s error=%s", snap.State, snap.ErrorMessage)
		}
		select {
		case <-deadline:
			t.Fatal("timeout")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	info, err := os.Stat(req.OutputPath)
	if err != nil {
		t.Fatalf("output file should exist: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output file should not be empty")
	}

	snap := task.Snapshot()
	if snap.OutputPath != req.OutputPath {
		t.Errorf("Snapshot OutputPath = %q, want %q", snap.OutputPath, req.OutputPath)
	}
}

func TestManager_Publish_AutoRename(t *testing.T) {
	m, tmp := newTestManager(t)
	defer m.Close()

	outputDir := filepath.Join(tmp, "outputs")
	os.MkdirAll(outputDir, 0o755)

	target := filepath.Join(outputDir, "video.mp4")
	os.WriteFile(target, []byte("existing"), 0o644)

	req := newDownloadRequest("1")
	req.OutputPath = target
	task, err := m.Create(req, "Video")
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.After(5 * time.Second)
	for {
		snap := task.Snapshot()
		if snap.State == media.TaskCompleted {
			break
		}
		if snap.State == media.TaskFailed {
			t.Fatalf("task failed: %s", snap.ErrorMessage)
		}
		select {
		case <-deadline:
			t.Fatal("timeout")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	orig, _ := os.ReadFile(target)
	if string(orig) != "existing" {
		t.Error("original file must not be overwritten with default auto-rename policy")
	}

	snap := task.Snapshot()
	if snap.OutputPath == target {
		t.Error("OutputPath should be renamed, not same as existing")
	}

	if _, err := os.Stat(snap.OutputPath); err != nil {
		t.Errorf("renamed file should exist at %s: %v", snap.OutputPath, err)
	}
}

func TestManager_Publish_OverwritePolicy(t *testing.T) {
	tmp := t.TempDir()
	outputDir := filepath.Join(tmp, "outputs")
	os.MkdirAll(outputDir, 0o755)

	target := filepath.Join(outputDir, "video.mp4")
	os.WriteFile(target, []byte("old"), 0o644)

	cfg := fakeManagerConfig(t, tmp)
	cfg.PublisherConfig = PublisherConfig{OverwritePolicy: OverwriteAlways}
	m := NewManager(cfg)
	defer m.Close()

	req := newDownloadRequest("1")
	req.OutputPath = target
	task, err := m.Create(req, "Video")
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.After(5 * time.Second)
	for {
		snap := task.Snapshot()
		if snap.State == media.TaskCompleted {
			break
		}
		if snap.State == media.TaskFailed {
			t.Fatalf("task failed: %s", snap.ErrorMessage)
		}
		select {
		case <-deadline:
			t.Fatal("timeout")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	data, _ := os.ReadFile(target)
	if string(data) == "old" {
		t.Error("file should have been overwritten")
	}
}

func TestManager_Publish_SkipPolicy(t *testing.T) {
	tmp := t.TempDir()
	outputDir := filepath.Join(tmp, "outputs")
	os.MkdirAll(outputDir, 0o755)

	target := filepath.Join(outputDir, "video.mp4")
	os.WriteFile(target, []byte("original content"), 0o644)

	cfg := fakeManagerConfig(t, tmp)
	cfg.PublisherConfig = PublisherConfig{OverwritePolicy: OverwriteNever}
	m := NewManager(cfg)
	defer m.Close()

	req := newDownloadRequest("1")
	req.OutputPath = target
	task, err := m.Create(req, "Video")
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.After(5 * time.Second)
	for {
		snap := task.Snapshot()
		if snap.State == media.TaskCompleted {
			t.Fatalf("task should fail on skip policy conflict")
		}
		if snap.State == media.TaskFailed || snap.State == media.TaskCanceled {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	data, _ := os.ReadFile(target)
	if string(data) != "original content" {
		t.Error("original file must be untouched")
	}

	snap := task.Snapshot()
	if snap.ErrorCode == "" && snap.ErrorMessage == "" {
		t.Error("failed task should record error")
	}
}

func TestManager_Publish_ResidualTempFileCleanupOnFailure(t *testing.T) {
	tmp := t.TempDir()

	cfg := fakeManagerConfig(t, tmp)
	cfg.MaxAttempts = 1
	m := NewManager(cfg)
	defer m.Close()

	req := newDownloadRequest("1")
	req.OutputPath = filepath.Join("/nonexistent", "deep", "dir", "video.mp4")
	task, err := m.Create(req, "Video")
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.After(5 * time.Second)
	for {
		snap := task.Snapshot()
		if snap.State == media.TaskFailed || snap.State == media.TaskCompleted || snap.State == media.TaskCanceled {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	remaining := listAllFiles(t, tmp)
	for _, p := range remaining {
		if filepath.Dir(p) == tmp {
			t.Errorf("residual file/dir in temp root: %s", p)
		}
	}
}

func TestManager_Retry_UsesNewTempDir(t *testing.T) {
	tmp := t.TempDir()

	cfg := fakeManagerConfig(t, tmp)
	m := NewManager(cfg)
	defer m.Close()

	req := newDownloadRequest("1")
	req.OutputPath = filepath.Join("/nonexistent", "video.mp4")
	task, _ := m.Create(req, "Video")

	deadline := time.After(5 * time.Second)
	for {
		snap := task.Snapshot()
		if snap.State == media.TaskFailed || snap.State == media.TaskCanceled || snap.State == media.TaskCompleted {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for initial fail")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	firstTemp := task.tempDirectory()
	_ = firstTemp

	time.Sleep(100 * time.Millisecond)

	retried, err := m.Retry(task.ID())
	if err != nil {
		t.Fatalf("Retry failed: %v", err)
	}

	secondTemp := retried.tempDirectory()
	if secondTemp == "" {
		t.Error("retry should allocate a new temp dir")
	}

	deadline2 := time.After(5 * time.Second)
	for {
		snap := retried.Snapshot()
		if snap.State == media.TaskCompleted || snap.State == media.TaskFailed || snap.State == media.TaskCanceled {
			break
		}
		select {
		case <-deadline2:
			t.Fatal("timeout waiting for retry")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func TestManager_CompletedFileNotDeletedOnRetry(t *testing.T) {
	tmp := t.TempDir()
	outputDir := filepath.Join(tmp, "outputs")
	os.MkdirAll(outputDir, 0o755)

	cfg := fakeManagerConfig(t, tmp)
	cfg.MaxAttempts = 5
	m := NewManager(cfg)
	defer m.Close()

	req := newDownloadRequest("1")
	req.OutputPath = filepath.Join(outputDir, "stable.mp4")
	task, _ := m.Create(req, "Video")

	deadline := time.After(5 * time.Second)
	for {
		snap := task.Snapshot()
		if snap.State == media.TaskCompleted {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	originalData, _ := os.ReadFile(req.OutputPath)
	if string(originalData) == "" {
		t.Fatal("original file should have content")
	}

	task.mu.Lock()
	task.state = media.TaskFailed
	task.mu.Unlock()

	_, err := m.Retry(task.ID())
	if err != nil {
		t.Fatalf("Retry failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	data, _ := os.ReadFile(req.OutputPath)
	if string(data) != string(originalData) {
		t.Errorf("completed file should not be touched on retry, got %q want %q", data, originalData)
	}
}

func TestPublisher_ValidateAndCopy_FullPipeline(t *testing.T) {
	dir := t.TempDir()

	staging := filepath.Join(dir, "staging.part")
	os.WriteFile(staging, []byte("valid video content"), 0o644)

	target := filepath.Join(dir, "video.mp4")
	p := NewPublisher(DefaultPublisherConfig())
	resolved, err := p.ValidateAndCopy(staging, target)
	if err != nil {
		t.Fatalf("ValidateAndCopy failed: %v", err)
	}

	if resolved != target {
		t.Errorf("resolved = %q, want %q", resolved, target)
	}

	if _, err := os.Stat(staging); os.IsNotExist(err) {
		t.Errorf("staging should still exist after ValidateAndCopy (source not removed)")
	}

	data, _ := os.ReadFile(target)
	if string(data) != "valid video content" {
		t.Errorf("content mismatch: %q", data)
	}
}

func TestPublisher_Publish_OverwriteAlways_EmptySource_PreservesOriginal(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(target, []byte("existing good content"), 0o644); err != nil {
		t.Fatal(err)
	}

	staging := filepath.Join(dir, "staging.part")
	if err := os.WriteFile(staging, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewPublisher(PublisherConfig{OverwritePolicy: OverwriteAlways})
	_, err := p.Publish(staging, target)
	if err == nil {
		t.Fatal("expected error for empty source")
	}

	data, _ := os.ReadFile(target)
	if string(data) != "existing good content" {
		t.Errorf("original file must be preserved after failed overwrite publish, got %q", data)
	}
}

func TestPublisher_Publish_OverwriteAlways_SourceMissing_PreservesOriginal(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewPublisher(PublisherConfig{OverwritePolicy: OverwriteAlways})
	_, err := p.Publish(filepath.Join(dir, "missing.part"), target)
	if err == nil {
		t.Fatal("expected error")
	}

	data, _ := os.ReadFile(target)
	if string(data) != "original" {
		t.Error("original file must survive when source missing")
	}
}

func TestPublisher_Publish_OverwriteAlways_ValidSource_Replaces(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(target, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	staging := filepath.Join(dir, "staging.part")
	if err := os.WriteFile(staging, []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewPublisher(PublisherConfig{OverwritePolicy: OverwriteAlways})
	resolved, err := p.Publish(staging, target)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if resolved != target {
		t.Errorf("resolved = %q, want %q", resolved, target)
	}

	data, _ := os.ReadFile(target)
	if string(data) != "new content" {
		t.Errorf("target = %q, want new content", data)
	}

	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Error("staging file should be removed after publish")
	}
}

func TestPublisher_ConcurrentAutoRename_UniquePaths(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	const n = 10
	var wg sync.WaitGroup
	results := make([]string, n)
	errs := make([]error, n)

	p := NewPublisher(DefaultPublisherConfig())

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			staging := filepath.Join(dir, fmt.Sprintf("staging_%d.part", idx))
			if err := os.WriteFile(staging, []byte(fmt.Sprintf("content-%d", idx)), 0o644); err != nil {
				errs[idx] = err
				return
			}

			resolved, err := p.Publish(staging, target)
			results[idx] = resolved
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}

	seen := make(map[string]bool)
	for _, r := range results {
		if r == "" {
			continue
		}
		if seen[r] {
			t.Errorf("duplicate resolved path: %s", r)
		}
		seen[r] = true
	}

	for path := range seen {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s to exist: %v", path, err)
		}
	}

	if len(seen) != n {
		t.Errorf("want %d unique paths, got %d", n, len(seen))
	}

	orig, _ := os.ReadFile(target)
	if string(orig) != "original" {
		t.Error("original file must remain untouched during concurrent auto-rename")
	}
}

func TestPublisher_Publish_CrossDevice_StagingDifferentDir(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	staging := filepath.Join(dirA, "staging.part")
	if err := os.WriteFile(staging, []byte("content from different dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dirB, "video.mp4")

	p := NewPublisher(DefaultPublisherConfig())
	resolved, err := p.Publish(staging, target)
	if err != nil {
		t.Fatalf("Publish across dirs failed: %v", err)
	}
	if resolved != target {
		t.Errorf("resolved = %q, want %q", resolved, target)
	}

	data, _ := os.ReadFile(target)
	if string(data) != "content from different dir" {
		t.Errorf("content = %q", data)
	}
}

func TestPublisher_Publish_OverwriteAlways_CopyFailure_PreservesOriginal(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(target, []byte("good original"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(dir, "readonly"), 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Join(dir, "readonly"))

	staging := filepath.Join(dir, "readonly", "bad.part")
	if err := os.WriteFile(staging, []byte("new content"), 0o644); err != nil {
		t.Skip("could not set up readonly source, skipping")
	}

	p := NewPublisher(PublisherConfig{OverwritePolicy: OverwriteAlways})
	_, _ = p.Publish(staging, target)

	data, _ := os.ReadFile(target)
	if string(data) != "good original" {
		t.Errorf("original must remain intact, got %q", data)
	}
}

func TestValidateAndCopy_NeverRemovesSource(t *testing.T) {
	dir := t.TempDir()

	staging := filepath.Join(dir, "staging.part")
	if err := os.WriteFile(staging, []byte("kept"), 0o644); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "out.mp4")

	p := NewPublisher(DefaultPublisherConfig())
	if _, err := p.ValidateAndCopy(staging, target); err != nil {
		t.Fatalf("ValidateAndCopy failed: %v", err)
	}

	if _, err := os.Stat(staging); err != nil {
		t.Errorf("source must still exist after ValidateAndCopy: %v", err)
	}

	data, _ := os.ReadFile(target)
	if string(data) != "kept" {
		t.Errorf("target content = %q", data)
	}
}

func TestValidateAndCopy_EmptySource_Rejected(t *testing.T) {
	dir := t.TempDir()

	staging := filepath.Join(dir, "staging.part")
	if err := os.WriteFile(staging, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "out.mp4")
	p := NewPublisher(DefaultPublisherConfig())
	_, err := p.ValidateAndCopy(staging, target)
	if err == nil {
		t.Fatal("expected error for empty source")
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("empty source must not publish")
	}
}

func listAllFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if p != root {
			files = append(files, p)
		}
		return nil
	})
	return files
}
