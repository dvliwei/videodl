package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"videodl/internal/analyzer"
	"videodl/internal/download"
	"videodl/internal/ffmpeg"
	"videodl/internal/media"
	"videodl/internal/settings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the single Wails facade bound to the frontend. It owns the analyzer,
// download manager and settings, and is the only place allowed to import the
// Wails runtime package. Business packages must stay free of Wails imports.
type App struct {
	ctx context.Context

	analyzer *analyzer.AnalysisService
	manager  *download.Manager
	settings *settings.Settings

	mu              sync.RWMutex
	analysisResults map[string]*media.AnalysisResult
	analysisCancel  map[string]context.CancelFunc
}

// NewApp creates an App with default internal services. It does not start the
// download pipeline; startup() wires up the Wails runtime callbacks and
// shutdown() tears everything down.
func NewApp() *App {
	safeClient := analyzer.NewSafeClient(analyzer.DefaultConfig())
	var invoker *ffmpeg.Invoker

	app := &App{
		analyzer:        analyzer.NewAnalysisService(safeClient, invoker),
		manager:         download.NewManager(download.ManagerConfig{}),
		settings:        settings.NewSettings(),
		analysisResults: make(map[string]*media.AnalysisResult),
		analysisCancel:  make(map[string]context.CancelFunc),
	}

	app.manager.SetEventCallback(app.emitTaskEvent)

	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	if a.manager != nil {
		_ = a.manager.Close()
	}
}

// --- event bridges ---

func (a *App) emitTaskEvent(evt media.TaskEvent) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, media.EventTaskUpdate, evt)
}

func (a *App) emitAnalysisEvent(evt media.AnalysisEvent) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, media.EventAnalysisUpdate, evt)
}

// --- Analyze ---

// AnalyzeRequest is the JSON body accepted by Analyze. It is intentionally
// small – all analyzer configuration lives server-side.
type AnalyzeRequest struct {
	URL string `json:"url"`
}

// AnalyzeResponse is returned by Analyze. The analysis runs asynchronously;
// the caller receives the analysisId immediately and must listen for
// EventAnalysisUpdate to get the final result or failure.
type AnalyzeResponse struct {
	AnalysisID string              `json:"analysisId"`
	Phase      media.AnalysisPhase `json:"phase"`
}

// Analyze accepts a URL and starts asynchronous analysis. It returns the
// analysisID that uniquely identifies this run; the full result is delivered
// via EventAnalysisUpdate. The context stored by Wails is used for cancellation.
func (a *App) Analyze(req AnalyzeRequest) (*AnalyzeResponse, error) {
	if a.analyzer == nil {
		return nil, errors.New("analyzer service not initialized")
	}

	id := newAnalysisRunID()

	a.emitAnalysisEvent(media.AnalysisEvent{
		Version:    media.EventVersion,
		AnalysisID: id,
		Phase:      media.AnalysisRunning,
	})

	runCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

	a.mu.Lock()
	a.analysisCancel[id] = cancel
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			delete(a.analysisCancel, id)
			a.mu.Unlock()
			cancel()
		}()

		result, err := a.analyzer.Analyze(runCtx, req.URL)
		if runCtx.Err() == context.Canceled {
			a.emitAnalysisEvent(media.AnalysisEvent{
				Version:    media.EventVersion,
				AnalysisID: id,
				Phase:      media.AnalysisCanceled,
			})
			return
		}
		if err != nil {
			a.emitAnalysisEvent(media.AnalysisEvent{
				Version:      media.EventVersion,
				AnalysisID:   id,
				Phase:        media.AnalysisFailed,
				ErrorCode:    classifyAnalyzerError(err),
				ErrorMessage: err.Error(),
			})
			return
		}

		if result != nil {
			a.mu.Lock()
			a.analysisResults[id] = result
			a.mu.Unlock()
		}

		a.emitAnalysisEvent(media.AnalysisEvent{
			Version:      media.EventVersion,
			AnalysisID:   id,
			Phase:        media.AnalysisCompleted,
			PageTitle:    result.PageTitle,
			CandidateCnt: len(result.Candidates),
			Result:       result,
		})
	}()

	return &AnalyzeResponse{
		AnalysisID: id,
		Phase:      media.AnalysisRunning,
	}, nil
}

// --- GetAnalysis ---

// GetAnalysis retrieves a previously cached AnalysisResult by its ID. The
// frontend typically does not need this since the full result arrives via
// EventAnalysisUpdate, but it is exposed for refresh / recovery scenarios.
func (a *App) GetAnalysis(analysisID string) (*media.AnalysisResult, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	r, ok := a.analysisResults[analysisID]
	if !ok {
		return nil, fmt.Errorf("analysis %s not found", analysisID)
	}
	return r, nil
}

// CancelAnalysis cancels a running analysis identified by analysisID. It is
// safe to call on an already-completed or already-canceled analysis. The
// caller receives no error when the analysis is already gone.
func (a *App) CancelAnalysis(analysisID string) error {
	a.mu.Lock()
	cancel, ok := a.analysisCancel[analysisID]
	a.mu.Unlock()
	if !ok {
		return nil
	}
	cancel()
	return nil
}

// --- StartDownload ---

