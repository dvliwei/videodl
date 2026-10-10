<script setup>
import { computed } from 'vue'
import { t } from '../../i18n/index.js'
import {
  formatBytes,
  formatSpeed,
  formatETA,
  taskStateLabel,
  ACTIVE_STATES,
  TERMINAL_STATES
} from '../../shared/format.js'
import {
  cancelTask,
  retryTask,
  openPath,
  openContainingFolder
} from '../../shared/wails.js'

const props = defineProps({
  task: { type: Object, required: true },
  showAdvancedActions: { type: Boolean, default: true }
})

const emit = defineEmits(['error'])

const stateLabel = computed(() => taskStateLabel(props.task.state))

const isActive = computed(() => ACTIVE_STATES.has(props.task.state))
const isTerminal = computed(() => TERMINAL_STATES.has(props.task.state))
const isCompleted = computed(() => props.task.state === 'completed')
const isFailed = computed(() => props.task.state === 'failed')
const isCanceled = computed(() => props.task.state === 'canceled')
const isCancelable = computed(() => ACTIVE_STATES.has(props.task.state))
const isRetryable = computed(() => props.task.state === 'failed' || props.task.state === 'canceled')

const progressPercent = computed(() => {
  const p = props.task.progress
  if (p == null || isNaN(p) || !isFinite(p)) return null
  if (p < 0) return null
  if (p > 100) return 100
  return Math.round(p)
})

const sizeDisplay = computed(() => formatBytes(props.task.sizeBytes))
const speedDisplay = computed(() => formatSpeed(props.task.speedBytesPerSecond))

const etaDisplay = computed(() => {
  if (props.task.sizeBytes == null) return null
  if (props.task.progress == null) return null
  const remaining = props.task.sizeBytes * (1 - props.task.progress / 100)
  return formatETA(remaining, props.task.speedBytesPerSecond)
})

const progressBarWidth = computed(() => {
  if (progressPercent.value == null) return '0%'
  return `${progressPercent.value}%`
})

function handleCancel() {
  cancelTask(props.task.id).catch((err) => {
    emit('error', { title: t('task.cancelFailTitle'), message: err.message || String(err) })
  })
}

function handleRetry() {
  retryTask(props.task.id).catch((err) => {
    emit('error', { title: t('task.retryFailTitle'), message: err.message || String(err) })
  })
}

function handleOpenFile() {
  if (!props.task.outputPath) return
  openPath(props.task.outputPath).catch((err) => {
    emit('error', { title: t('task.openFileFailTitle'), message: err.message || String(err) })
  })
}

function handleOpenFolder() {
  if (!props.task.outputPath) return
  openContainingFolder(props.task.outputPath).catch((err) => {
    emit('error', { title: t('task.openFolderFailTitle'), message: err.message || String(err) })
  })
}

function pathLeaf(path) {
  if (!path) return ''
  const idx = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'))
  return idx >= 0 ? path.slice(idx + 1) : path
}

function pathDir(path) {
  if (!path) return ''
  const idx = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'))
  return idx >= 0 ? path.slice(0, idx) : path
}
</script>

<template>
  <article class="task-card" :class="{
    'is-active': isActive,
    'is-completed': isCompleted,
    'is-failed': isFailed,
    'is-canceled': isCanceled
  }">
    <header class="task-header">
      <div class="task-title-wrap">
        <h4 class="task-title" :title="task.title">{{ task.title || t('task.unnamed') }}</h4>
        <span class="task-state-badge" :class="`state-${task.state}`">{{ stateLabel }}</span>
      </div>
      <div class="task-id" :title="task.id">{{ task.id.slice(0, 8) }}</div>
    </header>

    <div class="task-body">
      <template v-if="isActive">
        <div class="progress-track">
          <div class="progress-fill" :style="{ width: progressBarWidth }"></div>
          <div
            v-if="task.progress != null && progressPercent != null"
            class="progress-label"
          >
            {{ progressPercent }}%
          </div>
          <div v-else class="progress-label progress-label--indeterminate">{{ t('task.calculating') }}</div>
        </div>

        <div class="task-stats">
          <template v-if="progressPercent != null">
            <span class="stat">{{ progressPercent }}%</span>
            <span class="stat-sep">·</span>
          </template>
          <span v-if="sizeDisplay" class="stat">{{ sizeDisplay }}</span>
          <template v-if="speedDisplay">
            <span class="stat-sep">·</span>
            <span class="stat">{{ speedDisplay }}</span>
          </template>
          <template v-if="etaDisplay">
            <span class="stat-sep">·</span>
            <span class="stat">{{ t('task.etaPrefix', { eta: etaDisplay }) }}</span>
          </template>
          <template v-if="task.phase && task.phase !== task.state">
            <span class="stat-sep">·</span>
            <span class="stat stat-phase">{{ task.phase }}</span>
          </template>
        </div>
      </template>

      <template v-else-if="isCompleted && task.outputPath">
        <div class="task-output">
          <span class="task-output-label">{{ t('task.savedTo') }}</span>
          <span class="task-output-path" :title="task.outputPath">{{ pathLeaf(task.outputPath) }}</span>
        </div>
        <div class="task-stats">
          <span v-if="sizeDisplay" class="stat">{{ sizeDisplay }}</span>
          <span v-if="task.outputPath" class="stat stat-subtle" :title="pathDir(task.outputPath)">
            {{ pathDir(task.outputPath) }}
          </span>
        </div>
      </template>

      <template v-else-if="(isFailed || isCanceled) && task.errorMessage">
        <div class="task-error">
          <span class="task-error-icon" aria-hidden="true">!</span>
          <span class="task-error-text">{{ task.errorMessage }}</span>
        </div>
      </template>

      <template v-else-if="isFailed || isCanceled">
        <div class="task-error task-error--empty">
          <span class="task-error-icon" aria-hidden="true">!</span>
          <span class="task-error-text">{{ isCanceled ? t('task.canceled') : t('task.failed') }}</span>
        </div>
      </template>
    </div>

    <footer class="task-actions">
      <button
        v-if="isCancelable"
        type="button"
        class="task-btn task-btn--secondary"
        @click="handleCancel"
      >
        {{ t('common.cancel') }}
      </button>
      <button
        v-if="isRetryable"
        type="button"
        class="task-btn task-btn--primary"
        @click="handleRetry"
      >
        {{ t('common.retry') }}
      </button>
      <button
        v-if="isCompleted && task.outputPath"
        type="button"
        class="task-btn task-btn--secondary"
        @click="handleOpenFile"
      >
        {{ t('task.openFile') }}
      </button>
      <button
        v-if="isCompleted && task.outputPath"
        type="button"
        class="task-btn task-btn--secondary"
        @click="handleOpenFolder"
      >
        {{ t('task.openFolder') }}
      </button>
    </footer>
  </article>
</template>
