# yt-dlp Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add bundled yt-dlp website extraction, optional user-authorized browser-session extraction, safe stable-channel updates, and FFmpeg-compatible multi-input downloads without requiring Python.

**Architecture:** yt-dlp runs as a restricted child process for structured extraction and short-lived format URL resolution. Go remains responsible for URL safety, task lifecycle, cancellation, temporary files, atomic publishing, and FFmpeg execution. A local outbound proxy applies the existing public-address policy to yt-dlp requests.

**Tech Stack:** Go 1.25, Wails v2.16, Vue 3, Vite 7, existing FFmpeg/FFprobe resources, yt-dlp 2026.08.19 baseline release assets.

**Spec:** `docs/superpowers/specs/2026-10-10-yt-dlp-integration-design.md`

## Global Constraints

- The application must not require Python installed by the user.
- yt-dlp must run as a separate bundled executable; use the locked baseline `2026.08.19` for the first package.
- The first package uses the official stable assets `yt-dlp.exe` (Windows x86_64), `yt-dlp_linux` (Linux x86_64), and `yt-dlp_macos` (universal macOS).
- The app must continue to support only user-authorized, public, non-DRM HTTP/HTTPS media and must not bypass login, paywalls, access controls, or DRM.
- Default yt-dlp calls must ignore user configuration, browser cookies, and plugins; set `YTDLP_NO_PLUGINS=1` and pass `--ignore-config`.
- Browser-session extraction is opt-in per attempt, limited to a backend allowlist, and must never expose cookie contents to Vue or logs.
- yt-dlp network requests must use the application-controlled restricted proxy and the same public-address policy as the existing analyzer.
- Frontend requests contain only opaque analysis/media/variant IDs; signed media URLs and request headers remain backend-only.
- All subprocesses use argument arrays, bounded output, context cancellation, exit-code checks, and child-process cleanup.
- Final files are published only after successful FFmpeg completion and validation; update failures keep the previous yt-dlp version.
- Do not hand-edit `frontend/wailsjs`; regenerate bindings with the repository's Wails CLI after Go API changes.
- Preserve the nine existing unstaged files in the working tree; never reset, checkout, clean, or overwrite unrelated user changes.
- Each task is implemented and verified independently; no task may implement a later task's UI, updater, download, or documentation work.

## Review Focus

- A format with separate video and audio URLs must produce multiple FFmpeg inputs and correct stream maps; covered by Task 6's `TestBuildDownloadArgs_MultipleInputs` and manager integration test.
- A yt-dlp redirect or CONNECT target resolving to loopback/private/link-local/reserved IP must be rejected before dialing; covered by Task 4's proxy security tests.
- A user-supplied URL, format selector, browser name, or profile containing argument-like text must remain one validated argument and cannot inject flags; covered by Tasks 2, 3, and 8.
- A failed, canceled, or checksum-mismatched update must leave the previously verified executable usable; covered by Task 7's rollback tests.
- A browser-session authorization must not occur during default analysis and must not leak cookies or secrets into the result/event/log; covered by Tasks 3, 5, and 8.

---

### Task 1: Bundle manifest, platform assets, and runtime binary resolution

**Files:**
- Create: `build/resources/yt-dlp/manifest.yaml`
- Create: `build/resources/yt-dlp/CREDITS.md`
- Create: `build/resources/yt-dlp/THIRD-PARTY-NOTICES.txt`
- Create: `build/resources/yt-dlp/licenses/yt-dlp-UNLICENSE.txt`
- Create: `build/resources/yt-dlp/licenses/THIRD_PARTY_LICENSES.txt`
- Create: `build/resources/yt-dlp/licenses/GPL-3.0-or-later.txt`
- Create: `internal/ytdlp/binary.go`
- Test: `internal/ytdlp/binary_test.go`
- Create: `scripts/fetch_yt_dlp.sh`
- Create: `scripts/verify_yt_dlp.sh`
- Modify: `Makefile`

