package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger writes one structured slog line per request, so HTTP logs share
// the same JSON format as application logs and carry the request ID.
// Must be registered after RequestID.
func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			"request_id", c.GetString(RequestIDKey),
			"method", c.Request.Method,
			"path", path,
			"route", c.FullPath(),
			"status", status,
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
			"bytes", c.Writer.Size(),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		}

		logger.Log(c.Request.Context(), level, "http request", attrs...)
	}
}
