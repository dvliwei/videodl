export const YTDLP_PHASE = Object.freeze({
  IDLE: 'idle',
  CHECKING: 'checking',
  UPDATING: 'updating',
  READY: 'ready',
  UPDATE_FAILED: 'update-failed',
  AUTH_REQUIRED: 'auth-required',
  CANCELED: 'canceled',
  EMPTY: 'empty',
  ANALYSIS_FAILED: 'analysis-failed'
})

export function createYTDLPState(status = null) {
  return { phase: status ? YTDLP_PHASE.READY : YTDLP_PHASE.IDLE, status, error: null }
}

export function startChecking(state) {
  return { ...state, phase: YTDLP_PHASE.CHECKING, error: null }
}

export function startUpdating(state) {
  return { ...state, phase: YTDLP_PHASE.UPDATING, error: null }
}

export function receiveStatus(state, status) {
  return {
    ...state,
    phase: status?.available ? YTDLP_PHASE.READY : YTDLP_PHASE.EMPTY,
    status: status || null,
    error: null
  }
}

export function receiveUpdateFailure(state, error) {
  return { ...state, phase: YTDLP_PHASE.UPDATE_FAILED, error }
}

export function isUpdateDisabled(state) {
  return state.phase === YTDLP_PHASE.CHECKING || state.phase === YTDLP_PHASE.UPDATING
}

export function formatYTDLPSource(status) {
  switch (status?.source) {
    case 'bundled': return '随应用提供'
    case 'user-update': return '用户更新'
    default: return '不可用'
  }
}

export function createBrowserAuthorization(state) {
  return { ...state, phase: YTDLP_PHASE.AUTH_REQUIRED, error: null }
}

export function cancelBrowserAuthorization(state) {
  return { ...state, phase: YTDLP_PHASE.CANCELED, error: null }
}

export function shouldUseBrowserSession(state) {
  return state.phase === YTDLP_PHASE.AUTH_REQUIRED && state.browserAuthorized === true
}
