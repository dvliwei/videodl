<script setup>
import { computed } from 'vue'
import { formatDuration, formatBytes, formatResolution, formatBandwidth } from '../../shared/format.js'

const props = defineProps({
  candidate: { type: Object, required: true },
  analysisId: { type: String, required: true },
  selectedVariantId: { type: String, default: '' }
})

const emit = defineEmits(['download', 'select-variant'])

const resolution = computed(() => formatResolution(props.candidate.width, props.candidate.height))
const duration = computed(() => formatDuration(props.candidate.durationSeconds))
const size = computed(() => formatBytes(props.candidate.sizeBytes))

const sourceTypeLabel = computed(() => {
  switch (props.candidate.sourceType) {
    case 'hls': return 'HLS'
    case 'dash': return 'DASH'
    case 'direct': return '直链'
    default: return props.candidate.sourceType || ''
  }
})

const audioLabel = computed(() => {
  if (!props.candidate.hasAudio) return null
  return '有音频'
})

const videoLabel = computed(() => {
  if (!props.candidate.hasVideo) return null
  return '有视频'
})

const unsupportedReason = computed(() => props.candidate.unsupported || '')

const hasVariants = computed(() =>
  props.candidate.variants && props.candidate.variants.length > 0
)

function getVariantLabel(v) {
  const parts = []
  if (v.width && v.height) {
    parts.push(formatResolution(v.width, v.height))
  } else if (v.label) {
    parts.push(v.label)
  }
  if (v.bandwidth) {
    parts.push(formatBandwidth(v.bandwidth))
  }
  if (parts.length === 0 && v.label) return v.label
  return parts.join(' · ') || '默认'
}

const effectiveVariantId = computed(() => {
  if (props.selectedVariantId) return props.selectedVariantId
  if (props.candidate.sourceType === 'yt-dlp') return ''
  if (hasVariants.value && props.candidate.variants[0]) {
    return props.candidate.variants[0].id || ''
  }
  return ''
})

function handleDownload() {
  emit('download', {
    analysisId: props.analysisId,
    mediaId: props.candidate.id,
    variantId: effectiveVariantId.value,
    title: props.candidate.title || ''
  })
}

function handleSelectVariant(variantId) {
  emit('select-variant', variantId)
}
</script>

<template>
  <article class="media-card" :class="{ 'is-unsupported': unsupportedReason }">
    <header class="card-header">
      <div class="title-wrap">
        <h3 class="card-title" :title="candidate.title">{{ candidate.title || '未命名资源' }}</h3>
        <span class="source-badge">{{ sourceTypeLabel }}</span>
      </div>
      <div class="display-url" v-if="candidate.displayUrl" :title="candidate.displayUrl">
        {{ candidate.displayUrl }}
      </div>
    </header>

    <div class="card-body">
      <div v-if="unsupportedReason" class="unsupported-notice">
        <span class="unsupported-icon">!</span>
        <span>{{ unsupportedReason }}</span>
      </div>

      <template v-else>
        <div class="meta-grid">
          <div class="meta-item">
            <span class="meta-label">分辨率</span>
            <span class="meta-value" :class="{ 'is-unknown': !resolution }">
              {{ resolution || '未知' }}
            </span>
          </div>
          <div class="meta-item">
            <span class="meta-label">格式</span>
            <span class="meta-value" :class="{ 'is-unknown': !candidate.format }">
              {{ candidate.format || '未知' }}
            </span>
          </div>
          <div class="meta-item">
            <span class="meta-label">时长</span>
            <span class="meta-value" :class="{ 'is-unknown': !duration }">
              {{ duration || '未知' }}
            </span>
          </div>
          <div class="meta-item">
            <span class="meta-label">大小</span>
            <span class="meta-value" :class="{ 'is-unknown': !size }">
              {{ size || '未知' }}
            </span>
          </div>
        </div>

        <div class="stream-info">
          <span v-if="videoLabel" class="stream-tag is-video">{{ videoLabel }}</span>
          <span v-if="audioLabel" class="stream-tag is-audio">{{ audioLabel }}</span>
          <span v-if="!videoLabel && !audioLabel" class="stream-tag is-unknown">流信息未知</span>
        </div>

        <div v-if="hasVariants" class="variants-block">
          <label class="variants-label">清晰度 / 变体</label>
          <div class="variants-list">
            <button
              v-for="(v, i) in candidate.variants"
              :key="v.id || i"
              type="button"
              class="variant-chip"
              :class="{ 'is-selected': effectiveVariantId === (v.id || '') || (!effectiveVariantId && i === 0) }"
              @click="handleSelectVariant(v.id)"
            >
              {{ getVariantLabel(v) }}
            </button>
          </div>
        </div>
      </template>
    </div>

    <footer class="card-footer">
      <button
        class="btn-download"
        type="button"
        :disabled="!!unsupportedReason"
        @click="handleDownload"
      >
        下载
      </button>
    </footer>
  </article>
</template>
