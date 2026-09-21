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
      <el-table-column label="试卷版本" width="110">
        <template #default="{ row }">
          <el-tag v-if="row.current_version_id" size="small" type="success">v{{ row.current_version_no }} / {{ row.version_count }}</el-tag>
          <span v-else class="muted">未冻结</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="statusTag[row.status as keyof typeof statusTag]">{{ statusLabels[row.status as keyof typeof statusLabels] }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" min-width="380" fixed="right">
        <template #default="{ row }">
          <template v-if="isStudent">
            <el-button v-if="row.status === 'published'" type="primary" size="small" @click="$router.push(`/exam/${row.id}/take`)">开始考试</el-button>
          </template>
          <template v-else>
            <el-button size="small" @click="viewQuestions(row, 0)">题目</el-button>
            <el-button v-if="row.current_version_id" size="small" type="info" plain @click="viewVersions(row)">版本</el-button>
            <el-button v-if="row.status === 'draft'" type="success" size="small" @click="publish(row)">发布</el-button>
            <el-button v-if="row.status === 'published' || row.status === 'closed'" type="primary" size="small" plain @click="openRegroup(row)">重新组卷</el-button>
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

    <el-dialog v-model="questionsVisible" :title="questionsTitle" width="800px">
      <div class="paper-meta">
        <el-tag v-if="questionsVersionNo > 0" type="info">历史冻结版本 v{{ questionsVersionNo }}（只读）</el-tag>
        <el-tag v-else type="success">当前版本</el-tag>
      </div>
      <el-table :data="questions" border max-height="500">
        <el-table-column type="index" label="#" width="50" />
        <el-table-column label="题型" width="90">
          <template #default="{ row }">{{ typeLabels[row.question.type as keyof typeof typeLabels] }}</template>
        </el-table-column>
        <el-table-column prop="question.content" label="题干" min-width="220" show-overflow-tooltip />
        <el-table-column label="标准答案" width="110">
          <template #default="{ row }">{{ formatAnswer(row.question.answer) }}</template>
        </el-table-column>
        <el-table-column prop="score" label="分值" width="70" />
        <el-table-column prop="question.analysis" label="解析" min-width="120" show-overflow-tooltip />
      </el-table>
    </el-dialog>

    <el-dialog v-model="versionsVisible" title="试卷版本（冻结快照）" width="720px">
      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="发布时已冻结当时的题干、选项、分值与标准答案；题库后续修改或撤回不影响历史版本。重新组卷只会生成新版本，旧版本始终可查。"
        style="margin-bottom: 12px"
      />
      <el-table :data="versions" border>
        <el-table-column prop="version_no" label="版本" width="80">
          <template #default="{ row }">v{{ row.version_no }}</template>
        </el-table-column>
        <el-table-column prop="question_count" label="题数" width="70" />
        <el-table-column prop="total_score" label="总分" width="80" />
        <el-table-column label="冻结时间" width="180">
          <template #default="{ row }">{{ formatTime(row.snapshot_at) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag v-if="row.is_current" type="success" size="small">当前</el-tag>
            <el-tag v-else type="info" size="small">历史</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button link type="primary" @click="versionsExam && viewQuestions(versionsExam, row.version_no)">查看题目</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <el-dialog v-model="regroupVisible" title="重新组卷（生成新版本）" width="760px">
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        title="将立即生成新的冻结版本并作为当前版本；进行中的考试与历史成绩仍读取各自原来的旧版本。"
        style="margin-bottom: 12px"
      />
      <el-form label-width="100px">
        <el-form-item label="组卷参数">
          <div style="width: 100%">
            <div v-for="(cfg, idx) in regroupForm.question_config" :key="idx" class="config-row">
              <el-select v-model="cfg.type" style="width: 120px">
                <el-option v-for="(label, key) in typeLabels" :key="key" :label="label" :value="key" />
              </el-select>
              <el-input-number v-model="cfg.count" :min="1" placeholder="数量" />
              <el-input-number v-model="cfg.score" :min="0.5" :step="0.5" placeholder="分值" />
              <el-select v-model="cfg.difficulty" placeholder="难度" clearable style="width: 110px">
                <el-option v-for="(label, key) in difficultyLabels" :key="key" :label="label" :value="key" />
              </el-select>
              <el-button link type="danger" @click="regroupForm.question_config.splice(idx, 1)">删除</el-button>
            </div>
            <el-button size="small" @click="addRegroupConfig">添加题型</el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="regroupVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="onRegroup">生成新版本并发布</el-button>
      </template>
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
import dayjs from 'dayjs'
import { ElMessage, ElMessageBox } from 'element-plus'
import { examApi } from '../api'
import { useAuthStore } from '../stores/auth'
import type { Exam, PaperQuestionConfig, ExamStatResponse, PaperVersionSummary } from '../types'

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
const versionsVisible = ref(false)
const regroupVisible = ref(false)
const questions = ref<{ id: number; score: number; version_no?: number; question: { type: string; content: string; analysis?: string; answer?: unknown } }[]>([])
const questionsVersionNo = ref(0)
const questionsTitle = ref('试卷题目')
const versions = ref<PaperVersionSummary[]>([])
const versionsExam = ref<Exam | null>(null)
const regroupExam = ref<Exam | null>(null)
const statsVisible = ref(false)
const stats = ref<ExamStatResponse | null>(null)
const query = reactive({ page: 1, page_size: 10, status: '', keyword: '' })

const form = reactive({
  title: '',
  description: '',
  duration_minutes: 60,
  total_score: 100,
  question_config: [] as PaperQuestionConfig[]
})

const regroupForm = reactive({
  question_config: [] as PaperQuestionConfig[]
})

function formatTime(v?: string | null) {
  return v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-'
}

function formatAnswer(v: unknown) {
  if (v === null || v === undefined || v === '') return '-'
  if (Array.isArray(v)) return v.join('；')
  return String(v)
}

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
  try {
    await examApi.publish(row.id)
    ElMessage.success('发布成功，试卷已冻结为 v1')
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '发布失败')
  }
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

