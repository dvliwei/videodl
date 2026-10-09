<script setup>
import { ref } from 'vue'
import AnalyzePanel from './features/analyze/AnalyzePanel.vue'
import { startDownload } from './shared/wails.js'

const handleDownload = async (payload) => {
  try {
    await startDownload({
      analysisId: payload.analysisId,
      mediaId: payload.mediaId,
      variantId: payload.variantId || '',
      outputPath: '',
      profile: 'original'
    })
  } catch (err) {
    console.error('start download failed:', err)
  }
}
</script>

<template>
  <main class="app-shell">
    <header class="app-header">
      <div class="brand" aria-label="VideoDL">
        <span class="brand-mark" aria-hidden="true">V</span>
        <span>VideoDL</span>
      </div>
      <span class="header-caption">桌面视频下载器</span>
    </header>

    <section class="workspace" aria-labelledby="page-title">
      <div class="intro">
        <p class="eyebrow">VIDEO DOWNLOADER</p>
        <h1 id="page-title">从网页中找到视频</h1>
        <p class="intro-copy">粘贴公开网页地址，分析可用的视频资源。</p>
      </div>

      <AnalyzePanel @download="handleDownload" />

      <section class="panel tasks-panel" aria-labelledby="tasks-title">
        <div class="panel-heading">
          <div>
            <h2 id="tasks-title">下载任务</h2>
            <p>查看进度、状态和已完成的文件（后续任务中接入）</p>
          </div>
        </div>
        <div class="task-placeholder">暂无下载任务</div>
      </section>
    </section>

    <footer class="app-footer">
      <span>支持公开访问且非 DRM 保护的视频资源</span>
      <span>FFmpeg 将随应用提供</span>
    </footer>
  </main>
</template>
ENDOFFILE 