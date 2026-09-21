<template>
  <div class="page-card">
    <div class="page-header">
      <h2>成绩报告</h2>
      <el-button @click="$router.push('/attempts')">返回考试记录</el-button>
    </div>

    <template v-if="report">
      <el-descriptions :column="3" border>
        <el-descriptions-item label="考试">{{ report.exam_title }}</el-descriptions-item>
        <el-descriptions-item label="总分">{{ report.total_score }}</el-descriptions-item>
        <el-descriptions-item label="客观题得分">{{ report.objective_score }}</el-descriptions-item>
        <el-descriptions-item label="主观题得分">{{ report.subjective_score }}</el-descriptions-item>
        <el-descriptions-item label="客观题正确率">{{ report.accuracy }}%</el-descriptions-item>
        <el-descriptions-item label="排名">{{ report.rank }} / {{ report.participants }}</el-descriptions-item>
      </el-descriptions>

      <el-row :gutter="16" style="margin-top: 16px">
        <el-col :span="12">
          <div ref="chartRef" style="height: 320px"></div>
        </el-col>
        <el-col :span="12">
          <el-table :data="report.type_breakdown" border>
            <el-table-column prop="name" label="题型" />
            <el-table-column label="得分" width="120">
              <template #default="{ row }">{{ row.score }} / {{ row.max }}</template>
            </el-table-column>
            <el-table-column prop="count" label="题数" width="80" />
          </el-table>
        </el-col>
      </el-row>
    </template>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import * as echarts from 'echarts'
import { attemptApi } from '../api'
import type { ReportResponse } from '../types'

const route = useRoute()
const report = ref<ReportResponse | null>(null)
const chartRef = ref<HTMLElement | null>(null)

onMounted(async () => {
  const attemptId = Number(route.params.attemptId)
  report.value = await attemptApi.report(attemptId)
  await nextTick()
  renderChart()
})

function renderChart() {
  if (!chartRef.value || !report.value) return
  const chart = echarts.init(chartRef.value)
  const data = report.value.type_breakdown.map((item) => ({
    name: item.name,
    value: item.score
  }))
  chart.setOption({
    title: { text: '各题型得分分布', left: 'center' },
    tooltip: { trigger: 'item' },
    legend: { bottom: 0 },
    series: [
      {
        type: 'pie',
        radius: ['35%', '65%'],
        data,
        label: { formatter: '{b}: {c}分' }
      }
    ]
  })
}
</script>
