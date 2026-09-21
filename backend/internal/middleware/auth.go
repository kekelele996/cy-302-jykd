package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/service"
	"github.com/gbexam/online-exam/pkg/httpx"
)

const (
	ctxUserID   = "user_id"
	ctxUsername = "username"
	ctxRole     = "role"
)

// Auth validates the bearer JWT and injects identity into the request context.
func Auth(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if token == "" || token == header {
			httpx.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, "未登录或令牌缺失")
			c.Abort()
			return
		}
		userID, username, role, err := auth.ParseToken(token)
		if err != nil {
			httpx.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, "登录已失效，请重新登录")
			c.Abort()
			return
		}
		c.Set(ctxUserID, userID)
		c.Set(ctxUsername, username)
		c.Set(ctxRole, role)
		c.Next()
	}
}

// UserID returns the authenticated user id.
func UserID(c *gin.Context) uint {
	v, _ := c.Get(ctxUserID)
	id, _ := v.(uint)
	return id
}

// Username returns the authenticated username.
func Username(c *gin.Context) string {
	v, _ := c.Get(ctxUsername)
	s, _ := v.(string)
	return s
}

// Role returns the authenticated role.
func Role(c *gin.Context) string {
	v, _ := c.Get(ctxRole)
	s, _ := v.(string)
	return s
}
