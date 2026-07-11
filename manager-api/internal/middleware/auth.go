// Package middleware provides Gin middleware for the manager.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// Claims is the Sounder JWT payload: standard claims plus the operator role.
type Claims struct {
	Role      string `json:"role"`
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

// SessionCheck reports whether a session id is still active (not revoked/
// expired). nil disables the server-side session check (stateless JWT).
type SessionCheck func(ctx context.Context, sessionID string) bool

// APITokenCheck validates a presented API token, returning (id, name, ok). nil
// disables API-token auth (JWT only).
type APITokenCheck func(ctx context.Context, token, clientIP string) (id, name string, scopes []string, ok bool)

// Context keys for storing admin identity.
const (
	ctxAdminID     = "admin_id"
	ctxAdminUser   = "admin_username"
	ctxAdminRole   = "admin_role"
	ctxTokenScopes = "token_scopes"
	ctxSession     = "session_id"
)

// AuthMiddleware verifies a JWT Bearer token and sets the admin identity
// in the gin context. Returns 401 if missing/invalid.
func AuthMiddleware(secret string, sessions SessionCheck, apiTokens APITokenCheck) gin.HandlerFunc {
	// Fail closed: an empty secret means auth is misconfigured. Reject every
	// request rather than serve the protected surface unauthenticated (and an
	// attacker could otherwise forge a token by HMAC-signing with the empty key).
	if secret == "" {
		return func(c *gin.Context) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "server auth is misconfigured"})
		}
	}
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		if !strings.HasPrefix(raw, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authorization header"})
			return
		}
		tokenStr := raw[len("Bearer "):]

		// API token (M4): read-only credential for external tooling. Grants the
		// viewer role and skips the JWT/session path. Only where enabled.
		if apiTokens != nil && isAPIToken(tokenStr) {
			id, name, scopes, ok := apiTokens(c.Request.Context(), tokenStr, c.ClientIP())
			if !ok {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
				return
			}
			c.Set(ctxAdminID, "apitoken:"+id)
			c.Set(ctxAdminUser, "token:"+name)
			c.Set(ctxAdminRole, string(models.RoleViewer))
			c.Set(ctxTokenScopes, scopes)
			c.Next()
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		// Server-side session check (M3): reject revoked/expired sessions even
		// while the JWT itself is still cryptographically valid.
		if sessions != nil && !sessions(c.Request.Context(), claims.SessionID) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session revoked or expired"})
			return
		}

		// claims.Subject = admin ID; claims.ID = username; Role = permission level.
		c.Set(ctxAdminID, claims.Subject)
		c.Set(ctxAdminUser, claims.ID)
		c.Set(ctxAdminRole, claims.Role)
		c.Set(ctxSession, claims.SessionID)
		c.Next()
	}
}

// AdminID returns the authenticated admin's ID from the context, or "".
func AdminID(c *gin.Context) string {
	v, _ := c.Get(ctxAdminID)
	s, _ := v.(string)
	return s
}

// AdminUsername returns the authenticated admin's username from the context.
func AdminUsername(c *gin.Context) string {
	v, _ := c.Get(ctxAdminUser)
	s, _ := v.(string)
	return s
}

// AdminRole returns the authenticated admin's role from the context, or "".
func AdminRole(c *gin.Context) string {
	v, _ := c.Get(ctxAdminRole)
	s, _ := v.(string)
	return s
}

func isAPIToken(s string) bool { return len(s) > 4 && s[:4] == "snd_" }

// AdminSessionID returns the current session id from the context, or "".
func AdminSessionID(c *gin.Context) string {
	v, _ := c.Get(ctxSession)
	s, _ := v.(string)
	return s
}

// RequireRole aborts with 403 unless the authenticated admin's role is at least
// min. Must run after AuthMiddleware (M3: RBAC).
func RequireRole(min models.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !models.Role(AdminRole(c)).AtLeast(min) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

// MintToken creates a signed JWT for the given admin.
func MintToken(secret, adminID, username string, role models.Role, sessionID string, ttl time.Duration) (string, time.Time, error) {
	expiresAt := time.Now().Add(ttl)
	claims := &Claims{
		Role:      string(role),
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   adminID,
			ID:        username,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "jabali-sounder",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// Ensure context import is used.
var _ = context.Background
