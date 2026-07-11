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
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// Context keys for storing admin identity.
const (
	ctxAdminID   = "admin_id"
	ctxAdminUser = "admin_username"
	ctxAdminRole = "admin_role"
)

// AuthMiddleware verifies a JWT Bearer token and sets the admin identity
// in the gin context. Returns 401 if missing/invalid.
func AuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		if !strings.HasPrefix(raw, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authorization header"})
			return
		}
		tokenStr := raw[len("Bearer "):]

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

		// claims.Subject = admin ID; claims.ID = username; Role = permission level.
		c.Set(ctxAdminID, claims.Subject)
		c.Set(ctxAdminUser, claims.ID)
		c.Set(ctxAdminRole, claims.Role)
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
func MintToken(secret, adminID, username string, role models.Role, ttl time.Duration) (string, time.Time, error) {
	expiresAt := time.Now().Add(ttl)
	claims := &Claims{
		Role: string(role),
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
