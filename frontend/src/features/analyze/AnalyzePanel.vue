<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { validateUrl } from '../../shared/validate.js'
import {
  analyze,
  analyzeWithBrowserSession,
  cancelAnalysis,
  onAnalysisUpdate,
  isWailsAvailable
} from '../../shared/wails.js'
import {
  YTDLP_PHASE,
  createYTDLPState,
  createBrowserAuthorization,
  cancelBrowserAuthorization
} from '../settings/status.js'
import MediaCard from './MediaCard.vue'

const urlInput = ref('')
const inputError = ref('')
const analysisState = ref('idle')
const currentAnalysisId = ref('')
const analysisResult = ref(null)
const analysisError = ref(null)
const selectedVariants = ref({})
const browserOptions = [
  { value: 'chrome', label: 'Chrome' },
  { value: 'chromium', label: 'Chromium' },
  { value: 'edge', label: 'Edge' },
  { value: 'firefox', label: 'Firefox' },
  { value: 'brave', label: 'Brave' },
  { value: 'opera', label: 'Opera' },
  { value: 'safari', label: 'Safari' },
  { value: 'vivaldi', label: 'Vivaldi' },
  { value: 'whale', label: 'Whale' }
]
const selectedBrowser = ref('chrome')
const browserProfile = ref('')
const authState = ref(createYTDLPState())

let unwatchAnalysis = null
let analyzing = false

const STATE = {
  IDLE: 'idle',
  LOADING: 'loading',
  SUCCESS: 'success',
  EMPTY: 'empty',
  FAILED: 'failed',
  CANCELED: 'canceled'
}

const isLoading = computed(() => analysisState.value === STATE.LOADING)

const candidates = computed(() => analysisResult.value?.candidates || [])

const pageTitle = computed(() => analysisResult.value?.pageTitle || '')

const warnings = computed(() => analysisResult.value?.warnings || [])

function handleInput() {
  if (inputError.value) inputError.value = ''
}

function validateBeforeSubmit() {
  const result = validateUrl(urlInput.value)
  if (!result.ok) {
    inputError.value = result.reason
    return null
  }
  return result.url
}

async function startAnalysis(browserSession = null) {
  if (analyzing) return
  const url = validateBeforeSubmit()
  if (!url) return

  analyzing = true
  analysisState.value = STATE.LOADING
  analysisResult.value = null
  analysisError.value = null
  currentAnalysisId.value = ''
  selectedVariants.value = {}
  inputError.value = ''

  try {
    const resp = browserSession
      ? await analyzeWithBrowserSession(url, browserSession.browser, browserSession.profile)
      : await analyze(url)
    currentAnalysisId.value = resp.analysisId
  } catch (err) {
    analysisState.value = STATE.FAILED
    analysisError.value = { message: err.message || String(err) }
    analyzing = false
  }
}

async function confirmBrowserAuthorization() {
  authState.value = { ...authState.value, browserAuthorized: true }
  await startAnalysis({ browser: selectedBrowser.value, profile: browserProfile.value.trim() })
  authState.value = createYTDLPState()
}

function cancelBrowserAuthorizationPrompt() {
  authState.value = cancelBrowserAuthorization(authState.value)
  analysisState.value = STATE.CANCELED
  analysisError.value = null
}

async function handleCancel() {
  if (!currentAnalysisId.value) return
  try {
    await cancelAnalysis(currentAnalysisId.value)
  } catch (err) {
    // swallow
  }
}

function handleDownload(payload) {
  emit('download', {
    analysisId: payload.analysisId,
    mediaId: payload.mediaId,
    variantId: payload.variantId
  })
}

function handleSelectVariant(candidateId, variantId) {
  selectedVariants.value[candidateId] = variantId
}

const emit = defineEmits(['download'])

onMounted(() => {
  if (!isWailsAvailable()) return

  unwatchAnalysis = onAnalysisUpdate((evt) => {
    if (!evt) return
    if (evt.analysisId !== currentAnalysisId.value) return

    switch (evt.phase) {
      case 'completed':
        analyzing = false
        analysisResult.value = evt.result
        if (evt.result && evt.result.candidates && evt.result.candidates.length > 0) {
          analysisState.value = STATE.SUCCESS
        } else {
          analysisState.value = STATE.EMPTY
        }
        break
      case 'failed':
        analyzing = false
        analysisError.value = {
          code: evt.errorCode,
          message: evt.errorMessage || '分析失败'
        }
        if (evt.errorCode === 'ytdlp.auth_required') {
          authState.value = createBrowserAuthorization(authState.value)
        }
        analysisState.value = STATE.FAILED
        break
      case 'canceled':
        analyzing = false
        analysisState.value = STATE.CANCELED
        break
    }
  })
})

onUnmounted(() => {
  if (unwatchAnalysis) unwatchAnalysis()
  if (analyzing && currentAnalysisId.value) {
    cancelAnalysis(currentAnalysisId.value).catch(() => {})
  }
})
</script>

