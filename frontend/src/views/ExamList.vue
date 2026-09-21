<template>
  <div class="page-card">
    <div class="page-header">
      <h2>考试管理</h2>
      <el-button v-if="isStaff" type="primary" @click="openCreate">创建考试</el-button>
    </div>

    <el-table :data="rows" v-loading="loading" border>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="title" label="考试名称" min-width="180" />
      <el-table-column prop="duration_minutes" label="时长(分钟)" width="100" />
      <el-table-column prop="total_score" label="总分" width="80" />
      <el-table-column prop="question_count" label="题数" width="80" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="statusTag[row.status as keyof typeof statusTag]">{{ statusLabels[row.status as keyof typeof statusLabels] }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" min-width="300" fixed="right">
        <template #default="{ row }">
          <template v-if="isStudent">
            <el-button v-if="row.status === 'published'" type="primary" size="small" @click="$router.push(`/exam/${row.id}/take`)">开始考试</el-button>
          </template>
          <template v-else>
            <el-button size="small" @click="viewQuestions(row)">题目</el-button>
            <el-button v-if="row.status === 'draft'" type="success" size="small" @click="publish(row)">发布</el-button>
            <el-button v-if="row.status === 'published'" type="warning" size="small" @click="closeExam(row)">关闭</el-button>
            <el-button size="small" @click="viewStats(row)">统计</el-button>
            <el-button size="small" type="info" @click="$router.push(`/grading/${row.id}`)">批改</el-button>
            <el-button size="small" type="danger" @click="removeExam(row)">删除</el-button>
          </template>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      class="pager"
      v-model:current-page="query.page"
      v-model:page-size="query.page_size"
      :total="total"
      layout="total, prev, pager, next"
      @current-change="load"
    />

    <el-dialog v-model="dialogVisible" title="创建考试（自动组卷）" width="760px">
      <el-form :model="form" label-width="100px">
        <el-form-item label="考试名称">
          <el-input v-model="form.title" />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="考试时长">
          <el-input-number v-model="form.duration_minutes" :min="1" />
          <span style="margin-left: 8px">分钟</span>
        </el-form-item>
        <el-form-item label="总分">
          <el-input-number v-model="form.total_score" :min="0" :step="1" />
          <span style="margin-left: 8px">（0 表示自动计算）</span>
        </el-form-item>
        <el-form-item label="组卷参数">
          <div style="width: 100%">
            <div v-for="(cfg, idx) in form.question_config" :key="idx" class="config-row">
              <el-select v-model="cfg.type" style="width: 120px">
                <el-option v-for="(label, key) in typeLabels" :key="key" :label="label" :value="key" />
              </el-select>
              <el-input-number v-model="cfg.count" :min="1" placeholder="数量" />
              <el-input-number v-model="cfg.score" :min="0.5" :step="0.5" placeholder="分值" />
              <el-select v-model="cfg.difficulty" placeholder="难度" clearable style="width: 110px">
                <el-option v-for="(label, key) in difficultyLabels" :key="key" :label="label" :value="key" />
              </el-select>
              <el-button link type="danger" @click="form.question_config.splice(idx, 1)">删除</el-button>
            </div>
            <el-button size="small" @click="addConfig">添加题型</el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="onSave">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="questionsVisible" title="试卷题目" width="800px">
      <el-table :data="questions" border max-height="500">
        <el-table-column type="index" label="#" width="50" />
        <el-table-column label="题型" width="90">
          <template #default="{ row }">{{ typeLabels[row.question.type as keyof typeof typeLabels] }}</template>
        </el-table-column>
        <el-table-column prop="question.content" label="题干" min-width="220" show-overflow-tooltip />
        <el-table-column prop="score" label="分值" width="70" />
        <el-table-column prop="question.analysis" label="解析" min-width="120" show-overflow-tooltip />
      </el-table>
    </el-dialog>

    <el-dialog v-model="statsVisible" title="成绩统计" width="800px">
      <div v-if="stats">
        <el-descriptions :column="3" border>
          <el-descriptions-item label="考试">{{ stats.exam_title }}</el-descriptions-item>
          <el-descriptions-item label="参与人数">{{ stats.participant_count }}</el-descriptions-item>
          <el-descriptions-item label="平均分">{{ stats.average_score }}</el-descriptions-item>
          <el-descriptions-item label="最高分">{{ stats.highest_score }}</el-descriptions-item>
          <el-descriptions-item label="最低分">{{ stats.lowest_score }}</el-descriptions-item>
          <el-descriptions-item label="及格人数">{{ stats.pass_count }}</el-descriptions-item>
        </el-descriptions>
        <el-table :data="stats.ranking" border style="margin-top: 12px">
          <el-table-column prop="rank" label="排名" width="80" />
          <el-table-column prop="student_name" label="姓名" />
          <el-table-column prop="student_username" label="用户名" />
          <el-table-column prop="total_score" label="总分" width="100" />
        </el-table>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { examApi } from '../api'
import { useAuthStore } from '../stores/auth'
import type { Exam, PaperQuestionConfig, ExamStatResponse } from '../types'

const typeLabels: Record<string, string> = {
  single: '单选题',
  multiple: '多选题',
  true_false: '判断题',
  fill_blank: '填空题',
  short_answer: '简答题'
}
const difficultyLabels: Record<string, string> = { easy: '简单', medium: '中等', hard: '困难' }
const statusLabels: Record<string, string> = { draft: '草稿', published: '已发布', closed: '已关闭' }
const statusTag: Record<string, string> = { draft: 'info', published: 'success', closed: 'warning' }

const auth = useAuthStore()
const isStudent = computed(() => auth.role === 'student')
const isStaff = computed(() => auth.role === 'admin' || auth.role === 'teacher')

const loading = ref(false)
const saving = ref(false)
const rows = ref<Exam[]>([])
const total = ref(0)
const dialogVisible = ref(false)
const questionsVisible = ref(false)
const statsVisible = ref(false)
const questions = ref<{ id: number; score: number; question: { type: string; content: string; analysis?: string } }[]>([])
const stats = ref<ExamStatResponse | null>(null)
const query = reactive({ page: 1, page_size: 10, status: '', keyword: '' })

const form = reactive({
  title: '',
  description: '',
  duration_minutes: 60,
  total_score: 100,
  question_config: [] as PaperQuestionConfig[]
})

function addConfig() {
  form.question_config.push({ type: 'single', count: 5, score: 2, difficulty: 'easy' })
}

function openCreate() {
  form.title = ''
  form.description = ''
  form.duration_minutes = 60
  form.total_score = 100
  form.question_config = [
    { type: 'single', count: 5, score: 2, difficulty: 'easy' },
    { type: 'true_false', count: 5, score: 1, difficulty: 'easy' }
  ]
  dialogVisible.value = true
}

async function onSave() {
  saving.value = true
  try {
    await examApi.create({
      title: form.title,
      description: form.description,
      duration_minutes: form.duration_minutes,
      total_score: form.total_score,
      question_config: form.question_config
    })
    ElMessage.success('创建成功')
    dialogVisible.value = false
    load()
  } finally {
    saving.value = false
  }
}

async function publish(row: Exam) {
  await examApi.publish(row.id)
  ElMessage.success('发布成功')
  load()
}

async function closeExam(row: Exam) {
  await examApi.close(row.id)
  ElMessage.success('已关闭')
  load()
}

async function removeExam(row: Exam) {
  await ElMessageBox.confirm('确认删除该考试？', '提示', { type: 'warning' })
  await examApi.remove(row.id)
  ElMessage.success('删除成功')
  load()
}

async function viewQuestions(row: Exam) {
  questions.value = await examApi.questions(row.id)
  questionsVisible.value = true
}

async function viewStats(row: Exam) {
  stats.value = await examApi.stats(row.id)
  statsVisible.value = true
}

async function load() {
  loading.value = true
  try {
    const res = await examApi.list({ ...query })
    rows.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.config-row {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}
.pager {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>
