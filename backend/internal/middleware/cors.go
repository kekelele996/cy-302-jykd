package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS adds permissive CORS headers for local development.
func CORS(allowOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := allowOrigin
		if origin == "" || origin == "*" {
			origin = "*"
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, Accept")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
