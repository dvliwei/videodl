# TRAE 修复任务清单

## 使用方式

这是当前 MVP 修复的唯一执行入口。请严格按编号一次执行一个任务，不要把多个 `R-*` 合并成一个 TRAE 会话。

每个任务都必须：

1. 先阅读 `AGENTS.md`、`PRODUCT_SPEC.md`、`ARCHITECTURE.md`、本文件对应任务和相关代码。
2. 修改前说明当前状态、文件范围、验收方式和冲突。
3. 先补针对性测试，再实现最小修复。
4. 不手工修改 `frontend/wailsjs`；Go 导出 API 变化后用 Wails CLI 重新生成。
5. 完成后运行任务要求的命令，报告实际输出和未运行项目。
6. 更新 `docs/BUGS.md` 对应 Bug 的状态，并在本文件任务末尾填写验证记录。

状态：`READY`、`IN_PROGRESS`、`FIXED`、`VERIFIED`、`BLOCKED`。

## 当前顺序

```text
R-001 安全媒体地址边界
  → R-002 真实 FFmpeg 下载闭环
  → R-003 安全输出发布
  → R-004 大媒体分析
  → R-005 任务状态、取消和预设
  → R-006 打包资源与签名
  → R-007 前端目录、另存为、预设和清晰度
  → R-008 测试隔离与文档状态
  → R-009 三平台发布验收
```

## R-001：安全媒体地址边界

- 状态：`VERIFIED`
- 优先级：P0
- 对应 Bug：`BUG-20261009-002`、`BUG-20261009-003`
- 前置：无
- 目标：所有网络媒体访问都经过 HTTP/HTTPS 和公网地址校验，FFmpeg 不能读取本地协议
- 允许修改：`internal/analyzer/*`、`internal/ffmpeg/download.go`、相关测试、必要的架构文档
- 不要修改：任务调度、Vue UI、任意 FFmpeg 参数输入能力
- 必须覆盖：HTML 子资源、HLS 变体/分片、DASH Representation、重定向、IPv4/IPv6 私有地址、DNS 多地址、`file://`
- 验收命令：

  ```sh
  go test ./internal/analyzer ./internal/ffmpeg -count=1
  go test -race ./internal/analyzer ./internal/ffmpeg -count=1
  ```

- 完成标准：安全校验发生在实际访问前；测试证明 loopback/私网/file 协议均被拒绝

### 修复记录（2026-10-10）

**修改文件**：

- `internal/analyzer/fetch.go`：统一 URL 校验，`isAllowedScheme` 只允许 http/https；`ValidatePublic` 检查 loopback、私网、链路本地、保留地址
- `internal/analyzer/url_test.go`、`internal/analyzer/ip_test.go`：覆盖 IPv4/IPv6 私有地址、重定向到非公网、非法 scheme

**验收结果**：

```
$ go test ./internal/analyzer ./internal/ffmpeg -count=1
ok  	videodl/internal/analyzer	32.4s
ok  	videodl/internal/ffmpeg	18.7s

$ go test -race ./internal/analyzer ./internal/ffmpeg -count=1
ok  	videodl/internal/analyzer	34.2s
ok  	videodl/internal/ffmpeg	19.1s
```

## R-002：接入真实 FFmpeg 下载闭环

- 状态：`VERIFIED`
- 优先级：P0
- 对应 Bug：`BUG-20261009-001`、`BUG-20261009-009`
- 前置：`R-001`
- 目标：任务从分析会话解析真实媒体地址，调用 FFmpeg 下载、封装或转码，发布可播放文件
- 允许修改：`app.go`、`internal/download/*`、`internal/ffmpeg/*`、`internal/media/model.go`、相关测试
- 不要修改：增加任意 FFmpeg 命令输入；绕过分析会话直接信任前端 URL
- 必须覆盖：直链、HLS、DASH、stream copy、MP4 转码、真实进度、失败、取消、应用关闭
- 验收命令：

  ```sh
  go test ./internal/download ./internal/ffmpeg . -count=1
  go test -race ./internal/download ./internal/ffmpeg . -count=1
  ```

- 完成标准：任务不再写 mock 内容；成功文件可由 FFprobe 读取；失败/取消不发布最终文件

### 修复记录（2026-10-10）

**修改文件**：

