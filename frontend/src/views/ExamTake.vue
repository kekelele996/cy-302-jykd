<template>
  <div class="exam-page no-select" @contextmenu.prevent>
    <header class="exam-header">
      <div class="exam-title">{{ paper.title }}</div>
      <div class="exam-countdown">剩余时间：{{ countdownText }}</div>
      <el-button type="warning" @click="onSubmit">交卷</el-button>
    </header>

    <div class="exam-body">
      <main class="question-panel" v-if="current">
        <div class="question-head">
          <span class="question-no">第 {{ currentIndex + 1 }} / {{ paper.questions.length }} 题</span>
          <el-tag>{{ typeLabel(current.type) }}</el-tag>
          <el-tag type="info">分值 {{ current.score }}</el-tag>
          <el-button link type="warning" @click="toggleMark">{{ current.marked ? '取消标记' : '标记疑问' }}</el-button>
        </div>
        <div class="question-content answer-text">{{ current.content }}</div>

        <div class="question-answer">
          <template v-if="current.type === 'single'">
            <el-radio-group v-model="answerValue" @change="onAnswerChange" style="display: flex; flex-direction: column; gap: 8px">
              <el-radio v-for="opt in current.options" :key="opt.key" :value="opt.key">{{ opt.key }}. {{ opt.text }}</el-radio>
            </el-radio-group>
          </template>
          <template v-else-if="current.type === 'multiple'">
            <el-checkbox-group v-model="answerMultiple" @change="onAnswerChange" style="display: flex; flex-direction: column; gap: 8px">
              <el-checkbox v-for="opt in current.options" :key="opt.key" :value="opt.key">{{ opt.key }}. {{ opt.text }}</el-checkbox>
            </el-checkbox-group>
          </template>
          <template v-else-if="current.type === 'true_false'">
            <el-radio-group v-model="answerValue" @change="onAnswerChange">
              <el-radio value="T">正确</el-radio>
              <el-radio value="F">错误</el-radio>
            </el-radio-group>
          </template>
          <template v-else-if="current.type === 'fill_blank'">
            <el-input v-model="answerText" type="textarea" :rows="4" placeholder="每空一行填写答案" @change="onAnswerChange" />
          </template>
          <template v-else>
            <el-input v-model="answerText" type="textarea" :rows="5" placeholder="请输入答案" @change="onAnswerChange" />
          </template>
        </div>

        <div class="question-nav">
          <el-button :disabled="currentIndex === 0" @click="go(currentIndex - 1)">上一题</el-button>
          <el-button :disabled="currentIndex === paper.questions.length - 1" @click="go(currentIndex + 1)">下一题</el-button>
        </div>
      </main>

      <aside class="answer-sheet">
        <div class="sheet-title">答题卡</div>
        <div class="sheet-grid">
          <button
            v-for="(q, idx) in paper.questions"
            :key="q.exam_question_id"
            class="sheet-item"
            :class="{ answered: isAnswered(q.exam_question_id), marked: q.marked, current: idx === currentIndex }"
            @click="go(idx)"
          >
            {{ idx + 1 }}
          </button>
        </div>
        <div class="sheet-legend">
          <span><i class="dot answered"></i>已答</span>
          <span><i class="dot marked"></i>标记</span>
          <span><i class="dot current"></i>当前</span>
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { attemptApi } from '../api'
import type { AttemptStartResponse, ExamQuestionView, QuestionType } from '../types'

const route = useRoute()
const router = useRouter()
const examId = Number(route.params.id)
const paper = reactive<AttemptStartResponse>({
  attempt_id: 0,
  exam_id: examId,
  title: '',
  duration_minutes: 0,
  total_score: 0,
  started_at: '',
  deadline: '',
  questions: []
})
const currentIndex = ref(0)
const answers = reactive<Record<number, unknown>>({})
const markedMap = reactive<Record<number, boolean>>({})
const countdownText = ref('--:--:--')
let timer: number | undefined
let submitted = false

const current = computed(() => paper.questions[currentIndex.value] || null)

const answerValue = computed({
  get: () => {
    if (!current.value) return ''
    const val = answers[current.value.exam_question_id]
    return val as string
  },
  set: (v: string) => {
    if (current.value) answers[current.value.exam_question_id] = v
  }
})

const answerMultiple = computed({
  get: () => {
    if (!current.value) return []
    const val = answers[current.value.exam_question_id]
    return Array.isArray(val) ? (val as string[]) : []
  },
  set: (v: string[]) => {
    if (current.value) answers[current.value.exam_question_id] = v
  }
})

const answerText = computed({
  get: () => {
    if (!current.value) return ''
    const val = answers[current.value.exam_question_id]
    if (Array.isArray(val)) return (val as string[]).join('\n')
    return (val as string) || ''
  },
  set: (v: string) => {
    if (!current.value) {
      return
    }
    if (current.value.type === 'fill_blank') {
      answers[current.value.exam_question_id] = v.split('\n')
    } else {
      answers[current.value.exam_question_id] = v
    }
  }
})

function typeLabel(type: QuestionType) {
  const map: Record<string, string> = { single: '单选题', multiple: '多选题', true_false: '判断题', fill_blank: '填空题', short_answer: '简答题' }
  return map[type] || type
}

function isAnswered(id: number) {
  const val = answers[id]
  if (val === undefined || val === null || val === '') return false
  if (Array.isArray(val)) return val.length > 0
  return true
}

function go(idx: number) {
  if (idx < 0 || idx >= paper.questions.length) return
  currentIndex.value = idx
}

function buildAnswer(id: number) {
  return answers[id] ?? ''
}

