<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import AnalyzePanel from './features/analyze/AnalyzePanel.vue'
import TaskList from './features/tasks/TaskList.vue'
import {
  startDownload,
  getDefaultDirectory,
  chooseDirectory,
  saveAs,
  isWailsAvailable
} from './shared/wails.js'

const defaultDir = ref('')
const isDefaultDir = ref(false)
const dirHintLoading = ref(false)
const dirLoadError = ref(null)

const showFirstTimePrompt = computed(() =>
  isWailsAvailable() && isDefaultDir.value && defaultDir.value
)

onMounted(async () => {
  if (!isWailsAvailable()) return
  dirHintLoading.value = true
  try {
    const resp = await getDefaultDirectory()
    defaultDir.value = resp.path || ''
    isDefaultDir.value = resp.isDefault
  } catch (err) {
    dirLoadError.value = err.message || String(err)
  } finally {
    dirHintLoading.value = false
  }
})

async function openDirectoryDialog() {
  try {
    const chosen = await chooseDirectory(defaultDir.value)
    if (chosen) {
      defaultDir.value = chosen
      isDefaultDir.value = false
    }
  } catch (err) {
    dirLoadError.value = err.message || String(err)
  }
}

async function handleDownload(payload) {
  try {
    let outputPath = ''

    const wantSaveAs = false
    if (wantSaveAs) {
      const suggestedName = `${payload.title || 'video'}.mp4`
      const chosen = await saveAs(suggestedName, defaultDir.value)
      if (!chosen) return
      outputPath = chosen
    }

    await startDownload({
      analysisId: payload.analysisId,
      mediaId: payload.mediaId,
      variantId: payload.variantId || '',
      outputPath,
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

      <div v-if="isWailsAvailable() && defaultDir" class="dir-hint" aria-live="polite">
        <span class="dir-hint-label">下载目录</span>
        <span class="dir-hint-path" :title="defaultDir">{{ defaultDir }}</span>
        <span v-if="isDefaultDir" class="dir-hint-badge">默认</span>
        <button
          type="button"
          class="dir-change-btn"
          @click="openDirectoryDialog"
          :disabled="dirHintLoading"
        >
          更改
        </button>
      </div>

      <span v-else class="header-caption">桌面视频下载器</span>
    </header>

    <section class="workspace" aria-labelledby="page-title">
      <div class="intro">
        <p class="eyebrow">VIDEO DOWNLOADER</p>
        <h1 id="page-title">从网页中找到视频</h1>
        <p class="intro-copy">粘贴公开网页地址，分析可用的视频资源。</p>
      </div>

      <div v-if="showFirstTimePrompt" class="first-run-banner" role="status">
        <div class="first-run-content">
          <span class="first-run-icon" aria-hidden="true">ⓘ</span>
          <div class="first-run-text">
            <strong>首次使用：设置下载目录</strong>
            <span>当前使用默认目录 {{ defaultDir }}。点击更改以指定自定义位置。</span>
          </div>
          <button type="button" class="first-run-btn" @click="openDirectoryDialog">选择目录</button>
        </div>
      </div>

      <AnalyzePanel @download="handleDownload" />

      <TaskList />
    </section>

    <footer class="app-footer">
      <span>支持公开访问且非 DRM 保护的视频资源</span>
      <span>FFmpeg 将随应用提供</span>
    </footer>
  </main>
</template>
