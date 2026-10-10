import { ref, computed } from 'vue'
import { messages, SUPPORTED_LOCALES } from './messages.js'

const STORAGE_KEY = 'videodl.locale'
const DEFAULT_LOCALE = 'zh-CN'

// 检测初始语言：优先本地存储，其次系统语言，最后回退简体中文。
function detectLocale() {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored && messages[stored]) return stored
  } catch (e) {
    /* 非浏览器环境忽略 */
  }
  const nav = (typeof navigator !== 'undefined' && navigator.language) || ''
  const parts = nav.toLowerCase().split('-')
  const base = parts[0]
  const region = (parts[1] || '').toUpperCase()
  if (base === 'zh') return region === 'TW' || region === 'HK' || region === 'MO' ? 'zh-TW' : 'zh-CN'
  if (base === 'en') return 'en'
  if (base === 'ja') return 'ja'
  if (base === 'ko') return 'ko'
  return DEFAULT_LOCALE
}

const locale = ref(detectLocale())

const currentMessages = computed(() => messages[locale.value] || messages[DEFAULT_LOCALE])

export function setLocale(value) {
  if (!messages[value]) return
  locale.value = value
  try {
    localStorage.setItem(STORAGE_KEY, value)
  } catch (e) {
    /* 忽略存储失败 */
  }
}

function resolve(obj, path) {
  return path.split('.').reduce((acc, key) => (acc == null ? undefined : acc[key]), obj)
}

// 读取当前语言文案，{name} 占位符用 params 替换。缺键时回退到键名本身。
export function t(key, params) {
  let value = resolve(currentMessages.value, key)
  if (value == null) value = key
  if (params) {
    value = String(value).replace(/\{(\w+)\}/g, (match, name) =>
      params[name] != null ? params[name] : match
    )
  }
  return value
}

// 供模板 v-model 直接使用的响应式语言值。
export const activeLocale = locale

// 后端结构化错误码 → 本地化文案。已知码返回对应翻译；未知码回退到带原始信息的通用文案。
const KNOWN_ERROR_KEYS = {
  'analyzer.invalid_url': 'errors.analyzer.invalid_url',
  'analyzer.address_rejected': 'errors.analyzer.address_rejected',
  'analyzer.too_many_redirects': 'errors.analyzer.too_many_redirects',
  'analyzer.body_too_large': 'errors.analyzer.body_too_large',
  'ytdlp.auth_required': 'errors.ytdlp.auth_required',
  'ytdlp.extractor_unsupported': 'errors.ytdlp.extractor_unsupported',
  'ytdlp.no_formats': 'errors.ytdlp.no_formats'
}

export function errorMessage(code, raw) {
  const key = KNOWN_ERROR_KEYS[code]
  if (key) return t(key)
  if (raw) return t('errors.unknown', { msg: raw })
  return t('analyze.unknownError')
}

export { SUPPORTED_LOCALES, DEFAULT_LOCALE, currentMessages }
