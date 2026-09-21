package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/dto"
	"github.com/gbexam/online-exam/pkg/httpx"
)

// ListUsers handles GET /users.
func (s *Server) ListUsers(c *gin.Context) {
	var query dto.UserListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	result, err := s.users.List(c.Request.Context(), query)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, result)
}

// UpdateUserStatus handles PUT /users/:id/status.
func (s *Server) UpdateUserStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "用户 ID 不合法")
		return
	}
	var req dto.UserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	if err := s.users.UpdateStatus(c.Request.Context(), uint(id), req.Status); err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, gin.H{"message": "更新成功"})
}
