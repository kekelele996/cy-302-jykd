package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/dto"
	"github.com/gbexam/online-exam/internal/middleware"
	"github.com/gbexam/online-exam/pkg/httpx"
)

// StartAttempt handles POST /exams/:id/attempts.
func (s *Server) StartAttempt(c *gin.Context) {
	examID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "考试 ID 不合法")
		return
	}
	attempt, err := s.attempts.Start(c.Request.Context(), middleware.UserID(c), uint(examID))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.Created(c, attempt)
}

// CurrentAttempt handles GET /exams/:id/attempts/current.
func (s *Server) CurrentAttempt(c *gin.Context) {
	examID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "考试 ID 不合法")
		return
	}
	attempt, err := s.attempts.Current(c.Request.Context(), middleware.UserID(c), uint(examID))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, attempt)
}

// SaveAnswer handles POST /attempts/:id/answers.
func (s *Server) SaveAnswer(c *gin.Context) {
	attemptID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "答题记录 ID 不合法")
		return
	}
	var req dto.AnswerSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	if err := s.attempts.SaveAnswer(c.Request.Context(), middleware.UserID(c), uint(attemptID), req); err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, gin.H{"message": "已保存"})
}

// SubmitAttempt handles POST /attempts/:id/submit.
func (s *Server) SubmitAttempt(c *gin.Context) {
	attemptID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "答题记录 ID 不合法")
		return
	}
	if err := s.attempts.Submit(c.Request.Context(), middleware.UserID(c), uint(attemptID)); err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, gin.H{"message": "交卷成功"})
}

// ListAttempts handles GET /attempts.
func (s *Server) ListAttempts(c *gin.Context) {
	var query dto.AttemptListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	result, err := s.attempts.List(c.Request.Context(), middleware.UserID(c), query)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, result)
}

// GetAttemptDetail handles GET /attempts/:id.
func (s *Server) GetAttemptDetail(c *gin.Context) {
	attemptID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "答题记录 ID 不合法")
		return
	}
	detail, err := s.attempts.Detail(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(attemptID))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, detail)
}

// GradeAttempt handles PUT /attempts/:id/grade.
func (s *Server) GradeAttempt(c *gin.Context) {
	attemptID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "答题记录 ID 不合法")
		return
	}
	var req dto.GradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	if err := s.attempts.Grade(c.Request.Context(), middleware.UserID(c), middleware.Role(c), uint(attemptID), req); err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, gin.H{"message": "批改成功"})
}

// GetAttemptReport handles GET /attempts/:id/report.
func (s *Server) GetAttemptReport(c *gin.Context) {
	attemptID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "答题记录 ID 不合法")
		return
	}
	report, err := s.attempts.Report(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(attemptID))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, report)
}

// ListGrading handles GET /exams/:id/attempts.
func (s *Server) ListGrading(c *gin.Context) {
	examID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "考试 ID 不合法")
		return
	}
	attempts, err := s.attempts.ListGrading(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(examID))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, attempts)
}