function toggleMark() {
  if (!current.value) return
  current.value.marked = !current.value.marked
  markedMap[current.value.exam_question_id] = current.value.marked
  saveAnswer(current.value.exam_question_id, { marked: current.value.marked })
}

async function saveAnswer(examQuestionId: number, extra?: { marked?: boolean }) {
  if (submitted || !paper.attempt_id) return
  const val = answers[examQuestionId]
  try {
    await attemptApi.saveAnswer(paper.attempt_id, {
      exam_question_id: examQuestionId,
      answer: val,
      marked: extra?.marked ?? markedMap[examQuestionId] ?? false
    })
  } catch {
    // ignore save errors during exam
  }
}

async function onAnswerChange() {
  if (!current.value) return
  markedMap[current.value.exam_question_id] = current.value.marked
  await saveAnswer(current.value.exam_question_id)
}

async function onSubmit() {
  if (submitted) return
  await ElMessageBox.confirm('确认交卷？交卷后不可修改。', '提示', { type: 'warning' })
  await doSubmit()
}

async function doSubmit() {
  if (submitted || !paper.attempt_id) return
  submitted = true
  try {
    await attemptApi.submit(paper.attempt_id)
    ElMessage.success('交卷成功')
  } finally {
    exitFullscreen()
    router.replace(`/report/${paper.attempt_id}`)
  }
}

async function enterFullscreen() {
  try {
    await document.documentElement.requestFullscreen()
  } catch {
    // fullscreen may be blocked
  }
}

function exitFullscreen() {
  if (document.fullscreenElement) {
    document.exitFullscreen().catch(() => undefined)
  }
}

function onFullscreenChange() {
  if (!document.fullscreenElement && !submitted) {
    ElMessage.warning('考试已锁定全屏，请勿退出全屏（按 ESC 视为警告）')
  }
}

function tick() {
  const remain = new Date(paper.deadline).getTime() - Date.now()
  if (remain <= 0) {
    countdownText.value = '00:00:00'
    doSubmit()
    return
  }
  const total = Math.floor(remain / 1000)
  const h = String(Math.floor(total / 3600)).padStart(2, '0')
  const m = String(Math.floor((total % 3600) / 60)).padStart(2, '0')
  const s = String(total % 60).padStart(2, '0')
  countdownText.value = `${h}:${m}:${s}`
}

async function load() {
  const res = await attemptApi.start(examId)
  Object.assign(paper, res)
  res.questions.forEach((q: ExamQuestionView) => {
    if (q.answer !== undefined && q.answer !== null && q.answer !== '') {
      answers[q.exam_question_id] = q.answer
    }
    markedMap[q.exam_question_id] = q.marked
  })
  await enterFullscreen()
  timer = window.setInterval(tick, 1000)
}

onMounted(() => {
  document.addEventListener('fullscreenchange', onFullscreenChange)
  document.addEventListener('selectstart', preventSelect)
  window.addEventListener('beforeunload', onBeforeUnload)
  load().catch(() => router.replace('/exams'))
})

onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
  document.removeEventListener('fullscreenchange', onFullscreenChange)
  document.removeEventListener('selectstart', preventSelect)
  window.removeEventListener('beforeunload', onBeforeUnload)
})

function preventSelect(e: Event) {
  if (!(e.target as HTMLElement).closest('input, textarea')) {
    e.preventDefault()
  }
}

function onBeforeUnload() {
  if (paper.attempt_id && !submitted) {
    // 刷新/关闭页面视为交卷
    fetch(`/api/v1/attempts/${paper.attempt_id}/submit`, {
      method: 'POST',
      keepalive: true,
      headers: { Authorization: `Bearer ${localStorage.getItem('gbexam_token') || ''}` }
    })
  }
}

watch(currentIndex, async (idx) => {
  const q = paper.questions[idx]
  if (q && !q.marked) {
    markedMap[q.exam_question_id] = false
  }
})
</script>

<style scoped>
.exam-page {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: #f5f7fa;
}
.exam-header {
  height: 60px;
  background: #001529;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
}
.exam-title {
  font-size: 18px;
  font-weight: 600;
}
.exam-countdown {
  font-size: 20px;
  font-weight: 700;
  color: #ffd666;
}
.exam-body {
  flex: 1;
  display: flex;
  overflow: hidden;
}
.question-panel {
  flex: 1;
  padding: 24px;
  background: #fff;
  margin: 16px;
  border-radius: 8px;
  overflow: auto;
}
.question-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}
.question-no {
  font-weight: 600;
}
.question-content {
  font-size: 16px;
  margin-bottom: 24px;
}
.question-answer {
  margin-bottom: 24px;
}
.question-nav {
  display: flex;
  gap: 12px;
  justify-content: center;
  margin-top: 30px;
}
.answer-sheet {
  width: 260px;
  background: #fff;
  margin: 16px 16px 16px 0;
  border-radius: 8px;
  padding: 16px;
}
.sheet-title {
  font-weight: 600;
  margin-bottom: 12px;
}
.sheet-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 8px;
}
.sheet-item {
  height: 36px;
  border: 1px solid #dcdfe6;
  background: #fff;
  border-radius: 4px;
  cursor: pointer;
}
.sheet-item.answered {
  background: #1890ff;
  color: #fff;
  border-color: #1890ff;
}
.sheet-item.marked {
  outline: 2px solid #faad14;
}
.sheet-item.current {
  box-shadow: 0 0 0 2px #52c41a inset;
}
.sheet-legend {
  margin-top: 14px;
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  color: #606266;
  font-size: 13px;
}
.dot {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 2px;
  margin-right: 4px;
}
.dot.answered {
  background: #1890ff;
}
.dot.marked {
  background: #faad14;
}
.dot.current {
  box-shadow: 0 0 0 2px #52c41a inset;
}
</style>
