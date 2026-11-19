package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	RequestIDKey    = "request_id"
	RequestIDHeader = "X-Request-ID"
	maxRequestIDLen = 128
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(RequestIDHeader)

		// Don't blindly trust client-supplied IDs: they end up in logs and
		// response headers.
		if requestID == "" || len(requestID) > maxRequestIDLen || !isPrintableASCII(requestID) {
			requestID = uuid.NewString()
		}

		c.Set(RequestIDKey, requestID)
		c.Header(RequestIDHeader, requestID)

		c.Next()
	}
}

func isPrintableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x21 || s[i] > 0x7e {
			return false
		}
	}
	return true
}
