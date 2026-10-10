import { t } from '../i18n/index.js'

export function formatDuration(seconds) {
  if (seconds == null || isNaN(seconds) || !isFinite(seconds)) return null
  const s = Math.round(seconds)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  if (h > 0) {
    return `${h}:${String(m).padStart(2, '0')}:${String(sec).padStart(2, '0')}`
  }
  return `${m}:${String(sec).padStart(2, '0')}`
}

export function formatBytes(bytes) {
  if (bytes == null || bytes <= 0) return null
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let val = bytes
  while (val >= 1024 && i < units.length - 1) {
    val /= 1024
    i++
  }
  const digits = val >= 100 || i === 0 ? 0 : val >= 10 ? 1 : 2
  return `${val.toFixed(digits)} ${units[i]}`
}

export function formatSpeed(bytesPerSecond) {
  if (!bytesPerSecond || bytesPerSecond <= 0) return null
  const formatted = formatBytes(bytesPerSecond)
  return formatted ? `${formatted}/s` : null
}

export function formatResolution(w, h) {
  if (!w || !h) return null
  return `${w}×${h}`
}

export function formatBandwidth(bps) {
  if (!bps || bps <= 0) return null
  const mbps = bps / 1_000_000
  return `${mbps.toFixed(1)} Mbps`
}

export function formatETA(remainingBytes, bytesPerSecond) {
  if (!remainingBytes || !bytesPerSecond || bytesPerSecond <= 0) return null
  const seconds = remainingBytes / bytesPerSecond
  if (seconds < 60) {
    return `${Math.max(1, Math.round(seconds))}s`
  }
  const m = Math.floor(seconds / 60)
  const s = Math.round(seconds % 60)
  if (m < 60) {
    return `${m}m ${String(s).padStart(2, '0')}s`
  }
  const h = Math.floor(m / 60)
  const rem = m % 60
  return `${h}h ${rem}m`
}

export function isIndeterminateProgress(progress) {
  return progress == null || isNaN(progress) || !isFinite(progress)
}

// 任务状态到本地化标签。旧的对象常量改为函数，随当前语言实时更新。
const STATE_KEYS = {
  queued: 'task.state.queued',
  preparing: 'task.state.preparing',
  downloading: 'task.state.downloading',
  merging: 'task.state.merging',
  transcoding: 'task.state.transcoding',
  completed: 'task.state.completed',
  canceled: 'task.state.canceled',
  failed: 'task.state.failed'
}

export function taskStateLabel(state) {
  const key = STATE_KEYS[state]
  return key ? t(key) : state
}

export const ACTIVE_STATES = new Set(['queued', 'preparing', 'downloading', 'merging', 'transcoding'])
export const TERMINAL_STATES = new Set(['completed', 'canceled', 'failed'])
export const PROGRESS_STATES = new Set(['preparing', 'downloading', 'merging', 'transcoding'])
