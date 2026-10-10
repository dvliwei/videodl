# VideoDL

跨平台桌面视频下载器，使用 Go、Wails v2、Vue 3 和 Vite 构建。用户粘贴公开网页 URL，应用自动识别页面中的视频资源（支持直链、HLS、DASH），选择清晰度后即可下载、合并或转码。

> 当前状态：MVP 修复进行中，R-001 至 R-005 和 R-008 已完成并验证；R-006（签名顺序）、R-007（前端端到端）和 R-009（三平台发布）待外部条件。真实 FFmpeg 下载闭环已接入，安全边界已加固，测试隔离已修复。请按 [`docs/TRAE_REPAIR_BACKLOG.md`](docs/TRAE_REPAIR_BACKLOG.md) 查询最新修复状态。

## 功能特性

- **多平台支持**：Windows x86_64、macOS x86_64/ARM64、Linux x86_64
- **多种资源识别**：HTML `<video>`/`<source>` 直链、HLS (`.m3u8`) 清单、DASH (`.mpd`) 清单
- **视频网站解析**：内置官方 yt-dlp standalone，不要求用户安装 Python；解析失败时保留现有原生分析回退
- **安全抓取**：自动拒绝 loopback、私有网段、链路本地和重定向到非公网地址
- **任务管理**：并发限制、取消、重试、重名处理（覆盖/自动改名/取消）
- **两种输出模式**：原质量无损封装（stream copy）和兼容 MP4（H.264/AAC）转码预设
- **进度与状态**：每个任务显示准备、下载、合并/封装、转码、完成/取消/失败阶段
- **离线运行**：FFmpeg 8.0.3 和 FFprobe 随应用分发，运行时不依赖系统 PATH 或网络下载
- **完成后操作**：一键打开文件或所在目录
- **解析器维护**：界面提供“检查并更新 yt-dlp”按钮，默认只更新官方 stable 版本

## 安装

### macOS (x86_64 / ARM64)

1. 下载 `.dmg` 安装包，打开后将 VideoDL 拖入 Applications 文件夹
2. 首次运行时，macOS 可能提示安全警告：系统设置 → 隐私与安全性 → 允许打开
3. 应用已使用 ad-hoc 签名；正式发布将通过 Apple Developer 证书签名并经 Gatekeeper 公证

### Windows (x86_64)

1. 下载 `.exe` 安装器（基于 NSIS）
2. 运行安装器，选择安装路径后完成安装
3. 开始菜单和桌面快捷方式可直接启动

### Linux (x86_64)

1. 下载 `.AppImage` 或 `.deb`/`.rpm` 包
2. AppImage：赋予可执行权限后直接运行
   ```sh
   chmod +x VideoDL-*.AppImage
   ./VideoDL-*.AppImage
   ```
3. deb/rpm：使用包管理器安装
   ```sh
   sudo dpkg -i videodl_*.deb   # Debian/Ubuntu
   sudo rpm -i videodl-*.rpm   # Fedora/RHEL
   ```

**Linux 运行依赖**：需具备 GTK 3 和 WebKitGTK 库（Wails v2 运行时需求），多数主流发行版默认已安装。若启动时提示缺少 `libwebkit2gtk-4.1` 或类似库，请通过包管理器安装。

## 支持范围

### 支持的网页和资源

- HTTP/HTTPS 协议的公开、无需登录的网页
- HTML 页面中的 `<video>`、`<source>` 标签引用的直链媒体（MP4、WebM、MKV 等常见容器）
- HLS (`.m3u8`) 清单及其多清晰度变体（EXT-X-STREAM-INF）
- DASH (`.mpd`) 清单及多 Representation 音视频轨道
- 网页 Open Graph / Twitter Card 等公开媒体元数据

### 支持的功能操作

- 单个或多个资源分析（每个候选独立探测，部分失败不影响全部结果）
- 原质量 stream copy 封装（默认，无质量损失）
- 兼容 MP4 (H.264/AAC) 转码预设（固定参数集，不可自定义）
- 取消正在进行的分析或下载任务
- 下载失败后的重试（新建独立临时文件，不覆盖已有完整文件）
- 重名文件处理：覆盖 / 自动追加编号 / 跳过

