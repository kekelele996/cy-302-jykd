package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/dto"
	"github.com/gbexam/online-exam/internal/middleware"
	"github.com/gbexam/online-exam/pkg/httpx"
)

// Register handles POST /auth/register.
func (s *Server) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	profile, err := s.auth.Register(c.Request.Context(), req)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.Created(c, profile)
}

// Login handles POST /auth/login.
func (s *Server) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	resp, err := s.auth.Login(c.Request.Context(), req)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, resp)
}

// Logout handles POST /auth/logout.
func (s *Server) Logout(c *gin.Context) {
	s.auth.Logout(c.Request.Context(), middleware.UserID(c))
	httpx.OK(c, gin.H{"message": "已退出登录"})
}

// Profile handles GET /auth/profile.
func (s *Server) Profile(c *gin.Context) {
	profile, err := s.auth.Profile(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, profile)
}