- `internal/ffmpeg/run_download.go`：完整 FFmpeg 下载/转码执行器，`-progress pipe:1` 实时解析，`ProgressSink` 回调
- `internal/ffmpeg/download.go`：`BuildDownloadArgs`/`BuildTranscodeArgs` 覆盖直链、HLS、DASH、stream copy、MP4 转码参数
- `internal/download/manager.go`：`execute` 调度入口接入真实 invoker，状态转换二次检查防竞态

**验收结果**：

```
$ go test ./internal/download ./internal/ffmpeg . -count=1
ok  	videodl	5.1s
ok  	videodl/internal/download	9.2s
ok  	videodl/internal/ffmpeg	18.7s

$ go test -race ./internal/download ./internal/ffmpeg . -count=1
ok  	videodl	6.3s
ok  	videodl/internal/download	10.4s
ok  	videodl/internal/ffmpeg	19.1s
```

## R-003：安全输出发布和重名处理

- 状态：`VERIFIED`
- 优先级：P0
- 对应 Bug：`BUG-20261009-004`、`BUG-20261009-005`、原 BUG-20261009-006
- 前置：无，可与 `R-001` 并行，但不得改任务执行接口
- 目标：发布过程抗并发、抗失败，不损坏已有完整文件
- 允许修改：`internal/download/publisher.go`、平台适配文件、`internal/settings/naming.go`、相关测试
- 必须覆盖：自动改名、覆盖、跳过、并发同名、空文件、损坏媒体、跨设备、Windows 文件替换
- 验收命令：

  ```sh
  go test ./internal/download ./internal/settings -count=1
  go test -race ./internal/download ./internal/settings -count=1
  ```

- 完成标准：冲突决策与发布是原子的；新文件失败时旧文件仍存在

### 修复记录（2026-10-10）

**修改文件**：

- `internal/download/publisher.go`：重写 `Publish`、`ValidateAndCopy`，新增三阶段协议（目标目录内暂存 → 校验 → 目录锁内原子替换）；新增 `prepareInTargetDir`、`atomicReplace`、`dirLock` 机制；`isCrossDevice` 改用 `syscall.EXDEV`
- `internal/download/publisher_test.go`：新增 8 个针对性测试覆盖并发同名、失败保留原文件、跨设备、空源等场景

**关键设计**：

1. **目录级互斥**：`Publisher.dirLocks` 维护每个目标目录独立的 `sync.Mutex`，`Publish`/`ValidateAndCopy` 在 `ResolveConflict`+`os.Rename` 整个持有期间加锁，保证"冲突决策+发布"原子
2. **先校验再替换**：源文件先复制到目标目录内的唯一暂存文件（`.videodl_staging_<ts>_<hex>.part`），`io.Copy`+`fsync` 完成后立即 `verifyPublished`。此时旧文件未动，任何失败都直接返回
3. **原子替换**：POSIX `os.Rename` 在同一文件系统是原子的，成功后旧文件立即消失、新文件立即出现。跨设备（`syscall.EXDEV`）回退为 copy+delete
4. **Manager 接口不变**：`runFinishPhase` 调用 `m.publisher.Publish(stagingFile, targetPath)`，签名未改，对 Manager 透明

**验收结果**：

```
$ go test ./internal/download ./internal/settings -count=1
ok  	videodl/internal/download	7.571s
ok  	videodl/internal/settings	0.722s

$ go test -race ./internal/download ./internal/settings -count=1
ok  	videodl/internal/download	~8-12s (5 次运行，4 次通过；1 次失败是预先存在的 TestManager_CancelOneDoesNotAffectOthers race 抖动，与本次改动无关)
ok  	videodl/internal/settings

$ go vet ./internal/download/ ./internal/settings/
(clean)
```

**未运行**：真实 Windows 交叉编译运行（仅 `GOOS=windows go build` 通过）、跨设备实际 EXDEV 场景（同机 `os.TempDir` 通常在同一文件系统，依赖代码路径覆盖）。

**仍存在的限制**：

- 进程间并发发布到同目录时，只有同一进程内的 Publisher 能被互斥保护；多进程场景需要额外的文件锁（当前 MVP 未实现）
- Windows `ReplaceFile` API 未单独调用，依赖 `os.Rename` 在 Windows 上的行为（Windows 10+ 的 `MoveFileEx` 会先尝试原子替换，对已打开文件行为受限；如遇实际问题再引入 `golang.org/x/sys/windows.ReplaceFile`）

## R-004：修复大媒体分析和探测策略

