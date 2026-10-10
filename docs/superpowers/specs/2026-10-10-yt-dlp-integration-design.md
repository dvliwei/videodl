# yt-dlp 视频网站解析集成设计规格

**状态：已确认**  
**日期：2026-10-10**  
**面向：TRAE 单任务迭代开发**

## 1. 目标与背景

VideoDL 当前主要解析静态 HTML、直链、HLS 和 DASH。对于依赖站点专用接口、签名 URL、播放器元数据或动态页面的站点，现有解析能力不足。本次迭代接入 yt-dlp 的站点解析器，但不把 yt-dlp 作为新的产品边界：仍只支持用户有权访问的公开、非 DRM 资源，并继续由 Go 负责任务生命周期、文件发布和 FFmpeg 下载/封装。

本次交付目标：

1. 在不要求用户安装 Python 的前提下，随应用提供对应平台的 yt-dlp 独立可执行文件。
2. 使用 yt-dlp 的结构化 JSON 输出解析视频标题、清晰度、格式、时长、大小、音视频流和格式选择信息。
3. yt-dlp 解析成功后，下载任务仍进入现有 Go 任务管理器，并由随包 FFmpeg 完成下载、合并和封装。
4. 在设置区域增加“检查并更新 yt-dlp”按钮，默认只使用官方 stable 发布渠道。
5. 默认不读取浏览器 Cookie、账号密码、用户 yt-dlp 配置或第三方插件；只有用户主动授权时才启用浏览器会话能力。
6. 每个 TRAE 任务都可以单独测试、单独验收，不越界实现后续任务。

## 2. 明确不包含

- 不自动登录，不保存或提取用户账号密码。
- 不绕过付费墙、访问控制或 DRM；Cookie 也不能改变这一边界。
- 不默认读取浏览器 Cookie、Keychain、Windows 凭据库或 Linux Keyring。
- 不默认加载 `yt-dlp.conf`、插件目录或任何用户可执行配置。
- 不支持用户输入任意 yt-dlp、FFmpeg 或 shell 参数。
- 不新增播放列表批量下载、字幕下载、音频提取、缩略图下载或直播录制功能。
- 不允许 yt-dlp 绕过现有公网地址校验；yt-dlp 的所有网络请求必须经过应用控制的受限代理。

## 3. 方案选择

### 3.1 运行方式

采用独立 yt-dlp 可执行文件，不嵌入 Python。应用资源中提供构建时锁定的平台二进制，运行时优先使用应用数据目录中经校验的用户更新版本，找不到或校验失败时回退到随包版本。

官方发布文件存在许可证差异：仓库源码为 Unlicense，但官方说明 PyInstaller 打包的独立可执行文件包含 GPLv3+ 组件；发布包必须随附 yt-dlp 版本、来源、哈希和适用许可证/源码获取说明，不能只标记为 Unlicense。

### 3.2 解析与下载分工

解析阶段：

```text
用户 URL
  → Go URL/公网地址预检
  → 受限本地 HTTP/HTTPS 代理
  → yt-dlp --dump-single-json --no-playlist --ignore-config
  → Go 映射为 AnalysisResult / MediaCandidate / MediaVariant
```

下载阶段：

```text
用户选择候选和格式
  → Go 任务管理器创建任务
  → yt-dlp 使用同一 URL、格式 ID 和授权上下文重新解析短期媒体 URL
  → Go 将一个或多个媒体 URL、受控请求头传给 FFmpeg
  → 临时文件校验
  → 原子发布最终文件
```

不得把 yt-dlp 输出的原始 URL交给前端；前端只持有分析 ID、候选 ID 和变体 ID。短期签名 URL 只保存在后端分析会话或当前任务内，并按现有日志脱敏规则处理。

### 3.3 解析优先级

对 HTTP/HTTPS URL 先执行现有 URL 安全校验。之后优先尝试 yt-dlp：

- yt-dlp 成功并返回可下载格式：返回 yt-dlp 候选。
- yt-dlp 明确报告没有匹配的 extractor、无可下载格式或不支持：回退现有 HTML/HLS/DASH 分析器。
- 网络安全校验、超时、进程启动或安全代理错误：不静默吞掉，按结构化错误返回或回退并保留警告，具体分类由实现任务固定。

