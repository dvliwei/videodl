# yt-dlp 发行与许可证记录

VideoDL 以独立子进程调用官方 yt-dlp standalone executable，不要求用户安装 Python。首个集成版本固定为 `2026.08.19`，发布渠道为官方 stable。资源来源、平台文件名和 SHA-256 记录在同目录的 `manifest.yaml`。

上游项目：[yt-dlp/yt-dlp](https://github.com/yt-dlp/yt-dlp)

官方发布说明明确指出，PyInstaller 打包的 standalone executable 包含 GPLv3+ 及其他第三方组件；发布包必须同时分发 `licenses/THIRD_PARTY_LICENSES.txt` 和相关许可证说明。应用不加载用户配置、插件或任意外部参数。

更新时只接受官方仓库 stable release 的已知资产，并在原子替换前校验 SHA-256。SHA-256 用于完整性校验，不代表已完成 GPG 身份认证。
