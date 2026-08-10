package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"golang.org/x/crypto/bcrypt"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// passkeyChallengeTTL bounds how long a begun ceremony may sit before finish.
const passkeyChallengeTTL = 2 * time.Minute

// passkeySessionStore keeps WebAuthn SessionData between the begin and finish
// halves of a ceremony. Single-process, so an in-memory map is enough; entries
// are single-use (removed on take) and expire after passkeyChallengeTTL.
type passkeySessionStore struct {
	mu sync.Mutex
	m  map[string]passkeySession
}

type passkeySession struct {
	data    *webauthn.SessionData
	expires time.Time
}

func newPasskeySessionStore() *passkeySessionStore {
	return &passkeySessionStore{m: make(map[string]passkeySession)}
}

func (s *passkeySessionStore) put(data *webauthn.SessionData) string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	id := hex.EncodeToString(buf)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.m { // opportunistic GC
		if now.After(v.expires) {
			delete(s.m, k)
		}
	}
	s.m[id] = passkeySession{data: data, expires: now.Add(passkeyChallengeTTL)}
	return id
}

// take returns the session for id and removes it (single use). nil if missing
// or expired.
func (s *passkeySessionStore) take(id string) *webauthn.SessionData {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[id]
	if !ok {
		return nil
	}
	delete(s.m, id)
	if time.Now().After(v.expires) {
		return nil
	}
	return v.data
}

// webauthnUser adapts an admin + its stored passkeys to the go-webauthn User
// interface. The user handle is the admin ID bytes (stable, never the username).
type webauthnUser struct {
	id    []byte
	name  string
	creds []webauthn.Credential
}

func (u *webauthnUser) WebAuthnID() []byte                         { return u.id }
func (u *webauthnUser) WebAuthnName() string                       { return u.name }
func (u *webauthnUser) WebAuthnDisplayName() string                { return u.name }
func (u *webauthnUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func (h *authHandler) webauthnUserFor(ctx context.Context, admin *models.Admin) (*webauthnUser, error) {
	rows, err := h.cfg.WebAuthnCredRepo.ListByAdmin(ctx, admin.ID)
	if err != nil {
		return nil, err
	}
	creds := make([]webauthn.Credential, 0, len(rows))
	for _, r := range rows {
		var cr webauthn.Credential
		if json.Unmarshal(r.Data, &cr) == nil {
			creds = append(creds, cr)
		}
	}
	return &webauthnUser{id: []byte(admin.ID), name: admin.Username, creds: creds}, nil
}

// newWebAuthn builds a relying party bound to this request's origin. RP id and
// origin come from config when set, else are derived from the request — behind
// the panel reverse proxy that means the browser's Host + X-Forwarded-Proto.
func (h *authHandler) rpConfig(c *gin.Context) (rpID, origin string) {
	host := c.Request.Host
	rpID = h.cfg.WebAuthnRPID
	if rpID == "" {
		rpID = host
		if i := strings.IndexByte(rpID, ':'); i >= 0 {
			rpID = rpID[:i]
		}
	}
	origin = h.cfg.WebAuthnOrigin
	if origin == "" {
		scheme := "https"
		if xf := c.GetHeader("X-Forwarded-Proto"); xf != "" {
			scheme = xf
		} else if c.Request.TLS == nil {
			scheme = "http"
		}
		origin = scheme + "://" + host
	}
	return rpID, origin
}

func (h *authHandler) newWebAuthn(c *gin.Context) (*webauthn.WebAuthn, error) {
	rpID, origin := h.rpConfig(c)
	return webauthn.New(&webauthn.Config{
		RPID:                  rpID,
		RPDisplayName:         "Jabali Sounder",
		RPOrigins:             []string{origin},
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationRequired,
		},
	})
}

// --- registration (authenticated; requires the current password) ---

func (h *authHandler) passkeyRegisterBegin(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	admin, ok := h.currentAdmin(c)
	if !ok {
		return
	}
	// Re-verify the password: a hijacked session must not silently enroll a
	// persistent credential (mirrors the 2FA-disable posture).
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "current_password_incorrect"})
		return
	}
	wa, err := h.newWebAuthn(c)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	user, err := h.webauthnUserFor(c.Request.Context(), admin)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	options, session, err := wa.BeginRegistration(user)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"publicKey": options.Response, "session": h.passkeys.put(session)})
}