<template>
  <section class="panel analyze-panel" aria-labelledby="analyze-title">
    <div class="panel-heading">
      <div>
        <h2 id="analyze-title">视频资源分析</h2>
        <p v-if="analysisState === STATE.SUCCESS && pageTitle">来源：{{ pageTitle }}</p>
        <p v-else>粘贴网页地址，识别其中的视频资源</p>
      </div>
      <span class="panel-count" v-if="candidates.length > 0">{{ candidates.length }}</span>
    </div>

    <div class="panel-body">
      <form class="url-form" @submit.prevent="startAnalysis" :class="{ 'has-error': inputError }">
        <label class="sr-only" for="page-url">网页地址</label>
        <input
          id="page-url"
          v-model="urlInput"
          type="url"
          inputmode="url"
          placeholder="粘贴网页 URL，例如 https://example.com/video"
          autocomplete="url"
          spellcheck="false"
          @input="handleInput"
          @keydown.enter.prevent="startAnalysis"
        />
        <button type="submit" class="btn-primary" :disabled="isLoading">
          <span v-if="isLoading">分析中…</span>
          <span v-else>分析</span>
        </button>
        <button
          v-if="isLoading"
          type="button"
          class="btn-secondary btn-cancel"
          @click="handleCancel"
        >
          取消
        </button>
      </form>

      <p v-if="inputError" class="input-error" role="alert">{{ inputError }}</p>

      <div v-if="analysisState === STATE.IDLE" class="empty-state">
        <span class="empty-icon" aria-hidden="true">▶</span>
        <strong>输入 URL 开始分析</strong>
        <span>支持公开可访问的 HTTP/HTTPS 网页。</span>
      </div>

      <div v-else-if="analysisState === STATE.LOADING" class="loading-state">
        <span class="spinner" aria-hidden="true"></span>
        <strong>正在分析页面…</strong>
        <span>这通常需要几秒，取决于页面大小和网络情况。</span>
      </div>

      <div v-else-if="analysisState === STATE.CANCELED" class="empty-state">
        <span class="empty-icon" aria-hidden="true">⏹</span>
        <strong>分析已取消</strong>
        <span>重新输入地址即可再次尝试。</span>
      </div>

      <div v-else-if="authState.phase === YTDLP_PHASE.AUTH_REQUIRED" class="auth-state">
        <strong>此网站可能需要登录会话</strong>
        <p>只有在你明确授权后，VideoDL 才会从所选浏览器读取当前会话。Cookie 不会展示、上传或写入分析结果。</p>
        <div class="auth-controls">
          <label>
            浏览器
            <select v-model="selectedBrowser">
              <option v-for="browser in browserOptions" :key="browser.value" :value="browser.value">
                {{ browser.label }}
              </option>
            </select>
          </label>
          <label>
            配置名称（可选）
            <input v-model="browserProfile" type="text" maxlength="128" placeholder="例如 Default" />
          </label>
        </div>
        <div class="auth-actions">
          <button type="button" class="btn-primary" @click="confirmBrowserAuthorization">授权并重试</button>
          <button type="button" class="btn-secondary" @click="cancelBrowserAuthorizationPrompt">取消</button>
        </div>
      </div>

      <div v-else-if="analysisState === STATE.FAILED" class="error-state">
        <span class="error-icon" aria-hidden="true">!</span>
        <strong>分析失败</strong>
        <span class="error-message">{{ analysisError?.message || '未知错误' }}</span>
        <p class="hint">请确认地址正确、网络通畅，且目标页面是公开可访问的。</p>
      </div>

      <div v-else-if="analysisState === STATE.EMPTY" class="empty-state">
        <span class="empty-icon" aria-hidden="true">○</span>
        <strong>未发现可下载的视频资源</strong>
        <div class="support-scope">
          <p>VideoDL 目前支持：</p>
          <ul>
            <li>静态 HTML 中的 &lt;video&gt; 和 &lt;source&gt; 标签</li>
            <li>直接的媒体文件链接（.mp4、.webm、.mkv 等）</li>
            <li>公开的 HLS (.m3u8) 和 DASH (.mpd) 清单</li>
          </ul>
          <p class="support-limit">不支持：需要 JavaScript 渲染的动态页面、登录/付费内容、DRM 保护资源。</p>
        </div>
      </div>

      <div v-else-if="analysisState === STATE.SUCCESS" class="results-grid">
        <p v-if="warnings.length > 0" class="warnings">
          {{ warnings.join('；') }}
        </p>
        <MediaCard
          v-for="candidate in candidates"
          :key="candidate.id"
          :candidate="candidate"
          :analysis-id="currentAnalysisId"
          :selected-variant-id="selectedVariants[candidate.id] || ''"
          @download="handleDownload"
          @select-variant="(vid) => handleSelectVariant(candidate.id, vid)"
        />
      </div>
    </div>
  </section>
</template>
