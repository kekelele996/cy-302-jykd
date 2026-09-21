package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/pkg/httpx"
)

// RequireRole restricts access to one of the supplied roles.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		current := Role(c)
		for _, role := range roles {
			if role == current {
				c.Next()
				return
			}
		}
		httpx.Fail(c, http.StatusForbidden, constants.CodeForbidden, "无权限访问该资源")
		c.Abort()
	}
}