async function viewQuestions(row: Exam, versionNo = 0) {
  questions.value = await examApi.questions(row.id, versionNo)
  questionsVersionNo.value = versionNo
  questionsTitle.value = versionNo > 0 ? `试卷题目 · v${versionNo}（冻结版本）` : '试卷题目'
  questionsVisible.value = true
}

async function viewVersions(row: Exam) {
  versionsExam.value = row
  versions.value = await examApi.versions(row.id)
  versionsVisible.value = true
}

function addRegroupConfig() {
  regroupForm.question_config.push({ type: 'single', count: 5, score: 2, difficulty: 'easy' })
}

function openRegroup(row: Exam) {
  regroupExam.value = row
  regroupForm.question_config = [
    { type: 'single', count: 5, score: 2, difficulty: 'easy' },
    { type: 'true_false', count: 5, score: 1, difficulty: 'easy' }
  ]
  regroupVisible.value = true
}

async function onRegroup() {
  if (!regroupExam.value) return
  saving.value = true
  try {
    const res = await examApi.regroup(regroupExam.value.id, {
      expected_revision: regroupExam.value.revision,
      question_config: regroupForm.question_config
    })
    ElMessage.success(`已生成新版本 v${res.version.version_no}`)
    regroupVisible.value = false
    load()
  } catch (e: any) {
    if (e?.response?.status === 409) {
      ElMessage.error('试卷刚被其他人修改（发布或重新组卷），请刷新后重试')
    } else {
      ElMessage.error(e?.response?.data?.message || '重新组卷失败')
    }
  } finally {
    saving.value = false
  }
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
.muted {
  color: #909399;
  font-size: 12px;
}
.paper-meta {
  margin-bottom: 10px;
}
</style>