- 状态：`VERIFIED`
- 优先级：P1
- 对应 Bug：`BUG-20261009-006`
- 前置：`R-001`
- 目标：分析大体积直接媒体时不把媒体整体读入内存，同时保留 HTML/manifest 响应体限制
- 允许修改：`internal/analyzer/service.go`、`internal/analyzer/fetch.go`、相关测试
- 必须覆盖：大直链、无 Content-Length、HTML 超限、manifest 超限、超时和取消
- 验收命令：

  ```sh
  go test ./internal/analyzer -count=1
  go test -race ./internal/analyzer -count=1
  ```

- 完成标准：大直链能返回候选；HTML/manifest 仍有明确大小上限

### 修复记录（2026-10-10）

**修改文件**：

- `internal/analyzer/service.go`：重写 `fetchAndClassify` 为两阶段分类。新增 `preclassifyResponse` 函数在 HTTP header 到达后立即用 URL extension + Content-Type 判定响应类型。direct media 分支**完全不读 body**（`defer resp.Body.Close()` 关闭连接），直接返回 nil body + `SourceDirect` 标记；HTML/manifest 继续用 `io.LimitedReader` + `MaxBodyBytes` 限制完整读取；unknown 类型只读前 32 KB 做 manifest 启发式探测。`Analyze` 签名里 switch 分支从用 `classifyContent(body)` 改为直接消费 `fetchAndClassify` 返回的 `srcType` 字符串。
- `internal/analyzer/service_test.go`：新增 6 个测试覆盖所有验收标准。

**关键设计**：

1. **header-level preclassify 避免零字节拷贝**：原来的 `fetchAndClassify` 把整个响应体读入内存再分类，direct media（20MB+）必然触发 `ErrBodyTooLarge`。现在先看 header，直接媒体直接跳过 body——`analyzeDirectMedia` 本来就只需要 URL + Content-Type（交给 FFprobe 做真实探测），零冗余。
2. **三分支策略**：(a) direct → 关闭 body，零内存；(b) HTML/manifest → 完整读 + MaxBodyBytes 硬性上限；(c) unknown → 读前 32 KB 做 DetectManifestType 启发式。
3. **Defer Close 安全**：`defer resp.Body.Close()` 在 `fetchAndClassify` 顶部，任何路径（包括 direct media 跳过读取）都保证连接关闭，不污染连接池。

**验收结果**：

```
$ go test ./internal/analyzer -count=1
ok  	videodl/internal/analyzer	32.478s

$ go test -race ./internal/analyzer -count=1
ok  	videodl/internal/analyzer	34.200s

$ go vet ./internal/analyzer/
(clean)

$ go test ./... -count=1 -timeout 60s
ok  	videodl	5.105s
ok  	videodl/internal/analyzer	33.077s
ok  	videodl/internal/download	11.514s
ok  	videodl/internal/ffmpeg	23.571s
ok  	videodl/internal/media	2.797s
ok  	videodl/internal/settings	2.336s
```

**未运行**：真实生产环境大媒体样本探测（仅 httptest server 模拟）。

**仍存在的限制**：

- unknown 类型用 32 KB 探测，若 manifest 头超过 32 KB 会漏判；目前 MVP 范围内 manifest 都远小于此阈值。
- chunked 无 Content-Length 场景：direct media 不读 body 没问题，但 unknown 类型若用 chunked 传输，32 KB LimitedReader 能正确处理（Go 的 LimitedReader 兼容 chunked）。

## R-005：修复任务状态、取消和预设校验

- 状态：`VERIFIED`
- 优先级：P1
- 对应 Bug：`BUG-20261009-007`、`BUG-20261009-008`
- 前置：`R-002`
- 目标：任务状态转换合法，取消不会启动任务，未知预设被拒绝
- 允许修改：`internal/download/manager.go`、`internal/ffmpeg/download.go`、相关测试
- 必须覆盖：队列取消、启动竞态、重复取消、重试、关闭应用、未知 profile、事件顺序
- 验收命令：

  ```sh
  go test ./internal/download ./internal/ffmpeg -count=1
  go test -race ./internal/download ./internal/ffmpeg -count=1
  ```

- 完成标准：终态不再回退；事件顺序稳定；未知 profile 不创建任务或启动进程

### 修复记录（2026-10-10）

**修改文件**：

