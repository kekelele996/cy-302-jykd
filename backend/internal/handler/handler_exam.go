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
	if err := s.exams.Publish(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(id)); err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, gin.H{"message": "发布成功"})
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

// RegenerateExam handles POST /exams/:id/regenerate.
func (s *Server) RegenerateExam(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "考试 ID 不合法")
		return
	}
	var req dto.PaperRegenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	version, err := s.exams.Regenerate(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(id), req)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.Created(c, version)
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

// GetExamVersion handles GET /exams/:id/versions/:version_no.
func (s *Server) GetExamVersion(c *gin.Context) {
	examID, versionNo, ok := parseExamVersionParams(c)
	if !ok {
		return
	}
	version, err := s.exams.GetVersion(c.Request.Context(), middleware.Role(c), middleware.UserID(c), examID, versionNo)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, version)
}

// ListExamVersionQuestions handles GET /exams/:id/versions/:version_no/questions.
func (s *Server) ListExamVersionQuestions(c *gin.Context) {
	examID, versionNo, ok := parseExamVersionParams(c)
	if !ok {
		return
	}
	questions, err := s.exams.ListVersionQuestions(c.Request.Context(), middleware.Role(c), middleware.UserID(c), examID, versionNo)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, questions)
}

// DiscardExamVersion handles DELETE /exams/:id/versions/:version_no (draft only).
func (s *Server) DiscardExamVersion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "考试 ID 不合法")
		return
	}
	versionNo, err := strconv.Atoi(c.Param("version_no"))
	if err != nil || versionNo <= 0 {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "版本号不合法")
		return
	}
	version, err := s.exams.GetVersion(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(id), versionNo)
	if err != nil {
		s.respondError(c, err)
		return
	}
	if err := s.exams.DiscardDraft(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(id), version.ID); err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, gin.H{"message": "草稿版本已放弃"})
}

func parseExamVersionParams(c *gin.Context) (uint, int, bool) {
	examID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "考试 ID 不合法")
		return 0, 0, false
	}
	versionNo, err := strconv.Atoi(c.Param("version_no"))
	if err != nil || versionNo <= 0 {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "版本号不合法")
		return 0, 0, false
	}
	return uint(examID), versionNo, true
}

// ListExamQuestions handles GET /exams/:id/questions.
func (s *Server) ListExamQuestions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "考试 ID 不合法")
		return
	}
	questions, err := s.exams.ListPaperQuestions(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(id))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, questions)
}
