# FFmpeg 第三方许可证通知

本目录随 VideoDL 发布。以下组件来自固定的 FFmpeg 8.0.3 构建；完整版本、构建归档和每个平台配置审计链接见上级 `manifest.yaml` 与 `CREDITS.md`。

| 组件 | 用途 | 许可证/通知来源 |
| --- | --- | --- |
| FFmpeg | 媒体探测、封装、转码 | [LGPL-2.1-or-later](https://github.com/FFmpeg/FFmpeg/blob/n8.0.3/COPYING.LGPLv2.1) |
| libvpx | VP8/VP9 编解码 | [BSD-3-Clause](https://chromium.googlesource.com/webm/libvpx/+/main/LICENSE) |
| libaom | AV1 编解码 | [BSD-2-Clause 与 AOM Patent License](https://aomedia.googlesource.com/aom/+/main/LICENSE) |
| libopus | Opus 编解码 | [BSD-3-Clause](https://github.com/xiph/opus/blob/v1.5.2/COPYING) |
| libvorbis/libogg | Vorbis/Ogg 编解码 | [BSD-3-Clause](https://github.com/xiph/vorbis/blob/v1.3.7/COPYING) |
| zlib | 压缩支持 | [zlib License](https://zlib.net/zlib_license.html) |

供应方的 `*.configure.txt` 审计文件是本版本实际启用组件和版本的权威记录。若供应方构建配置与本通知不一致，构建必须停止，不能仅修改说明文件继续发布。
