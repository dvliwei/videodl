const VALID_SCHEMES = ['http:', 'https:']

export function validateUrl(input) {
  const trimmed = input.trim()
  if (!trimmed) return { ok: false, reason: '请输入网页地址' }

  let url
  try {
    url = new URL(trimmed)
  } catch {
    return { ok: false, reason: '地址格式不正确' }
  }

  if (!VALID_SCHEMES.includes(url.protocol)) {
    return { ok: false, reason: '仅支持 http 和 https 地址' }
  }

  if (!url.hostname) {
    return { ok: false, reason: '地址缺少主机名' }
  }

  if (url.username || url.password) {
    return { ok: false, reason: '地址中不应包含用户凭据' }
  }

  return { ok: true, url: trimmed }
}
