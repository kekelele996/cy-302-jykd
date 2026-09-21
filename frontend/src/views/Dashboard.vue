<template>
  <div>
    <div class="page-header">
      <h2>首页概览</h2>
    </div>
    <el-row :gutter="16">
      <el-col :span="6" v-for="card in cards" :key="card.label">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-value">{{ card.value }}</div>
          <div class="stat-label">{{ card.label }}</div>
        </el-card>
      </el-col>
    </el-row>
    <el-card class="welcome" shadow="never">
      <p>欢迎使用在线考试平台，当前角色：<el-tag>{{ roleLabel }}</el-tag></p>
      <p>平台支持题库管理、自动组卷、限时考试、自动阅卷、成绩分析和错题本功能。</p>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { statsApi } from '../api'
import { useAuthStore } from '../stores/auth'
import type { OverviewResponse } from '../types'

const auth = useAuthStore()
const overview = ref<OverviewResponse>({ user_count: 0, question_count: 0, exam_count: 0, attempt_count: 0 })

const roleLabel = computed(() => {
  const map: Record<string, string> = { admin: '管理员', teacher: '教师', student: '学生' }
  return map[auth.role] || auth.role
})

const cards = computed(() => [
  { label: '用户数', value: overview.value.user_count },
  { label: '题目数', value: overview.value.question_count },
  { label: '考试数', value: overview.value.exam_count },
  { label: '答题次数', value: overview.value.attempt_count }
])

onMounted(async () => {
  try {
    overview.value = await statsApi.overview()
  } catch {
    // error already handled by interceptor
  }
})
</script>

<style scoped>
.stat-card {
  text-align: center;
}
.stat-value {
  font-size: 30px;
  font-weight: 700;
  color: #1890ff;
}
.stat-label {
  margin-top: 6px;
  color: #909399;
}
.welcome {
  margin-top: 16px;
  line-height: 1.8;
}
</style>
