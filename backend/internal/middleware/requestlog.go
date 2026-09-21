package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// statusWriter captures the response status code for logging.
type statusWriter struct {
	gin.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// RequestLog logs request_id, method, path, status and latency_ms.
func RequestLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = c.GetString("request_id")
		}
		start := time.Now()
		writer := &statusWriter{ResponseWriter: c.Writer, status: 200}
		c.Writer = writer
		c.Next()
		latency := time.Since(start).Milliseconds()
		logger.Info("http_request",
			"request_id", requestID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", writer.status,
			"latency_ms", latency,
		)
	}
}
