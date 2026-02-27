// Package handler — IndieWeb protocol implementations.
// Implements: Webmention (send + receive), IndieAuth (authorization + token), Micropub (create).
package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Digvijay/skriva/internal/store"
)

// --- Webmention ---

// HandleWebmentionReceive accepts incoming webmentions per W3C Webmention spec.
// POST /webmention with source= and target= form params.
func (h *BlogHandler) HandleWebmentionReceive(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	if !site.WebmentionEnabled {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	source := strings.TrimSpace(r.FormValue("source"))
	target := strings.TrimSpace(r.FormValue("target"))

	if source == "" || target == "" {
		http.Error(w, "source and target parameters are required", http.StatusBadRequest)
		return
	}

	// Validate URLs
	sourceURL, err := url.Parse(source)
	if err != nil || (sourceURL.Scheme != "http" && sourceURL.Scheme != "https") {
		http.Error(w, "Invalid source URL", http.StatusBadRequest)
		return
	}
	targetURL, err := url.Parse(target)
	if err != nil || (targetURL.Scheme != "http" && targetURL.Scheme != "https") {
		http.Error(w, "Invalid target URL", http.StatusBadRequest)
		return
	}

	// Target must be on our domain
	if site.BaseURL == "" || !strings.HasPrefix(target, site.BaseURL) {
		http.Error(w, "Target is not on this site", http.StatusBadRequest)
		return
	}

	// Extract slug from target URL
	slug := strings.TrimPrefix(target, site.BaseURL+"/")
	slug = strings.Split(slug, "?")[0]
	slug = strings.Split(slug, "#")[0]

	// Verify asynchronously — accept immediately per spec
	go h.verifyWebmention(source, target, slug)

	w.WriteHeader(http.StatusAccepted)
	fmt.Fprint(w, "Webmention accepted for processing")
}

// verifyWebmention fetches the source URL and verifies it links to the target.
func (h *BlogHandler) verifyWebmention(source, target, slug string) {
	// SECURITY: Validate URL to prevent SSRF to internal/private IPs
	if err := validateExternalURL(source); err != nil {
		h.logger.Warn("webmention source blocked (SSRF)", "source", source, "error", err)
		return
	}

	client := safeHTTPClient()
	resp, err := client.Get(source)
	if err != nil {
		h.logger.Error("fetching webmention source", "source", source, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		h.logger.Warn("webmention source not reachable", "source", source, "status", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		h.logger.Error("reading webmention source", "source", source, "error", err)
		return
	}

	// Verify the source actually contains a link to the target
	if !strings.Contains(string(body), target) {
		h.logger.Warn("webmention source does not link to target", "source", source, "target", target)
		return
	}

	// Try to extract author info (simplified h-card/microformat parsing)
	authorName := extractMeta(string(body), `class="p-name"`, `class="p-author"`)
	authorURL := extractHref(string(body), `class="u-url"`, `rel="author"`)
	authorAvatar := extractSrc(string(body), `class="u-photo"`)
	contentSnippet := extractMeta(string(body), `class="e-content"`, `class="p-summary"`)

	// Determine mention type
	mentionType := "mention"
	bodyStr := string(body)
	switch {
	case strings.Contains(bodyStr, `class="u-like-of"`):
		mentionType = "like"
	case strings.Contains(bodyStr, `class="u-repost-of"`):
		mentionType = "repost"
	case strings.Contains(bodyStr, `class="u-in-reply-to"`):
		mentionType = "reply"
	}

	if authorName == "" {
		authorName = source
	}
	if len(contentSnippet) > 500 {
		contentSnippet = contentSnippet[:500] + "..."
	}

	// SECURITY: Sanitize all extracted content to prevent stored XSS
	authorName = sanitizeUntrustedHTML(authorName)
	authorURL = sanitizeUntrustedHTML(authorURL)
	authorAvatar = sanitizeUntrustedHTML(authorAvatar)
	contentSnippet = sanitizeUntrustedHTML(contentSnippet)

	wm := &store.Webmention{
		Source:       source,
		Target:       target,
		PostSlug:     slug,
		AuthorName:   authorName,
		AuthorURL:    authorURL,
		AuthorAvatar: authorAvatar,
		Content:      contentSnippet,
		MentionType:  mentionType,
		Verified:     true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.store.SaveWebmention(ctx, wm); err != nil {
		h.logger.Error("saving webmention", "source", source, "error", err)
	} else {
		h.logger.Info("webmention verified and saved", "source", source, "target", target, "type", mentionType)
	}
}

// SendWebmentions discovers webmention endpoints for links in a post and sends mentions.
func (h *BlogHandler) SendWebmentions(postSlug, htmlContent string) {
	site := h.cfg.GetSite()
	if !site.WebmentionEnabled {
		return
	}
	if site.BaseURL == "" {
		return
	}

	sourceURL := site.BaseURL + "/" + postSlug

	// Extract all URLs from the post content
	links := extractLinks(htmlContent)
	if len(links) == 0 {
		return
	}

	client := safeHTTPClient()

	for _, link := range links {
		// Skip same-domain links
		if strings.HasPrefix(link, site.BaseURL) {
			continue
		}

		// Discover webmention endpoint
		endpoint := discoverWebmentionEndpoint(client, link)
		if endpoint == "" {
			continue
		}

		// SECURITY: Validate discovered endpoint URL to prevent SSRF
		if err := validateExternalURL(endpoint); err != nil {
			h.logger.Warn("webmention endpoint blocked (SSRF)", "endpoint", endpoint, "error", err)
			continue
		}

		// Send webmention
		resp, err := client.PostForm(endpoint, url.Values{
			"source": {sourceURL},
			"target": {link},
		})
		if err != nil {
			h.logger.Error("sending webmention", "target", link, "error", err)
			continue
		}
		resp.Body.Close()

		h.logger.Info("webmention sent", "source", sourceURL, "target", link, "endpoint", endpoint, "status", resp.StatusCode)
	}
}

// discoverWebmentionEndpoint finds the webmention endpoint for a URL.
func discoverWebmentionEndpoint(client *http.Client, targetURL string) string {
	// SECURITY: Validate URL to prevent SSRF
	if err := validateExternalURL(targetURL); err != nil {
		return ""
	}

	resp, err := client.Get(targetURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	// Check Link header first
	for _, link := range resp.Header.Values("Link") {
		if strings.Contains(link, `rel="webmention"`) || strings.Contains(link, `rel=webmention`) {
			// Extract URL from <url>; rel="webmention"
			start := strings.Index(link, "<")
			end := strings.Index(link, ">")
			if start >= 0 && end > start {
				endpoint := link[start+1 : end]
				return resolveRelativeURL(targetURL, endpoint)
			}
		}
	}

	// Check HTML body
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ""
	}

	// Find <link rel="webmention" href="...">
	re := regexp.MustCompile(`<link[^>]*rel=["']?webmention["']?[^>]*href=["']([^"']+)["'][^>]*/?>`)
	matches := re.FindSubmatch(body)
	if len(matches) >= 2 {
		return resolveRelativeURL(targetURL, string(matches[1]))
	}

	// Try reverse order: href before rel
	re2 := regexp.MustCompile(`<link[^>]*href=["']([^"']+)["'][^>]*rel=["']?webmention["']?[^>]*/?>`)
	matches = re2.FindSubmatch(body)
	if len(matches) >= 2 {
		return resolveRelativeURL(targetURL, string(matches[1]))
	}

	return ""
}

func resolveRelativeURL(base, ref string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return ref
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return baseURL.ResolveReference(refURL).String()
}

// --- IndieAuth ---

// HandleIndieAuthMetadata returns the IndieAuth server metadata.
func (h *BlogHandler) HandleIndieAuthMetadata(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	if !site.IndieAuthEnabled {
		http.NotFound(w, r)
		return
	}

	meta := map[string]interface{}{
		"issuer":                           site.BaseURL,
		"authorization_endpoint":           site.BaseURL + "/indieauth/auth",
		"token_endpoint":                   site.BaseURL + "/indieauth/token",
		"code_challenge_methods_supported": []string{"S256"},
		"scopes_supported":                 []string{"create", "update", "delete", "media"},
		"response_types_supported":         []string{"code"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

// HandleIndieAuthAuthorize handles the IndieAuth authorization flow.
// GET shows a consent form, POST processes the authorization.
func (h *BlogHandler) HandleIndieAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	if !site.IndieAuthEnabled {
		http.NotFound(w, r)
		return
	}

	if r.Method == http.MethodGet {
		// Must be logged in as admin to approve
		if !h.isAuthenticated(r) {
			http.Redirect(w, r, "/admin/login?redirect="+url.QueryEscape(r.URL.String()), http.StatusSeeOther)
			return
		}

		// Show consent form
		clientID := r.URL.Query().Get("client_id")
		redirectURI := r.URL.Query().Get("redirect_uri")
		state := r.URL.Query().Get("state")
		scope := r.URL.Query().Get("scope")
		codeChallenge := r.URL.Query().Get("code_challenge")
		codeChallengeMethod := r.URL.Query().Get("code_challenge_method")

		if clientID == "" || redirectURI == "" {
			http.Error(w, "client_id and redirect_uri required", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>Authorize</title>
<style>body{font-family:sans-serif;max-width:400px;margin:3rem auto;padding:1rem}
.card{background:#fff;border:1px solid #ddd;border-radius:8px;padding:1.5rem}
button{padding:.5rem 1rem;border:none;border-radius:4px;cursor:pointer;font-size:1rem;margin-right:.5rem}
.approve{background:#1a7f37;color:#fff}.deny{background:#cf222e;color:#fff}
</style></head><body>
<div class="card">
<h2>Authorize Application</h2>
<p><strong>%s</strong> wants to access your blog.</p>
<p>Scopes: <code>%s</code></p>
<form method="POST">
<input type="hidden" name="client_id" value="%s">
<input type="hidden" name="redirect_uri" value="%s">
<input type="hidden" name="state" value="%s">
<input type="hidden" name="scope" value="%s">
<input type="hidden" name="code_challenge" value="%s">
<input type="hidden" name="code_challenge_method" value="%s">
<input type="hidden" name="csrf_token" value="%s">
<button type="submit" name="action" value="approve" class="approve">Approve</button>
<button type="submit" name="action" value="deny" class="deny">Deny</button>
</form>
</div></body></html>`,
			html.EscapeString(clientID), html.EscapeString(scope),
			html.EscapeString(clientID), html.EscapeString(redirectURI),
			html.EscapeString(state), html.EscapeString(scope),
			html.EscapeString(codeChallenge), html.EscapeString(codeChallengeMethod),
			h.generateCSRFToken())
		return
	}

	// POST — process authorization
	if !h.isAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(r.FormValue("csrf_token")) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	action := r.FormValue("action")
	redirectURI := r.FormValue("redirect_uri")
	clientID := r.FormValue("client_id")
	state := r.FormValue("state")

	// SECURITY: Validate redirect_uri matches client_id domain to prevent open redirect
	clientURL, err := url.Parse(clientID)
	redirectURL, err2 := url.Parse(redirectURI)
	if err != nil || err2 != nil || clientURL.Host == "" || redirectURL.Host == "" {
		http.Error(w, "Invalid client_id or redirect_uri", http.StatusBadRequest)
		return
	}
	if clientURL.Host != redirectURL.Host {
		http.Error(w, "redirect_uri must be on the same domain as client_id", http.StatusBadRequest)
		return
	}

	if action != "approve" {
		http.Redirect(w, r, redirectURI+"?error=access_denied&state="+url.QueryEscape(state), http.StatusSeeOther)
		return
	}

	// Generate authorization code (short-lived, stored in memory via HMAC)
	code := h.generateAuthCode(r.FormValue("client_id"), r.FormValue("redirect_uri"), r.FormValue("scope"), r.FormValue("code_challenge"))

	redirect := redirectURI + "?code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(state)
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// HandleIndieAuthToken exchanges an authorization code for a token.
func (h *BlogHandler) HandleIndieAuthToken(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	if !site.IndieAuthEnabled {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	grantType := r.FormValue("grant_type")
	if grantType != "authorization_code" {
		jsonError(w, "Unsupported grant_type", http.StatusBadRequest)
		return
	}

	code := r.FormValue("code")
	clientID := r.FormValue("client_id")
	redirectURI := r.FormValue("redirect_uri")
	codeVerifier := r.FormValue("code_verifier")

	// Verify the authorization code
	scope, err := h.verifyAuthCode(code, clientID, redirectURI, codeVerifier)
	if err != nil {
		jsonError(w, "Invalid authorization code: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Generate access token (HMAC-based, no storage needed)
	token := h.generateIndieAuthToken(scope)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token": token,
		"token_type":   "Bearer",
		"scope":        scope,
		"me":           site.BaseURL + "/",
	})
}

// generateAuthCode creates a signed auth code encoding client_id, redirect_uri, scope, code_challenge.
func (h *BlogHandler) generateAuthCode(clientID, redirectURI, scope, codeChallenge string) string {
	data := clientID + "|" + redirectURI + "|" + scope + "|" + codeChallenge + "|" + fmt.Sprintf("%d", time.Now().Add(10*time.Minute).Unix())
	secrets := h.cfg.GetSecrets()
	mac := indieHMACSHA256(secrets.SessionSecret, data)
	return base64Encode(data + "|" + mac)
}

// verifyAuthCode validates a signed auth code.
func (h *BlogHandler) verifyAuthCode(code, clientID, redirectURI, codeVerifier string) (string, error) {
	decoded, err := base64Decode(code)
	if err != nil {
		return "", fmt.Errorf("invalid code encoding")
	}

	parts := strings.SplitN(decoded, "|", 6)
	if len(parts) != 6 {
		return "", fmt.Errorf("malformed code")
	}

	storedClientID := parts[0]
	storedRedirectURI := parts[1]
	scope := parts[2]
	storedChallenge := parts[3]
	expiryStr := parts[4]
	storedMAC := parts[5]

	// Verify MAC (constant-time comparison to prevent timing attacks)
	secrets := h.cfg.GetSecrets()
	data := strings.Join(parts[:5], "|")
	expectedMAC := indieHMACSHA256(secrets.SessionSecret, data)
	if !hmac.Equal([]byte(storedMAC), []byte(expectedMAC)) {
		return "", fmt.Errorf("invalid signature")
	}

	// Check expiry
	var expiry int64
	_, _ = fmt.Sscanf(expiryStr, "%d", &expiry)
	if time.Now().Unix() > expiry {
		return "", fmt.Errorf("code expired")
	}

	// Verify client_id and redirect_uri match
	if storedClientID != clientID || storedRedirectURI != redirectURI {
		return "", fmt.Errorf("client_id or redirect_uri mismatch")
	}

	// Verify PKCE code_verifier against stored code_challenge (S256)
	// SECURITY: PKCE is mandatory — reject if code_challenge was not provided
	if storedChallenge == "" {
		return "", fmt.Errorf("PKCE code_challenge is required")
	}
	if codeVerifier == "" {
		return "", fmt.Errorf("PKCE code_verifier is required")
	}
	if !indieVerifyPKCE(codeVerifier, storedChallenge) {
		return "", fmt.Errorf("PKCE verification failed")
	}

	return scope, nil
}

func (h *BlogHandler) generateIndieAuthToken(scope string) string {
	data := scope + "|" + fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix())
	secrets := h.cfg.GetSecrets()
	mac := indieHMACSHA256(secrets.SessionSecret, data)
	return base64Encode(data + "|" + mac)
}

// VerifyIndieAuthToken validates a Bearer token from IndieAuth.
func (h *BlogHandler) VerifyIndieAuthToken(token string) (string, error) {
	decoded, err := base64Decode(token)
	if err != nil {
		return "", fmt.Errorf("invalid token")
	}

	parts := strings.SplitN(decoded, "|", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed token")
	}

	scope := parts[0]
	expiryStr := parts[1]
	storedMAC := parts[2]

	secrets := h.cfg.GetSecrets()
	data := parts[0] + "|" + parts[1]
	expectedMAC := indieHMACSHA256(secrets.SessionSecret, data)
	if !hmac.Equal([]byte(storedMAC), []byte(expectedMAC)) {
		return "", fmt.Errorf("invalid token signature")
	}

	var expiry int64
	_, _ = fmt.Sscanf(expiryStr, "%d", &expiry)
	if time.Now().Unix() > expiry {
		return "", fmt.Errorf("token expired")
	}

	return scope, nil
}

// --- Micropub ---

// HandleMicropub handles Micropub requests (create, update, delete posts via API).
func (h *BlogHandler) HandleMicropub(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	if !site.MicropubEnabled {
		http.NotFound(w, r)
		return
	}

	if r.Method == http.MethodGet {
		h.handleMicropubQuery(w, r)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify IndieAuth Bearer token
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	scope, err := h.VerifyIndieAuthToken(token)
	if err != nil {
		http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Parse request to determine action
	contentType := r.Header.Get("Content-Type")
	var action string

	if strings.Contains(contentType, "application/json") {
		// Read body and re-parse for each sub-handler
		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		var body map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &body); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		action, _ = body["action"].(string)

		switch action {
		case "update":
			if !hasScope(scope, "update") {
				http.Error(w, "Insufficient scope (need 'update')", http.StatusForbidden)
				return
			}
			h.handleMicropubUpdate(w, r, body)
			return
		case "delete":
			if !hasScope(scope, "delete") {
				http.Error(w, "Insufficient scope (need 'delete')", http.StatusForbidden)
				return
			}
			h.handleMicropubDelete(w, r, body)
			return
		default:
			// Fall through to create
			if !hasScope(scope, "create") {
				http.Error(w, "Insufficient scope (need 'create')", http.StatusForbidden)
				return
			}
			h.handleMicropubCreateJSON(w, r, body)
			return
		}
	}

	// Form-encoded — check for action
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	action = r.FormValue("action")

	switch action {
	case "update":
		if !hasScope(scope, "update") {
			http.Error(w, "Insufficient scope (need 'update')", http.StatusForbidden)
			return
		}
		http.Error(w, "Form-encoded update not supported; use JSON", http.StatusBadRequest)
		return
	case "delete":
		if !hasScope(scope, "delete") {
			http.Error(w, "Insufficient scope (need 'delete')", http.StatusForbidden)
			return
		}
		urlStr := r.FormValue("url")
		body := map[string]interface{}{"action": "delete", "url": urlStr}
		h.handleMicropubDelete(w, r, body)
		return
	default:
		if !hasScope(scope, "create") {
			http.Error(w, "Insufficient scope (need 'create')", http.StatusForbidden)
			return
		}
		h.handleMicropubCreateForm(w, r)
		return
	}
}

// handleMicropubCreateJSON creates a post from JSON Micropub request.
func (h *BlogHandler) handleMicropubCreateJSON(w http.ResponseWriter, r *http.Request, body map[string]interface{}) {
	var title, content string
	var tags []string
	isDraft := false

	if props, ok := body["properties"].(map[string]interface{}); ok {
		title = firstString(props["name"])
		content = firstString(props["content"])
		tags = allStrings(props["category"])
		if firstString(props["post-status"]) == "draft" {
			isDraft = true
		}
	}

	h.micropubCreatePost(w, title, content, tags, isDraft)
}

// handleMicropubCreateForm creates a post from form-encoded Micropub request.
func (h *BlogHandler) handleMicropubCreateForm(w http.ResponseWriter, r *http.Request) {
	title := r.FormValue("name")
	content := r.FormValue("content")
	tags := r.Form["category"]
	isDraft := r.FormValue("post-status") == "draft"

	h.micropubCreatePost(w, title, content, tags, isDraft)
}

// micropubCreatePost is the shared create logic.
func (h *BlogHandler) micropubCreatePost(w http.ResponseWriter, title, content string, tags []string, isDraft bool) {
	if content == "" {
		http.Error(w, "Content is required", http.StatusBadRequest)
		return
	}

	// SECURITY: Sanitize Micropub content to prevent stored XSS via raw HTML in markdown.
	// Micropub clients may send arbitrary HTML; strip dangerous elements/attributes.
	content = sanitizeUntrustedHTML(content)
	title = sanitizeUntrustedHTML(title)

	if title == "" {
		lines := strings.SplitN(content, "\n", 2)
		title = strings.TrimSpace(strings.TrimLeft(lines[0], "# "))
		if len(title) > 80 {
			title = title[:80]
		}
	}

	slug := slugify(title)
	if slug == "" {
		slug = fmt.Sprintf("post-%d", time.Now().Unix())
	}

	date := time.Now().Format("2006-01-02")
	tagsStr := ""
	if len(tags) > 0 {
		tagsStr = strings.Join(quoteStrings(tags), ", ")
	}

	frontmatter := fmt.Sprintf(`---
title: %q
slug: %q
date: %s
tags: [%s]
draft: %v
---

%s`, title, slug, date, tagsStr, isDraft, content)

	postDir := filepath.Join(h.contentDir, "posts", slug)
	if err := os.MkdirAll(postDir, 0o755); err != nil {
		http.Error(w, "Failed to create post directory", http.StatusInternalServerError)
		return
	}

	indexPath := filepath.Join(postDir, "index.md")
	if err := os.WriteFile(indexPath, []byte(frontmatter), 0o644); err != nil {
		http.Error(w, "Failed to write post", http.StatusInternalServerError)
		return
	}

	if err := h.loader.Reload(); err != nil {
		h.logger.Error("reloading content after micropub create", "error", err)
	}

	site := h.cfg.GetSite()
	postURL := site.BaseURL + "/" + slug

	w.Header().Set("Location", postURL)
	w.WriteHeader(http.StatusCreated)

	h.logger.Info("micropub post created", "slug", slug, "title", title)

	// Notify fediverse followers and send webmentions for published posts
	if !isDraft {
		if post, ok := h.loader.PostBySlug(slug); ok {
			go h.NotifyFollowersNewPost(post)
			if renderedHTML, err := h.engine.RenderMarkdown(post.Content); err == nil {
				go h.SendWebmentions(post.Slug, renderedHTML)
			}
		}
	}
}

// handleMicropubUpdate handles Micropub update requests.
func (h *BlogHandler) handleMicropubUpdate(w http.ResponseWriter, r *http.Request, body map[string]interface{}) {
	urlStr, _ := body["url"].(string)
	if urlStr == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	// Extract slug from URL
	site := h.cfg.GetSite()
	slug := strings.TrimPrefix(urlStr, site.BaseURL+"/")
	slug = strings.Split(slug, "?")[0]
	slug = strings.Split(slug, "#")[0]

	post, ok := h.loader.PostBySlugAdmin(slug)
	if !ok {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	// Apply updates using "replace" properties
	title := post.Title
	content := post.Content
	tags := post.Tags

	if replace, ok := body["replace"].(map[string]interface{}); ok {
		if v := firstString(replace["name"]); v != "" {
			title = v
		}
		if v := firstString(replace["content"]); v != "" {
			content = v
		}
		if cats := allStrings(replace["category"]); len(cats) > 0 {
			tags = cats
		}
	}

	// Apply "add" for tags
	if add, ok := body["add"].(map[string]interface{}); ok {
		if cats := allStrings(add["category"]); len(cats) > 0 {
			tags = append(tags, cats...)
		}
	}

	// Apply "delete" for tags
	if del, ok := body["delete"].(map[string]interface{}); ok {
		if cats := allStrings(del["category"]); len(cats) > 0 {
			delSet := make(map[string]bool)
			for _, c := range cats {
				delSet[c] = true
			}
			var filtered []string
			for _, t := range tags {
				if !delSet[t] {
					filtered = append(filtered, t)
				}
			}
			tags = filtered
		}
	}

	// Rebuild frontmatter
	tagsStr := ""
	if len(tags) > 0 {
		tagsStr = strings.Join(quoteStrings(tags), ", ")
	}

	frontmatter := fmt.Sprintf(`---
title: %q
slug: %q
date: %s
tags: [%s]
description: %q
image: %q
featured: %v
draft: %v
---

%s`,
		title, slug, post.Date.Format("2006-01-02"),
		tagsStr,
		post.Description, post.Image, post.Featured, post.Draft, content)

	indexPath := filepath.Join(post.Dir, "index.md")
	if err := os.WriteFile(indexPath, []byte(frontmatter), 0o644); err != nil {
		http.Error(w, "Failed to update post", http.StatusInternalServerError)
		return
	}

	if err := h.loader.Reload(); err != nil {
		h.logger.Error("reloading content after micropub update", "error", err)
	}

	h.logger.Info("micropub post updated", "slug", slug)
	w.WriteHeader(http.StatusNoContent)

	// Send webmentions for updated posts
	if !post.Draft {
		if updatedPost, ok := h.loader.PostBySlug(slug); ok {
			if renderedHTML, err := h.engine.RenderMarkdown(updatedPost.Content); err == nil {
				go h.SendWebmentions(updatedPost.Slug, renderedHTML)
			}
		}
	}
}

// handleMicropubDelete handles Micropub delete requests.
func (h *BlogHandler) handleMicropubDelete(w http.ResponseWriter, r *http.Request, body map[string]interface{}) {
	urlStr, _ := body["url"].(string)
	if urlStr == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	site := h.cfg.GetSite()
	slug := strings.TrimPrefix(urlStr, site.BaseURL+"/")
	slug = strings.Split(slug, "?")[0]
	slug = strings.Split(slug, "#")[0]

	post, ok := h.loader.PostBySlugAdmin(slug)
	if !ok {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	// Safety: verify the directory is within content/posts
	postsBase := filepath.Join(h.contentDir, "posts")
	cleaned := filepath.Clean(post.Dir)
	if !strings.HasPrefix(cleaned, postsBase) {
		http.Error(w, "Invalid post path", http.StatusBadRequest)
		return
	}

	if err := os.RemoveAll(cleaned); err != nil {
		http.Error(w, "Failed to delete post", http.StatusInternalServerError)
		return
	}

	if err := h.loader.Reload(); err != nil {
		h.logger.Error("reloading content after micropub delete", "error", err)
	}

	h.logger.Info("micropub post deleted", "slug", slug)
	w.WriteHeader(http.StatusNoContent)
}

// HandleMicropubMedia handles media uploads via Micropub.
func (h *BlogHandler) HandleMicropubMedia(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	if !site.MicropubEnabled {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify IndieAuth Bearer token with media scope
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	scope, err := h.VerifyIndieAuthToken(token)
	if err != nil {
		http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}
	if !hasScope(scope, "media") && !hasScope(scope, "create") {
		http.Error(w, "Insufficient scope (need 'media' or 'create')", http.StatusForbidden)
		return
	}

	// Parse multipart form (max 20MB)
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "File too large or invalid form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing file upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read first 512 bytes for content type detection
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	detectedType := http.DetectContentType(buf[:n])

	// Validate content type
	allowedTypes := map[string]string{
		"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif",
		"image/webp": ".webp", "image/svg+xml": ".svg",
		"video/mp4": ".mp4", "audio/mpeg": ".mp3",
	}
	ext, ok := allowedTypes[detectedType]
	if !ok {
		http.Error(w, "Unsupported file type: "+detectedType, http.StatusBadRequest)
		return
	}

	// Generate unique filename
	filename := fmt.Sprintf("media-%d%s", time.Now().UnixNano(), ext)

	// Save to static/media directory
	mediaDir := filepath.Join(h.contentDir, "static", "media")
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		http.Error(w, "Failed to create media directory", http.StatusInternalServerError)
		return
	}

	destPath := filepath.Join(mediaDir, filename)
	// Security: verify path stays within media dir
	if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(mediaDir)) {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	dest, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer dest.Close()

	// Write the already-read bytes first, then the rest
	_, _ = dest.Write(buf[:n])
	if _, err := io.Copy(dest, file); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	site = h.cfg.GetSite()
	mediaURL := site.BaseURL + "/static/media/" + filename

	w.Header().Set("Location", mediaURL)
	w.WriteHeader(http.StatusCreated)

	h.logger.Info("micropub media uploaded", "file", filename, "type", detectedType, "original", header.Filename)
}

func (h *BlogHandler) handleMicropubQuery(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	site := h.cfg.GetSite()

	switch q {
	case "config":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"media-endpoint": site.BaseURL + "/micropub/media",
		})
	case "syndicate-to":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"syndicate-to": []interface{}{},
		})
	case "source":
		// Return post data for a given URL
		urlStr := r.URL.Query().Get("url")
		if urlStr == "" {
			http.Error(w, "url parameter required", http.StatusBadRequest)
			return
		}
		slug := strings.TrimPrefix(urlStr, site.BaseURL+"/")
		slug = strings.Split(slug, "?")[0]
		slug = strings.Split(slug, "#")[0]
		post, ok := h.loader.PostBySlugAdmin(slug)
		if !ok {
			http.Error(w, "Post not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type": []string{"h-entry"},
			"properties": map[string]interface{}{
				"name":      []string{post.Title},
				"content":   []string{post.Content},
				"published": []string{post.Date.Format(time.RFC3339)},
				"category":  post.Tags,
				"url":       []string{site.BaseURL + "/" + post.Slug},
				"post-status": []string{func() string {
					if post.Draft {
						return "draft"
					}
					return "published"
				}()},
			},
		})
	case "category":
		// Return all known tags
		tags := h.loader.Tags()
		var names []string
		for _, t := range tags {
			names = append(names, t.Name)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"categories": names,
		})
	default:
		http.Error(w, "Unknown query", http.StatusBadRequest)
	}
}

// --- Helpers ---

func extractLinks(htmlContent string) []string {
	re := regexp.MustCompile(`href=["']([^"']+)["']`)
	matches := re.FindAllStringSubmatch(htmlContent, -1)
	var links []string
	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) >= 2 {
			link := m[1]
			if (strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://")) && !seen[link] {
				links = append(links, link)
				seen[link] = true
			}
		}
	}
	return links
}

func extractMeta(body string, selectors ...string) string {
	for _, sel := range selectors {
		idx := strings.Index(body, sel)
		if idx < 0 {
			continue
		}
		// Find the content after the tag
		rest := body[idx:]
		gtIdx := strings.Index(rest, ">")
		if gtIdx < 0 {
			continue
		}
		rest = rest[gtIdx+1:]
		ltIdx := strings.Index(rest, "<")
		if ltIdx > 0 {
			text := strings.TrimSpace(rest[:ltIdx])
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func extractHref(body string, selectors ...string) string {
	for _, sel := range selectors {
		idx := strings.Index(body, sel)
		if idx < 0 {
			continue
		}
		// Look backwards/forwards for href
		region := body[max(0, idx-200):min(len(body), idx+200)]
		re := regexp.MustCompile(`href=["']([^"']+)["']`)
		if m := re.FindStringSubmatch(region); len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

func extractSrc(body string, selectors ...string) string {
	for _, sel := range selectors {
		idx := strings.Index(body, sel)
		if idx < 0 {
			continue
		}
		region := body[max(0, idx-200):min(len(body), idx+200)]
		re := regexp.MustCompile(`src=["']([^"']+)["']`)
		if m := re.FindStringSubmatch(region); len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

func firstString(v interface{}) string {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		if s, ok := arr[0].(string); ok {
			return s
		}
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func allStrings(v interface{}) []string {
	if arr, ok := v.([]interface{}); ok {
		var out []string
		for _, item := range arr {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func base64Encode(s string) string {
	encoded := base64.URLEncoding.EncodeToString([]byte(s))
	return strings.TrimRight(encoded, "=")
}

func base64Decode(s string) (string, error) {
	// Add padding
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	data, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// indieHMACSHA256 computes HMAC-SHA256 for IndieAuth tokens.
func indieHMACSHA256(key, data string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func indieVerifyPKCE(verifier, challenge string) bool {
	h := sha256.Sum256([]byte(verifier))
	computed := strings.TrimRight(base64.URLEncoding.EncodeToString(h[:]), "=")
	return hmac.Equal([]byte(computed), []byte(challenge))
}

// Override the generic hmacSHA256 reference with the real one
func init() {
	// Ensure html package is used
	_ = html.EscapeString
}

var _ store.Webmention