**Interfaces:**
- Produces `ytdlp.PlatformAsset`, `ytdlp.BinarySource`, `ytdlp.BinaryCandidate`, and `ytdlp.ResolveBundledCandidate(root string) (BinaryCandidate, error)`.
- Produces `ytdlp.BinaryResolver` with `ResolveBundledCandidate(root string) (BinaryCandidate, error)` and `ResolveUserCandidate(root string) (BinaryCandidate, error)` for the updater.
- `BinaryCandidate` exposes only the verified executable path, version, platform key, and source label to later tasks; it never exposes an arbitrary PATH lookup.

- [ ] **Step 1: Write failing manifest and resolver tests**

  Add tests for all four supported platform keys, rejection of an unknown platform, missing executable, non-executable Unix file, wrong filename, and resolution from macOS `Contents/Resources/yt-dlp` versus Windows/Linux `resources/yt-dlp`.

- [ ] **Step 2: Run the focused tests and verify they fail for missing resolver/manifest behavior**

  Run: `go test ./internal/ytdlp -run 'Test(ResolveBundledCandidate|PlatformAsset)' -count=1`

  Expected: FAIL because the package and resolver do not yet exist.

- [ ] **Step 3: Add the locked 2026.08.19 manifest and resolver**

  Record the official asset URLs, exact SHA-256 values from the release `SHA2-256SUMS`, release tag, platform, architecture, filename, and license files. Use no PATH fallback. Use the existing FFmpeg resource-resolution convention for bundle roots.

- [ ] **Step 4: Add fetch/verify scripts and Make targets**

  Add `yt-dlp-fetch`, `yt-dlp-verify`, `yt-dlp-clean`, and `yt-dlp-manifest-test`. Fetch only the three official assets and checksum/license materials; verify before accepting a file. Do not commit downloaded binaries unless the existing repository policy already tracks equivalent FFmpeg binaries.

- [ ] **Step 5: Run focused and manifest checks**

  Run: `go test ./internal/ytdlp -run 'Test(ResolveBundledCandidate|PlatformAsset)' -count=1`

  Run: `make yt-dlp-manifest-test` and `make yt-dlp-verify`

  Expected: resolver tests pass; verification either passes with fetched assets or reports the precise missing local asset without modifying user files.

- [ ] **Step 6: Commit only Task 1 files**

  Stage only the manifest, license materials, resolver, scripts, Makefile changes, and tests. Do not stage the nine pre-existing user changes.

### Task 2: yt-dlp subprocess wrapper and stable JSON extraction

**Files:**
- Create: `internal/ytdlp/process.go`
- Create: `internal/ytdlp/extract.go`
- Create: `internal/ytdlp/types.go`
- Test: `internal/ytdlp/process_test.go`
- Test: `internal/ytdlp/extract_test.go`
- Create: `internal/ytdlp/testdata/single.json`
- Create: `internal/ytdlp/testdata/separate-av.json`
- Create: `internal/ytdlp/testdata/no-duration.json`
- Create: `internal/ytdlp/testdata/playlist.json`

**Interfaces:**
- `type RunConfig struct { BinaryPath string; MaxStdoutBytes int64; MaxStderrBytes int64; Env []string }`
- `type Runner struct { ... }`
- `func NewRunner(cfg RunConfig) *Runner`
- `func (r *Runner) Run(ctx context.Context, args []string) (RunResult, error)`
- `func (r *Runner) Version(ctx context.Context) (string, error)`
- `type BrowserSession struct { Browser string; Profile string }`
- `type ExtractRequest struct { URL string; FormatSelector string; Browser *BrowserSession; ProxyURL string }`
- `func (r *Runner) Extract(ctx context.Context, req ExtractRequest) (*Info, error)`
- `Run` must return bounded stdout/stderr and a structured process error; callers must not receive unbounded output.

- [ ] **Step 1: Write failing process and JSON tests**

  Test argument ordering, fixed security flags, `YTDLP_NO_PLUGINS=1`, stdout/stderr limits, invalid JSON, playlist rejection, cancellation, timeout, non-zero exit, and version parsing. Assert URL, selector, browser and profile values remain individual argv entries.

- [ ] **Step 2: Run the focused tests and verify the expected red state**

  Run: `go test ./internal/ytdlp -run 'Test(Run|Extract|Version)' -count=1`

  Expected: FAIL because the wrapper and types are absent.

