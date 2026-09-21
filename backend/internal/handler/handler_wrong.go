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

// ListWrongQuestions handles GET /wrong-questions.
func (s *Server) ListWrongQuestions(c *gin.Context) {
	var query dto.WrongQuestionListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	result, err := s.wrong.List(c.Request.Context(), middleware.UserID(c), query)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, result)
}

// DeleteWrongQuestion handles DELETE /wrong-questions/:id.
func (s *Server) DeleteWrongQuestion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "错题 ID 不合法")
		return
	}
	if err := s.wrong.Delete(c.Request.Context(), middleware.UserID(c), uint(id)); err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, gin.H{"message": "删除成功"})
}

// PracticeWrongQuestions handles GET /wrong-questions/practice.
func (s *Server) PracticeWrongQuestions(c *gin.Context) {
	practice, err := s.wrong.Practice(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, practice)
}

// SubmitPractice handles POST /wrong-questions/practice.
func (s *Server) SubmitPractice(c *gin.Context) {
	var req dto.PracticeAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	result, err := s.wrong.SubmitPractice(c.Request.Context(), middleware.UserID(c), req)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, result)
}