回退不表示 yt-dlp 可以访问更宽的网络范围；两条分析路径都必须使用同一公网地址安全策略。

## 4. 权限与隐私策略

### 4.1 默认模式

每次 yt-dlp 调用都显式传入或等价实现以下约束：

- `--ignore-config`
- `--no-cookies-from-browser`
- 通过 `YTDLP_NO_PLUGINS=1` 禁用插件目录加载
- 固定受控缓存目录，或默认关闭持久化缓存
- 不允许 `--exec`、`--proxy` 外部覆盖、任意 postprocessor 和任意 extractor 参数

默认模式只需要应用已有的网络访问能力，以及写入应用临时目录和应用数据目录的能力。

### 4.2 浏览器会话授权

当 yt-dlp 返回“需要登录/浏览器会话”类结果时，前端显示明确提示，由用户主动点击授权后选择浏览器和可选 profile。授权范围为当前分析/下载尝试，不持久化 Cookie 路径和 Cookie 内容。

允许的浏览器值由后端白名单限制为 yt-dlp 当前支持的浏览器：Brave、Chrome、Chromium、Edge、Firefox、Opera、Safari、Vivaldi、Whale。Chromium 在 Linux 上可能需要读取 basictext、GNOME Keyring、KWallet 等解密来源；失败时显示可操作错误，不要求管理员权限，不尝试绕过系统凭据保护。

浏览器授权必须满足：

- 前端显示“将使用本机浏览器会话访问目标站点”的隐私提示。
- Go 只向 yt-dlp 子进程传递选中的浏览器参数，不把 Cookie 读入前端或应用日志。
- 任务结束、取消或失败后清理应用生成的临时授权材料。
- 仍拒绝 DRM、付费墙、访问控制绕过和站点明确禁止的场景。

首个实现迭代不支持账号密码、二次验证码、`.netrc` 和自动登录。若未来新增，必须单独设计凭据存储、清理和错误提示，不能顺手加入本次任务。

### 4.3 用户配置和插件

默认忽略所有用户配置。未来若用户主动选择“导入 yt-dlp 配置”，只能读取并转换安全白名单字段，例如代理地址、User-Agent 和有限请求头；不得原样传给 yt-dlp。尤其禁止导入 `--exec`、任意输出模板、任意外部 downloader、任意 postprocessor 参数和插件路径。

第三方 yt-dlp 插件本质上是可执行代码，本次不加载、不更新、不从网络安装。

## 5. 网络安全边界

yt-dlp 自己发起的请求不能直接绕过现有 `internal/analyzer` 安全客户端。因此新增 `internal/netguard` 受限代理，至少满足：

- 仅监听本机随机端口，生命周期绑定一次解析/下载尝试。
- 仅允许 HTTP、HTTPS 和 yt-dlp/FFmpeg 任务明确需要的 CONNECT 流量。
- 在建立上游连接前，对主机名解析到的每个地址拒绝 loopback、私有、链路本地、组播、保留和其他非公网地址。
- 对重定向和 CONNECT 目标重复校验；不能只校验初始 URL。
- 不记录完整 Cookie、Authorization、签名 query 或响应体。
- 连接、响应体、重定向和总耗时受现有分析/任务上限约束。

代理不能成为用户自定义代理入口。若未来支持用户代理，应作为单独的、明确的设置能力并限制协议和日志脱敏。

## 6. 领域接口设计

以下接口是计划接口，实际实现前需以现有代码为准；任何接口变化必须同步更新本规格和相关文档。

### 6.1 `internal/ytdlp`

建议拆分为职责单一的文件：

- `binary.go`：平台路径、随包版本、用户更新版本解析与可执行校验。
- `process.go`：参数数组启动、context 取消、stdout/stderr 上限、退出码和敏感输出脱敏。
- `extract.go`：调用 `--dump-single-json` 并解码有限 JSON 结构。
- `formats.go`：yt-dlp format/playlist 信息到 `media.MediaCandidate` 的映射。
- `update.go`：stable 发布查询、资产选择、SHA-256 校验、临时文件下载、原子替换和失败回滚。

