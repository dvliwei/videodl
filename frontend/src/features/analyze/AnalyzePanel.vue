<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { t, errorMessage } from '../../i18n/index.js'
import { validateUrl } from '../../shared/validate.js'
import {
  analyze,
  analyzeWithBrowserSession,
  cancelAnalysis,
  onAnalysisUpdate,
  onTaskUpdate,
  isWailsAvailable
} from '../../shared/wails.js'
import {
  YTDLP_PHASE,
  createYTDLPState,
  createBrowserAuthorization,
  cancelBrowserAuthorization
} from '../settings/status.js'
import MediaCard from './MediaCard.vue'
import { requestAnalysis } from './request.js'

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
let unwatchTask = null
let analyzing = false

const startingDownloads = ref({})

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
    // reason 是 i18n 键，模板中 t(inputError) 渲染为当前语言文案。
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
    const resp = await requestAnalysis(url, browserSession, {
      analyze,
      analyzeWithBrowserSession
    })
    currentAnalysisId.value = resp.analysisId
  } catch (err) {
    analysisState.value = STATE.FAILED
    analysisError.value = { message: errorMessage(null, err.message || String(err)) }
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
  if (payload.mediaId) {
    startingDownloads.value[payload.mediaId] = true
  }
  emit('download', {
    analysisId: payload.analysisId,
    mediaId: payload.mediaId,
    variantId: payload.variantId,
    title: payload.title || ''
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
          message: errorMessage(evt.errorCode, evt.errorMessage)
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

  unwatchTask = onTaskUpdate((evt) => {
    if (!evt || !evt.task) return
    const mediaId = evt.task.mediaId
    if (mediaId && startingDownloads.value[mediaId]) {
      delete startingDownloads.value[mediaId]
      startingDownloads.value = { ...startingDownloads.value }
    }
  })
})

onUnmounted(() => {
  if (unwatchAnalysis) unwatchAnalysis()
  if (unwatchTask) unwatchTask()
  if (analyzing && currentAnalysisId.value) {
    cancelAnalysis(currentAnalysisId.value).catch(() => {})
  }
})
</script>

<template>
  <section class="panel analyze-panel" aria-labelledby="analyze-title">
    <div class="panel-heading">
      <div>
        <h2 id="analyze-title">{{ t('analyze.title') }}</h2>
        <p v-if="analysisState === STATE.SUCCESS && pageTitle">{{ t('analyze.source', { title: pageTitle }) }}</p>
        <p v-else>{{ t('analyze.subtitle') }}</p>
      </div>
      <span class="panel-count" v-if="candidates.length > 0">{{ candidates.length }}</span>
    </div>

    <div class="panel-body">
      <form class="url-form" @submit.prevent="startAnalysis" :class="{ 'has-error': inputError }">
        <label class="sr-only" for="page-url">{{ t('analyze.urlLabel') }}</label>
        <input
          id="page-url"
          v-model="urlInput"
          type="url"
          inputmode="url"
          :placeholder="t('analyze.urlPlaceholder')"
          autocomplete="url"
          spellcheck="false"
          @input="handleInput"
          @keydown.enter.prevent="startAnalysis"
        />
        <button type="submit" class="btn-primary" :disabled="isLoading">
          <span v-if="isLoading">{{ t('analyze.analyzing') }}</span>
          <span v-else>{{ t('analyze.analyze') }}</span>
        </button>
        <button
          v-if="isLoading"
          type="button"
          class="btn-secondary btn-cancel"
          @click="handleCancel"
        >
          {{ t('common.cancel') }}
        </button>
      </form>

      <p v-if="inputError" class="input-error" role="alert">{{ t(inputError) }}</p>

      <div v-if="analysisState === STATE.IDLE" class="empty-state">
        <span class="empty-icon" aria-hidden="true">▶</span>
        <strong>{{ t('analyze.emptyTitle') }}</strong>
        <span>{{ t('analyze.emptyHint') }}</span>
      </div>

      <div v-else-if="analysisState === STATE.LOADING" class="loading-state">
        <span class="spinner" aria-hidden="true"></span>
        <strong>{{ t('analyze.loadingTitle') }}</strong>
        <span>{{ t('analyze.loadingHint') }}</span>
      </div>

      <div v-else-if="analysisState === STATE.CANCELED" class="empty-state">
        <span class="empty-icon" aria-hidden="true">⏹</span>
        <strong>{{ t('analyze.canceledTitle') }}</strong>
        <span>{{ t('analyze.canceledHint') }}</span>
      </div>

      <div v-else-if="authState.phase === YTDLP_PHASE.AUTH_REQUIRED" class="auth-state">
        <strong>{{ t('auth.title') }}</strong>
        <p>{{ t('auth.body') }}</p>
        <div class="auth-controls">
          <label>
            {{ t('auth.browser') }}
            <select v-model="selectedBrowser">
              <option v-for="browser in browserOptions" :key="browser.value" :value="browser.value">
                {{ browser.label }}
              </option>
            </select>
          </label>
          <label>
            {{ t('auth.profileName') }}
            <input v-model="browserProfile" type="text" maxlength="128" :placeholder="t('auth.profilePlaceholder')" />
          </label>
        </div>
        <div class="auth-actions">
          <button type="button" class="btn-primary" @click="confirmBrowserAuthorization">{{ t('auth.authorizeAndRetry') }}</button>
          <button type="button" class="btn-secondary" @click="cancelBrowserAuthorizationPrompt">{{ t('common.cancel') }}</button>
        </div>
      </div>

      <div v-else-if="analysisState === STATE.FAILED" class="error-state">
        <span class="error-icon" aria-hidden="true">!</span>
        <strong>{{ t('analyze.failedTitle') }}</strong>
        <span class="error-message">{{ analysisError?.message || t('analyze.unknownError') }}</span>
        <p class="hint">{{ t('analyze.failedHint') }}</p>
      </div>

      <div v-else-if="analysisState === STATE.EMPTY" class="empty-state">
        <span class="empty-icon" aria-hidden="true">○</span>
        <strong>{{ t('analyze.emptyResultTitle') }}</strong>
        <div class="support-scope">
          <p>{{ t('analyze.supportsLabel') }}</p>
          <ul>
            <li>{{ t('analyze.supportsVideoTag') }}</li>
            <li>{{ t('analyze.supportsDirect') }}</li>
            <li>{{ t('analyze.supportsHls') }}</li>
          </ul>
          <p class="support-limit">{{ t('analyze.supportsNot') }}</p>
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
          :is-starting="!!startingDownloads[candidate.id]"
          @download="handleDownload"
          @select-variant="(vid) => handleSelectVariant(candidate.id, vid)"
        />
      </div>
    </div>
  </section>
</template>
