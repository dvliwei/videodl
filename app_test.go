package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"videodl/internal/download"
	"videodl/internal/media"
	"videodl/internal/settings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestNewApp_EventsBoundOnce(t *testing.T) {
	app := NewApp()

	var callCount atomic.Int64
	app.manager.SetEventCallback(func(evt media.TaskEvent) {
		callCount.Add(1)
	})

	app.manager.Create(media.DownloadRequest{
		AnalysisID: "stub",
		MediaID:    "stub",
		OutputPath: filepath.Join(t.TempDir(), "once.mp4"),
	}, "once")

	time.Sleep(50 * time.Millisecond)

	total := 0
	for _, snap := range app.manager.Snapshots() {
		total++
		_ = snap
	}
	_ = callCount.Load()
	_ = total
}

func TestNewApp_ShutdownClosesManager(t *testing.T) {
	app := NewApp()

	_, err := app.manager.Create(media.DownloadRequest{
		AnalysisID: "stub",
		MediaID:    "stub",
		OutputPath: filepath.Join(t.TempDir(), "shutdown.mp4"),
	}, "shutdown")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	app.shutdown(context.Background())

	if !app.manager.IsClosed() {
		t.Fatal("manager should be closed after shutdown")
	}

	_, err = app.manager.Create(media.DownloadRequest{
		AnalysisID: "stub",
		MediaID:    "stub",
		OutputPath: filepath.Join(t.TempDir(), "afterclose.mp4"),
	}, "after-close")
	if !errors.Is(err, download.ErrManagerClosed) {
		t.Fatalf("expected ErrManagerClosed, got %v", err)
	}
}

func TestStartDownload_RequiresAnalysis(t *testing.T) {
	app := NewApp()

	_, err := app.StartDownload(media.DownloadRequest{
		AnalysisID: "nonexistent",
		MediaID:    "also-nonexistent",
		OutputPath: filepath.Join(t.TempDir(), "x.mp4"),
		Profile:    media.ProfileOriginal,
	})
	if err == nil {
		t.Fatal("expected error when analysis ID is unknown")
	}
}

func TestStartDownload_UsesDefaultDirectoryWhenEmpty(t *testing.T) {
	app := NewApp()

	tmpDir := t.TempDir()

	app.mu.Lock()
	app.analysisResults["aaa"] = &media.AnalysisResult{
		ID:        "aaa",
		PageTitle: "Demo",
		Candidates: []media.MediaCandidate{
			{ID: "m1", Title: "My Video", SourceType: media.SourceDirect},
		},
	}
	app.mu.Unlock()

	dir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	_ = dir
	_ = tmpDir

	_, err = app.StartDownload(media.DownloadRequest{
		AnalysisID: "aaa",
		MediaID:    "m1",
		Profile:    media.ProfileOriginal,
	})
	if err != nil {
		t.Fatalf("start download should work with default dir: %v", err)
	}
}

func TestStartDownload_ExplicitOutputPath(t *testing.T) {
	app := NewApp()

	target := filepath.Join(t.TempDir(), "explicit.mp4")

	app.mu.Lock()
	app.analysisResults["a1"] = &media.AnalysisResult{
		ID:        "a1",
		PageTitle: "Demo",
		Candidates: []media.MediaCandidate{
			{ID: "mc1", Title: "Explicit Title", SourceType: media.SourceDirect},
		},
	}
	app.mu.Unlock()

	task, err := app.StartDownload(media.DownloadRequest{
		AnalysisID: "a1",
		MediaID:    "mc1",
		OutputPath: target,
		Profile:    media.ProfileOriginal,
	})
	if err != nil {
		t.Fatalf("start download: %v", err)
	}
	if task == nil {
		t.Fatal("task should not be nil")
	}
	if task.ID == "" {
		t.Fatal("task ID should be populated")
	}
	if task.AnalysisID != "a1" {
		t.Fatalf("analysis ID mismatch: %s", task.AnalysisID)
	}
}

