const VALID_SCHEMES = ['http:', 'https:']

// 返回的 reason 是 i18n 键（见 validate.*），由调用方用 t() 渲染为当前语言文案。
export function validateUrl(input) {
  const trimmed = input.trim()
  if (!trimmed) return { ok: false, reason: 'validate.enterUrl' }

  let url
  try {
    url = new URL(trimmed)
  } catch {
    return { ok: false, reason: 'validate.invalidFormat' }
  }

  if (!VALID_SCHEMES.includes(url.protocol)) {
    return { ok: false, reason: 'validate.schemeOnly' }
  }

  if (!url.hostname) {
    return { ok: false, reason: 'validate.noHost' }
  }

  if (url.username || url.password) {
    return { ok: false, reason: 'validate.noCredential' }
  }

  return { ok: true, url: trimmed }
}
