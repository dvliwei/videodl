# Bug 记录

## 记录范围

- 发现日期：2026-10-09
- 关联范围：阶段 4：下载任务闭环
- Review 范围：`139f38c..d0f97df`
- 当前状态：以下问题均为 `Open`，尚未修复

## BUG-20261009-001：任务管理器没有执行真实下载

- 优先级：P0
- 状态：Open
- 位置：`internal/download/manager.go:634-690`
- 现象：下载阶段写入 `mock video data`，合并和转码阶段只等待，不调用 `internal/ffmpeg`。
- 复现：创建任意任务并等待完成；任务会生成非媒体文本文件并报告 `completed`。
- 影响：下载、合并、转码闭环实际上没有完成，用户无法得到真实视频。
- 修复方向：为任务管理器注入真实 FFmpeg 执行器和分析会话解析器；使用独立临时输出，接入真实进度回调，并在发布前执行媒体校验。

## BUG-20261009-002：默认输出路径未使用下载目录且可被标题穿透

- 优先级：P0
- 状态：Open
- 位置：`internal/download/manager.go:711-713,898-901`
- 现象：`OutputPath` 为空时直接把网页标题拼接为相对路径，没有调用文件名清理，也没有使用配置的默认下载目录。
- 复现：让标题为 `../../outside` 或包含路径分隔符，并省略 `OutputPath`。
- 影响：文件可能写入当前工作目录之外，甚至覆盖用户不希望被覆盖的路径。
- 修复方向：使用配置的默认下载目录；通过 `settings.SanitizeFileName` 清理标题，拒绝绝对路径和路径分隔符。

## BUG-20261009-003：并发发布存在目标文件冲突竞态

- 优先级：P0
- 状态：Open
- 位置：`internal/download/publisher.go:52-71,83-107`
- 现象：发布前先检查目标文件，再计算自动改名，最后执行 `Rename`，没有原子占位或目标级锁。
- 复现：并发启动两个任务，令它们使用同一个输出路径；两个任务可能同时选中同一个自动改名路径。
- 影响：后发布的任务可能覆盖先发布的完整文件，导致下载结果静默丢失。
- 修复方向：使用原子独占创建/占位，或按目标路径串行化“冲突决策 + 发布”操作。

## BUG-20261009-004：覆盖发布校验失败会删除原有完整文件

- 优先级：P0
- 状态：Open
- 位置：`internal/download/publisher.go:93-114,161-172`
- 现象：`OverwriteAlways` 先用 `Rename` 替换目标，再做校验；校验只检查文件非空。
- 复现：目标路径已有完整文件，新的临时文件为空或校验失败；发布后 `verifyPublished` 会删除目标路径。
- 影响：失败任务可能删除用户已有的有效视频，且无法恢复。
- 修复方向：发布前完成媒体格式/可读性校验；覆盖时保留旧文件，使用同目录临时文件和安全替换流程。

## BUG-20261009-005：网络下载协议白名单允许访问本地文件

- 优先级：P0
- 状态：Open
- 位置：`internal/ffmpeg/download.go:66-68`
- 现象：FFmpeg 协议白名单包含 `file`，远程 HLS/DASH 清单中的嵌套资源可使用本地文件协议。
- 复现：使用远程清单引用 `file:///...` 资源，并启动 FFmpeg 下载流程。
- 影响：恶意公开网页可能诱导应用读取本地文件；分析阶段的公网地址校验无法覆盖 FFmpeg 自己读取的嵌套资源。
- 修复方向：网络下载路径移除 `file`；本地输入使用独立的本地媒体流程，或由后端预取并校验所有嵌套资源。

## BUG-20261009-006：Windows 覆盖发布和跨设备回退不可靠

- 优先级：P1
- 状态：Open
- 位置：`internal/download/publisher.go:93-107,212-221`
- 现象：所有平台统一使用 `os.Rename` 覆盖目标；跨设备判断依赖英文错误字符串。
- 复现：Windows 上目标文件已存在且选择覆盖，或临时目录与输出目录位于不同卷。
- 影响：覆盖任务可能失败；跨设备时可能无法进入复制回退流程。
- 修复方向：增加平台适配的安全替换逻辑，使用平台错误码判断跨设备错误，并补充 Windows 测试。

## BUG-20261009-007：取消任务存在非法状态顺序

- 优先级：P1
- 状态：Open
- 位置：`internal/download/manager.go:345-355,479-497`
- 现象：取消线程和调度线程分别操作队列与状态，执行线程可能在任务已变为 `canceled` 后仍发送 `preparing`。
- 复现：在调度线程取出队列任务、但执行线程尚未设置 `preparing` 的窗口内调用取消。
- 影响：事件可能出现 `canceled -> preparing -> canceled`，取消任务也可能短暂启动下载工作。
- 修复方向：在同一原子状态转换中完成“取出并开始执行”；执行入口先确认任务仍可运行，并拒绝终态后的状态转换。

## BUG-20261009-008：未知下载预设没有被拒绝

- 优先级：P1
- 状态：Open
- 位置：`internal/ffmpeg/download.go:79-80`、`internal/download/manager.go:258-267`
- 现象：`BuildDownloadArgs` 忽略 `GetPreset` 返回的 `ok`，任务管理器也接受任意 profile 字符串。
- 复现：提交一个不存在的 profile，例如 `unknown`。
- 影响：FFmpeg 可能使用默认编码行为，绕过产品要求的固定预设；任务状态还会错误地进入转码分支。
- 修复方向：在任务创建和参数构建两处拒绝未知 profile，并让所有执行路径复用同一套预设校验。

## BUG-20261009-009：阶段进度与真实 FFmpeg 执行尚未接通

- 优先级：P1
- 状态：Open
- 位置：`internal/download/manager.go:546-582`、`internal/ffmpeg/run_download.go:49-150`
- 现象：FFmpeg 进度解析器有独立测试，但任务管理器只使用固定的 0% 到 100% mock 进度。
- 复现：观察任意任务的进度；进度总是按固定时间和固定速度增长，与输入媒体无关。
- 影响：用户看到的速度、大小和百分比不代表真实下载，无法判断任务是否卡住或完成。
- 修复方向：将 `ProgressSink` 接入任务更新；未知总时长时保持进度为空，不伪造百分比。

## BUG-20261009-010：全量 Go 测试当前不通过

- 优先级：P1
- 状态：Open
- 位置：`internal/media/model_test.go`、`internal/settings/sanitize_test.go`、`internal/settings/settings.go:183-204`
- 现象：`go test ./... -count=1` 失败。
- 复现结果：
  - `internal/media`：`TestAnalysisResult_JSONFieldNames`、`TestAnalysisEvent_FailedPhase` 失败。
  - `internal/settings`：`TestSanitizeFileName_LengthLimitUnicode` 失败。
  - `internal/settings`：`TestSettings_EnsureDirectory_Creates` 发生 nil pointer panic。
- 影响：全量验收无法通过，阶段 4 不能据此宣称整体测试通过。
- 修复方向：修正模型 JSON 契约或测试预期，修正 Unicode 文件名长度处理，并保证 `IsWritable` 在测试文件系统返回空文件句柄时不会 panic。

## Review 验证记录

- `go test -race ./internal/download ./internal/ffmpeg -count=1`：通过。
- `go vet ./...`：通过。
- `git diff --check`：通过。
- Windows amd64 编译级检查：通过。
- 尚未完成：真实直链/HLS/DASH 下载、三平台运行冒烟和真实 FFmpeg 产物验证。
