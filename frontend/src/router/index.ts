import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: () => import('../views/Login.vue'), meta: { public: true } },
  { path: '/register', name: 'register', component: () => import('../views/Register.vue'), meta: { public: true } },
  {
    path: '/',
    component: () => import('../components/AppLayout.vue'),
    children: [
      { path: '', redirect: '/dashboard' },
      { path: 'dashboard', name: 'dashboard', component: () => import('../views/Dashboard.vue') },
      { path: 'questions', name: 'questions', component: () => import('../views/QuestionBank.vue'), meta: { roles: ['admin', 'teacher'] } },
      { path: 'exams', name: 'exams', component: () => import('../views/ExamList.vue') },
      { path: 'exam/:id/take', name: 'exam-take', component: () => import('../views/ExamTake.vue'), meta: { roles: ['student'] } },
      { path: 'attempts', name: 'attempts', component: () => import('../views/MyAttempts.vue'), meta: { roles: ['student'] } },
      { path: 'report/:attemptId', name: 'report', component: () => import('../views/ExamReport.vue') },
      { path: 'grading/:examId', name: 'grading', component: () => import('../views/Grading.vue'), meta: { roles: ['admin', 'teacher'] } },
      { path: 'wrong', name: 'wrong', component: () => import('../views/WrongBook.vue'), meta: { roles: ['student'] } },
      { path: 'users', name: 'users', component: () => import('../views/Users.vue'), meta: { roles: ['admin'] } }
    ]
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.public) {
    if (auth.isLoggedIn && (to.name === 'login' || to.name === 'register')) {
      return '/dashboard'
    }
    return true
  }
  if (!auth.isLoggedIn) {
    return '/login'
  }
  const roles = to.meta.roles as string[] | undefined
  if (roles && roles.length > 0 && !roles.includes(auth.role)) {
    return '/dashboard'
  }
  return true
})

export default router
