package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/middleware"
	"github.com/gbexam/online-exam/pkg/httpx"
)

// Overview handles GET /stats/overview.
func (s *Server) Overview(c *gin.Context) {
	overview, err := s.stats.Overview(c.Request.Context())
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, overview)
}

// ExamStats handles GET /exams/:id/stats.
func (s *Server) ExamStats(c *gin.Context) {
	examID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "考试 ID 不合法")
		return
	}
	stats, err := s.stats.ExamStats(c.Request.Context(), middleware.Role(c), middleware.UserID(c), uint(examID))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, stats)
}