- [ ] **Step 3: Implement the minimal process wrapper**

  Use `exec.CommandContext`, the platform child-process attributes already used by `internal/ffmpeg`, bounded buffers, and explicit environment construction. Do not invoke a shell. Do not include full URLs or cookie-related arguments in error text.

- [ ] **Step 4: Implement `Extract` using `--dump-single-json --no-playlist --ignore-config --no-cookies-from-browser`**

  Decode only the fields needed by later mapping. Treat a playlist response as a structured unsupported result rather than silently downloading multiple entries.

- [ ] **Step 5: Run focused tests and the full Go suite**

  Run: `go test ./internal/ytdlp -count=1`

  Run: `go test ./... -count=1`

  Expected: both commands pass; existing packages remain green.

- [ ] **Step 6: Commit only Task 2 files**

### Task 3: yt-dlp format model, media mapping, and authorization validation

**Files:**
- Modify: `internal/media/model.go`
- Modify: `internal/ytdlp/types.go`
- Create: `internal/ytdlp/formats.go`
- Test: `internal/ytdlp/formats_test.go`
- Modify: `internal/media/model_test.go`

**Interfaces:**
- Add `media.SourceYTDLP` without changing existing source JSON values.
- Add backend-only `media.YTDLPSource` containing page URL, format selector, browser name, and profile; tag all secret-bearing or authority-bearing fields with `json:"-"`.
- Add `func MapInfoToCandidate(info *Info) (media.MediaCandidate, error)`.
- Add `func ValidateBrowserSession(s BrowserSession) error` with the allowlist `brave`, `chrome`, `chromium`, `edge`, `firefox`, `opera`, `safari`, `vivaldi`, `whale`.
- Add stable error values for extractor unsupported, auth required, no formats, invalid browser, and playlist input.

- [ ] **Step 1: Write failing mapping and authorization tests**

  Cover one combined format, separate video/audio formats, missing filesize/duration, unknown codec values, stable opaque IDs, playlist rejection, supported browsers, unsupported browsers, path-like profile injection, and JSON serialization that excludes source URLs, headers, Cookie values, and format selectors.

- [ ] **Step 2: Run focused tests and verify failure**

  Run: `go test ./internal/ytdlp ./internal/media -run 'Test(MapInfo|ValidateBrowser|YTDLP)' -count=1`

  Expected: FAIL because the new source/model mapping does not exist.

- [ ] **Step 3: Implement the minimal raw-to-domain mapping**

  Map title, extractor, extension, resolution, duration, size, audio/video presence, and variants. Generate opaque IDs from stable backend values; never use a signed URL as a frontend ID. Store selector and request context only in backend-only fields.

- [ ] **Step 4: Run focused tests and inspect JSON output**

  Run: `go test ./internal/ytdlp ./internal/media -count=1`

  Expected: PASS with no authority-bearing fields in serialized candidates.

- [ ] **Step 5: Commit only Task 3 files**

### Task 4: Restricted outbound proxy for yt-dlp

**Files:**
- Create: `internal/netguard/proxy.go`
- Create: `internal/netguard/resolver.go`
- Test: `internal/netguard/proxy_test.go`
- Test: `internal/netguard/resolver_test.go`
- Modify: `internal/analyzer/ip.go` only if a reusable validator must be extracted without changing behavior

**Interfaces:**
- `type AddressValidator interface { ValidateHost(ctx context.Context, host string) error }`
- `type ProxyConfig struct { MaxConnections int; ConnectTimeout time.Duration; TotalTimeout time.Duration }`
- `func NewProxy(ctx context.Context, validator AddressValidator, cfg ProxyConfig) (*Proxy, error)`
- `func (p *Proxy) URL() string`
- `func (p *Proxy) Close() error`
- The proxy must support HTTP requests and HTTPS CONNECT, but only after validating every upstream hostname/IP. It must bind to loopback on an ephemeral port and close with the analysis/download context.

- [ ] **Step 1: Write failing proxy security tests**

  Test rejection before dial for loopback, private IPv4, link-local IPv4, multicast/reserved IPv4, IPv6 loopback/link-local/private/reserved addresses, blocked redirects, unsupported schemes, and CONNECT targets. Use injected DNS/dial behavior so tests do not depend on real network access.

