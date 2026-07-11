package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
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

// safeRemoteError logs a managed-panel fetch failure server-side and returns a
// user-safe summary for the per-server "error" field in monitor/mail responses,
// never leaking the raw error (dial errors expose internal IPs; crypto errors
// expose key state) to API consumers (SND-9).
func safeRemoteError(log *slog.Logger, server, part string, code int, err error) string {
	if log == nil {
		log = slog.Default()
	}
	log.Warn("remote fetch failed", "server", server, "part", part, "code", code, "error", err)
	if code > 0 {
		return part + " unavailable (HTTP " + strconv.Itoa(code) + ")"
	}
	return part + " unavailable"
}

// auditServerMutation emits a structured audit record for a privileged server
// mutation (SND-10): the acting admin, the action, the target server, source IP,
// and correlation ID. Never logs secret material. When an audit repository is
// supplied it also persists the record so the trail is queryable (SND-24);
// persistence failure is non-fatal to the mutation.
func auditServerMutation(log *slog.Logger, audit repository.AuditRepository, c *gin.Context, action, serverID, serverName string) {
	if log == nil {
		log = slog.Default()
	}
	event := "server." + action
	actor := middleware.AdminUsername(c)
	actorID := middleware.AdminID(c)
	sourceIP := c.ClientIP()
	requestID := middleware.GetRequestID(c)

	log.Info("audit",
		"event", event,
		"actor", actor,
		"actor_id", actorID,
		"server_id", serverID,
		"server_name", serverName,
		"source_ip", sourceIP,
		"request_id", requestID,
	)
	if audit == nil {
		return
	}
	if err := audit.Create(c.Request.Context(), &models.AuditLog{
		ID:         ids.NewULID(),
		Event:      event,
		Actor:      actor,
		ActorID:    actorID,
		ServerID:   serverID,
		ServerName: serverName,
		SourceIP:   sourceIP,
		RequestID:  requestID,
		CreatedAt:  time.Now().UTC(),
	}); err != nil {
		log.Warn("audit persist failed", "event", event, "error", err)
	}
}
