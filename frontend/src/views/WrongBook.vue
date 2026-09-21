<template>
  <div class="page-card">
    <div class="page-header">
      <h2>错题本</h2>
      <el-button type="primary" @click="startPractice">练习错题</el-button>
    </div>

    <el-table :data="rows" v-loading="loading" border>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="question.content" label="题干" min-width="240" show-overflow-tooltip />
      <el-table-column prop="knowledge_point" label="知识点" width="130" />
      <el-table-column prop="wrong_count" label="错误次数" width="90" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 'resolved' ? 'success' : 'danger'">{{ row.status === 'resolved' ? '已掌握' : '未掌握' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button link type="danger" @click="onDelete(row)">移除</el-button>
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

    <el-dialog v-model="practiceVisible" title="错题练习" width="760px" :close-on-click-modal="false">
      <template v-if="practice.length">
        <div v-for="(q, idx) in practice" :key="q.question_id" class="practice-item">
          <div class="answer-text">{{ idx + 1 }}. {{ q.content }}</div>
          <div style="margin-top: 8px">
            <el-radio-group v-if="q.type === 'single'" v-model="practiceAnswers[q.question_id]">
              <el-radio v-for="opt in q.options" :key="opt.key" :value="opt.key">{{ opt.key }}. {{ opt.text }}</el-radio>
            </el-radio-group>
            <el-checkbox-group v-else-if="q.type === 'multiple'" v-model="practiceMultiple[q.question_id]">
              <el-checkbox v-for="opt in q.options" :key="opt.key" :value="opt.key">{{ opt.key }}. {{ opt.text }}</el-checkbox>
            </el-checkbox-group>
            <el-radio-group v-else-if="q.type === 'true_false'" v-model="practiceAnswers[q.question_id]">
              <el-radio value="T">正确</el-radio>
              <el-radio value="F">错误</el-radio>
            </el-radio-group>
          </div>
        </div>
      </template>
      <el-empty v-else description="没有需要练习的错题" />
      <template #footer>
        <el-button @click="practiceVisible = false">关闭</el-button>
        <el-button type="primary" :disabled="!practice.length" :loading="practicing" @click="submitPractice">提交</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { wrongApi } from '../api'
import type { PracticeQuestion, WrongQuestionItem } from '../types'

const rows = ref<WrongQuestionItem[]>([])
const total = ref(0)
const loading = ref(false)
const practicing = ref(false)
const practiceVisible = ref(false)
const practice = ref<PracticeQuestion[]>([])
const practiceAnswers = reactive<Record<number, unknown>>({})
const practiceMultiple = reactive<Record<number, string[]>>({})
const query = reactive({ page: 1, page_size: 10 })

async function load() {
  loading.value = true
  try {
    const res = await wrongApi.list({ ...query })
    rows.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

async function onDelete(row: WrongQuestionItem) {
  await ElMessageBox.confirm('确认移除该错题？', '提示', { type: 'warning' })
  await wrongApi.remove(row.id)
  ElMessage.success('已移除')
  load()
}

async function startPractice() {
  const res = await wrongApi.practice()
  practice.value = res.questions
  Object.keys(practiceAnswers).forEach((k) => delete practiceAnswers[Number(k)])
  Object.keys(practiceMultiple).forEach((k) => delete practiceMultiple[Number(k)])
  practiceVisible.value = true
}

async function submitPractice() {
  practicing.value = true
  try {
    const answers = practice.value.map((q) => ({
      question_id: q.question_id,
      answer: q.type === 'multiple' ? practiceMultiple[q.question_id] || [] : practiceAnswers[q.question_id] || ''
    }))
    const res = await wrongApi.submitPractice(answers)
    ElMessage.success(`练习完成：答对 ${res.correct} / ${res.total}`)
    practiceVisible.value = false
    load()
  } finally {
    practicing.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.pager {
  margin-top: 16px;
  justify-content: flex-end;
}
.practice-item {
  padding: 12px 0;
  border-bottom: 1px solid #f0f0f0;
}
</style>
