import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime.js'
import * as App from '../../wailsjs/go/main/App.js'

const EVENT_ANALYSIS_UPDATE = 'analysis:update'
const EVENT_TASK_UPDATE = 'task:update'

let analysisListeners = []
let taskListeners = []
let analysisEmitterBound = false
let taskEmitterBound = null

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

function bindAnalysisEmitter() {
  if (analysisEmitterBound) return
  analysisEmitterBound = true
  EventsOn(EVENT_ANALYSIS_UPDATE, (evt) => {
    for (const cb of [...analysisListeners]) cb(evt)
  })
}

function bindTaskEmitter() {
  if (taskEmitterBound) return
  taskEmitterBound = true
  EventsOn(EVENT_TASK_UPDATE, (evt) => {
    for (const cb of [...taskListeners]) cb(evt)
  })
}

export function onAnalysisUpdate(callback) {
  analysisListeners.push(callback)
  bindAnalysisEmitter()
  return () => {
    const i = analysisListeners.indexOf(callback)
    if (i >= 0) analysisListeners.splice(i, 1)
  }
}

export function onTaskUpdate(callback) {
  taskListeners.push(callback)
  bindTaskEmitter()
  return () => {
    const i = taskListeners.indexOf(callback)
    if (i >= 0) taskListeners.splice(i, 1)
  }
}

export function isWailsAvailable() {
  return typeof window !== 'undefined' && window['go'] !== undefined
}