- [ ] **Step 2: Run focused tests and verify failure**

  Run: `go test ./internal/netguard -count=1`

  Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement the minimum proxy and shared validation adapter**

  Reuse the existing IP classification rules rather than creating a second blacklist. Do not log request headers, Cookie, Authorization, query strings, or response bodies.

- [ ] **Step 4: Run focused tests, race tests, and existing analyzer IP tests**

  Run: `go test ./internal/netguard ./internal/analyzer -run 'Test.*(IP|Address|Proxy|Redirect)' -count=1`

  Run: `go test -race ./internal/netguard -count=1`

  Expected: PASS with no goroutine or listener leak.

- [ ] **Step 5: Commit only Task 4 files**

### Task 5: yt-dlp-first analyzer with native fallback

**Files:**
- Modify: `internal/analyzer/service.go`
- Modify: `internal/analyzer/service_test.go`
- Create: `internal/analyzer/ytdlp_integration_test.go`
- Modify: `app.go` only for dependency wiring required by the analyzer, not for new Wails methods

**Interfaces:**
- Add `type AnalyzeOptions struct { BrowserSession *ytdlp.BrowserSession }`.
- Preserve `func (s *AnalysisService) Analyze(ctx context.Context, rawURL string)` as the default no-authorization entry point.
- Add `func (s *AnalysisService) AnalyzeWithOptions(ctx context.Context, rawURL string, opts AnalyzeOptions) (*media.AnalysisResult, error)`.
- `AnalysisService` receives a yt-dlp client and a `netguard` factory through configuration; existing callers/tests must still work with yt-dlp disabled.

- [ ] **Step 1: Write failing service tests**

  Cover yt-dlp success, extractor unsupported falling back to existing HTML/HLS/DASH analysis, auth-required without browser authorization, browser-authorized extraction, blocked network target, yt-dlp timeout, cancellation, and a failed yt-dlp candidate not suppressing a valid native candidate when fallback is selected.

- [ ] **Step 2: Run focused tests and verify failure**

  Run: `go test ./internal/analyzer -run 'Test(Analyze.*YTDLP|YTDLP.*Fallback|Browser)' -count=1`

  Expected: FAIL because the service has no yt-dlp client path.

- [ ] **Step 3: Implement yt-dlp-first orchestration**

  Validate the URL with the existing validator, create/close a restricted proxy per attempt, call yt-dlp with the fixed default security options, map successful info, and classify only explicit unsupported/no-format outcomes as fallback candidates. Preserve existing analyzer behavior when yt-dlp is unavailable.

- [ ] **Step 4: Run analyzer and full Go tests**

  Run: `go test ./internal/analyzer -count=1`

  Run: `go test ./... -count=1`

  Expected: PASS with existing native analyzer tests unchanged.

- [ ] **Step 5: Commit only Task 5 files**

### Task 6: Re-resolve yt-dlp formats into multi-input FFmpeg downloads

**Files:**
- Modify: `internal/ffmpeg/download.go`
- Modify: `internal/ffmpeg/run_download.go` only if input/progress plumbing requires it
- Modify: `internal/ffmpeg/download_test.go`
- Modify: `internal/download/manager.go`
- Modify: `internal/download/manager_test.go`
- Modify: `app.go` resolver implementation
- Modify: `internal/media/model.go` only for the finalized backend source fields

**Interfaces:**
- Add `type Input struct { URL string; Headers map[string]string }` in `internal/ffmpeg`.
- Change `DownloadOptions` to carry `Inputs []Input`; keep one-input callers behavior-compatible through a small normalization helper.
- `func BuildDownloadArgs(opts DownloadOptions) ([]string, error)` emits one `-i` per input and deterministic `-map` values for separate video/audio inputs.
- Change `download.MediaSourceResolver` to `Resolve(ctx context.Context, analysisID, mediaID, variantID string) (*MediaSource, error)`.
- Change `download.MediaSource` to `Inputs []ffmpeg.Input`, `SourceType media.SourceType`, and `DurationSeconds *float64`.
- `App.Resolve` re-runs yt-dlp with the stored page URL/format selector and current browser-session choice, then returns short-lived inputs and controlled headers; native candidates return one input.

