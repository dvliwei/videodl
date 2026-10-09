# FFmpeg 发行与许可证记录

VideoDL 将 FFmpeg 和 FFprobe 作为独立子进程调用，不链接 FFmpeg 的库。每个发布包必须同时带上本目录中的许可证材料；构建前必须运行 `make ffmpeg-fetch`，发布前必须运行 `make ffmpeg-verify`。

## 固定版本与来源

| 项目 | 值 |
| --- | --- |
| FFmpeg 版本 | **8.0.3** |
| FFmpeg 上游标签 | `n8.0.3` |
| 构建发行标签 | `ffmpeg-8.0.3-build4` |
| 构建来源 | [AtlasYang/ffmpeg-static-builds](https://github.com/AtlasYang/ffmpeg-static-builds) |
| 上游源码 | [FFmpeg 8.0.3 源码](https://ffmpeg.org/releases/ffmpeg-8.0.3.tar.xz) |
| 上游签名 | [ffmpeg-8.0.3.tar.xz.asc](https://ffmpeg.org/releases/ffmpeg-8.0.3.tar.xz.asc) |
| 配置审计 | 每个平台的 `*.configure.txt` 链接记录在 `manifest.yaml` |

## 平台资产

| 目标 | 归档 SHA-256 | 最低系统要求 |
| --- | --- | --- |
| Windows x86_64 | `fd3473736674343cd1948eb45f28b46f377616432a5cc15eb896bdaabea506f2` | Windows 10 |
| Linux x86_64 | `2612ed26322c864a7411c3578b74fe7f229127e6f9f906d3aa994937a4c9620e` | glibc 2.35 |
| macOS x86_64 | `b84b4515aaf2fe443a76445f02d05544a384dcd46cc6059fb8a1a4f9aaa3035d` | macOS 10.15 |
| macOS ARM64 | `26f6269b117a51c5fdfd3a870c4e0a62b99f0575740a3933fc3d852f0a3f80b1` | macOS 11 |

Linux ARM64 资产虽可从供应方获得，但不在当前首发矩阵内，须完成独立测试和发布说明后才能启用。

## 构建许可策略

固定构建必须包含以下策略开关：

```text
--disable-gpl
--disable-nonfree
--disable-version3
--disable-autodetect
```

因此当前发行构建不包含 GPL 的 x264/x265/Xvid，也不包含 nonfree 的 FDK-AAC。H.264/HEVC 的硬件编码能力依赖目标平台和驱动，不能作为所有设备都可用的固定软件编码器承诺。

随构建记录的编码集合为：

- 视频：libvpx VP8/VP9、libaom AV1；
- 音频：FFmpeg native AAC、libopus、libvorbis；
- 解码：常用 H.264/H.265/AV1/VP8/VP9/AAC/MP3/Opus/FLAC，实际注册项以对应配置审计文件为准。

编码器专利许可与 LGPL 著作权许可是两件事；产品只提供固定预设，不向用户暴露任意 FFmpeg 参数。

## 分发义务与源码提供

FFmpeg 本身按 LGPL-2.1-or-later 发行；仓库中保留许可证副本、第三方通知、精确构建来源、归档哈希和每个平台配置审计链接。发布材料必须继续提供：

1. `licenses/FFmpeg-COPYING.LGPLv2.1`；
2. `licenses/THIRD-PARTY-NOTICES.md`；
3. 本文件和 `manifest.yaml`；
4. 上游 FFmpeg 8.0.3 源码及签名链接；
5. 四个平台对应的构建归档与 `*.configure.txt` 链接。

本项目不修改 FFmpeg 源码，也不把 FFmpeg 库链接进 VideoDL。若以后改为库链接或修改 FFmpeg，必须重新做 LGPL 重链接/源码义务审查，未经维护者确认不得发布。

这份记录不是法律意见；发布前由维护者确认目标司法辖区的 LGPL、第三方许可证和编解码器专利事项。