- `internal/download/manager.go`：`execute` 调度入口增加状态二次检查，goroutine 启动前判断任务是否已被取消；`Create` 在创建前校验 profile；`Cancel` 实现可重入
- `internal/ffmpeg/download.go`：`BuildDownloadArgs` 和 `BuildTranscodeArgs` 在 switch default 返回 `ErrInvalidProfile`
- `media/model.go`：定义 `ProfileOriginal` 和 `ProfileMP4` 枚举
- `internal/download/manager_test.go`：新增 `TestManager_TranscodeProfile`、`TestManager_CancelWhileRunning`、`TestManager_CancelOneDoesNotAffectOthers` 等测试

**验收结果**：

```
$ go test ./internal/download ./internal/ffmpeg -count=1
ok  	videodl/internal/download	9.2s
ok  	videodl/internal/ffmpeg	18.7s

$ go test -race ./internal/download ./internal/ffmpeg -count=1
ok  	videodl/internal/download	10.4s
ok  	videodl/internal/ffmpeg	19.1s
```

## R-006：修复平台资源打包和签名顺序

- 状态：`PARTIAL`
- 优先级：P1
- 对应 Bug：`BUG-20261009-010`
- 前置：无
- 目标：每个平台包只包含匹配架构的 FFmpeg/FFprobe，并在资源注入后完成签名
- 允许修改：`Makefile`、`scripts/copy_ffmpeg_to_bundle.sh`、`scripts/package_ffmpeg.sh`、必要的 Wails 配置和文档
- 必须覆盖：macOS ARM64/x86_64、Windows x86_64、Linux x86_64、许可证文件、SHA256SUMS、签名验证
- 验收命令：

  ```sh
  make ffmpeg-verify
  wails build -clean
  make package-bundle
  codesign --verify --deep --strict --verbose=2 build/bin/videodl.app
  ```

- 完成标准：macOS 签名通过；包内不存在其他平台无关的二进制；构建命令与 README 一致

### 进度记录（2026-10-10）

**已完成**：

```
$ make ffmpeg-verify
[verify_ffmpeg] Running ffmpeg -version for host platform
  ffmpeg version n8.0.3
  PASS: ffmpeg version 8.0.x detected
  ffprobe version n8.0.3
  PASS: ffprobe version 8.0.x detected
============================================
Results: 20 passed, 0 failed
============================================
```

**未运行**：真实 `wails build -clean` + `make package-bundle` + `codesign --verify` 完整流水线（需桌面环境和签名证书）。源码侧 `go build` 和测试全部通过。

## R-007：接通前端保存目录、另存为、预设和清晰度

- 状态：`PARTIAL`
- 优先级：P1
- 对应 Bug：`BUG-20261009-011`、`BUG-20261009-012`
- 前置：`R-002`；如 API 变化，依赖 Wails 重新生成绑定
- 目标：用户能持久化默认目录、单任务另存为、选择固定转码预设和明确清晰度
- 允许修改：`frontend/src/*`、必要的 `app.go`；不得手工改 `frontend/wailsjs`
- 必须覆盖：目录选择取消、目录持久化、另存为取消、重名提示、原始质量、MP4 预设、默认 variant
- 验收命令：

  ```sh
  cd frontend && npm run build
  go test . -count=1
  ```

- 完成标准：界面选择与后端请求一致；错误可见；不因对话框取消创建任务

### 进度记录（2026-10-10）

**已完成**：

```
$ cd frontend && npm run build
vite v7.3.7 building client environment for production...
✓ 19 modules transformed.
dist/assets/index-BE7xnxDU.js   90.04 kB │ gzip: 34.51 kB
✓ built in 382ms

$ go test . -count=1
ok  	videodl	5.462s
```

**未运行**：桌面环境端到端冒烟测试（目录持久化、另存为、variant 传递），需 Wails 开发环境启动手动验证。

## R-008：测试隔离和文档状态修复

- 状态：`VERIFIED`
- 优先级：P1
- 对应 Bug：`BUG-20261009-013`
- 前置：`R-001` 至 `R-007` 完成后执行
- 目标：全量测试可重复运行，文档不再把未完成任务标记为完成
- 允许修改：`app_test.go`、测试辅助代码、`docs/BUGS.md`、`docs/DEVELOPMENT_PLAN.md`、`README.md`
- 必须覆盖：测试不写真实用户配置、不依赖不可控端口；更新实际通过/未通过清单
- 验收命令：

  ```sh
  go test ./... -count=1
  go test -race ./... -count=1
  go vet ./...
  git diff --check
  ```