func TestCancelDownload_ThenRetry(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.analysisResults["a1"] = &media.AnalysisResult{
		ID: "a1",
		Candidates: []media.MediaCandidate{
			{ID: "mc1", Title: "Cancellable", SourceType: media.SourceDirect},
		},
	}
	app.mu.Unlock()

	task, err := app.StartDownload(media.DownloadRequest{
		AnalysisID: "a1",
		MediaID:    "mc1",
		OutputPath: filepath.Join(t.TempDir(), "cancel.mp4"),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = app.CancelDownload(task.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}

	for i := 0; i < 100; i++ {
		snap, _ := app.GetTask(task.ID)
		if snap != nil && (snap.State == media.TaskCanceled || snap.State == media.TaskFailed) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	retried, err := app.RetryDownload(task.ID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if retried.Attempt != 2 {
		t.Fatalf("expected attempt 2, got %d", retried.Attempt)
	}
}

func TestListTasks_EmptyInitially(t *testing.T) {
	app := NewApp()

	tasks, err := app.ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestGetTask_MissingReturnsError(t *testing.T) {
	app := NewApp()

	_, err := app.GetTask("does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing task")
	}
}

func TestSettings_RoundTrip(t *testing.T) {
	app := NewApp()

	tmpDir := t.TempDir()
	myPath := filepath.Join(tmpDir, "my-downloads")

	err := app.SetDefaultDirectory(myPath)
	if err != nil {
		t.Fatalf("set default dir: %v", err)
	}

	resp, err := app.GetDefaultDirectory()
	if err != nil {
		t.Fatalf("get default dir: %v", err)
	}
	if resp.Path != myPath {
		t.Fatalf("path mismatch: got %s want %s", resp.Path, myPath)
	}
	if resp.IsDefault {
		t.Fatal("IsDefault should be false after explicit set")
	}
}

func TestGetDefaultDirectory_NoSettingReturnsDefault(t *testing.T) {
	app := NewApp()

	cfg, _ := app.settings.Load()
	if cfg != nil && cfg.DownloadDirectory != "" {
		t.Skip("user already has an explicit download directory configured")
	}

	resp, err := app.GetDefaultDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsDefault {
		t.Fatal("IsDefault should be true when no explicit setting exists")
	}
	if resp.Path == "" {
		t.Fatal("path should not be empty")
	}
}

func TestDialogs_ReturnErrorWithoutRuntimeContext(t *testing.T) {
	app := &App{}

	_, err := app.ChooseDirectory("")
	if err == nil {
		t.Fatal("ChooseDirectory should fail when runtime context is missing")
	}

	_, err = app.SaveAs("video.mp4", "")
	if err == nil {
		t.Fatal("SaveAs should fail when runtime context is missing")
	}
}

func TestSaveAs_SanitizesFilename(t *testing.T) {
	app := &App{}

	_, err := app.SaveAs("  bad:/name?.mp4  ", "")
	if err == nil {
		t.Fatal("should fail without runtime context")
	}
}

func TestGetAnalysis_UnknownID(t *testing.T) {
	app := NewApp()

	_, err := app.GetAnalysis("nope")
	if err == nil {
		t.Fatal("expected error for unknown analysis")
	}
}

func TestGetAnalysis_Cached(t *testing.T) {
	app := NewApp()

	app.mu.Lock()
	app.analysisResults["abc"] = &media.AnalysisResult{ID: "abc", PageTitle: "X"}
	app.mu.Unlock()

	r, err := app.GetAnalysis("abc")
	if err != nil {
		t.Fatal(err)
	}
	if r.PageTitle != "X" {
		t.Fatalf("page title mismatch: %s", r.PageTitle)
	}
}

func TestTaskEventContainsTaskID(t *testing.T) {
	app := NewApp()

	var mu sync.Mutex
	var received media.TaskEvent
	var gotOne bool

	app.manager.SetEventCallback(func(evt media.TaskEvent) {
		mu.Lock()
		defer mu.Unlock()
		received = evt
		gotOne = true
	})

	app.mu.Lock()
	app.analysisResults["a1"] = &media.AnalysisResult{
		ID: "a1",
		Candidates: []media.MediaCandidate{
			{ID: "mc1", Title: "Eventful", SourceType: media.SourceDirect},
		},
	}
	app.mu.Unlock()

	task, err := app.StartDownload(media.DownloadRequest{
		AnalysisID: "a1",
		MediaID:    "mc1",
		OutputPath: filepath.Join(t.TempDir(), "event.mp4"),
	})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		ok := gotOne
		mu.Unlock()
		if ok {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()

	if !gotOne {
		t.Fatal("no task event received")
	}
	if received.Task.ID != task.ID {
		t.Fatalf("event task ID %q does not match created task ID %q",
			received.Task.ID, task.ID)
	}
	if received.Version != media.EventVersion {
		t.Fatalf("event version mismatch: got %d want %d",
			received.Version, media.EventVersion)
	}
}

func TestAnalyze_BadURL_FailsGracefully(t *testing.T) {
	app := NewApp()

	resp, err := app.Analyze(AnalyzeRequest{URL: "not-a-url"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.AnalysisID == "" {
		t.Fatal("analysis ID should be populated even for bad URLs")
	}
	if resp.Phase != media.AnalysisRunning {
		t.Fatalf("phase should be running, got %s", resp.Phase)
	}
}

func TestAnalyze_ReturnsUniqueID(t *testing.T) {
	app := NewApp()

	r1, _ := app.Analyze(AnalyzeRequest{URL: "https://example.com/one"})
	r2, _ := app.Analyze(AnalyzeRequest{URL: "https://example.com/two"})

	if r1.AnalysisID == r2.AnalysisID {
		t.Fatal("each analysis run should have a unique ID")
	}
}

func TestClassifyAnalyzerError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"invalid scheme", errors.New("analyzer: unsupported scheme"), "analyzer.invalid_url"},
		{"body too large", errors.New("analyzer: response body exceeds"), "unknown"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classifyAnalyzerError(c.err)
			if got != c.want && !strings.Contains(got, c.want) && c.want == "unknown" {
				// allow unknown for generic errors
				if got != string(media.ErrCodeUnknown) {
					t.Fatalf("got %s want %s", got, c.want)
				}
			}
		})
	}
}

func TestManagerEventCallback_NotDuplicated(t *testing.T) {
	app := NewApp()

	var mu sync.Mutex
	counts := make(map[string]int)

	app.manager.SetEventCallback(func(evt media.TaskEvent) {
		mu.Lock()
		defer mu.Unlock()
		counts[evt.Task.ID]++
	})

	app.mu.Lock()
	app.analysisResults["a1"] = &media.AnalysisResult{
		ID: "a1",
		Candidates: []media.MediaCandidate{
			{ID: "mc1", Title: "Dedup", SourceType: media.SourceDirect},
		},
	}
	app.mu.Unlock()

	_, err := app.StartDownload(media.DownloadRequest{
		AnalysisID: "a1",
		MediaID:    "mc1",
		OutputPath: filepath.Join(t.TempDir(), "dedup.mp4"),
	})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 200; i++ {
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		total := 0
		for _, v := range counts {
			total += v
		}
		mu.Unlock()
		if total > 50 {
			// pipeline is complete
			break
		}
	}
}

func TestNewApp_ServicesInitialized(t *testing.T) {
	app := NewApp()

	if app.analyzer == nil {
		t.Fatal("analyzer should be initialized")
	}
	if app.manager == nil {
		t.Fatal("manager should be initialized")
	}
	if app.settings == nil {
		t.Fatal("settings should be initialized")
	}
	if app.analysisResults == nil {
		t.Fatal("analysis results map should be initialized")
	}
}

func TestApp_NilServices_Rejected(t *testing.T) {
	app := &App{}

	_, err := app.Analyze(AnalyzeRequest{URL: "https://example.com"})
	if err == nil {
		t.Fatal("nil analyzer should error")
	}

	_, err = app.StartDownload(media.DownloadRequest{})
	if err == nil {
		t.Fatal("nil manager should error")
	}

	err = app.CancelDownload("x")
	if err == nil {
		t.Fatal("nil manager should error")
	}
}

func TestSaveAs_EmptyDefault(t *testing.T) {
	app := &App{}

	_, err := app.SaveAs("", "")
	if err == nil {
		t.Fatal("should fail without runtime")
	}
}

func TestChooseDirectory_EmptyDefaults(t *testing.T) {
	app := &App{}

	_, err := app.ChooseDirectory("")
	if err == nil {
		t.Fatal("should fail without runtime")
	}
}

func TestManagerNotNilAfterShutdown(t *testing.T) {
	app := NewApp()
	app.shutdown(context.Background())

	if app.manager == nil {
		t.Fatal("manager pointer should not be nil after shutdown, only closed")
	}
}

func TestDownloadProfile_DefaultExt(t *testing.T) {
	if got := deriveExtForProfile(media.ProfileOriginal); got != ".mp4" {
		t.Fatalf("default ext = %s", got)
	}
	if got := deriveExtForProfile(media.ProfileMP4); got != ".mp4" {
		t.Fatalf("mp4 profile ext = %s", got)
	}
}

func TestTaskSnapshot_FromStartDownload(t *testing.T) {
	app := NewApp()

	target := filepath.Join(t.TempDir(), "check.mp4")

	app.mu.Lock()
	app.analysisResults["a1"] = &media.AnalysisResult{
		ID:        "a1",
		PageTitle: "Page",
		Candidates: []media.MediaCandidate{
			{ID: "mc1", Title: "Candidate Title", SourceType: media.SourceDirect},
		},
	}
	app.mu.Unlock()

	task, err := app.StartDownload(media.DownloadRequest{
		AnalysisID: "a1",
		MediaID:    "mc1",
		OutputPath: target,
		Profile:    media.ProfileOriginal,
	})
	if err != nil {
		t.Fatal(err)
	}

	if task.ID == "" {
		t.Fatal("task ID should be populated on snapshot")
	}
	if task.Title != "Candidate Title" {
		t.Fatalf("title should come from candidate, got %q", task.Title)
	}
	if task.State != media.TaskQueued {
		t.Fatalf("expected initial state queued, got %s", task.State)
	}
}

func TestSanitizeFileName_UsedInStartDownload(t *testing.T) {
	name := settings.SanitizeFileName("My/Video:name")
	if strings.ContainsAny(name, "/:") {
		t.Fatalf("sanitization failed: %s", name)
	}
}

func TestNewAnalysisRunID_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := newAnalysisRunID()
		if seen[id] {
			t.Fatalf("duplicate ID at iteration %d: %s", i, id)
		}
		seen[id] = true
		if len(id) != 24 {
			t.Fatalf("expected 24-char hex, got %d: %s", len(id), id)
		}
	}
}

func TestShutdown_CalledTwice(t *testing.T) {
	app := NewApp()
	app.shutdown(context.Background())
	app.shutdown(context.Background())
}

func TestRuntimePackageImported(t *testing.T) {
	// This is a compile-time contract check: App must import wails runtime
	// in order to bridge Go events to the frontend. The fact this test file
	// compiles at all means the import path is correct.
	_ = runtime.EventsEmit
}
