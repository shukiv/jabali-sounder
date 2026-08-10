package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/descope/virtualwebauthn"
	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// TestPasskeyRegisterAndDiscoverableLogin drives the full WebAuthn ceremony
// against the real handlers using a virtual authenticator: password login →
// register a passkey → passwordless (discoverable) login → it is listed.
func TestPasskeyRegisterAndDiscoverableLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()

	dbPath := filepath.Join(t.TempDir(), "pk.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	g, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	adminRepo := repository.NewAdminRepository(g)
	admin, err := NewAdmin("admin", "correct-horse-battery", models.RoleOwner)
	if err != nil {
		t.Fatalf("new admin: %v", err)
	}
	if err := adminRepo.Create(ctx, admin); err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	r := gin.New()
	r.Use(middleware.RequestID())
	RegisterAuthRoutes(r.Group("/api/v1"), AuthHandlerConfig{
		AdminRepo:        adminRepo,
		JWTSecret:        "test-jwt-secret-not-empty-0000",
		JWTTTL:           time.Hour,
		ExtendedTTL:      240 * time.Hour,
		SessionRepo:      repository.NewSessionRepository(g),
		WebAuthnCredRepo: repository.NewWebAuthnCredentialRepository(g),
		AllowPlaintext:   true,
		LoginMaxFailures: 100,
		LoginLockout:     time.Hour,
		LoginWindow:      time.Hour,
	})

	do := func(method, path, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Host = "example.com"
		req.Header.Set("X-Forwarded-Proto", "https")
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	tokenOf := func(w *httptest.ResponseRecorder) string {
		var resp struct {
			Token string `json:"token"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		return resp.Token
	}
	beginOf := func(w *httptest.ResponseRecorder) (publicKey json.RawMessage, session string) {
		var resp struct {
			PublicKey json.RawMessage `json:"publicKey"`
			Session   string          `json:"session"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode begin: %v (%s)", err, w.Body)
		}
		return resp.PublicKey, resp.Session
	}

	// Password login (needed to authorize passkey registration).
	w := do("POST", "/api/v1/auth/login", `{"username":"admin","password":"correct-horse-battery"}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("password login: %d %s", w.Code, w.Body)
	}
	token := tokenOf(w)

	rp := virtualwebauthn.RelyingParty{ID: "example.com", Name: "Jabali Sounder", Origin: "https://example.com"}
	authr := virtualwebauthn.NewAuthenticator()
	authr.Options.UserHandle = []byte(admin.ID) // returned as userHandle on discoverable login
	cred := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)

	// --- registration ---
	w = do("POST", "/api/v1/auth/passkeys/register/begin", `{"password":"correct-horse-battery"}`, token)
	if w.Code != http.StatusOK {
		t.Fatalf("register begin: %d %s", w.Code, w.Body)
	}
	pk, session := beginOf(w)
	attOpts, err := virtualwebauthn.ParseAttestationOptions(`{"publicKey":` + string(pk) + `}`)
	if err != nil {
		t.Fatalf("parse attestation options: %v", err)
	}
	attResp := virtualwebauthn.CreateAttestationResponse(rp, authr, cred, *attOpts)
	w = do("POST", "/api/v1/auth/passkeys/register/finish?session="+session+"&label=test-key", attResp, token)
	if w.Code != http.StatusOK {
		t.Fatalf("register finish: %d %s", w.Code, w.Body)
	}
	authr.AddCredential(cred)

	// --- passwordless (discoverable) login ---
	w = do("POST", "/api/v1/auth/passkeys/login/begin", `{}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login begin: %d %s", w.Code, w.Body)
	}
	pk, session = beginOf(w)
	assOpts, err := virtualwebauthn.ParseAssertionOptions(`{"publicKey":` + string(pk) + `}`)
	if err != nil {
		t.Fatalf("parse assertion options: %v", err)
	}
	assResp := virtualwebauthn.CreateAssertionResponse(rp, authr, cred, *assOpts)
	w = do("POST", "/api/v1/auth/passkeys/login/finish?session="+session+"&remember=true", assResp, "")
	if w.Code != http.StatusOK {
		t.Fatalf("passkey login: %d %s", w.Code, w.Body)
	}
	if tokenOf(w) == "" {
		t.Fatal("passkey login returned no token")
	}

	// --- it is listed for the admin ---
	w = do("GET", "/api/v1/auth/passkeys", "", token)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "test-key") {
		t.Fatalf("list passkeys: %d %s", w.Code, w.Body)
	}
}
