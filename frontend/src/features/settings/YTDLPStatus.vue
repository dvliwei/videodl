<script setup>
import { computed } from 'vue'
import { t } from '../../i18n/index.js'
import { formatYTDLPSource, isUpdateDisabled, YTDLP_PHASE } from './status.js'

const props = defineProps({
  state: { type: Object, required: true }
})

const emit = defineEmits(['update'])

const versionText = computed(() => {
  if (!props.state.status?.currentVersion) return t('ytdlp.notDetected')
  return t('ytdlp.current', { version: props.state.status.currentVersion })
})

const actionText = computed(() => {
  if (props.state.phase === YTDLP_PHASE.CHECKING) return t('ytdlp.checking')
  if (props.state.phase === YTDLP_PHASE.UPDATING) return t('ytdlp.updating')
  return t('ytdlp.checkUpdate')
})

function handleUpdate() {
  if (!isUpdateDisabled(props.state)) emit('update')
}
</script>

<template>
  <div class="ytdlp-status" aria-live="polite">
    <div class="ytdlp-status-copy">
      <strong>{{ t('ytdlp.title') }}</strong>
      <span>{{ versionText }} · {{ formatYTDLPSource(state.status) }}</span>
      <small>{{ t('ytdlp.privacy') }}</small>
    </div>
    <button
      type="button"
      class="btn-secondary ytdlp-update-btn"
      :disabled="isUpdateDisabled(state)"
      @click="handleUpdate"
    >
      {{ actionText }}
    </button>
    <span v-if="state.error" class="ytdlp-error" role="alert">{{ t('ytdlp.updateFailed') }}</span>
  </div>
</template>
