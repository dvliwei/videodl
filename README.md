# VideoDL

跨平台桌面视频下载器，使用 Go、Wails v2、Vue 3 和 Vite 构建。

## 开发入口

- [`AGENTS.md`](AGENTS.md)：AI 协作、架构边界和完成标准。
- [`docs/PRODUCT_SPEC.md`](docs/PRODUCT_SPEC.md)：MVP 产品与技术要求。
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)：当前架构审查、模块边界和接口约定。
- [`docs/DEVELOPMENT_PLAN.md`](docs/DEVELOPMENT_PLAN.md)：按依赖顺序排列的实现任务和验收条件。
- [`docs/TRAE_GUIDE.md`](docs/TRAE_GUIDE.md)：使用 TRAE 按任务实现和交付的说明。

## 本地开发

安装 Go、Node.js/npm、Wails CLI 以及当前操作系统所需的 Wails 构建依赖后，在项目根目录运行 `wails dev`。

## 构建

在项目根目录运行 `wails build`。跨平台构建与 FFmpeg/FFprobe 随包要求见 [`docs/PRODUCT_SPEC.md`](docs/PRODUCT_SPEC.md)；首发支持矩阵需按开发计划逐平台验收。

## FFmpeg 发行材料

FFmpeg 8.0.3 的来源、平台资产 SHA-256、配置审计链接、编码器集合和许可证义务见 [`build/resources/ffmpeg/manifest.yaml`](build/resources/ffmpeg/manifest.yaml) 与 [`build/resources/ffmpeg/CREDITS.md`](build/resources/ffmpeg/CREDITS.md)。

获取并校验构建输入：

```sh
make ffmpeg-manifest-test
make ffmpeg-fetch
make ffmpeg-verify
```

`ffmpeg-fetch` 只在构建时下载固定资产；应用运行时不联网获取 FFmpeg/FFprobe，也不使用系统 `PATH` 回退。
