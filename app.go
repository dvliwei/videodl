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
	"videodl/internal/netguard"
	"videodl/internal/openpath"
	"videodl/internal/proxy"
	"videodl/internal/settings"
	"videodl/internal/ytdlp"

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
	invoker  *ffmpeg.Invoker

	ytdlpUpdater      *ytdlp.Updater
	ytdlpRunner       *ytdlp.Runner
	ytdlpProxyFactory analyzer.YTDLPProxyFactory
	ytdlpUserDir      string
	ytdlpUpdateMu     sync.Mutex

	mu              sync.RWMutex
	analysisResults map[string]*media.AnalysisResult
	analysisCancel  map[string]context.CancelFunc
}

// NewApp creates an App with default internal services. It probes for bundled
// FFmpeg/FFprobe at construction time (via ResolveFromExecutable + cwd
// fallback) and configures the analyzer with the resulting invoker. If no
// bundled tools are available yet the invoker stays nil and the analyzer runs
// in probe-less mode. startup() re-tries the probe so a different working
// directory does not matter.
func NewApp() *App {
	safeClient := analyzer.NewSafeClient(analyzer.DefaultConfig())
	invoker := tryResolveInvoker()
	appSettings := settings.NewSettings()
	ytdlpProxyFactory := defaultYTDLPProxyFactory
	ytdlpRunner := tryResolveYTDLPRunner(appSettings)

	app := &App{
		analyzer:          analyzer.NewAnalysisServiceWithYTDLP(safeClient, invoker, ytdlpRunner, ytdlpProxyFactory),
		settings:          appSettings,
		invoker:           invoker,
		ytdlpRunner:       ytdlpRunner,
		ytdlpProxyFactory: ytdlpProxyFactory,
		analysisResults:   make(map[string]*media.AnalysisResult),
		analysisCancel:    make(map[string]context.CancelFunc),
	}
	if userDir, err := appSettings.ConfigPath(); err == nil {
		app.ytdlpUserDir = filepath.Dir(userDir)
		app.ytdlpUpdater = ytdlp.NewUpdater(
			ytdlp.NewOfficialReleaseClient(nil),
			ytdlp.DefaultBinaryResolver{},
			app.ytdlpUserDir,
		)
	}

	app.manager = download.NewManager(download.ManagerConfig{
		Invoker:  invoker,
		Resolver: appMediaSourceResolver{app: app},
	})

	app.manager.SetEventCallback(app.emitTaskEvent)

	return app
}

func tryResolveInvoker() *ffmpeg.Invoker {
	resourcesRoot, err := ffmpeg.ResolveFromExecutable()
	if err != nil {
		return nil
	}
	paths, err := ffmpeg.ResolvePaths(resourcesRoot)
	if err != nil {
		return nil
	}
	return ffmpeg.NewInvoker(paths)
}

func tryResolveYTDLPRunner(s *settings.Settings) *ytdlp.Runner {
	if s == nil {
		return nil
	}
	resolver := ytdlp.DefaultBinaryResolver{}
	if configPath, err := s.ConfigPath(); err == nil {
		userRoot := filepath.Join(filepath.Dir(configPath), "yt-dlp")
		if candidate, err := resolver.ResolveUserCandidate(userRoot); err == nil {
			return ytdlp.NewRunner(ytdlp.RunConfig{BinaryPath: candidate.Path})
		}
	}
	candidate, err := resolver.ResolveBundledCandidate("")
	if err != nil {
		return nil
	}
	return ytdlp.NewRunner(ytdlp.RunConfig{BinaryPath: candidate.Path})
}

