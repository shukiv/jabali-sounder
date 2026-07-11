package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func scopeRouter(scopes []string, hasToken bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if hasToken {
			c.Set(ctxTokenScopes, scopes)
		}
		c.Next()
	})
	r.Use(TokenScopeGuard())
	ok := func(c *gin.Context) { c.String(http.StatusOK, "ok") }
	r.GET("/api/v1/admin/servers", ok)
	r.GET("/api/v1/admin/audit", ok)
	r.GET("/api/v1/metrics/prometheus", ok)
	r.GET("/api/v1/version", ok)
	return r
}

func get(r *gin.Engine, path string) int {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w.Code
}

func TestScopeGuardAllowsJWT(t *testing.T) {
	r := scopeRouter(nil, false) // not a token request
	if c := get(r, "/api/v1/admin/audit"); c != http.StatusOK {
		t.Fatalf("JWT request must pass, got %d", c)
	}
}

func TestScopeGuardWildcard(t *testing.T) {
	r := scopeRouter([]string{"read:*"}, true)
	for _, p := range []string{"/api/v1/admin/servers", "/api/v1/admin/audit", "/api/v1/metrics/prometheus"} {
		if c := get(r, p); c != http.StatusOK {
			t.Fatalf("read:* must allow %s, got %d", p, c)
		}
	}
}

func TestScopeGuardRestricts(t *testing.T) {
	r := scopeRouter([]string{"fleet"}, true)
	if c := get(r, "/api/v1/admin/servers"); c != http.StatusOK {
		t.Fatalf("fleet scope must allow /admin/servers, got %d", c)
	}
	if c := get(r, "/api/v1/admin/audit"); c != http.StatusForbidden {
		t.Fatalf("fleet scope must forbid /admin/audit, got %d", c)
	}
	// Version is always allowed regardless of scope.
	if c := get(r, "/api/v1/version"); c != http.StatusOK {
		t.Fatalf("version must always be allowed, got %d", c)
	}
}

func TestScopeGuardEmptyScopesAllowAll(t *testing.T) {
	r := scopeRouter([]string{}, true) // token with no scopes = full read
	if c := get(r, "/api/v1/admin/audit"); c != http.StatusOK {
		t.Fatalf("empty scopes must allow all, got %d", c)
	}
}
