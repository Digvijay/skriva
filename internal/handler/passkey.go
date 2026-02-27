// Package handler — WebAuthn passkey support for passwordless 2FA.
// Implements registration and login using the go-webauthn library.
package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/Digvijay/skriva/internal/store"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// webAuthnUser implements the webauthn.User interface for the admin user.
type webAuthnUser struct {
	id          []byte
	name        string
	displayName string
	credentials []webauthn.Credential
}

func (u *webAuthnUser) WebAuthnID() []byte                         { return u.id }
func (u *webAuthnUser) WebAuthnName() string                       { return u.name }
func (u *webAuthnUser) WebAuthnDisplayName() string                { return u.displayName }
func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

// webAuthnState holds in-memory session data for WebAuthn ceremonies.
// Uses a map keyed by challenge to prevent DoS via ceremony overwrites.
type webAuthnState struct {
	mu       sync.Mutex
	sessions map[string]*webauthn.SessionData
}

var waState = &webAuthnState{sessions: make(map[string]*webauthn.SessionData)}

func (ws *webAuthnState) store(session *webauthn.SessionData) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	// Limit stored sessions to prevent memory abuse
	if len(ws.sessions) > 10 {
		for k := range ws.sessions {
			delete(ws.sessions, k)
			break
		}
	}
	ws.sessions[session.Challenge] = session
}

func (ws *webAuthnState) consume(challenge string) *webauthn.SessionData {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	s := ws.sessions[challenge]
	delete(ws.sessions, challenge)
	return s
}

// initWebAuthn creates a WebAuthn instance from the site config.
func (h *BlogHandler) initWebAuthn() (*webauthn.WebAuthn, error) {
	site := h.cfg.GetSite()
	rpID := "localhost"
	origin := "http://localhost:8080"

	if site.BaseURL != "" {
		origin = site.BaseURL
		// Extract hostname for RPID
		if idx := len("https://"); len(site.BaseURL) > idx && site.BaseURL[:idx] == "https://" {
			rpID = site.BaseURL[idx:]
		} else if idx := len("http://"); len(site.BaseURL) > idx && site.BaseURL[:idx] == "http://" {
			rpID = site.BaseURL[idx:]
		}
		// Strip port if present
		for i, c := range rpID {
			if c == ':' || c == '/' {
				rpID = rpID[:i]
				break
			}
		}
	}

	displayName := site.Title
	if displayName == "" {
		displayName = "Blog Admin"
	}

	return webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: displayName,
		RPOrigins:     []string{origin},
	})
}

// getWebAuthnUser builds the admin user with credentials from the DB.
func (h *BlogHandler) getWebAuthnUser() (*webAuthnUser, error) {
	passkeys, err := h.store.AllPasskeys(context.Background())
	if err != nil {
		return nil, fmt.Errorf("loading passkeys: %w", err)
	}

	var creds []webauthn.Credential
	for i := range passkeys {
		transport := make([]protocol.AuthenticatorTransport, len(passkeys[i].Transports))
		for j, t := range passkeys[i].Transports {
			transport[j] = protocol.AuthenticatorTransport(t)
		}
		creds = append(creds, webauthn.Credential{
			ID:              passkeys[i].CredentialID,
			PublicKey:       passkeys[i].PublicKey,
			AttestationType: passkeys[i].AttestationType,
			Authenticator: webauthn.Authenticator{
				AAGUID:    passkeys[i].AAGUID,
				SignCount: passkeys[i].SignCount,
			},
			Transport: transport,
		})
	}

	return &webAuthnUser{
		id:          []byte("admin"),
		name:        "admin",
		displayName: "Blog Admin",
		credentials: creds,
	}, nil
}

// --- WebAuthn Registration ---

