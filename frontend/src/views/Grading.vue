<template>
  <div class="page-card">
    <div class="page-header">
      <h2>主观题批改</h2>
      <el-button @click="$router.push('/exams')">返回</el-button>
    </div>

    <div class="toolbar">
      <span>选择答题记录：</span>
      <el-select v-model="attemptId" placeholder="请选择" style="width: 320px" @change="loadDetail">
        <el-option v-for="a in attempts" :key="a.attempt_id" :label="`#${a.attempt_id} 客观分 ${a.objective_score}`" :value="a.attempt_id" />
      </el-select>
    </div>

    <template v-if="detail">
      <el-table :data="subjectiveQuestions" border>
        <el-table-column prop="content" label="题干" min-width="220" show-overflow-tooltip />
        <el-table-column label="学生答案" min-width="220">
          <template #default="{ row }">
            <div class="answer-text">{{ formatAnswer(row.student_answer) }}</div>
          </template>
        </el-table-column>
        <el-table-column label="参考答案" min-width="200">
          <template #default="{ row }">
            <div class="answer-text">{{ formatAnswer(row.correct_answer) }}</div>
          </template>
        </el-table-column>
        <el-table-column label="得分" width="160">
          <template #default="{ row }">
            <el-input-number v-model="scores[row.exam_question_id]" :min="0" :max="row.max_score" :step="0.5" />
          </template>
        </el-table-column>
      </el-table>
      <el-button v-if="subjectiveQuestions.length" type="primary" style="margin-top: 16px" :loading="saving" @click="onSubmit">保存批改</el-button>
      <el-empty v-else description="该试卷没有主观题" />
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { attemptApi, examApi } from '../api'
import type { AttemptDetail, AttemptSummary } from '../types'

const route = useRoute()
const examId = Number(route.params.examId)
const attempts = ref<AttemptSummary[]>([])
const attemptId = ref<number | null>(null)
const detail = ref<AttemptDetail | null>(null)
const scores = reactive<Record<number, number>>({})
const saving = ref(false)

const subjectiveQuestions = computed(() => {
  if (!detail.value) return []
  return detail.value.questions.filter((q) => q.type === 'fill_blank' || q.type === 'short_answer')
})

function formatAnswer(v: unknown) {
  if (v === null || v === undefined) return '-'
  if (Array.isArray(v)) return v.join('；')
  return String(v)
}

async function loadDetail() {
  if (!attemptId.value) return
  detail.value = await attemptApi.detail(attemptId.value)
  detail.value.questions.forEach((q) => {
    if (q.type === 'fill_blank' || q.type === 'short_answer') {
      scores[q.exam_question_id] = q.score || 0
    }
  })
}

async function onSubmit() {
  if (!attemptId.value) return
  saving.value = true
  try {
    const items = Object.entries(scores).map(([id, score]) => ({
      exam_question_id: Number(id),
      score
    }))
    await attemptApi.grade(attemptId.value, items)
    ElMessage.success('批改成功')
    loadDetail()
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  attempts.value = await examApi.attempts(examId)
  if (attempts.value.length) {
    attemptId.value = attempts.value[0].attempt_id
    await loadDetail()
  }
})
</script>
