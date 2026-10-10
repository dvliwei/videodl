<script setup>
import { computed } from 'vue'
import { formatYTDLPSource, isUpdateDisabled, YTDLP_PHASE } from './status.js'

const props = defineProps({
  state: { type: Object, required: true }
})

const emit = defineEmits(['update'])

const versionText = computed(() => {
  if (!props.state.status?.currentVersion) return '未检测到内置解析器'
  return `当前 ${props.state.status.currentVersion}`
})

const actionText = computed(() => {
  if (props.state.phase === YTDLP_PHASE.CHECKING) return '检查中…'
  if (props.state.phase === YTDLP_PHASE.UPDATING) return '更新中…'
  return '检查并更新 yt-dlp'
})

function handleUpdate() {
  if (!isUpdateDisabled(props.state)) emit('update')
}
</script>

<template>
  <div class="ytdlp-status" aria-live="polite">
    <div class="ytdlp-status-copy">
      <strong>网站解析能力</strong>
      <span>{{ versionText }} · {{ formatYTDLPSource(state.status) }}</span>
      <small>默认不读取浏览器 Cookie；需要时会单独征得授权。</small>
    </div>
    <button
      type="button"
      class="btn-secondary ytdlp-update-btn"
      :disabled="isUpdateDisabled(state)"
      @click="handleUpdate"
    >
      {{ actionText }}
    </button>
    <span v-if="state.error" class="ytdlp-error" role="alert">更新失败，请稍后重试。</span>
  </div>
</template>