// HandleAdminAPIPasskeyBeginRegister starts passkey registration.
func (h *BlogHandler) HandleAdminAPIPasskeyBeginRegister(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	wa, err := h.initWebAuthn()
	if err != nil {
		h.logger.Error("initializing webauthn", "error", err)
		jsonError(w, "WebAuthn configuration error", http.StatusInternalServerError)
		return
	}

	user, err := h.getWebAuthnUser()
	if err != nil {
		h.logger.Error("loading user for webauthn", "error", err)
		jsonError(w, "Failed to load credentials", http.StatusInternalServerError)
		return
	}

	creation, session, err := wa.BeginRegistration(user,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred),
	)
	if err != nil {
		h.logger.Error("beginning webauthn registration", "error", err)
		jsonError(w, "Failed to begin registration", http.StatusInternalServerError)
		return
	}

	waState.store(session)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(creation)
}

// HandleAdminAPIPasskeyFinishRegister completes passkey registration.
func (h *BlogHandler) HandleAdminAPIPasskeyFinishRegister(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	wa, err := h.initWebAuthn()
	if err != nil {
		jsonError(w, "WebAuthn configuration error", http.StatusInternalServerError)
		return
	}

	user, err := h.getWebAuthnUser()
	if err != nil {
		jsonError(w, "Failed to load credentials", http.StatusInternalServerError)
		return
	}

	// We need the session from the begin step.
	// Parse the client response to extract the challenge for precise session matching.
	var parsedResp struct {
		Response struct {
			ClientDataJSON string `json:"clientDataJSON"`
		} `json:"response"`
	}

	// Read the request body to peek at clientDataJSON, then reset it for FinishRegistration
	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Try to extract challenge from clientDataJSON
	var challengeKey string
	if json.Unmarshal(bodyBytes, &parsedResp) == nil && parsedResp.Response.ClientDataJSON != "" {
		cdJSON, err := base64.RawURLEncoding.DecodeString(parsedResp.Response.ClientDataJSON)
		if err == nil {
			var cd struct {
				Challenge string `json:"challenge"`
			}
			if json.Unmarshal(cdJSON, &cd) == nil {
				challengeKey = cd.Challenge
			}
		}
	}

	// Look up session by challenge; fall back to most recent if parsing failed
	var session *webauthn.SessionData
	if challengeKey != "" {
		session = waState.consume(challengeKey)
	}
	if session == nil {
		// Fallback: consume any session (single-admin, only one ceremony at a time)
		waState.mu.Lock()
		for _, s := range waState.sessions {
			session = s
			break
		}
		if session != nil {
			delete(waState.sessions, session.Challenge)
		}
		waState.mu.Unlock()
	}

	if session == nil {
		jsonError(w, "No registration in progress", http.StatusBadRequest)
		return
	}

	cred, err := wa.FinishRegistration(user, *session, r)
	if err != nil {
		h.logger.Error("finishing webauthn registration", "error", err)
		jsonError(w, "Registration failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Get name from query param
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "Passkey"
	}

	transports := make([]string, len(cred.Transport))
	for i, t := range cred.Transport {
		transports[i] = string(t)
	}

	passkey := &store.Passkey{
		Name:            name,
		CredentialID:    cred.ID,
		PublicKey:       cred.PublicKey,
		AttestationType: cred.AttestationType,
		AAGUID:          cred.Authenticator.AAGUID,
		SignCount:       cred.Authenticator.SignCount,
		Transports:      transports,
	}

	if err := h.store.AddPasskey(r.Context(), passkey); err != nil {
		h.logger.Error("saving passkey", "error", err)
		jsonError(w, "Failed to save passkey", http.StatusInternalServerError)
		return
	}

	h.logger.Info("passkey registered", "name", name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"id":     passkey.ID,
		"name":   passkey.Name,
	})
}

// --- WebAuthn Login ---

