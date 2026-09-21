package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/pkg/httpx"
)

// Recovery converts panics into a unified 500 response.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				_ = debug.Stack()
				httpx.Fail(c, http.StatusInternalServerError, constants.CodeInternal, "服务器内部错误")
				c.Abort()
			}
		}()
		c.Next()
	}
}