func defaultYTDLPProxyFactory(ctx context.Context) (analyzer.YTDLPProxy, error) {
	return netguard.NewProxy(ctx, netguard.NewResolver(nil), netguard.ProxyConfig{
		UpstreamProxyURL: proxy.ResolveSystemProxyURL(),
	})
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	if a.invoker == nil {
		if retry := tryResolveInvoker(); retry != nil {
			a.invoker = retry
			a.analyzer.SetInvoker(retry)
		}
	}

	if a.invoker != nil {
		if err := a.invoker.VerifyFFmpegVersion(ctx); err != nil {
			println("startup: ffmpeg version check failed:", err.Error())
		}
		if err := a.invoker.VerifyNetworkProtocols(ctx); err != nil {
			println("startup: bundled ffmpeg cannot download HTTPS media:", err.Error())
		}
		if err := a.invoker.VerifyFFprobeVersion(ctx); err != nil {
			println("startup: ffprobe version check failed:", err.Error())
		}
	} else {
		println("startup: no bundled FFmpeg found — analyzer will run without media probing")
	}

	if a.ytdlpRunner == nil {
		a.ytdlpRunner = tryResolveYTDLPRunner(a.settings)
		if a.ytdlpRunner != nil {
			a.analyzer.SetYTDLP(a.ytdlpRunner, a.ytdlpProxyFactory)
		}
	}
	if a.ytdlpRunner != nil {
		if _, err := a.ytdlpRunner.Version(ctx); err != nil {
			println("startup: yt-dlp version check failed:", err.Error())
		}
	} else {
		println("startup: no bundled yt-dlp found — website extraction is unavailable")
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(a.analysisCancel))
	for id, cancel := range a.analysisCancel {
		cancels = append(cancels, cancel)
		delete(a.analysisCancel, id)
	}
	a.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	if a.manager != nil {
		_ = a.manager.Close()
	}
}

// --- download.MediaSourceResolver ---

// Resolve remains a Wails-compatible method. The manager uses the context-
// aware adapter below so yt-dlp re-resolution can be canceled with a task.
func (a *App) Resolve(analysisID, mediaID, variantID string) (*download.MediaSource, error) {
	return a.resolveSource(context.Background(), analysisID, mediaID, variantID)
}

type appMediaSourceResolver struct{ app *App }

func (r appMediaSourceResolver) Resolve(ctx context.Context, analysisID, mediaID, variantID string) (*download.MediaSource, error) {
	return r.app.resolveSource(ctx, analysisID, mediaID, variantID)
}

