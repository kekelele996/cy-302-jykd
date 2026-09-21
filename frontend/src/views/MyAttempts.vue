<template>
  <div class="page-card">
    <div class="page-header">
      <h2>考试记录</h2>
    </div>
    <el-table :data="rows" v-loading="loading" border>
      <el-table-column prop="attempt_id" label="记录 ID" width="90" />
      <el-table-column prop="exam_title" label="考试名称" min-width="180" />
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="row.status === 'submitted' ? 'success' : 'warning'">{{ row.status === 'submitted' ? '已交卷' : '进行中' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="objective_score" label="客观题得分" width="110" />
      <el-table-column prop="total_score" label="总分" width="90" />
      <el-table-column label="交卷时间" min-width="170">
        <template #default="{ row }">{{ formatTime(row.submitted_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button v-if="row.status === 'submitted'" link type="primary" @click="$router.push(`/report/${row.attempt_id}`)">成绩报告</el-button>
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
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import dayjs from 'dayjs'
import { attemptApi } from '../api'
import type { AttemptSummary } from '../types'

const rows = ref<AttemptSummary[]>([])
const total = ref(0)
const loading = ref(false)
const query = reactive({ page: 1, page_size: 10 })

function formatTime(v?: string | null) {
  return v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-'
}

async function load() {
  loading.value = true
  try {
    const res = await attemptApi.list({ ...query })
    rows.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.pager {
  margin-top: 16px;
  justify-content: flex-end;
}
</style>