// StartDownload creates a new download task from a previously analyzed item.
// The requested OutputPath is the desired target; the manager may rewrite it
// when resolving conflicts. The returned DownloadTask snapshot carries the
// initial queued state.
func (a *App) StartDownload(req media.DownloadRequest) (*media.DownloadTask, error) {
	if a.manager == nil {
		return nil, errors.New("download manager not initialized")
	}

	title, err := a.resolveDownloadTitle(req.AnalysisID, req.MediaID)
	if err != nil {
		return nil, err
	}

	if req.OutputPath == "" {
		dir, dirErr := a.settings.GetDownloadDirectory()
		if dirErr != nil {
			return nil, fmt.Errorf("resolve default directory: %w", dirErr)
		}
		safe := settings.SanitizeFileName(title)
		ext := deriveExtForProfile(req.Profile)
		req.OutputPath = filepath.Join(dir, safe+ext)
	}

	task, err := a.manager.Create(req, title)
	if err != nil {
		return nil, err
	}

	snap := task.Snapshot()
	return &snap, nil
}

func (a *App) resolveDownloadTitle(analysisID, mediaID string) (string, error) {
	a.mu.RLock()
	res, ok := a.analysisResults[analysisID]
	a.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("analysis %s not found", analysisID)
	}
	for _, c := range res.Candidates {
		if c.ID == mediaID {
			if c.Title != "" {
				return c.Title, nil
			}
			if res.PageTitle != "" {
				return res.PageTitle, nil
			}
			return "untitled", nil
		}
	}
	return "", fmt.Errorf("media %s not found in analysis %s", mediaID, analysisID)
}

func deriveExtForProfile(profile media.DownloadProfile) string {
	switch profile {
	case media.ProfileMP4:
		return ".mp4"
	default:
		return ".mp4"
	}
}

// --- Task control ---

func (a *App) GetTask(id string) (*media.DownloadTask, error) {
	if a.manager == nil {
		return nil, errors.New("download manager not initialized")
	}
	t, err := a.manager.Get(id)
	if err != nil {
		return nil, err
	}
	snap := t.Snapshot()
	return &snap, nil
}

func (a *App) ListTasks() ([]media.DownloadTask, error) {
	if a.manager == nil {
		return nil, errors.New("download manager not initialized")
	}
	return a.manager.Snapshots(), nil
}

func (a *App) CancelDownload(id string) error {
	if a.manager == nil {
		return errors.New("download manager not initialized")
	}
	return a.manager.Cancel(id)
}

func (a *App) RetryDownload(id string) (*media.DownloadTask, error) {
	if a.manager == nil {
		return nil, errors.New("download manager not initialized")
	}
	t, err := a.manager.Retry(id)
	if err != nil {
		return nil, err
	}
	snap := t.Snapshot()
	return &snap, nil
}

// --- Settings ---

type DownloadDirectoryResponse struct {
	Path      string `json:"path"`
	IsDefault bool   `json:"isDefault"`
}

// GetDefaultDirectory returns the currently configured download directory.
// When no explicit setting exists it falls back to the platform Downloads
// folder and marks IsDefault so the UI can hint the user to set one.
func (a *App) GetDefaultDirectory() (*DownloadDirectoryResponse, error) {
	cfg, err := a.settings.Load()
	if err != nil {
		return nil, err
	}
	dir, err := a.settings.GetDownloadDirectory()
	if err != nil {
		return nil, err
	}
	return &DownloadDirectoryResponse{
		Path:      dir,
		IsDefault: cfg == nil || cfg.DownloadDirectory == "",
	}, nil
}

// SetDefaultDirectory persists an explicit download directory. It ensures the
// folder exists and is writable before returning.
func (a *App) SetDefaultDirectory(path string) error {
	return a.settings.SetDownloadDirectory(path)
}

// --- Dialogs ---

// ChooseDirectory opens the native folder-picker dialog. If the user cancels
// the dialog, an empty string is returned with no error – this lets the
// frontend distinguish "user canceled" from a genuine failure without needing
// an extra boolean sentinel.
func (a *App) ChooseDirectory(defaultDir string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("runtime not ready")
	}
	if defaultDir == "" {
		if d, err := a.settings.GetDownloadDirectory(); err == nil {
			defaultDir = d
		}
	}
	if defaultDir == "" {
		defaultDir, _ = os.UserHomeDir()
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		DefaultDirectory: defaultDir,
		Title:            "选择下载目录",
	})
}

// SaveAs opens the native save dialog. defaultName is an optional suggested
// filename (with extension); dirHint hints the initial directory. Canceling
// returns ("", nil).
func (a *App) SaveAs(defaultName string, dirHint string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("runtime not ready")
	}
	if dirHint == "" {
		if d, err := a.settings.GetDownloadDirectory(); err == nil {
			dirHint = d
		}
	}
	if dirHint == "" {
		dirHint, _ = os.UserHomeDir()
	}
	if defaultName == "" {
		defaultName = "video.mp4"
	}
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultDirectory: dirHint,
		DefaultFilename:  settings.SanitizeFileName(defaultName),
		Title:            "另存为",
	})
}

// --- helpers ---

func newAnalysisRunID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))[:24]
	}
	return hex.EncodeToString(b)
}

func classifyAnalyzerError(err error) string {
	if errors.Is(err, analyzer.ErrInvalidScheme) ||
		errors.Is(err, analyzer.ErrURLHasUserInfo) ||
		errors.Is(err, analyzer.ErrInvalidHost) {
		return "analyzer.invalid_url"
	}
	if errors.Is(err, analyzer.ErrBlacklistedAddress) ||
		errors.Is(err, analyzer.ErrRedirectLoopback) {
		return "analyzer.address_rejected"
	}
	if errors.Is(err, analyzer.ErrTooManyRedirects) {
		return "analyzer.too_many_redirects"
	}
	if errors.Is(err, analyzer.ErrBodyTooLarge) {
		return "analyzer.body_too_large"
	}
	return string(media.ErrCodeUnknown)
}