- [ ] **Step 1: Write failing FFmpeg and manager tests**

  Cover one direct input, separate video/audio inputs, headers on each input, deterministic mapping, empty input list, invalid non-HTTP input, resolver context cancellation, yt-dlp re-resolution failure, successful staging, cancellation, non-zero FFmpeg exit, and no final file publication on failure.

- [ ] **Step 2: Run focused tests and verify failure**

  Run: `go test ./internal/ffmpeg ./internal/download -run 'Test(BuildDownloadArgs|YTDLP|MultiInput|Resolve)' -count=1`

  Expected: FAIL because `Inputs` and context-aware resolution do not exist.

- [ ] **Step 3: Implement multi-input argument construction**

  Keep the existing fixed protocol whitelist, stream-copy/transcode presets, and progress pipe. Never build a shell command or accept a raw command string. Preserve the current single-input test expectations.

- [ ] **Step 4: Implement context-aware source resolution and yt-dlp re-resolution**

  Resolve fresh signed URLs at task execution time, not when the user first analyzes. Reuse the same restricted proxy and authorization context. Ensure the task manager owns cancellation and cleans temporary outputs.

- [ ] **Step 5: Run focused, race, and full Go tests**

  Run: `go test ./internal/ffmpeg ./internal/download -count=1`

  Run: `go test -race ./internal/ffmpeg ./internal/download -count=1`

  Run: `go test ./... -count=1`

  Expected: PASS and no regression in the native download pipeline.

- [ ] **Step 6: Commit only Task 6 files**

### Task 7: Stable yt-dlp update service with verification and rollback

**Files:**
- Create: `internal/ytdlp/update.go`
- Test: `internal/ytdlp/update_test.go`
- Modify: `internal/ytdlp/binary.go`
- Modify: `internal/settings/settings.go` or add a focused settings path helper if required
- Create: `internal/ytdlp/testdata/release.json`

**Interfaces:**
- `type UpdateStatus struct { Available bool; CurrentVersion string; LatestVersion string; Source string }`
- `type ReleaseClient interface { LatestStable(ctx context.Context) (Release, error); Download(ctx context.Context, asset Asset, dst io.Writer) error }`
- `type Updater struct { ... }`
- `func NewUpdater(client ReleaseClient, resolver BinaryResolver, userDir string) *Updater`
- `func (u *Updater) Status(ctx context.Context) (*UpdateStatus, error)`
- `func (u *Updater) Update(ctx context.Context) (*UpdateStatus, error)`
- The updater accepts only the official repository, known asset names, known platform keys, and the locked checksum file. It uses a temp file, validates executable/version/architecture, atomically replaces the user copy, and retains the prior copy until health verification succeeds.

- [ ] **Step 1: Write failing updater tests**

  Cover latest-version comparison, unknown repository, unknown asset, checksum mismatch, truncated download, canceled download, invalid executable, atomic replacement failure, rollback to previous executable, update mutex, and status source reporting.

- [ ] **Step 2: Run focused tests and verify failure**

  Run: `go test ./internal/ytdlp -run 'Test(Update|Status|Rollback)' -count=1`

  Expected: FAIL because the updater does not exist.

- [ ] **Step 3: Implement release metadata and checksum validation**

  Use the official `yt-dlp/yt-dlp` stable release endpoint and exact platform asset mapping. Do not use `yt-dlp -U` because the app must control staging, validation, and rollback. Keep the SHA-256 versus signature-verification limitation explicit in errors/docs.

- [ ] **Step 4: Implement user-data atomic replacement**

  Never overwrite bundle resources. On Windows, account for a running child process before replacement; on macOS, keep updates outside the signed app bundle. Set executable permissions on Unix and re-run the version check after replacement.

- [ ] **Step 5: Run focused tests, race tests, and full Go tests**

  Run: `go test ./internal/ytdlp -count=1`

  Run: `go test -race ./internal/ytdlp -count=1`

  Run: `go test ./... -count=1`

  Expected: PASS; an update failure leaves the old binary and status usable.

- [ ] **Step 6: Commit only Task 7 files**