业务包不得导入 Wails runtime。

### 6.2 `internal/media` 变化

在不暴露真实媒体地址的前提下增加：

- 候选来源标识，例如 `Extractor` 或等价字段，用于区分 `native` 与 `yt-dlp`。
- 变体内部格式选择信息，例如 yt-dlp `format_id` 或后端内部 selector；前端只看到不透明的 `VariantID`。
- 多输入媒体源所需的后端结构；请求头只存在后端，不进入 JSON/Wails 返回值。
- yt-dlp 解析和更新的稳定错误码，例如 extractor 不支持、需要浏览器会话、二进制缺失、版本更新失败。

不要把完整 yt-dlp JSON 作为 Wails API 返回值，也不要把 query 中可能带签名的 URL 序列化给前端。

### 6.3 Wails API

建议新增以下小接口：

- `GetYTDLPStatus() (*YTDLPStatus, error)`：返回是否可用、当前版本、来源（随包/用户更新）、是否可更新；不返回路径。
- `UpdateYTDLP() (*YTDLPStatus, error)`：仅更新官方 stable；单次执行互斥，更新失败保留旧版本。
- `Analyze(req AnalyzeRequest)`：内部使用 yt-dlp 优先策略；若需要授权，返回结构化 `auth_required`，不弹系统权限窗口。
- `AnalyzeWithBrowserSession(req AnalyzeWithBrowserSessionRequest)`：仅接受后端白名单的 browser/profile 参数，且由用户明确触发；不接受 Cookie 内容或任意文件路径。

更新按钮必须在更新过程中禁用，显示当前版本、目标版本（若已知）、阶段和可读错误。更新不应中断已有下载任务；如果当前版本正在执行解析/下载，更新排队或返回“有任务使用中”，具体策略由计划任务固定并测试。

## 7. 更新机制

### 7.1 版本和文件布局

随包资源建议：

```text
build/resources/yt-dlp/
  manifest.yaml
  licenses/
  tools/windows-x64/yt-dlp.exe
  tools/linux-x64/yt-dlp_linux
  tools/darwin-universal/yt-dlp_macos
```

用户更新版本放在现有 settings 应用数据目录下的专用子目录，不覆盖签名应用包内资源。解析顺序为“已验证用户版本 → 随包版本”。

### 7.2 更新步骤

1. 请求官方 `yt-dlp/yt-dlp` stable release 元数据。
2. 按 `GOOS/GOARCH` 精确选择资产，拒绝未知名称和非官方仓库。
3. 下载到应用临时目录，不直接覆盖当前可执行文件。
4. 校验官方 SHA-256 清单；校验失败删除临时文件并保留旧版本。
5. 校验版本可执行并解析 `--version`，确认平台架构匹配。
6. 在同一文件系统内原子替换用户版本，并保留上一版本直到新版本健康检查通过。
7. 更新 Wails 状态；不得在日志或错误文案中暴露完整下载 URL 的敏感参数。

SHA-256 可防止下载损坏和传输篡改，但不等同于签名验证。若实现阶段无法使用官方签名验证库，文档必须明确这一限制；不得宣称完成了 GPG 身份认证。后续可增加官方 `SHA2-256SUMS.sig` 验证作为独立任务。

## 8. 错误和用户提示

后端错误码必须稳定，前端不解析 yt-dlp 普通日志。至少区分：

- `ytdlp.unavailable`：没有可执行文件或平台不支持。
- `ytdlp.version_invalid`：版本无法读取或架构不匹配。
- `ytdlp.extractor_unsupported`：yt-dlp 没有可用 extractor，允许回退现有分析器。
- `ytdlp.auth_required`：需要用户主动授权浏览器会话。
- `ytdlp.no_formats`：识别到页面但没有可下载格式。
- `ytdlp.network_blocked`：公网地址安全校验拒绝。
- `ytdlp.timeout`：解析或更新超时。
- `ytdlp.update_failed`：更新失败，旧版本仍保留。
- `ytdlp.process_exit`：子进程非零退出；UI 显示简短可读原因，诊断日志保留脱敏信息。