### 不支持的场景

- **DRM / 付费保护**：FairPlay、Widevine、PlayReady 等加密资源不识别、不绕过
- **需要登录的页面**：默认不读取 Cookie；若用户明确授权，可按支持的浏览器和 profile 进行单次会话分析，但不绕过访问控制
- **复杂反爬站点**：完全依赖 JavaScript 渲染、需要浏览器执行环境的页面
- **任意 FFmpeg 参数**：前端无法传递自定义命令行，仅支持后端预设映射的有限转码方案
- **暂停/续传**：首版不保证中断后从断点继续
- **直播流录制**：不针对实时流做时长限制外的特殊处理
- **字幕提取**：不识别或下载字幕轨

### 已知限制

- HLS 需为公开可访问的 `.m3u8`，不支持嵌套的私有密钥清单
- DASH 需使用标准 MPD 结构，不支持自定义 SegmentTimeline 之外的非标准布局
- 网页分析不执行 JavaScript；SPA 框架页面需先有服务端渲染的 HTML 结构
- 部分站点可能通过 Referer 校验拦截请求，VideoDL 会自动传递分析时的页面 URL 作为 Referer

## 开发入口

- [`AGENTS.md`](AGENTS.md)：AI 协作、架构边界和完成标准。
- [`docs/PRODUCT_SPEC.md`](docs/PRODUCT_SPEC.md)：MVP 产品与技术要求。
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)：当前架构审查、模块边界和接口约定。
- [`docs/DEVELOPMENT_PLAN.md`](docs/DEVELOPMENT_PLAN.md)：按依赖顺序排列的实现任务和验收条件。
- [`docs/TRAE_GUIDE.md`](docs/TRAE_GUIDE.md)：使用 TRAE 按任务实现和交付的说明。
- [`docs/BUGS.md`](docs/BUGS.md)：已确认 Bug、优先级、影响和验收要求。
- [`docs/TRAE_REPAIR_BACKLOG.md`](docs/TRAE_REPAIR_BACKLOG.md)：TRAE 单任务修复入口。

## 本地开发

1. 安装 Go 1.25、Node.js 20+、Wails CLI v2.16+ 以及当前操作系统所需的 Wails 构建依赖
2. 拉取 FFmpeg 与 yt-dlp 资源：`make ffmpeg-fetch && make ffmpeg-verify && make yt-dlp-fetch && make yt-dlp-verify`
3. 在项目根目录运行 `wails dev` 启动开发模式（前端热重载 + Go 后端）

### Wails 构建依赖

| 平台  | 构建依赖                                           |
|-------|---------------------------------------------------|
| macOS | Xcode Command Line Tools (`xcode-select --install`) |
| Windows | MSYS2 / MinGW-w64 + NSIS（安装器打包）             |
| Linux | build-essential + libgtk-3-dev + libwebkit2gtk-4.1-dev + libayatana-appindicator3-dev |

## 构建

开发构建：

```sh
# 前端开发服务器 + Go 后端热重载
wails dev
```

正式打包（请在目标操作系统上执行）：

```sh
# macOS（当前架构，生成 build/bin/videodl.app）
make build-darwin

# Windows x86_64（需在 Windows 上构建）
make build-windows

# Linux x86_64（需在 Linux 上构建）
make build-linux
```

以上命令会自动获取并校验 FFmpeg/FFprobe 与 yt-dlp，执行 Wails 生产构建，将对应平台的媒体工具和许可证材料打入应用包；Windows/Linux 产物位于 `build/bin/`。Windows 和 Linux 必须分别在对应系统上打包，当前 Makefile 不提供跨平台交叉打包。

仅需快速构建当前主机版本时可执行：

```sh
make quick-build
```

跨平台构建与 FFmpeg/FFprobe 随包要求见 [`docs/PRODUCT_SPEC.md`](docs/PRODUCT_SPEC.md)；首发支持矩阵按开发计划逐平台验收。

