<script setup>
import { computed } from 'vue'
import { t } from '../../i18n/index.js'
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
    case 'direct': return t('media.direct')
    default: return props.candidate.sourceType || ''
  }
})

const audioLabel = computed(() => {
  if (!props.candidate.hasAudio) return null
  return t('media.hasAudio')
})

const videoLabel = computed(() => {
  if (!props.candidate.hasVideo) return null
  return t('media.hasVideo')
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
  return parts.join(' · ') || t('media.defaultVariant')
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
        <h3 class="card-title" :title="candidate.title">{{ candidate.title || t('media.unnamed') }}</h3>
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
            <span class="meta-label">{{ t('media.resolution') }}</span>
            <span class="meta-value" :class="{ 'is-unknown': !resolution }">
              {{ resolution || t('common.unknown') }}
            </span>
          </div>
          <div class="meta-item">
            <span class="meta-label">{{ t('media.format') }}</span>
            <span class="meta-value" :class="{ 'is-unknown': !candidate.format }">
              {{ candidate.format || t('common.unknown') }}
            </span>
          </div>
          <div class="meta-item">
            <span class="meta-label">{{ t('media.duration') }}</span>
            <span class="meta-value" :class="{ 'is-unknown': !duration }">
              {{ duration || t('common.unknown') }}
            </span>
          </div>
          <div class="meta-item">
            <span class="meta-label">{{ t('media.size') }}</span>
            <span class="meta-value" :class="{ 'is-unknown': !size }">
              {{ size || t('common.unknown') }}
            </span>
          </div>
        </div>

        <div class="stream-info">
          <span v-if="videoLabel" class="stream-tag is-video">{{ videoLabel }}</span>
          <span v-if="audioLabel" class="stream-tag is-audio">{{ audioLabel }}</span>
          <span v-if="!videoLabel && !audioLabel" class="stream-tag is-unknown">{{ t('media.streamUnknown') }}</span>
        </div>

        <div v-if="hasVariants" class="variants-block">
          <label class="variants-label">{{ t('media.variants') }}</label>
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
        {{ t('common.download') }}
      </button>
    </footer>
  </article>
</template>
