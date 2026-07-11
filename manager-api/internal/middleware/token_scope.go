package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const apiPrefix = "/api/v1"

// tokenScopePrefixes maps a coarse read scope to the request-path prefixes it
// grants (SND-31). "read:*" (or "*") grants everything and bypasses this map.
var tokenScopePrefixes = map[string][]string{
	"fleet":     {apiPrefix + "/admin/servers", apiPrefix + "/admin/dashboard"},
	"monitor":   {apiPrefix + "/admin/monitor"},
	"inventory": {apiPrefix + "/admin/users", apiPrefix + "/admin/domains", apiPrefix + "/admin/mail"},
	"metrics":   {apiPrefix + "/metrics/prometheus"},
	"audit":     {apiPrefix + "/admin/audit"},
	"backups":   {apiPrefix + "/admin/backups"},
}

// TokenScopeNames returns the recognised scope names (for validation/UI).
func TokenScopeNames() []string {
	out := make([]string, 0, len(tokenScopePrefixes)+1)
	out = append(out, "read:*")
	for k := range tokenScopePrefixes {
		out = append(out, k)
	}
	return out
}

// alwaysAllowedForToken are low-sensitivity endpoints any valid token may reach
// regardless of scopes (build/version introspection).
var alwaysAllowedForToken = []string{apiPrefix + "/version"}

// TokenScopeGuard restricts API-token requests to their granted read scopes.
// JWT/session requests (no scopes in context) pass through untouched.
func TokenScopeGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get(ctxTokenScopes)
		if !ok {
			c.Next() // not a token-authenticated request
			return
		}
		scopes, _ := v.([]string)
		// Empty or wildcard scopes grant everything.
		if len(scopes) == 0 {
			c.Next()
			return
		}
		for _, s := range scopes {
			if s == "read:*" || s == "*" {
				c.Next()
				return
			}
		}
		path := c.Request.URL.Path
		for _, p := range alwaysAllowedForToken {
			if strings.HasPrefix(path, p) {
				c.Next()
				return
			}
		}
		for _, s := range scopes {
			for _, prefix := range tokenScopePrefixes[s] {
				if strings.HasPrefix(path, prefix) {
					c.Next()
					return
				}
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient_token_scope"})
	}
}
