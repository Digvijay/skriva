// auth.go — authentication, session management, CSRF, IP lockout.
package handler

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ipLockout tracks failed login attempts per IP and implements progressive lockout.
// After maxAttempts failures, the IP is locked for lockoutDuration.
type ipLockout struct {
	mu       sync.Mutex
	attempts map[string]*lockoutEntry
}

type lockoutEntry struct {
	failures int
	lockedAt time.Time
	lastFail time.Time
}

const (
	lockoutMaxAttempts = 10               // failures before lockout
	lockoutDuration    = 15 * time.Minute // initial lockout period
)

var loginLockout = &ipLockout{attempts: make(map[string]*lockoutEntry)}

// recordFailure records a failed login attempt. Returns true if the IP is now locked out.
func (l *ipLockout) recordFailure(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.attempts[ip]
	if !ok {
		entry = &lockoutEntry{}
		l.attempts[ip] = entry
	}

	// If lockout expired, reset
	if !entry.lockedAt.IsZero() && time.Since(entry.lockedAt) > lockoutDuration {
		entry.failures = 0
		entry.lockedAt = time.Time{}
	}

	entry.failures++
	entry.lastFail = time.Now()

	if entry.failures >= lockoutMaxAttempts {
		entry.lockedAt = time.Now()
		return true
	}
	return false
}

// isLockedOut returns true if the IP is currently in lockout.
func (l *ipLockout) isLockedOut(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.attempts[ip]
	if !ok {
		return false
	}

	if entry.lockedAt.IsZero() {
		return false
	}

	// Lockout expired?
	if time.Since(entry.lockedAt) > lockoutDuration {
		entry.failures = 0
		entry.lockedAt = time.Time{}
		return false
	}
	return true
}

// clearIP resets the failure count for an IP (called on successful login).
func (l *ipLockout) clearIP(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

// cleanup removes stale lockout entries. Called periodically.
func (l *ipLockout) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-1 * time.Hour)
	for ip, entry := range l.attempts {
		if entry.lastFail.Before(cutoff) {
			delete(l.attempts, ip)
		}
	}
}

// serveAdminHTML injects a CSRF meta tag into admin HTML and writes to response.
func (h *BlogHandler) serveAdminHTML(w http.ResponseWriter, html string) {
	token := h.generateCSRFToken()
	// Inject CSRF meta tag after <head> and a JS helper to attach it to all fetch calls
	csrfSnippet := `<meta name="csrf-token" content="` + token + `">
<script>!function(){const t=document.querySelector('meta[name="csrf-token"]').content;const orig=window.fetch;window.fetch=function(u,o){o=o||{};if(o.method&&o.method!=='GET'){o.headers=o.headers||{};if(o.headers instanceof Headers){o.headers.set('X-CSRF-Token',t)}else{o.headers['X-CSRF-Token']=t}}return orig.call(this,u,o)}}()</script>`
	result := strings.Replace(html, "<meta charset=\"utf-8\">", "<meta charset=\"utf-8\">\n"+csrfSnippet, 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, result)
}

// HandleAdmin serves the admin dashboard.
func (h *BlogHandler) HandleAdmin(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	h.serveAdminHTML(w, adminDashboardHTML)
}

// HandleAdminLogin serves the login page.
func (h *BlogHandler) HandleAdminLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, adminLoginHTML)
}

// audit logs an auditable admin event to the persistent audit trail.
func (h *BlogHandler) audit(r *http.Request, event, detail string) {
	ip := ""
	if r != nil {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}
	}
	ctx := context.Background()
	if err := h.store.LogAuditEvent(ctx, event, detail, ip); err != nil {
		h.logger.Error("audit log failed", "event", event, "error", err)
	}
}