DRM、付费保护、登录限制和站点不支持应显示明确的支持边界，不提示用户关闭证书校验、导入未知插件或绕过限制。

## 9. 测试与验收

### Go 单测/集成测试

- JSON 解码：单视频、多个 format、视频+音频分离、无大小、无时长、畸形 JSON、playlist 被拒绝。
- 参数安全：固定参数包含 `--ignore-config`；用户 URL、format ID、browser/profile 不可注入额外参数。
- 进程生命周期：启动失败、非零退出、stdout 超限、超时、context 取消、Windows 子进程清理。
- 解析映射：标题、清晰度、格式、时长、大小、音视频标识和不透明 ID 稳定。
- 代理安全：loopback、私网、链路本地、IPv6 保留地址、重定向和 CONNECT 目标均被拒绝。
- 浏览器授权：默认不传 Cookie；仅白名单 browser/profile 可传；授权结束后临时材料清理。
- 更新：未知资产、SHA-256 不匹配、版本无效、替换失败、取消、旧版本回滚和重复点击互斥。
- 现有下载闭环：yt-dlp 格式重新解析后，多输入 FFmpeg 参数使用数组传递，成功后原子发布，取消/失败不发布最终文件。

### 前端构建和交互验收

- 分析页能区分“无结果”“不支持”“需要浏览器授权”“失败”。
- 授权前显示访问范围和隐私提示；取消授权不启动任务。
- 设置页显示 yt-dlp 当前版本和来源；更新中按钮禁用；失败时旧版本仍可用。
- 未知进度不显示错误的 100%；长错误文案不破坏窄窗口布局。

### 平台验收

- Windows x86_64、macOS x86_64/ARM64、Linux x86_64 均使用正确资产。
- 未安装 Python 时仍能完成版本检测和解析进程启动。
- macOS 应用包中的只读/签名资源不被运行时更新覆盖。
- 发布材料包含 yt-dlp 来源、版本、哈希、许可证和源码获取说明。

## 10. TRAE 迭代规则

每次只执行一个任务，任务开始前必须报告“当前差距、修改文件、验收条件、潜在冲突”。不得实现后续任务，不得手工修改 `frontend/wailsjs` 生成文件。

建议迭代顺序：

1. 资源清单、许可证材料和平台二进制定位。
2. yt-dlp 进程包装与结构化 JSON 解析。
3. yt-dlp format 到现有媒体模型的映射。
4. 受限代理与 yt-dlp 网络安全接入。
5. 分析服务的 yt-dlp 优先与现有解析器回退。
6. 多输入媒体源到 FFmpeg 下载参数的闭环。
7. 用户更新版本、哈希校验、原子替换和回滚。
8. Wails 状态/更新 API 与绑定生成。
9. 前端分析提示、浏览器会话授权和设置页更新按钮。
10. 文档、许可证材料、平台构建和验收收尾。

每个任务都必须先写失败测试，再写最小实现；任务完成时运行与变更相称的 Go 测试、前端构建或平台检查，并在 TRAE 交付说明中列出未运行项。

## 11. 待最终计划固定的决策

以下内容在实施计划中必须给出唯一答案，不能留给 TRAE 自行猜测：

- yt-dlp 具体锁定版本、四个平台资产名称和 SHA-256。
- GPLv3+ 独立可执行文件的许可证材料路径和发布说明。
- 受限代理是否复用现有安全客户端的 DNS 校验代码，以及代理的 CONNECT 实现边界。
- `MediaSource` 多输入字段、请求头字段和 FFmpeg 参数构造函数的准确签名。
- 浏览器授权请求的 Wails 请求结构、browser/profile 枚举和临时授权生命周期。
- 更新接口的 GitHub API/Release 资产获取方式、超时和回滚文件名。
- 当前已有未提交改动的保留策略：实现者不得重置、覆盖或清理这些改动。

