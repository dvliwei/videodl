<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { t } from '../../i18n/index.js'
import TaskCard from './TaskCard.vue'
import {
  listTasks,
  onTaskUpdate,
  isWailsAvailable
} from '../../shared/wails.js'
import {
  ACTIVE_STATES,
  TERMINAL_STATES
} from '../../shared/format.js'

const tasks = ref([])
const loadError = ref(null)
const taskErrors = ref([])

let unwatchTask = null
let loaded = false

const sortedTasks = computed(() => {
  const arr = [...tasks.value]
  arr.sort((a, b) => {
    const aActive = ACTIVE_STATES.has(a.state) ? 0 : 1
    const bActive = ACTIVE_STATES.has(b.state) ? 0 : 1
    if (aActive !== bActive) return aActive - bActive
    return new Date(b.createdAt) - new Date(a.createdAt)
  })
  return arr
})

const activeCount = computed(() =>
  tasks.value.filter((t) => ACTIVE_STATES.has(t.state)).length
)
const terminalCount = computed(() =>
  tasks.value.filter((t) => TERMINAL_STATES.has(t.state)).length
)
const completedCount = computed(() =>
  tasks.value.filter((t) => t.state === 'completed').length
)
const failedCount = computed(() =>
  tasks.value.filter((t) => t.state === 'failed').length
)

const hasTasks = computed(() => tasks.value.length > 0)

function upsertTask(task) {
  const idx = tasks.value.findIndex((t) => t.id === task.id)
  if (idx >= 0) {
    tasks.value[idx] = task
  } else {
    tasks.value.push(task)
  }
}

function handleTaskError(err) {
  if (!err) return
  const id = `${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
  taskErrors.value.push({ ...err, id })
  setTimeout(() => {
    const i = taskErrors.value.findIndex((e) => e.id === id)
    if (i >= 0) taskErrors.value.splice(i, 1)
  }, 4000)
}

async function loadInitialTasks() {
  if (!isWailsAvailable()) {
    loaded = true
    return
  }
  try {
    const snapshots = await listTasks()
    tasks.value = snapshots || []
    loaded = true
    loadError.value = null
  } catch (err) {
    loaded = true
    loadError.value = err.message || String(err)
  }
}

onMounted(() => {
  if (!isWailsAvailable()) return
  loadInitialTasks()
  unwatchTask = onTaskUpdate((evt) => {
    if (!evt || !evt.task) return
    upsertTask(evt.task)
    loaded = true
  })
})

onUnmounted(() => {
  if (unwatchTask) unwatchTask()
})

defineEmits(['directory-needed'])
</script>

<template>
  <section class="panel tasks-panel" aria-labelledby="tasks-title">
    <div class="panel-heading">
      <div>
        <h2 id="tasks-title">{{ t('task.title') }}</h2>
        <template v-if="hasTasks">
          <p>
            {{ t('task.activeSummary', { active: activeCount, completed: completedCount }) }}
            <template v-if="failedCount > 0"> {{ t('task.failedCount', { failed: failedCount }) }}</template>
          </p>
        </template>
        <template v-else-if="loaded">
          <p>{{ t('task.none') }}</p>
        </template>
      </div>
      <span v-if="hasTasks" class="panel-count">{{ tasks.length }}</span>
    </div>

    <div class="panel-body task-list-body">
      <template v-if="!loaded && !hasTasks">
        <div class="empty-state">
          <span class="empty-icon" aria-hidden="true">⬇</span>
          <strong>{{ t('task.waitTitle') }}</strong>
          <span>{{ t('task.waitHint') }}</span>
        </div>
      </template>

      <template v-else-if="loadError">
        <div class="empty-state">
          <span class="empty-icon" aria-hidden="true">!</span>
          <strong>{{ t('task.loadFailed') }}</strong>
          <span>{{ loadError }}</span>
        </div>
      </template>

      <template v-else-if="hasTasks">
        <div class="task-list">
          <TaskCard
            v-for="task in sortedTasks"
            :key="task.id"
            :task="task"
            @error="handleTaskError"
          />
        </div>
      </template>

      <div v-for="err in taskErrors" :key="err.id" class="toast-error">
        <strong>{{ err.title }}</strong>
        <span>{{ err.message }}</span>
      </div>
    </div>
  </section>
</template>