// HandleAdminAPILogin handles admin authentication with optional TOTP.
func (h *BlogHandler) HandleAdminAPILogin(w http.ResponseWriter, r *http.Request) {
	// SECURITY: Limit request body to prevent memory abuse via oversized JSON
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16) // 64KB max

	// Check IP lockout
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	if loginLockout.isLockedOut(ip) {
		h.audit(r, "login.locked_out", "IP is locked out due to repeated failures")
		http.Error(w, "Too many failed login attempts. Please try again later.", http.StatusTooManyRequests)
		return
	}

	var req struct {
		Password string `json:"password"`
		TOTPCode string `json:"totp_code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if !h.cfg.VerifyAdminPassword(req.Password) {
		locked := loginLockout.recordFailure(ip)
		h.audit(r, "login.failed", "Invalid password")
		h.logger.Warn("failed admin login attempt", "remote", r.RemoteAddr)
		if locked {
			h.audit(r, "login.lockout_triggered", fmt.Sprintf("IP locked out after %d failures", lockoutMaxAttempts))
			http.Error(w, "Too many failed login attempts. Please try again later.", http.StatusTooManyRequests)
			return
		}
		jsonError(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	// Check TOTP if enabled
	if h.cfg.IsTOTPEnabled() {
		secrets := h.cfg.GetSecrets()
		if req.TOTPCode == "" {
			// Password correct but TOTP required — tell client to show TOTP field
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"totp_required": true, "error": "TOTP code required"})
			return
		}
		if !validateTOTP(secrets.TOTPSecret, req.TOTPCode) {
			loginLockout.recordFailure(ip)
			h.audit(r, "login.totp_failed", "Invalid TOTP code")
			h.logger.Warn("failed TOTP verification", "remote", r.RemoteAddr)
			jsonError(w, "Invalid TOTP code", http.StatusUnauthorized)
			return
		}
	}

	// Successful login — clear lockout and audit
	loginLockout.clearIP(ip)
	h.audit(r, "login.success", "Admin logged in")

	// Create session token
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

	h.logger.Info("admin login successful", "remote", r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleAdminAPILogout ends the admin session.
func (h *BlogHandler) HandleAdminAPILogout(w http.ResponseWriter, r *http.Request) {
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/admin/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	h.audit(r, "logout", "Admin logged out")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleAdminAPIChangePassword allows the admin to change their password.
func (h *BlogHandler) HandleAdminAPIChangePassword(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		jsonError(w, "Both current and new password are required", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < 8 {
		jsonError(w, "New password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// SECURITY: bcrypt silently truncates passwords longer than 72 bytes.
	// Reject overly long passwords to prevent a false sense of security.
	if len(req.NewPassword) > 72 {
		jsonError(w, "Password must not exceed 72 characters (bcrypt limit)", http.StatusBadRequest)
		return
	}

	// Verify current password
	if !h.cfg.VerifyAdminPassword(req.CurrentPassword) {
		jsonError(w, "Current password is incorrect", http.StatusUnauthorized)
		return
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("hashing new password", "error", err)
		jsonError(w, "Failed to process new password", http.StatusInternalServerError)
		return
	}

	// Save to secrets
	secrets := h.cfg.GetSecrets()
	secrets.AdminPasswordHash = string(hash)

	// SECURITY: Rotate session secret on password change to invalidate all existing sessions.
	// This ensures that stolen session tokens cannot be used after a password change.
	newSessionSecret := make([]byte, 32)
	rand.Read(newSessionSecret)
	secrets.SessionSecret = hex.EncodeToString(newSessionSecret)

	if err := h.cfg.UpdateSecrets(secrets); err != nil {
		h.logger.Error("saving new password", "error", err)
		jsonError(w, "Failed to save new password", http.StatusInternalServerError)
		return
	}

	// Issue a new session for the current admin with the new secret
	newToken := h.createSessionToken()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    newToken,
		Path:     "/admin/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionMaxAge.Seconds()),
	})

	h.logger.Info("admin password changed, session secret rotated")
	h.audit(r, "password.changed", "Admin password changed and session secret rotated")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// generateCSRFToken creates a signed CSRF token: timestamp.HMAC(secret, timestamp).
func (h *BlogHandler) generateCSRFToken() string {
	secrets := h.cfg.GetSecrets()
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte("csrf:"+secrets.SessionSecret))
	mac.Write([]byte(timestamp))
	signature := hex.EncodeToString(mac.Sum(nil))
	return timestamp + "." + signature
}

// validateCSRFToken verifies a CSRF token is valid and not expired.
func (h *BlogHandler) validateCSRFToken(token string) bool {
	if token == "" {
		return false
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	timestamp, signature := parts[0], parts[1]

	// Verify HMAC
	secrets := h.cfg.GetSecrets()
	mac := hmac.New(sha256.New, []byte("csrf:"+secrets.SessionSecret))
	mac.Write([]byte(timestamp))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return false
	}

	// Verify not expired
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	return time.Since(time.Unix(ts, 0)) < csrfTokenMaxAge
}

// csrfFromRequest extracts the CSRF token from form value or header.
func csrfFromRequest(r *http.Request) string {
	if token := r.FormValue("csrf_token"); token != "" {
		return token
	}
	return r.Header.Get("X-CSRF-Token")
}

func (h *BlogHandler) isAuthenticated(r *http.Request) bool {
	// Check session cookie first (full admin access)
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && h.verifySessionToken(cookie.Value) {
		return true
	}

	// Check Bearer token (API tokens — scope-limited)
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer sk_") {
		token := strings.TrimPrefix(auth, "Bearer ")
		tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
		apiToken, err := h.store.ValidateAPIToken(r.Context(), tokenHash)
		if err != nil {
			return false
		}
		scopes := apiToken.Scopes
		// Enforce scopes with exact matching to prevent substring attacks.
		// Write operations need "write" or "admin" scope.
		method := r.Method
		if method == "POST" || method == "PUT" || method == "DELETE" {
			if !hasScope(scopes, "write") && !hasScope(scopes, "admin") {
				return false
			}
		}
		// SECURITY: Sensitive admin endpoints (password, TOTP, passkeys, tokens,
		// subscribers, webhooks, settings) require "admin" scope.
		path := r.URL.Path
		sensitiveAdminPaths := []string{
			"/admin/api/password", "/admin/api/totp/", "/admin/api/passkeys/",
			"/admin/api/tokens", "/admin/api/subscribers", "/admin/api/webhooks",
			"/admin/api/settings", "/admin/api/export",
		}
		for _, prefix := range sensitiveAdminPaths {
			if strings.HasPrefix(path, prefix) {
				if !hasScope(scopes, "admin") {
					return false
				}
				break
			}
		}
		return true
	}

	return false
}

func (h *BlogHandler) createSessionToken() string {
	secrets := h.cfg.GetSecrets()
	// Include random nonce for uniqueness and non-predictability
	nonce := make([]byte, 16)
	rand.Read(nonce)
	nonceHex := hex.EncodeToString(nonce)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	payload := timestamp + "." + nonceHex
	mac := hmac.New(sha256.New, []byte(secrets.SessionSecret))
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))
	return payload + "." + signature
}

func (h *BlogHandler) verifySessionToken(token string) bool {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return false
	}

	timestamp, nonce, signature := parts[0], parts[1], parts[2]
	payload := timestamp + "." + nonce

	// Verify signature
	secrets := h.cfg.GetSecrets()
	mac := hmac.New(sha256.New, []byte(secrets.SessionSecret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return false
	}

	// Verify not expired
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}

	return time.Since(time.Unix(ts, 0)) < sessionMaxAge
}
