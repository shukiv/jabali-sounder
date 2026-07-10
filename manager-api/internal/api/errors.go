package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
)

// failInternal logs the real error server-side (with the request correlation ID)
// and returns an opaque 500 to the caller — never leaking err.Error(), GORM/OS
// internals, or crypto details to API consumers (SND-2).
func failInternal(c *gin.Context, log *slog.Logger, err error) {
	failCode(c, log, http.StatusInternalServerError, "internal_error", err)
}

// failCode is the general form: log err server-side, return {error, request_id}.
// A nil err still returns the code but logs nothing (used for expected 4xx).
func failCode(c *gin.Context, log *slog.Logger, status int, code string, err error) {
	if log == nil {
		log = slog.Default()
	}
	rid := middleware.GetRequestID(c)
	if err != nil {
		log.Error("api error",
			"code", code,
			"status", status,
			"request_id", rid,
			"method", c.Request.Method,
			"path", c.FullPath(),
			"error", err,
		)
	}
	c.JSON(status, gin.H{"error": code, "request_id": rid})
}
