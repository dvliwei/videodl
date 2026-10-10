import assert from 'node:assert/strict'
import {
  YTDLP_PHASE,
  createYTDLPState,
  receiveStatus,
  startChecking,
  startUpdating,
  receiveUpdateFailure,
  isUpdateDisabled,
  formatYTDLPSource,
  createBrowserAuthorization,
  cancelBrowserAuthorization,
  shouldUseBrowserSession
} from './status.js'
import { requestAnalysis } from '../analyze/request.js'

function test(name, fn) {
  try {
    fn()
    console.log(`ok - ${name}`)
  } catch (error) {
    console.error(`not ok - ${name}`)
    throw error
  }
}

test('update button is disabled while checking or updating', () => {
  const idle = createYTDLPState()
  assert.equal(isUpdateDisabled(idle), false)
  assert.equal(isUpdateDisabled(startChecking(idle)), true)
  assert.equal(isUpdateDisabled(startUpdating(receiveStatus(idle, {
    available: true,
    canUpdate: true,
    source: 'bundled'
  }))), true)
})

test('status keeps source and renders a concise label', () => {
  const state = receiveStatus(createYTDLPState(), {
    available: true,
    currentVersion: '2026.08.19',
    latestVersion: '2026.08.19',
    source: 'user-update',
    canUpdate: false
  })
  assert.equal(state.phase, YTDLP_PHASE.READY)
  assert.equal(state.status.source, 'user-update')
  assert.equal(formatYTDLPSource(state.status), '用户更新')
})

test('update failure retains the last known status', () => {
  const state = receiveStatus(createYTDLPState(), {
    available: true,
    currentVersion: '2026.08.19',
    source: 'bundled',
    canUpdate: true
  })
  const failed = receiveUpdateFailure(startUpdating(state), new Error('network'))
  assert.equal(failed.phase, YTDLP_PHASE.UPDATE_FAILED)
  assert.equal(failed.status.currentVersion, '2026.08.19')
  assert.equal(failed.error.message, 'network')
})

test('browser authorization is opt-in and cancel does not start it', () => {
  const idle = createYTDLPState()
  const prompt = createBrowserAuthorization(idle)
  assert.equal(prompt.phase, YTDLP_PHASE.AUTH_REQUIRED)
  assert.equal(shouldUseBrowserSession(prompt), false)
  const canceled = cancelBrowserAuthorization(prompt)
  assert.equal(canceled.phase, YTDLP_PHASE.CANCELED)
  assert.equal(shouldUseBrowserSession(canceled), false)
})

test('default analysis never opts into a browser session', () => {
  assert.equal(shouldUseBrowserSession(createYTDLPState()), false)
})

test('form submission event does not opt into a browser session', () => {
  const calls = []
  const api = {
    analyze: (url) => calls.push(['default', url]),
    analyzeWithBrowserSession: (...args) => calls.push(['browser', ...args])
  }

  requestAnalysis('https://www.youtube.com/watch?v=example', { type: 'submit' }, api)

  assert.deepEqual(calls, [['default', 'https://www.youtube.com/watch?v=example']])
})

test('explicit browser session uses the authorized analysis path', () => {
  const calls = []
  const api = {
    analyze: (url) => calls.push(['default', url]),
    analyzeWithBrowserSession: (...args) => calls.push(['browser', ...args])
  }

  requestAnalysis('https://www.youtube.com/watch?v=example', { browser: 'chrome', profile: 'Default' }, api)

  assert.deepEqual(calls, [['browser', 'https://www.youtube.com/watch?v=example', 'chrome', 'Default']])
})
