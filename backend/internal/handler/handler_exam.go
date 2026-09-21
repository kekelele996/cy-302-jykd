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

// ListExams handles GET /exams.
func (s *Server) ListExams(c *gin.Context) {
	var query dto.ExamListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	result, err := s.exams.List(c.Request.Context(), middleware.Role(c), middleware.UserID(c), query)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, result)
}

// GetExam handles GET /exams/:id.
func (s *Server) GetExam(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "考试 ID 不合法")
		return
	}
	exam, err := s.exams.Get(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(id))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, exam)
}

// CreateExam handles POST /exams.
func (s *Server) CreateExam(c *gin.Context) {
	var req dto.ExamCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	exam, err := s.exams.Create(c.Request.Context(), middleware.UserID(c), req)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.Created(c, exam)
}

// PublishExam handles POST /exams/:id/publish.
func (s *Server) PublishExam(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "考试 ID 不合法")
		return
	}
	version, err := s.exams.Publish(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(id))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, gin.H{"message": "发布成功", "version": version})
}

// RegroupExam handles POST /exams/:id/regroup — re-generate paper as a new
// immutable version while keeping old versions readable.
func (s *Server) RegroupExam(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "考试 ID 不合法")
		return
	}
	var req dto.RegroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	resp, err := s.exams.Regroup(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(id), req)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, gin.H{"message": "已生成新版本", "version": resp})
}

// ListExamVersions handles GET /exams/:id/versions.
func (s *Server) ListExamVersions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "考试 ID 不合法")
		return
	}
	versions, err := s.exams.ListVersions(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(id))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, versions)
}

// CloseExam handles POST /exams/:id/close.
func (s *Server) CloseExam(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "考试 ID 不合法")
		return
	}
	if err := s.exams.Close(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(id)); err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, gin.H{"message": "已关闭"})
}

// DeleteExam handles DELETE /exams/:id.
func (s *Server) DeleteExam(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "考试 ID 不合法")
		return
	}
	if err := s.exams.Delete(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(id)); err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, gin.H{"message": "删除成功"})
}

// ListExamQuestions handles GET /exams/:id/questions?version_no=N.
func (s *Server) ListExamQuestions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "考试 ID 不合法")
		return
	}
	versionNo := 0
	if raw := c.Query("version_no"); raw != "" {
		v, parseErr := strconv.Atoi(raw)
		if parseErr != nil || v < 0 {
			httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "版本号不合法")
			return
		}
		versionNo = v
	}
	questions, err := s.exams.ListPaperQuestions(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(id), versionNo)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, questions)
}
