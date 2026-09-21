<template>
  <div class="page-card">
    <div class="page-header">
      <h2>用户管理</h2>
    </div>
    <div class="toolbar">
      <el-input v-model="query.keyword" placeholder="用户名/姓名" clearable style="width: 200px" @keyup.enter="load" />
      <el-select v-model="query.role" placeholder="角色" clearable style="width: 130px">
        <el-option label="管理员" value="admin" />
        <el-option label="教师" value="teacher" />
        <el-option label="学生" value="student" />
      </el-select>
      <el-button type="primary" @click="load">查询</el-button>
    </div>
    <el-table :data="rows" v-loading="loading" border>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="username" label="用户名" />
      <el-table-column prop="name" label="姓名" />
      <el-table-column label="角色" width="110">
        <template #default="{ row }">
          <el-tag>{{ roleLabel(row.role) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'danger'">{{ row.status === 'active' ? '正常' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="140" fixed="right">
        <template #default="{ row }">
          <el-button v-if="row.role !== 'admin'" link :type="row.status === 'active' ? 'danger' : 'success'" @click="toggle(row)">
            {{ row.status === 'active' ? '禁用' : '启用' }}
          </el-button>
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
import { ElMessage } from 'element-plus'
import { userApi } from '../api'
import type { UserProfile } from '../types'

const rows = ref<UserProfile[]>([])
const total = ref(0)
const loading = ref(false)
const query = reactive({ page: 1, page_size: 10, keyword: '', role: '' })

function roleLabel(role: string) {
  const map: Record<string, string> = { admin: '管理员', teacher: '教师', student: '学生' }
  return map[role] || role
}

async function toggle(row: UserProfile) {
  const status = row.status === 'active' ? 'disabled' : 'active'
  await userApi.updateStatus(row.id, status)
  ElMessage.success('更新成功')
  load()
}

async function load() {
  loading.value = true
  try {
    const res = await userApi.list({ ...query })
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