// HandleAdminAPIPasskeyBeginLogin starts passkey authentication.
func (h *BlogHandler) HandleAdminAPIPasskeyBeginLogin(w http.ResponseWriter, r *http.Request) {
	wa, err := h.initWebAuthn()
	if err != nil {
		jsonError(w, "WebAuthn configuration error", http.StatusInternalServerError)
		return
	}

	user, err := h.getWebAuthnUser()
	if err != nil || len(user.credentials) == 0 {
		// SECURITY: Return generic error to prevent enumeration of passkey existence.
		// Attacker should not be able to distinguish "no passkeys" from "error".
		jsonError(w, "Passkey login unavailable", http.StatusBadRequest)
		return
	}

	assertion, session, err := wa.BeginLogin(user)
	if err != nil {
		h.logger.Error("beginning webauthn login", "error", err)
		jsonError(w, "Failed to begin login", http.StatusInternalServerError)
		return
	}

	waState.store(session)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assertion)
}

// HandleAdminAPIPasskeyFinishLogin completes passkey authentication and creates a session.
func (h *BlogHandler) HandleAdminAPIPasskeyFinishLogin(w http.ResponseWriter, r *http.Request) {
	wa, err := h.initWebAuthn()
	if err != nil {
		jsonError(w, "WebAuthn configuration error", http.StatusInternalServerError)
		return
	}

	user, err := h.getWebAuthnUser()
	if err != nil {
		jsonError(w, "Failed to load credentials", http.StatusInternalServerError)
		return
	}

	// Get the login session — match by challenge from client response
	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var challengeKey string
	var parsedLogin struct {
		Response struct {
			ClientDataJSON string `json:"clientDataJSON"`
		} `json:"response"`
	}
	if json.Unmarshal(bodyBytes, &parsedLogin) == nil && parsedLogin.Response.ClientDataJSON != "" {
		cdJSON, decErr := base64.RawURLEncoding.DecodeString(parsedLogin.Response.ClientDataJSON)
		if decErr == nil {
			var cd struct {
				Challenge string `json:"challenge"`
			}
			if json.Unmarshal(cdJSON, &cd) == nil {
				challengeKey = cd.Challenge
			}
		}
	}

	var session *webauthn.SessionData
	if challengeKey != "" {
		session = waState.consume(challengeKey)
	}
	if session == nil {
		waState.mu.Lock()
		for _, s := range waState.sessions {
			session = s
			break
		}
		if session != nil {
			delete(waState.sessions, session.Challenge)
		}
		waState.mu.Unlock()
	}

	if session == nil {
		jsonError(w, "No login in progress", http.StatusBadRequest)
		return
	}

	cred, err := wa.FinishLogin(user, *session, r)
	if err != nil {
		h.logger.Error("finishing webauthn login", "error", err)
		jsonError(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	// Update sign count
	_ = h.store.UpdatePasskeySignCount(r.Context(), cred.ID, cred.Authenticator.SignCount)

	// Issue session
	token := h.createSessionToken()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/admin/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionMaxAge.Seconds()),
	})

	h.logger.Info("passkey login successful", "remote", r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// --- Passkey Management ---

// HandleAdminAPIPasskeys lists all registered passkeys.
func (h *BlogHandler) HandleAdminAPIPasskeys(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	passkeys, err := h.store.AllPasskeys(r.Context())
	if err != nil {
		jsonError(w, "Failed to load passkeys", http.StatusInternalServerError)
		return
	}

	// Return only safe fields
	type passkeySafe struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
	}
	result := make([]passkeySafe, len(passkeys))
	for i := range passkeys {
		result[i] = passkeySafe{ID: passkeys[i].ID, Name: passkeys[i].Name, CreatedAt: passkeys[i].CreatedAt.Format("2006-01-02")}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleAdminAPIDeletePasskey removes a passkey.
func (h *BlogHandler) HandleAdminAPIDeletePasskey(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "Invalid passkey ID", http.StatusBadRequest)
		return
	}

	if err := h.store.DeletePasskey(r.Context(), id); err != nil {
		jsonError(w, "Failed to delete passkey", http.StatusInternalServerError)
		return
	}

	h.logger.Info("passkey deleted", "id", id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
