package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DefaultMaxBodyBytes is the fallback request-body cap (1 MiB) when none is
// configured.
const DefaultMaxBodyBytes int64 = 1 << 20

// BodyLimit caps the size of any request body to guard against memory-exhaustion
// DoS (SND-5). A request declaring a larger Content-Length is rejected up front
// with 413; bodies without a declared length are wrapped in a MaxBytesReader so
// reads abort past the cap instead of buffering unbounded memory. maxBytes <= 0
// falls back to DefaultMaxBodyBytes.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request_body_too_large"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
