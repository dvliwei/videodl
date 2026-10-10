# yt-dlp 集成、更新与许可证说明

## 运行方式

VideoDL 使用 yt-dlp 官方 standalone 可执行文件作为网站解析器，随应用分发，不要求用户安装 Python。Go 通过稳定的单视频 JSON 接口读取标题、格式和短期媒体地址；下载、取消、重试、FFmpeg 合并和最终文件发布仍由现有 Go 任务体系负责。

默认调用会忽略用户配置、插件和浏览器 Cookie，并通过应用控制的公网地址代理访问目标站点。VideoDL 不从 PATH 查找 yt-dlp，也不会将签名媒体 URL、请求头或 Cookie 返回给 Vue。

## 浏览器会话授权

当站点明确需要登录会话时，界面只在分析失败后提示用户。用户必须主动选择支持的浏览器并点击“授权并重试”；浏览器名称和可选配置名称会逐次传递给后端，Cookie 内容不会展示、上传、写入日志或进入分析结果。

该能力不是登录或访问控制绕过功能。付费墙、DRM、站点禁止的访问和无法通过公开会话读取的内容仍会明确失败。用户应确保自己有权读取本机浏览器会话及目标内容。

## 稳定版更新

“检查并更新 yt-dlp”只查询官方 `yt-dlp/yt-dlp` GitHub stable release，当前首个内置版本为 `2026.08.19`。更新器只接受三个已知平台资产，下载到用户配置目录的独立副本后校验 SHA-256、执行 `--version` 健康检查，再原子替换；失败、取消或校验不匹配会保留上一份可用文件。

SHA-256 是完整性校验，不等同于 GPG/签名身份认证。发布前仍需维护者核对官方 release、来源和平台构建产物。

## 许可证和来源

上游项目：[yt-dlp/yt-dlp](https://github.com/yt-dlp/yt-dlp)。其官方 PyInstaller standalone 资产包含 GPLv3+ 及其他第三方组件；应用包应随 `manifest.yaml`、`CREDITS.md`、`THIRD-PARTY-NOTICES.txt` 和 `licenses/` 一并分发。上游许可证与第三方通知的参考副本位于：

- `build/resources/yt-dlp/manifest.yaml`
- `build/resources/yt-dlp/CREDITS.md`
- `build/resources/yt-dlp/THIRD-PARTY-NOTICES.txt`
- `build/resources/yt-dlp/licenses/`

构建前运行 `make yt-dlp-fetch` 和 `make yt-dlp-verify`；正式打包脚本只复制清单指定资产，不使用系统 PATH 中的 yt-dlp。
