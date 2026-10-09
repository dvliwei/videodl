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

export function formatResolution(w, h) {
  if (!w || !h) return null
  return `${w}×${h}`
}

export function formatBandwidth(bps) {
  if (!bps || bps <= 0) return null
  const mbps = bps / 1_000_000
  return `${mbps.toFixed(1)} Mbps`
}
