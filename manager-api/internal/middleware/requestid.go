package middleware

import (
	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
)

// RequestIDKey is the gin-context key holding the per-request correlation ID.
const RequestIDKey = "request_id"

// RequestIDHeader is the response (and optional inbound) header carrying it.
const RequestIDHeader = "X-Request-ID"

// RequestID assigns a correlation ID to each request (reusing a caller-supplied
// X-Request-ID when present) and echoes it back. Error responses return this ID
// so an operator can find the matching server-side log line without the API
// leaking internal error details (SND-2).
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(RequestIDHeader)
		if rid == "" {
			rid = ids.NewULID()
		}
		c.Set(RequestIDKey, rid)
		c.Header(RequestIDHeader, rid)
		c.Next()
	}
}

// GetRequestID returns the correlation ID for the current request, or "".
func GetRequestID(c *gin.Context) string {
	v, _ := c.Get(RequestIDKey)
	s, _ := v.(string)
	return s
}
