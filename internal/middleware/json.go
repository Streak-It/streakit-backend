package middleware

import (
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/response"
)

// RequireJSON rejects state-changing requests whose Content-Type is not
// application/json. HTML forms cannot send that type cross-site without a
// CORS preflight, which closes the "simple request" CSRF vector.
func RequireJSON() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			mt, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
			if err != nil || mt != "application/json" {
				response.Error(c, apperror.UnsupportedMediaType())
				return
			}
		}
		c.Next()
	}
}

// BodyLimit caps request bodies to n bytes.
func BodyLimit(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		}
		c.Next()
	}
}
