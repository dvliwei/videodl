import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime.js'
import * as App from '../../wailsjs/go/main/App.js'

const EVENT_ANALYSIS_UPDATE = 'analysis:update'
const EVENT_TASK_UPDATE = 'task:update'

let analysisListener = null
let taskListener = null

export function analyze(url) {
  return App.Analyze({ url })
}

export function analyzeWithBrowserSession(url, browser, profile = '') {
  return App.AnalyzeWithBrowserSession({ url, browser, profile })
}

export function getYTDLPStatus() {
  return App.GetYTDLPStatus()
}

export function updateYTDLP() {
  return App.UpdateYTDLP()
}

export function cancelAnalysis(id) {
  return App.CancelAnalysis(id)
}

export function getAnalysis(id) {
  return App.GetAnalysis(id)
}

export function startDownload(request) {
  return App.StartDownload(request)
}

export function cancelTask(id) {
  return App.CancelDownload(id)
}

export function retryTask(id) {
  return App.RetryDownload(id)
}

export function listTasks() {
  return App.ListTasks()
}

export function getTask(id) {
  return App.GetTask(id)
}

export function getDefaultDirectory() {
  return App.GetDefaultDirectory()
}

export function setDefaultDirectory(path) {
  return App.SetDefaultDirectory(path)
}

export function chooseDirectory(defaultDir) {
  return App.ChooseDirectory(defaultDir)
}

export function saveAs(defaultName, dirHint) {
  return App.SaveAs(defaultName, dirHint)
}

export function openPath(path) {
  return App.OpenPath(path)
}

export function openContainingFolder(path) {
  return App.OpenContainingFolder(path)
}

export function onAnalysisUpdate(callback) {
  if (analysisListener) {
    EventsOff(EVENT_ANALYSIS_UPDATE)
  }
  analysisListener = EventsOn(EVENT_ANALYSIS_UPDATE, callback)
  return () => {
    EventsOff(EVENT_ANALYSIS_UPDATE)
    analysisListener = null
  }
}

export function onTaskUpdate(callback) {
  if (taskListener) {
    EventsOff(EVENT_TASK_UPDATE)
  }
  taskListener = EventsOn(EVENT_TASK_UPDATE, callback)
  return () => {
    EventsOff(EVENT_TASK_UPDATE)
    taskListener = null
  }
}

export function isWailsAvailable() {
  return typeof window !== 'undefined' && window['go'] !== undefined
}