### Task 8: Wails API, lifecycle wiring, and browser-session authorization

**Files:**
- Modify: `app.go`
- Modify: `app_test.go`
- Modify: `internal/media/model.go`
- Modify: `frontend/wailsjs/go/main/App.d.ts` only through generation, never by hand
- Modify: `frontend/wailsjs/go/main/App.js` only through generation, never by hand
- Modify: `frontend/wailsjs/go/models.ts` only through generation, never by hand

**Interfaces:**
- Add `type YTDLPStatus struct { Available bool; CurrentVersion string; LatestVersion string; Source string; CanUpdate bool }` with no filesystem path.
- Add `func (a *App) GetYTDLPStatus() (*YTDLPStatus, error)`.
- Add `func (a *App) UpdateYTDLP() (*YTDLPStatus, error)` with a mutex and no interruption of active tasks.
- Add `type AnalyzeWithBrowserSessionRequest struct { URL string; Browser string; Profile string }`.
- Add `func (a *App) AnalyzeWithBrowserSession(req AnalyzeWithBrowserSessionRequest) (*AnalyzeResponse, error)`; validate browser/profile before starting asynchronous work.
- Existing `Analyze`, `CancelAnalysis`, download methods, and event names remain compatible.

- [ ] **Step 1: Write failing app API tests**

  Cover status when bundled yt-dlp is missing, update mutex, update failure preserving old version, browser allowlist rejection, default Analyze not passing browser context, authorized Analyze storing only browser/profile, cancellation, and event payloads with no URL/cookie/header leakage.

- [ ] **Step 2: Run focused app tests and verify failure**

  Run: `go test . -run 'Test(YTDLP|AnalyzeWithBrowser|App)' -count=1`

  Expected: FAIL because the Wails methods and wiring do not exist.

- [ ] **Step 3: Wire services in `NewApp`, startup, and shutdown**

  Resolve bundled/user yt-dlp without PATH fallback, verify the selected version at startup, inject the client into the analyzer and manager resolver, and close active proxy/process resources on shutdown.

- [ ] **Step 4: Add the Wails methods and structured error classification**

  Map internal error codes to user-readable messages without returning raw yt-dlp stderr or sensitive URLs. Keep browser-session authorization explicit and per analysis.

- [ ] **Step 5: Regenerate Wails bindings and run API tests**

  Run the repository's Wails binding generation command for the installed CLI, then `go test . -count=1` and `go test ./... -count=1`. Do not manually edit generated files.

- [ ] **Step 6: Commit only Task 8 files**

### Task 9: Vue analysis authorization flow and yt-dlp update control

**Files:**
- Modify: `frontend/src/shared/wails.js`
- Modify: `frontend/src/features/analyze/AnalyzePanel.vue`
- Modify: `frontend/src/App.vue`
- Create: `frontend/src/features/settings/YTDLPStatus.vue`
- Create: `frontend/src/features/settings/status.js`
- Test: `frontend/src/features/settings/status.test.js`
- Modify: `frontend/src/style.css`
- Modify: `frontend/package.json`

**Interfaces:**
- `wails.js` exports `getYTDLPStatus()`, `updateYTDLP()`, and `analyzeWithBrowserSession(url, browser, profile)`.
- Vue state distinguishes idle, checking, updating, ready, update-failed, auth-required, canceled, empty, and analysis-failed.
- The browser selector uses the same backend allowlist; it never accepts a filesystem path or raw Cookie value.

- [ ] **Step 1: Write failing frontend state-helper tests**

  Add pure state helpers in `status.js` and test update button disabled during update, status/source rendering, update error retaining old status, auth prompt requiring an explicit click, canceling auth without starting analysis, and default analysis never invoking the browser-session method.

- [ ] **Step 2: Run the focused frontend checks and verify failure**

  Add the `test:unit` script as `node --test src/features/settings/status.test.js`, then run `npm run test:unit`.

  Expected: FAIL because the state helpers are not implemented.

- [ ] **Step 3: Add Wails wrappers and the smallest settings/status UI**

  Add a visible “检查并更新 yt-dlp” button, current version/source text, progress/disabled state, and concise privacy copy for browser-session authorization. Keep layout compatible with the existing narrow-window styles.

