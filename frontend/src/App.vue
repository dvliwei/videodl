<script setup>
import { ref, computed, onMounted } from 'vue'
import { t, setLocale, activeLocale, SUPPORTED_LOCALES } from './i18n/index.js'
import AnalyzePanel from './features/analyze/AnalyzePanel.vue'
import TaskList from './features/tasks/TaskList.vue'
import {
  startDownload,
  getDefaultDirectory,
  setDefaultDirectory,
  chooseDirectory,
  saveAs,
  getYTDLPStatus,
  updateYTDLP,
  isWailsAvailable
} from './shared/wails.js'
import {
  createYTDLPState,
  startChecking,
  startUpdating,
  receiveStatus,
  receiveUpdateFailure
} from './features/settings/status.js'
import YTDLPStatus from './features/settings/YTDLPStatus.vue'

const profileOptions = computed(() => [
  { value: 'original', label: t('dir.profileOriginal') },
  { value: 'mp4-h264-aac', label: t('dir.profileCompat') }
])

const brandInitial = computed(() => (t('app.brand') || 'V').charAt(0))

const defaultDir = ref('')
const isDefaultDir = ref(false)
const dirHintLoading = ref(false)
const dirLoadError = ref(null)

const selectedProfile = ref('original')
const saveAsMode = ref(false)

const downloadError = ref(null)
const ytdlpState = ref(createYTDLPState())

const showFirstTimePrompt = computed(() =>
  isWailsAvailable() && isDefaultDir.value && defaultDir.value
)

onMounted(async () => {
  if (!isWailsAvailable()) return
  ytdlpState.value = startChecking(ytdlpState.value)
  try {
    ytdlpState.value = receiveStatus(ytdlpState.value, await getYTDLPStatus())
  } catch (err) {
    ytdlpState.value = receiveUpdateFailure(ytdlpState.value, err)
  }
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

async function handleYTDLPUpdate() {
  ytdlpState.value = startUpdating(ytdlpState.value)
  try {
    ytdlpState.value = receiveStatus(ytdlpState.value, await updateYTDLP())
  } catch (err) {
    ytdlpState.value = receiveUpdateFailure(ytdlpState.value, err)
  }
}

async function openDirectoryDialog() {
  try {
    const chosen = await chooseDirectory(defaultDir.value)
    if (!chosen) return
    await setDefaultDirectory(chosen)
    defaultDir.value = chosen
    isDefaultDir.value = false
  } catch (err) {
    dirLoadError.value = err.message || String(err)
  }
}

async function handleDownload(payload) {
  downloadError.value = null
  try {
    let outputPath = ''

    if (saveAsMode.value) {
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
      profile: selectedProfile.value
    })
  } catch (err) {
    downloadError.value = err.message || String(err)
  }
}

function clearDownloadError() {
  downloadError.value = null
}
</script>

<template>
  <main class="app-shell">
    <header class="app-header">
      <div class="brand" :aria-label="t('app.brand')">
        <span class="brand-mark" aria-hidden="true">{{ brandInitial }}</span>
        <span>{{ t('app.brand') }}</span>
      </div>

      <label class="locale-switcher" aria-label="Language">
        <span class="sr-only">{{ t('app.language') }}</span>
        <select :value="activeLocale" @change="(e) => setLocale(e.target.value)">
          <option v-for="loc in SUPPORTED_LOCALES" :key="loc.value" :value="loc.value">
            {{ loc.label }}
          </option>
        </select>
      </label>

      <div v-if="isWailsAvailable() && defaultDir" class="settings-toolbar" :aria-label="t('dir.settings')">
        <div class="setting-item dir-setting" aria-live="polite">
          <span class="setting-label">{{ t('dir.label') }}</span>
          <span class="dir-hint-path" :title="defaultDir">{{ defaultDir }}</span>
          <span v-if="isDefaultDir" class="dir-hint-badge">{{ t('dir.defaultBadge') }}</span>
          <button
            type="button"
            class="dir-change-btn"
            @click="openDirectoryDialog"
            :disabled="dirHintLoading"
          >
            {{ t('dir.change') }}
          </button>
        </div>

        <div class="setting-divider" aria-hidden="true"></div>

        <div class="setting-item format-setting">
          <label class="setting-label" for="profile-select">{{ t('dir.outputFormat') }}</label>
          <select id="profile-select" v-model="selectedProfile" class="profile-dropdown">
            <option v-for="opt in profileOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
        </div>

        <div class="setting-divider" aria-hidden="true"></div>

        <label class="setting-item save-as-toggle">
          <input type="checkbox" v-model="saveAsMode" />
          <span>{{ t('dir.saveAsToggle') }}</span>
        </label>
      </div>

      <span v-else class="header-caption">{{ t('app.desktopVideoDownloader') }}</span>
    </header>

    <section class="workspace" aria-labelledby="page-title">
      <div class="intro">
        <p class="eyebrow">{{ t('app.eyebrow') }}</p>
        <h1 id="page-title">{{ t('app.h1') }}</h1>
        <p class="intro-copy">{{ t('app.introCopy') }}</p>
      </div>

      <div v-if="showFirstTimePrompt" class="first-run-banner" role="status">
        <div class="first-run-content">
          <span class="first-run-icon" aria-hidden="true">ⓘ</span>
          <div class="first-run-text">
            <strong>{{ t('dir.firstRunTitle') }}</strong>
            <span>{{ t('dir.firstRunBody', { dir: defaultDir }) }}</span>
          </div>
          <button type="button" class="first-run-btn" @click="openDirectoryDialog">{{ t('dir.choose') }}</button>
        </div>
      </div>

      <div v-if="dirLoadError" class="error-banner" role="alert">
        <span>{{ t('dir.operationFailed', { msg: dirLoadError }) }}</span>
        <button type="button" class="banner-close" @click="dirLoadError = null">×</button>
      </div>

      <div v-if="downloadError" class="error-banner" role="alert">
        <span>{{ t('common.downloadFailed', { msg: downloadError }) }}</span>
        <button type="button" class="banner-close" @click="clearDownloadError">×</button>
      </div>

      <YTDLPStatus
        v-if="isWailsAvailable()"
        :state="ytdlpState"
        @update="handleYTDLPUpdate"
      />

      <AnalyzePanel @download="handleDownload" />

      <TaskList />
    </section>

    <footer class="app-footer">
      <span>{{ t('app.footerScope') }}</span>
      <span>{{ t('app.footerFFmpeg') }}</span>
    </footer>
  </main>
</template>
