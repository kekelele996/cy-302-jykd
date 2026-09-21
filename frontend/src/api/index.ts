import http from './http'
import type {
  AttemptDetail,
  AttemptStartResponse,
  AttemptSummary,
  Exam,
  ExamCreatePayload,
  ExamStatResponse,
  LoginResponse,
  OverviewResponse,
  PageResult,
  PracticeResultResponse,
  PracticeQuestion,
  Question,
  ReportResponse,
  UserProfile,
  WrongQuestionItem
} from '../types'

export const authApi = {
  register(data: { username: string; password: string; name: string; role: string }) {
    return http.post<never, UserProfile>('/auth/register', data)
  },
  login(data: { username: string; password: string }) {
    return http.post<never, LoginResponse>('/auth/login', data)
  },
  profile() {
    return http.get<never, UserProfile>('/auth/profile')
  },
  logout() {
    return http.post<never, { message: string }>('/auth/logout')
  }
}

export const userApi = {
  list(params: { page?: number; page_size?: number; keyword?: string; role?: string }) {
    return http.get<never, PageResult<UserProfile>>('/users', { params })
  },
  updateStatus(id: number, status: string) {
    return http.put<never, { message: string }>(`/users/${id}/status`, { status })
  }
}

export const questionApi = {
  list(params: { page?: number; page_size?: number; type?: string; difficulty?: string; knowledge_point?: string; keyword?: string }) {
    return http.get<never, PageResult<Question>>('/questions', { params })
  },
  create(data: Partial<Question>) {
    return http.post<never, Question>('/questions', data)
  },
  update(id: number, data: Partial<Question>) {
    return http.put<never, Question>(`/questions/${id}`, data)
  },
  remove(id: number) {
    return http.delete<never, { message: string }>(`/questions/${id}`)
  },
  batchImport(file: File) {
    const form = new FormData()
    form.append('file', file)
    return http.post<never, { imported: number; failed: number; errors: string[] }>('/questions/batch-import', form)
  }
}

export const examApi = {
  list(params: { page?: number; page_size?: number; status?: string; keyword?: string }) {
    return http.get<never, PageResult<Exam>>('/exams', { params })
  },
  create(data: ExamCreatePayload) {
    return http.post<never, Exam>('/exams', data)
  },
  get(id: number) {
    return http.get<never, Exam>(`/exams/${id}`)
  },
  publish(id: number) {
    return http.post<never, { message: string }>(`/exams/${id}/publish`)
  },
  close(id: number) {
    return http.post<never, { message: string }>(`/exams/${id}/close`)
  },
  remove(id: number) {
    return http.delete<never, { message: string }>(`/exams/${id}`)
  },
  questions(id: number) {
    return http.get<never, { id: number; score: number; question: Question }[]>(`/exams/${id}/questions`)
  },
  stats(id: number) {
    return http.get<never, ExamStatResponse>(`/exams/${id}/stats`)
  },
  attempts(id: number) {
    return http.get<never, AttemptSummary[]>(`/exams/${id}/attempts`)
  }
}

export const attemptApi = {
  start(examId: number) {
    return http.post<never, AttemptStartResponse>(`/exams/${examId}/attempts`)
  },
  current(examId: number) {
    return http.get<never, AttemptStartResponse>(`/exams/${examId}/attempts/current`)
  },
  saveAnswer(attemptId: number, data: { exam_question_id: number; answer?: unknown; marked?: boolean }) {
    return http.post<never, { message: string }>(`/attempts/${attemptId}/answers`, data)
  },
  submit(attemptId: number) {
    return http.post<never, { message: string }>(`/attempts/${attemptId}/submit`)
  },
  list(params: { page?: number; page_size?: number; exam_id?: number }) {
    return http.get<never, PageResult<AttemptSummary>>('/attempts', { params })
  },
  detail(attemptId: number) {
    return http.get<never, AttemptDetail>(`/attempts/${attemptId}`)
  },
  report(attemptId: number) {
    return http.get<never, ReportResponse>(`/attempts/${attemptId}/report`)
  },
  grade(attemptId: number, items: { exam_question_id: number; score: number }[]) {
    return http.put<never, { message: string }>(`/attempts/${attemptId}/grade`, { items })
  }
}

export const wrongApi = {
  list(params: { page?: number; page_size?: number; knowledge_point?: string }) {
    return http.get<never, PageResult<WrongQuestionItem>>('/wrong-questions', { params })
  },
  remove(id: number) {
    return http.delete<never, { message: string }>(`/wrong-questions/${id}`)
  },
  practice() {
    return http.get<never, { questions: PracticeQuestion[] }>('/wrong-questions/practice')
  },
  submitPractice(answers: { question_id: number; answer?: unknown }[]) {
    return http.post<never, PracticeResultResponse>('/wrong-questions/practice', { answers })
  }
}

export const statsApi = {
  overview() {
    return http.get<never, OverviewResponse>('/stats/overview')
  }
}