- [ ] **Step 4: Add analysis auth-required handling**

  Show the browser-session consent text only after the backend returns `ytdlp.auth_required`; browser/profile selection is explicit, and canceling the prompt leaves the analysis in a recoverable state.

- [ ] **Step 5: Regenerate bindings if Go contracts changed and run the frontend build**

  Run: `npm run test:unit`

  Run: `npm run build`

  Expected: production build exits 0 with no generated-binding edits made by hand.

- [ ] **Step 6: Commit only Task 9 files**

### Task 10: Product docs, resource packaging, license notices, and release verification

**Files:**
- Modify: `docs/PRODUCT_SPEC.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/DEVELOPMENT_PLAN.md`
- Modify: `README.md`
- Modify: `Makefile`
- Modify: `scripts/copy_ffmpeg_to_bundle.sh` only if shared resource packaging needs a safe extension
- Create: `scripts/copy_yt_dlp_to_bundle.sh`
- Create: `scripts/test_yt_dlp_manifest.sh`
- Modify: `wails.json` only if the final resource layout requires it
- Create or update: `docs/yt-dlp/UPDATE_AND_LICENSE.md`

**Interfaces:**
- Documentation must state support boundaries, browser-session opt-in behavior, no-Python requirement, stable update channel, checksum/signature limitation, and yt-dlp license/source obligations.
- Build scripts must select the current platform asset and never use a system `yt-dlp` from PATH.

- [ ] **Step 1: Write failing manifest/package verification checks**

  Cover missing platform asset, wrong architecture/filename, missing license notice, bundle path mismatch, and accidental PATH fallback.

- [ ] **Step 2: Run the checks and verify the expected red state**

  Run: `bash scripts/test_yt_dlp_manifest.sh`

  Expected: FAIL until the final resource layout and release documents are wired.

- [ ] **Step 3: Update product, architecture, development, README, and license documentation**

  Add the new API/permission boundaries and mark only completed yt-dlp tasks as complete. Do not rewrite the existing repair-backlog status or claim Windows/Linux/signing validation from this macOS workspace.

- [ ] **Step 4: Add bundle copying and manifest verification**

  Copy only the selected platform yt-dlp binary and required notices into the application resource layout. Verify the copied executable without invoking it from PATH.

- [ ] **Step 5: Run final verification available on this host**

  Run: `gofmt -w` on changed Go files, `go test ./... -count=1`, `go test -race ./internal/... -count=1`, `npm run build`, `make yt-dlp-manifest-test`, and the host-platform bundle smoke check.

  Expected: all host-available checks pass. Windows/Linux builds, formal macOS signing/notarization, and real authorized-site downloads remain explicitly listed as not run unless the required environment and samples exist.

- [ ] **Step 6: Commit only Task 10 files and perform a final diff audit**

  Confirm no downloaded media, secrets, generated binaries, user paths, or unrelated pre-existing changes were staged.

## TRAE Task Prompt Template

Use this prompt once per task, replacing only the task number and title:

```text
请按仓库 AGENTS.md、docs/TRAE_GUIDE.md、docs/superpowers/specs/2026-10-10-yt-dlp-integration-design.md 和 docs/superpowers/plans/2026-10-10-yt-dlp-integration.md 工作。

本次只执行“Task N：<任务标题>”。不要实现后续任务，不要改变已确认的权限、安全边界、MVP 范围或现有 FFmpeg 行为。

开始修改前，请先报告：
1. 当前代码状态与本任务差距；
2. 本任务允许修改/新增的文件及职责；
3. 本任务对应的失败测试、验收命令和预期结果；
4. 与现有未提交改动、接口或平台约束的冲突。

严格遵循 TDD：先写一个能证明需求缺失的失败测试，运行确认失败，再写最小实现，最后运行本任务测试和必要的全量测试。不要手工编辑 frontend/wailsjs 生成文件；Go 导出 API 变化后使用当前 Wails CLI 重新生成。不要使用 shell 拼接 URL、Cookie、格式 ID 或命令参数。完成后报告实际文件差异、实际运行的命令、原始失败原因（若有）和未运行检查。
```

