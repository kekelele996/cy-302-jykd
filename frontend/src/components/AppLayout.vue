<template>
  <el-container class="layout">
    <el-aside width="220px" class="aside">
      <div class="brand">在线考试平台</div>
      <el-menu :default-active="$route.path" router background-color="#001529" text-color="#c0c4cc" active-text-color="#ffffff">
        <el-menu-item index="/dashboard">
          <el-icon><DataBoard /></el-icon><span>首页概览</span>
        </el-menu-item>
        <el-menu-item v-if="isStaff" index="/questions">
          <el-icon><Collection /></el-icon><span>题库管理</span>
        </el-menu-item>
        <el-menu-item index="/exams">
          <el-icon><Tickets /></el-icon><span>考试管理</span>
        </el-menu-item>
        <el-menu-item v-if="isStudent" index="/attempts">
          <el-icon><Document /></el-icon><span>考试记录</span>
        </el-menu-item>
        <el-menu-item v-if="isStudent" index="/wrong">
          <el-icon><Warning /></el-icon><span>错题本</span>
        </el-menu-item>
        <el-menu-item v-if="isAdmin" index="/users">
          <el-icon><User /></el-icon><span>用户管理</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div class="header-title">在线考试平台</div>
        <div class="header-right">
          <el-tag>{{ roleLabel }}</el-tag>
          <span class="username">{{ auth.user?.name || auth.user?.username }}</span>
          <el-button link type="primary" @click="onLogout">退出登录</el-button>
        </div>
      </el-header>
      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()

const isAdmin = computed(() => auth.role === 'admin')
const isStaff = computed(() => auth.role === 'admin' || auth.role === 'teacher')
const isStudent = computed(() => auth.role === 'student')
const roleLabel = computed(() => {
  const map: Record<string, string> = { admin: '管理员', teacher: '教师', student: '学生' }
  return map[auth.role] || auth.role
})

async function onLogout() {
  await auth.logout()
  router.push('/login')
}
</script>

<style scoped>
.layout {
  height: 100vh;
}
.aside {
  background: #001529;
}
.brand {
  height: 60px;
  line-height: 60px;
  text-align: center;
  color: #fff;
  font-weight: 700;
  font-size: 18px;
}
.aside :deep(.el-menu) {
  border-right: none;
}
.aside :deep(.el-menu-item.is-active) {
  background: #1890ff;
}
.header {
  background: #fff;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #e4e7ed;
}
.header-title {
  font-size: 18px;
  font-weight: 600;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.username {
  color: #303133;
}
.main {
  overflow: auto;
}
</style>