func (h *authHandler) passkeyRegisterFinish(c *gin.Context) {
	session := h.passkeys.take(c.Query("session"))
	if session == nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_session"})
		return
	}
	admin, ok := h.currentAdmin(c)
	if !ok {
		return
	}
	wa, err := h.newWebAuthn(c)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	user, err := h.webauthnUserFor(c.Request.Context(), admin)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	cred, err := wa.FinishRegistration(user, *session, c.Request)
	if err != nil {
		h.cfg.Log.Warn("passkey registration failed", "err", err)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "passkey_registration_failed"})
		return
	}
	label := strings.TrimSpace(c.Query("label"))
	if label == "" {
		label = "Passkey"
	}
	if len(label) > 120 {
		label = label[:120]
	}
	data, err := json.Marshal(cred)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	row := &models.WebAuthnCredential{
		ID:           ids.NewULID(),
		AdminID:      admin.ID,
		CredentialID: cred.ID,
		Data:         data,
		Label:        label,
		CreatedAt:    time.Now().UTC(),
	}
	if err := h.cfg.WebAuthnCredRepo.Create(c.Request.Context(), row); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": row.ID, "label": row.Label})
}

// --- passwordless login (discoverable credential) ---

func (h *authHandler) passkeyLoginBegin(c *gin.Context) {
	wa, err := h.newWebAuthn(c)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	options, session, err := wa.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"publicKey": options.Response, "session": h.passkeys.put(session)})
}

func (h *authHandler) passkeyLoginFinish(c *gin.Context) {
	session := h.passkeys.take(c.Query("session"))
	if session == nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_session"})
		return
	}
	wa, err := h.newWebAuthn(c)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	ctx := c.Request.Context()
	var loggedIn *models.Admin
	handler := func(_, userHandle []byte) (webauthn.User, error) {
		admin, err := h.cfg.AdminRepo.FindByID(ctx, string(userHandle))
		if err != nil {
			return nil, err
		}
		u, err := h.webauthnUserFor(ctx, admin)
		if err != nil {
			return nil, err
		}
		loggedIn = admin
		return u, nil
	}
	cred, err := wa.FinishDiscoverableLogin(handler, *session, c.Request)
	if err != nil || loggedIn == nil {
		h.cfg.Log.Warn("passkey login failed", "err", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials"})
		return
	}
	// Persist the advanced sign counter (best-effort; don't fail the login).
	if row, ferr := h.cfg.WebAuthnCredRepo.FindByCredentialID(ctx, cred.ID); ferr == nil {
		if data, merr := json.Marshal(cred); merr == nil {
			_ = h.cfg.WebAuthnCredRepo.UpdateData(ctx, row.ID, data, time.Now().UTC())
		}
	}
	role := loggedIn.Role
	if !role.Valid() {
		role = models.RoleViewer
	}
	h.acctFail.RecordSuccess(loggedIn.Username)
	token, expiresAt, err := h.issueSession(c, loggedIn, role, c.Query("remember") == "true")
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"expires_at": expiresAt,
		"admin":      gin.H{"id": loggedIn.ID, "username": loggedIn.Username, "role": role},
	})
}

// --- management ---

func (h *authHandler) listPasskeys(c *gin.Context) {
	rows, err := h.cfg.WebAuthnCredRepo.ListByAdmin(c.Request.Context(), middleware.AdminID(c))
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		item := gin.H{"id": r.ID, "label": r.Label, "created_at": r.CreatedAt}
		if r.LastUsedAt.Valid {
			item["last_used_at"] = r.LastUsedAt.Time
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{"passkeys": out})
}

func (h *authHandler) deletePasskey(c *gin.Context) {
	removed, err := h.cfg.WebAuthnCredRepo.Delete(c.Request.Context(), c.Param("id"), middleware.AdminID(c))
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	if !removed {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