func (a *App) resolveSource(ctx context.Context, analysisID, mediaID, variantID string) (*download.MediaSource, error) {
	a.mu.RLock()
	result, ok := a.analysisResults[analysisID]
	if !ok {
		a.mu.RUnlock()
		return nil, fmt.Errorf("analysis session %q not found", analysisID)
	}

	var candidate *media.MediaCandidate
	for i := range result.Candidates {
		if result.Candidates[i].ID == mediaID {
			copy := result.Candidates[i]
			candidate = &copy
			break
		}
	}
	a.mu.RUnlock()
	if candidate == nil {
		return nil, fmt.Errorf("media %q not found in analysis %q", mediaID, analysisID)
	}

	if candidate.InternalYTDLPSource != nil {
		selector := candidate.InternalYTDLPSource.FormatSelector
		if variantID != "" {
			for _, variant := range candidate.Variants {
				if variant.ID == variantID {
					selector = variant.InternalFormatSelector
					break
				}
			}
		}
		inputs, err := a.analyzer.ResolveYTDLPInputs(ctx, candidate.InternalYTDLPSource, selector)
		if err != nil {
			return nil, fmt.Errorf("yt-dlp re-resolution failed: %w", err)
		}
		return &download.MediaSource{Inputs: inputs, SourceType: candidate.SourceType, DurationSeconds: candidate.DurationSeconds}, nil
	}

	inputURL := candidate.InternalSourceURL
	if variantID != "" {
		if sub, ok := candidate.InternalVariantManifests[variantID]; ok && sub != "" {
			inputURL = sub
		}
	}
	if inputURL == "" {
		return nil, fmt.Errorf("candidate %q has no internal source URL", mediaID)
	}

	return &download.MediaSource{
		Inputs:          []ffmpeg.Input{{URL: inputURL}},
		SourceType:      candidate.SourceType,
		DurationSeconds: candidate.DurationSeconds,
	}, nil
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

// AnalyzeWithBrowserSessionRequest is an explicit, per-attempt opt-in to
// read cookies from one supported local browser profile. It never contains
// cookie contents or a filesystem path.
type AnalyzeWithBrowserSessionRequest struct {
	URL     string `json:"url"`
	Browser string `json:"browser"`
	Profile string `json:"profile,omitempty"`
}

// YTDLPStatus is the frontend-safe update status. Executable paths, cookie
// data and release URLs are intentionally not exposed.
type YTDLPStatus struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"currentVersion,omitempty"`
	LatestVersion  string `json:"latestVersion,omitempty"`
	Source         string `json:"source"`
	CanUpdate      bool   `json:"canUpdate"`
}

// Analyze accepts a URL and starts asynchronous analysis. It returns the
// analysisID that uniquely identifies this run; the full result is delivered
// via EventAnalysisUpdate. The context stored by Wails is used for cancellation.
func (a *App) Analyze(req AnalyzeRequest) (*AnalyzeResponse, error) {
	return a.startAnalysis(req.URL, analyzer.AnalyzeOptions{})
}

// AnalyzeWithBrowserSession starts an analysis with an explicit browser-cookie
// authorization for this attempt only. Validation happens synchronously so an
// invalid browser can never create an asynchronous analysis session.
func (a *App) AnalyzeWithBrowserSession(req AnalyzeWithBrowserSessionRequest) (*AnalyzeResponse, error) {
	session := ytdlp.BrowserSession{Browser: req.Browser, Profile: req.Profile}
	if err := ytdlp.ValidateBrowserSession(session); err != nil {
		return nil, err
	}
	return a.startAnalysis(req.URL, analyzer.AnalyzeOptions{BrowserSession: &session})
}

func (a *App) startAnalysis(rawURL string, options analyzer.AnalyzeOptions) (*AnalyzeResponse, error) {
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

		result, err := a.analyzer.AnalyzeWithOptions(runCtx, rawURL, options)
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

// GetYTDLPStatus checks the official stable release and reports whether a
// usable bundled or user-updated executable is available. A missing bundle is
// represented as an unavailable status so first-run UI can explain packaging
// problems without treating them as a network failure.
func (a *App) GetYTDLPStatus() (*YTDLPStatus, error) {
	if a == nil || a.ytdlpUpdater == nil {
		return &YTDLPStatus{Source: "unavailable"}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	status, err := a.ytdlpUpdater.Status(ctx)
	if err != nil {
		if errors.Is(err, ytdlp.ErrBinaryNotFound) || errors.Is(err, ytdlp.ErrPlatformNotSupported) {
			return &YTDLPStatus{Source: "unavailable"}, nil
		}
		return nil, err
	}
	return mapYTDLPStatus(status), nil
}

// UpdateYTDLP downloads only the official stable release after checksum
// verification and executable health-checking. It updates the runner used by
// future analyses without interrupting tasks already in progress.
func (a *App) UpdateYTDLP() (*YTDLPStatus, error) {
	if a == nil || a.ytdlpUpdater == nil {
		return nil, fmt.Errorf("yt-dlp updater is unavailable")
	}
	a.ytdlpUpdateMu.Lock()
	defer a.ytdlpUpdateMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	status, err := a.ytdlpUpdater.Update(ctx)
	if err != nil {
		return nil, err
	}
	if runner := tryResolveYTDLPRunner(a.settings); runner != nil && a.analyzer != nil {
		a.ytdlpRunner = runner
		a.analyzer.SetYTDLP(runner, a.ytdlpProxyFactory)
	}
	return mapYTDLPStatus(status), nil
}

func mapYTDLPStatus(status *ytdlp.UpdateStatus) *YTDLPStatus {
	if status == nil {
		return &YTDLPStatus{Source: "unavailable"}
	}
	return &YTDLPStatus{
		Available:      status.CurrentVersion != "",
		CurrentVersion: status.CurrentVersion,
		LatestVersion:  status.LatestVersion,
		Source:         status.Source,
		CanUpdate:      status.Available,
	}
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

// --- File reveal ---

func (a *App) OpenPath(path string) error {
	return openpath.OpenPath(path)
}

func (a *App) OpenContainingFolder(path string) error {
	return openpath.RevealInFinder(path)
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
	if errors.Is(err, ytdlp.ErrAuthRequired) {
		return "ytdlp.auth_required"
	}
	if errors.Is(err, ytdlp.ErrExtractorUnsupported) {
		return "ytdlp.extractor_unsupported"
	}
	if errors.Is(err, ytdlp.ErrNoFormats) {
		return "ytdlp.no_formats"
	}
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
