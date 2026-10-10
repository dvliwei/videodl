export function requestAnalysis(url, browserSession, api) {
  if (browserSession && typeof browserSession.browser === 'string' && browserSession.browser.trim() !== '') {
    return api.analyzeWithBrowserSession(url, browserSession.browser, browserSession.profile || '')
  }
  return api.analyze(url)
}