### 构建产物验证

生产构建不会自动完成全部发布验收。构建后必须手动执行 `make ffmpeg-verify`，并按目标平台检查资源、签名、安装器和运行依赖。macOS 资源注入必须在最终签名前完成。

## FFmpeg 发行材料

VideoDL 内嵌 FFmpeg 8.0.3 静态构建，使用独立子进程模式（不链接 FFmpeg 库）。

### 许可证

- **类型**：LGPL-2.1-or-later
- **分发模式**：独立子进程；VideoDL 不静态或动态链接 FFmpeg 库
- **排除组件**：不包含 GPL、nonfree 或 version3-only 组件（x264、x265、FDK-AAC 等）

详细来源、SHA-256、配置审计链接和编解码器集合见：
- [`build/resources/ffmpeg/manifest.yaml`](build/resources/ffmpeg/manifest.yaml) — 完整发行清单
- [`build/resources/ffmpeg/CREDITS.md`](build/resources/ffmpeg/CREDITS.md) — 上游来源与贡献者
- [`build/resources/ffmpeg/licenses/FFmpeg-COPYING.LGPLv2.1`](build/resources/ffmpeg/licenses/FFmpeg-COPYING.LGPLv2.1) — LGPL-2.1 许可证文本
- [`build/resources/ffmpeg/licenses/THIRD-PARTY-NOTICES.md`](build/resources/ffmpeg/licenses/THIRD-PARTY-NOTICES.md) — 第三方组件通知

### 获取和校验构建输入

```sh
# 验证 manifest 完整性
make ffmpeg-manifest-test

# 从固定来源下载四平台 FFmpeg 二进制
make ffmpeg-fetch

# 校验已下载二进制的 SHA-256 和许可证文件
make ffmpeg-verify

# 清理已下载的二进制
make ffmpeg-clean
```

`ffmpeg-fetch` 只在构建时下载固定资产；应用运行时不联网获取 FFmpeg/FFprobe，也不使用系统 `PATH` 回退。

## yt-dlp 发行材料

yt-dlp 版本、平台资产、SHA-256、来源和许可证记录见 [`build/resources/yt-dlp/manifest.yaml`](build/resources/yt-dlp/manifest.yaml)；更新与许可证义务见 [`docs/yt-dlp/UPDATE_AND_LICENSE.md`](docs/yt-dlp/UPDATE_AND_LICENSE.md)。官方 standalone 包含 GPLv3+ 及其他第三方组件，发布时必须保留随包通知文件。完整性校验使用 SHA-256，不代表 GPG/签名身份认证。

```sh
make yt-dlp-manifest-test
make yt-dlp-fetch
make yt-dlp-verify
```

## 发布验收清单

- [x] 所有 Go 单元测试通过 (`go test ./... -count=1` 和 race 测试) — R-008 已修复测试隔离
- [x] Go 内部包测试和 race 测试通过 (`go test ./internal/...`)
- [x] 前端生产构建通过 (`npm run build`)
- [ ] macOS ARM64 最终包签名验证 (`codesign --verify --deep --strict`) — 等待 R-006 资源注入顺序修复
- [ ] macOS x86_64 交叉构建验证（需 CI 或 x86_64 机器）
- [ ] Windows x86_64 构建 + NSIS 安装器验证（需 Windows 机器）
- [ ] Linux x86_64 构建 + 运行时依赖检查（需 Linux 机器）
- [ ] macOS 正式签名与 Gatekeeper 公证（需 Apple Developer 证书）
- [ ] Windows 安装器代码签名（需 Authenticode 证书）
- [x] FFmpeg 四平台资产 SHA-256 校验
- [x] FFmpeg 许可证材料随包分布
- [x] 不支持场景文档随 README 可查
- [ ] 真实直链/HLS/DASH 下载、取消、重试、重名和转码冒烟测试 — 后端闭环已接入，需桌面环境手动验证（R-007）
