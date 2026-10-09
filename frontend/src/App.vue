<script setup>
import { ref } from 'vue'

const pageURL = ref('')
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

      <form class="url-form" @submit.prevent>
        <label class="sr-only" for="page-url">网页地址</label>
        <input
          id="page-url"
          v-model="pageURL"
          type="url"
          inputmode="url"
          placeholder="粘贴网页 URL，例如 https://example.com/video"
          autocomplete="url"
          spellcheck="false"
        />
        <button type="submit" disabled title="分析服务将在后续开发任务中接入">
          分析
        </button>
      </form>
      <p class="service-note">分析与下载服务正在搭建中，当前界面用于后续功能接入。</p>

      <section class="panel results-panel" aria-labelledby="results-title">
        <div class="panel-heading">
          <div>
            <h2 id="results-title">视频资源</h2>
            <p>分辨率、格式和音视频信息将在分析后显示</p>
          </div>
          <span class="panel-count">0</span>
        </div>
        <div class="empty-state">
          <span class="empty-icon" aria-hidden="true">▶</span>
          <strong>还没有分析结果</strong>
          <span>分析完成后，可在这里选择视频并开始下载。</span>
        </div>
      </section>

      <section class="panel tasks-panel" aria-labelledby="tasks-title">
        <div class="panel-heading">
          <div>
            <h2 id="tasks-title">下载任务</h2>
            <p>查看进度、状态和已完成的文件</p>
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