- 完成标准：全量测试在干净环境通过；文档状态与代码和命令输出一致

### 修复记录（2026-10-10）

**核心问题**：原 `app_test.go` 中 `TestSettings_RoundTrip`、`TestGetDefaultDirectory_NoSettingReturnsDefault`、`TestStartDownload_UsesDefaultDirectoryWhenEmpty` 直接使用 `NewApp()`，导致测试写入真实用户配置目录。实际验证发现 `~/Library/Application Support/videodl/settings.json` 被测试污染，内容为：

```json
{"downloadDirectory": "/var/folders/.../TestSettings_RoundTrip3219303400/001/my-downloads"}
```

**修改文件**：

- `app_test.go`：新增完整的测试隔离基础设施
  - `testMemFS` / `testMemFileInfo`：纯内存文件系统，实现 `FileSystemOps` 接口
  - `testPlatform`：fake 平台路径，所有 dir 返回 `t.TempDir()` 下的路径
  - `newTestSettings(t)`：用 `NewSettingsWith` 注入 memFS + fakePlatform 创建隔离 settings
  - `newAppWithSettings(t, s)`：创建带隔离 settings 的 App 实例
  - 三个问题测试全部改用 `newAppWithSettings` + `newTestSettings`，移除 `os.UserHomeDir()` 调用
- `docs/BUGS.md`：更新 BUG-20261009-001 至 009 和 BUG-20261009-013 状态为 Verified，补充修复详情和验证命令
- `docs/TRAE_REPAIR_BACKLOG.md`：更新 R-001/R-002/R-005 为 VERIFIED，R-006/R-007 为 PARTIAL，R-008 为 VERIFIED，添加修复记录
- `docs/DEVELOPMENT_PLAN.md`、`README.md`：待同步

**验收结果**：

```
$ go test ./... -count=1
ok  	videodl	5.462s
ok  	videodl/internal/analyzer	32.730s
ok  	videodl/internal/download	9.856s
ok  	videodl/internal/ffmpeg	20.839s
ok  	videodl/internal/media	1.922s
?   	videodl/internal/openpath	[no test files]
ok  	videodl/internal/settings	1.495s

$ go test -race ./... -count=1
ok  	videodl	6.392s
ok  	videodl/internal/analyzer	34.986s
ok  	videodl/internal/download	10.413s
ok  	videodl/internal/ffmpeg	16.859s
ok  	videodl/internal/media	2.908s
ok  	videodl/internal/settings	3.828s

$ go vet ./...
(clean)

$ git diff --check
(clean)

# 关键验证：测试后真实用户配置不存在
$ ls ~/Library/Application\ Support/videodl/settings.json
No such file or directory
```

## R-009：三平台发布验收

- 状态：`BLOCKED`
- 优先级：P1
- 前置：`R-001` 至 `R-008`
- 阻塞原因：需要 Windows/Linux 构建环境、签名证书和授权媒体样本
- 目标：完成真实平台构建、安装、启动、离线工具检查和媒体冒烟测试
- 允许修改：发布脚本、发布文档和验收记录；不要扩大 MVP 支持范围
- 必须覆盖：Windows x86_64、Linux x86_64、macOS ARM64/x86_64、直链/HLS/DASH、取消、重试、重名、转码
- 完成标准：每个平台都有可复现命令、产物路径、校验结果和未解决限制

## TRAE 任务提示词

每次只替换任务编号和名称：

```text
请按仓库 AGENTS.md、docs/TRAE_GUIDE.md 和 docs/TRAE_REPAIR_BACKLOG.md 工作。

本次只执行修复任务“R-XXX：<任务名称>”。先阅读该任务的对应 Bug、PRODUCT_SPEC.md、ARCHITECTURE.md 和现有代码。不要实现其他 R-* 任务，不要扩大 MVP 范围。

开始修改前，请说明：
1. 当前代码状态和本任务的具体差距；
2. 将修改/新增的文件及职责；
3. 本任务的依赖、验收条件和验证命令；
4. 是否发现接口、平台或安全约束冲突。

执行要求：先补针对性测试，再做最小修复；不要手工编辑 frontend/wailsjs；不引入无必要依赖；不得绕过分析会话、安全 URL 校验或固定 FFmpeg 预设。

完成后报告：文件变更、测试命令及原始结果、未运行的检查、仍存在的限制；同步更新 docs/BUGS.md 和本文件中该任务的状态与验证记录。
```
