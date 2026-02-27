// admin_api.go — admin CRUD API endpoints for posts, comments, media, settings, and more.
package handler

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	blog "github.com/Digvijay/skriva"
	"github.com/Digvijay/skriva/internal/config"
	"github.com/Digvijay/skriva/internal/plugin"
	"github.com/Digvijay/skriva/internal/store"
	"github.com/skip2/go-qrcode"
)

// HandleAdminEditor serves the post editor.
func (h *BlogHandler) HandleAdminEditor(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	h.serveAdminHTML(w, adminEditorHTML)
}

// HandleAdminAPIPosts returns all posts as JSON (for admin).
func (h *BlogHandler) HandleAdminAPIPosts(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	posts := h.loader.AllPosts()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}

// HandleAdminAPICreatePost creates a new post from the editor.
func (h *BlogHandler) HandleAdminAPICreatePost(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req struct {
		Title       string   `json:"title"`
		Slug        string   `json:"slug"`
		Content     string   `json:"content"`
		Tags        []string `json:"tags"`
		Description string   `json:"description"`
		Image       string   `json:"image"`
		Draft       bool     `json:"draft"`
		Featured    bool     `json:"featured"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		jsonError(w, "Title is required", http.StatusBadRequest)
		return
	}

	if req.Slug == "" {
		req.Slug = slugify(req.Title)
	} else {
		req.Slug = slugify(req.Slug) // Always sanitize user-provided slugs
	}

	if req.Slug == "" {
		jsonError(w, "Invalid slug", http.StatusBadRequest)
		return
	}

	// Create post directory — verify path stays within posts dir
	postDir := filepath.Join(h.contentDir, "posts", req.Slug)
	postsBase := filepath.Join(h.contentDir, "posts")
	if !strings.HasPrefix(filepath.Clean(postDir), postsBase) {
		jsonError(w, "Invalid slug", http.StatusBadRequest)
		return
	}
	if err := os.MkdirAll(postDir, 0o755); err != nil {
		h.logger.Error("creating post directory", "error", err)
		jsonError(w, "Failed to create post", http.StatusInternalServerError)
		return
	}

	// Create media directory
	if err := os.MkdirAll(filepath.Join(postDir, "media"), 0o755); err != nil {
		h.logger.Error("creating media directory", "error", err)
	}

	// Build frontmatter
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
		req.Title, req.Slug, time.Now().Format("2006-01-02"),
		strings.Join(quoteStrings(req.Tags), ", "),
		req.Description, req.Image, req.Featured, req.Draft, req.Content)

	// Write index.md
	indexPath := filepath.Join(postDir, "index.md")
	if err := os.WriteFile(indexPath, []byte(frontmatter), 0o644); err != nil {
		h.logger.Error("writing post file", "error", err)
		jsonError(w, "Failed to write post", http.StatusInternalServerError)
		return
	}

	// Reload content
	if err := h.loader.Reload(); err != nil {
		h.logger.Error("reloading content after create", "error", err)
	}

	h.logger.Info("post created", "slug", req.Slug)
	h.audit(r, "post.created", "Created post: "+req.Slug)

	// Fire plugin hooks
	go h.plugins.Fire(plugin.EventPostCreated, plugin.PostEvent{
		Slug: req.Slug, Title: req.Title, Tags: req.Tags, Draft: req.Draft,
	})

	// Notify fediverse followers and send webmentions for published posts
	if !req.Draft {
		if post, ok := h.loader.PostBySlug(req.Slug); ok {
			go h.NotifyFollowersNewPost(post)
			if renderedHTML, err := h.engine.RenderMarkdown(post.Content); err == nil {
				go h.SendWebmentions(post.Slug, renderedHTML)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"slug": req.Slug, "status": "created"})
}

// HandleAdminAPIUpdatePost updates an existing post.
func (h *BlogHandler) HandleAdminAPIUpdatePost(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	slug := r.PathValue("slug")
	post, ok := h.loader.PostBySlugAdmin(slug)
	if !ok {
		jsonError(w, "Post not found", http.StatusNotFound)
		return
	}

	var req struct {
		Title       string   `json:"title"`
		Content     string   `json:"content"`
		Tags        []string `json:"tags"`
		Description string   `json:"description"`
		Image       string   `json:"image"`
		Draft       bool     `json:"draft"`
		Featured    bool     `json:"featured"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Save revision of the current version before overwriting
	metadata, _ := json.Marshal(map[string]interface{}{
		"tags": post.Tags, "description": post.Description,
		"image": post.Image, "draft": post.Draft, "featured": post.Featured,
	})
	_ = h.store.SavePostRevision(r.Context(), slug, post.Title, post.Content, string(metadata))

	// Rebuild frontmatter + content
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
		req.Title, slug, post.Date.Format("2006-01-02"),
		strings.Join(quoteStrings(req.Tags), ", "),
		req.Description, req.Image, req.Featured, req.Draft, req.Content)

	indexPath := filepath.Join(post.Dir, "index.md")
	if err := os.WriteFile(indexPath, []byte(frontmatter), 0o644); err != nil {
		h.logger.Error("updating post file", "error", err)
		jsonError(w, "Failed to update post", http.StatusInternalServerError)
		return
	}

	if err := h.loader.Reload(); err != nil {
		h.logger.Error("reloading content after update", "error", err)
	}

	h.logger.Info("post updated", "slug", slug)
	h.audit(r, "post.updated", "Updated post: "+slug)

	// Fire plugin hooks
	go h.plugins.Fire(plugin.EventPostUpdated, plugin.PostEvent{
		Slug: slug, Title: req.Title, Tags: req.Tags, Draft: req.Draft,
	})

	// Notify fediverse followers and send webmentions when a post is published
	// (either newly published or was draft and is now published)
	if !req.Draft {
		if updatedPost, ok := h.loader.PostBySlug(slug); ok {
			// Only notify followers if the post was previously a draft (i.e., just published)
			if post.Draft {
				go h.NotifyFollowersNewPost(updatedPost)
			}
			if renderedHTML, err := h.engine.RenderMarkdown(updatedPost.Content); err == nil {
				go h.SendWebmentions(updatedPost.Slug, renderedHTML)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"slug": slug, "status": "updated"})
}

// HandleAdminAPIDeletePost deletes a post and its directory.
func (h *BlogHandler) HandleAdminAPIDeletePost(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	slug := r.PathValue("slug")
	post, ok := h.loader.PostBySlugAdmin(slug)
	if !ok {
		jsonError(w, "Post not found", http.StatusNotFound)
		return
	}

	// Safety: verify the directory is within content/posts
	postsBase := filepath.Join(h.contentDir, "posts")
	cleaned := filepath.Clean(post.Dir)
	if !strings.HasPrefix(cleaned, postsBase) {
		jsonError(w, "Invalid post path", http.StatusBadRequest)
		return
	}

	if err := os.RemoveAll(cleaned); err != nil {
		h.logger.Error("deleting post directory", "error", err)
		jsonError(w, "Failed to delete post", http.StatusInternalServerError)
		return
	}

	if err := h.loader.Reload(); err != nil {
		h.logger.Error("reloading content after delete", "error", err)
	}

	h.logger.Info("post deleted", "slug", slug)
	h.audit(r, "post.deleted", "Deleted post: "+slug)

	// Fire plugin hooks
	go h.plugins.Fire(plugin.EventPostDeleted, plugin.PostEvent{Slug: slug})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// HandleAdminAPIComments returns all comments as JSON.
func (h *BlogHandler) HandleAdminAPIComments(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	comments, err := h.store.AllComments(r.Context())
	if err != nil {
		h.logger.Error("listing comments", "error", err)
		jsonError(w, "Failed to load comments", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comments)
}

// HandleAdminAPIDeleteComment deletes a comment by ID.
func (h *BlogHandler) HandleAdminAPIDeleteComment(w http.ResponseWriter, r *http.Request) {
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
		jsonError(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteComment(r.Context(), id); err != nil {
		h.logger.Error("deleting comment", "id", id, "error", err)
		jsonError(w, "Failed to delete comment", http.StatusInternalServerError)
		return
	}

	h.logger.Info("comment deleted", "id", id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// HandleAdminAPIApproveComment approves a pending comment.
func (h *BlogHandler) HandleAdminAPIApproveComment(w http.ResponseWriter, r *http.Request) {
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
		jsonError(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	if err := h.store.ApproveComment(r.Context(), id); err != nil {
		h.logger.Error("approving comment", "id", id, "error", err)
		jsonError(w, "Failed to approve comment", http.StatusInternalServerError)
		return
	}

	h.logger.Info("comment approved", "id", id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "approved"})
}

// HandleAdminAPIUnapproveComment sets an approved comment back to pending.
func (h *BlogHandler) HandleAdminAPIUnapproveComment(w http.ResponseWriter, r *http.Request) {
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
		jsonError(w, "Invalid comment ID", http.StatusBadRequest)
		return
	}

	if err := h.store.UnapproveComment(r.Context(), id); err != nil {
		h.logger.Error("unapproving comment", "id", id, "error", err)
		jsonError(w, "Failed to unapprove comment", http.StatusInternalServerError)
		return
	}

	h.logger.Info("comment unapproved", "id", id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "unapproved"})
}

// HandleAdminAPITOTPStatus returns whether TOTP is enabled.
func (h *BlogHandler) HandleAdminAPITOTPStatus(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": h.cfg.IsTOTPEnabled()})
}

// HandleAdminAPITOTPSetup generates a new TOTP secret for the admin.
func (h *BlogHandler) HandleAdminAPITOTPSetup(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req struct {
		Code string `json:"code"` // Verification code to confirm setup
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// If no code provided, generate a new secret for the user to register
	if req.Code == "" {
		secret, err := generateTOTPSecret()
		if err != nil {
			h.logger.Error("generating TOTP secret", "error", err)
			jsonError(w, "Failed to generate secret", http.StatusInternalServerError)
			return
		}

		site := h.cfg.GetSite()
		issuer := site.Title
		if issuer == "" {
			issuer = "Blog"
		}
		uri := totpProvisioningURI(secret, issuer, "admin")

		// Generate QR code as base64 PNG
		qrB64 := ""
		if png, err := qrcode.Encode(uri, qrcode.Medium, 256); err == nil {
			qrB64 = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"secret": secret,
			"uri":    uri,
			"qr":     qrB64,
			"status": "pending",
		})
		return
	}

	// If code was provided in this endpoint, redirect to /confirm
	jsonError(w, "Use /admin/api/totp/confirm to verify the code", http.StatusBadRequest)
}

// HandleAdminAPITOTPConfirm confirms TOTP setup by verifying a code against the pending secret.
func (h *BlogHandler) HandleAdminAPITOTPConfirm(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req struct {
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Secret == "" || req.Code == "" {
		jsonError(w, "Secret and code are required", http.StatusBadRequest)
		return
	}

	// Verify the code matches the given secret
	if !validateTOTP(req.Secret, req.Code) {
		jsonError(w, "Invalid code. Please check your authenticator and try again.", http.StatusBadRequest)
		return
	}

	// Save the TOTP secret
	secrets := h.cfg.GetSecrets()
	secrets.TOTPSecret = req.Secret
	if err := h.cfg.UpdateSecrets(secrets); err != nil {
		h.logger.Error("saving TOTP secret", "error", err)
		jsonError(w, "Failed to save TOTP configuration", http.StatusInternalServerError)
		return
	}

	h.logger.Info("TOTP 2FA enabled")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "enabled"})
}

// HandleAdminAPITOTPDisable disables TOTP 2FA.
func (h *BlogHandler) HandleAdminAPITOTPDisable(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Require a valid TOTP code to disable (prevent unauthorized disable)
	secrets := h.cfg.GetSecrets()
	if secrets.TOTPSecret != "" && !validateTOTP(secrets.TOTPSecret, req.Code) {
		jsonError(w, "Invalid TOTP code", http.StatusBadRequest)
		return
	}

	secrets.TOTPSecret = ""
	if err := h.cfg.UpdateSecrets(secrets); err != nil {
		h.logger.Error("disabling TOTP", "error", err)
		jsonError(w, "Failed to disable 2FA", http.StatusInternalServerError)
		return
	}

	h.logger.Info("TOTP 2FA disabled")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "disabled"})
}

// HandleAdminAPIPreviewMarkdown renders markdown to HTML for the editor preview.
func (h *BlogHandler) HandleAdminAPIPreviewMarkdown(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Markdown string `json:"markdown"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	renderedHTML, err := h.engine.RenderMarkdown(req.Markdown)
	if err != nil {
		h.logger.Error("preview markdown render", "error", err)
		jsonError(w, "Render failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"html": renderedHTML})
}

// HandleAdminAPIStats returns dashboard statistics.
func (h *BlogHandler) HandleAdminAPIStats(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	stats, err := h.store.GetDashboardStats(r.Context())
	if err != nil {
		h.logger.Error("loading dashboard stats", "error", err)
		jsonError(w, "Failed to load stats", http.StatusInternalServerError)
		return
	}

	stats.TotalPosts = int64(len(h.loader.AllPosts()))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandleAdminAPIUploadMedia handles file uploads for a post.
func (h *BlogHandler) HandleAdminAPIUploadMedia(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	slug := r.PathValue("slug")

	r.Body = http.MaxBytesReader(w, r.Body, maxFileSize)
	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		jsonError(w, "File too large (max 20MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate content type by reading magic bytes
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	contentType := http.DetectContentType(buf[:n])

	allowedTypes := map[string]bool{
		"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true,
		"video/mp4": true, "video/webm": true,
		"audio/mpeg": true, "audio/wav": true, "audio/ogg": true,
	}

	if !allowedTypes[contentType] {
		jsonError(w, "Unsupported file type: "+contentType, http.StatusBadRequest)
		return
	}

	// Reset file reader
	if seeker, ok := file.(io.ReadSeeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	// Ensure post media directory exists
	mediaDir := filepath.Join(h.contentDir, "posts", slug, "media")
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		h.logger.Error("creating media directory", "error", err)
		jsonError(w, "Failed to create media directory", http.StatusInternalServerError)
		return
	}

	// Safe filename — force extension from detected content type to prevent type confusion XSS
	rawName := strings.TrimSuffix(filepath.Base(header.Filename), filepath.Ext(filepath.Base(header.Filename)))
	safeName := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, rawName)
	mediaExtMap := map[string]string{
		"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp",
		"video/mp4": ".mp4", "video/webm": ".webm",
		"audio/mpeg": ".mp3", "audio/wav": ".wav", "audio/ogg": ".ogg",
	}
	safeExt := mediaExtMap[contentType]
	if safeExt == "" {
		safeExt = ".bin"
	}
	filename := safeName + safeExt

	destPath := filepath.Join(mediaDir, filename)
	dest, err := os.Create(destPath)
	if err != nil {
		h.logger.Error("creating media file", "error", err)
		jsonError(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		h.logger.Error("writing media file", "error", err)
		jsonError(w, "Failed to write file", http.StatusInternalServerError)
		return
	}

	h.logger.Info("media uploaded", "slug", slug, "file", filename)

	// Optimize images — resize if wider than 1920px
	go optimizeImage(destPath, contentType, 1920)

	mediaURL := fmt.Sprintf("/media/%s/media/%s", slug, filename)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"filename": filename,
		"url":      mediaURL,
	})
}

// HandleAdminAPIUnsplash proxies Unsplash API search requests.
func (h *BlogHandler) HandleAdminAPIUnsplash(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	secrets := h.cfg.GetSecrets()
	if secrets.UnsplashAccessKey == "" {
		jsonError(w, "Unsplash API key not configured", http.StatusServiceUnavailable)
		return
	}

	query := r.URL.Query().Get("query")
	if query == "" {
		jsonError(w, "Search query required", http.StatusBadRequest)
		return
	}

	// Proxy to Unsplash API
	client := safeHTTPClient()
	req, _ := http.NewRequestWithContext(r.Context(), "GET",
		"https://api.unsplash.com/search/photos?query="+url.QueryEscape(query)+"&per_page=12", http.NoBody)
	req.Header.Set("Authorization", "Client-ID "+secrets.UnsplashAccessKey)

	resp, err := client.Do(req)
	if err != nil {
		h.logger.Error("unsplash API request failed", "error", err)
		jsonError(w, "Unsplash API error", http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// HandleAdminAPIFediverseStats returns ActivityPub and IndieWeb stats for the dashboard.
func (h *BlogHandler) HandleAdminAPIFediverseStats(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	followerCount, _ := h.store.ActivityPubFollowerCount(ctx)
	followers, _ := h.store.AllActivityPubFollowers(ctx)
	if followers == nil {
		followers = []store.ActivityPubFollower{}
	}

	// Count webmentions
	webmentionCount, _ := h.store.WebmentionCount(ctx)

	secrets := h.cfg.GetSecrets()
	apEnabled := secrets.ActivityPubPrivateKey != ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":          apEnabled,
		"fedi_address":     h.fediAddress(),
		"follower_count":   followerCount,
		"followers":        followers,
		"webmention_count": webmentionCount,
	})
}

// HandleAdminAPIWebmentions returns all webmentions for admin moderation.
func (h *BlogHandler) HandleAdminAPIWebmentions(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	wms, err := h.store.AllWebmentions(r.Context())
	if err != nil {
		jsonError(w, "Failed to load webmentions", http.StatusInternalServerError)
		return
	}
	if wms == nil {
		wms = []store.Webmention{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wms)
}

// HandleAdminAPIDeleteWebmention deletes a webmention by ID.
func (h *BlogHandler) HandleAdminAPIDeleteWebmention(w http.ResponseWriter, r *http.Request) {
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
		jsonError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteWebmention(r.Context(), id); err != nil {
		jsonError(w, "Failed to delete", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleAdminAPIDeleteFollower removes an ActivityPub follower.
func (h *BlogHandler) HandleAdminAPIDeleteFollower(w http.ResponseWriter, r *http.Request) {
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
		jsonError(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.store.RemoveActivityPubFollowerByID(r.Context(), id); err != nil {
		jsonError(w, "Failed to delete follower", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleAdminAPIAuditLog returns recent audit log entries.
func (h *BlogHandler) HandleAdminAPIAuditLog(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	entries, err := h.store.AuditLogs(r.Context(), limit)
	if err != nil {
		jsonError(w, "Failed to load audit log", http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []store.AuditEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// HandleAdminAPIClearAuditLog clears audit entries older than 24 hours.
func (h *BlogHandler) HandleAdminAPIClearAuditLog(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	// Delete entries older than 24 hours (the store enforces the 24h minimum)
	deleted, err := h.store.CleanupAuditLog(r.Context(), 24*time.Hour)
	if err != nil {
		jsonError(w, "Failed to clear audit log", http.StatusInternalServerError)
		return
	}

	h.audit(r, "audit.cleared", fmt.Sprintf("Cleared %d old audit entries", deleted))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "deleted": deleted})
}

// StartAuditLogCleanup periodically removes audit entries older than 14 days.
func (h *BlogHandler) StartAuditLogCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				deleted, err := h.store.CleanupAuditLog(ctx, 14*24*time.Hour)
				if err != nil {
					h.logger.Error("audit log cleanup failed", "error", err)
				} else if deleted > 0 {
					h.logger.Info("audit log cleanup", "deleted", deleted)
				}
				// Also cleanup IP lockout entries
				loginLockout.cleanup()
			}
		}
	}()
}

// HandleAdminAPIPostRevisions lists all revisions for a post.
func (h *BlogHandler) HandleAdminAPIPostRevisions(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	slug := r.PathValue("slug")
	revisions, err := h.store.PostRevisions(r.Context(), slug)
	if err != nil {
		jsonError(w, "Failed to load revisions", http.StatusInternalServerError)
		return
	}
	if revisions == nil {
		revisions = []store.PostRevision{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(revisions)
}

// HandleAdminAPIRestoreRevision restores a post to a previous revision.
func (h *BlogHandler) HandleAdminAPIRestoreRevision(w http.ResponseWriter, r *http.Request) {
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
		jsonError(w, "Invalid revision ID", http.StatusBadRequest)
		return
	}

	rev, err := h.store.GetPostRevision(r.Context(), id)
	if err != nil {
		jsonError(w, "Revision not found", http.StatusNotFound)
		return
	}

	post, ok := h.loader.PostBySlugAdmin(rev.PostSlug)
	if !ok {
		jsonError(w, "Post not found", http.StatusNotFound)
		return
	}

	// Save current version as a revision before restoring
	currentMeta, _ := json.Marshal(map[string]interface{}{
		"tags": post.Tags, "description": post.Description,
		"image": post.Image, "draft": post.Draft, "featured": post.Featured,
	})
	_ = h.store.SavePostRevision(r.Context(), rev.PostSlug, post.Title, post.Content, string(currentMeta))

	// Restore the revision
	var meta map[string]interface{}
	json.Unmarshal([]byte(rev.Metadata), &meta)

	tags := []string{}
	if t, ok := meta["tags"].([]interface{}); ok {
		for _, v := range t {
			if s, ok := v.(string); ok {
				tags = append(tags, s)
			}
		}
	}
	desc, _ := meta["description"].(string)
	image, _ := meta["image"].(string)
	draft, _ := meta["draft"].(bool)
	featured, _ := meta["featured"].(bool)

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
		rev.Title, rev.PostSlug, post.Date.Format("2006-01-02"),
		strings.Join(quoteStrings(tags), ", "),
		desc, image, featured, draft, rev.Content)

	indexPath := filepath.Join(post.Dir, "index.md")
	if err := os.WriteFile(indexPath, []byte(frontmatter), 0o644); err != nil {
		jsonError(w, "Failed to restore revision", http.StatusInternalServerError)
		return
	}

	_ = h.loader.Reload()
	h.logger.Info("post revision restored", "slug", rev.PostSlug, "revision_id", id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "restored"})
}

// HandleAdminAPIBulkPosts handles bulk post operations (delete, publish, draft).
func (h *BlogHandler) HandleAdminAPIBulkPosts(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req struct {
		Action string   `json:"action"` // delete, publish, draft
		Slugs  []string `json:"slugs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if len(req.Slugs) == 0 {
		jsonError(w, "No posts selected", http.StatusBadRequest)
		return
	}

	postsBase := filepath.Join(h.contentDir, "posts")
	processed := 0

	for _, slug := range req.Slugs {
		post, ok := h.loader.PostBySlugAdmin(slug)
		if !ok {
			continue
		}

		switch req.Action {
		case "delete":
			cleaned := filepath.Clean(post.Dir)
			if strings.HasPrefix(cleaned, postsBase) {
				os.RemoveAll(cleaned)
				processed++
			}
		case "publish", "draft":
			isDraft := req.Action == "draft"
			indexPath := filepath.Join(post.Dir, "index.md")
			data, err := os.ReadFile(indexPath)
			if err != nil {
				continue
			}
			content := string(data)
			// SECURITY: Only modify the frontmatter section (between --- delimiters),
			// not the post body, to prevent corrupting content that mentions "draft:".
			fmEnd := -1
			if strings.HasPrefix(content, "---\n") {
				fmEnd = strings.Index(content[4:], "\n---")
				if fmEnd >= 0 {
					fmEnd += 4 + 4 // offset past "---\n" prefix and "\n---"
				}
			}
			if fmEnd > 0 {
				frontmatter := content[:fmEnd]
				body := content[fmEnd:]
				frontmatter = strings.Replace(frontmatter, fmt.Sprintf("draft: %v", !isDraft), fmt.Sprintf("draft: %v", isDraft), 1)
				content = frontmatter + body
			} else {
				// Fallback: replace first occurrence (original behavior)
				content = strings.Replace(content, fmt.Sprintf("draft: %v", !isDraft), fmt.Sprintf("draft: %v", isDraft), 1)
			}
			os.WriteFile(indexPath, []byte(content), 0o644)
			processed++
		}
	}

	_ = h.loader.Reload()
	h.logger.Info("bulk post operation", "action", req.Action, "count", processed)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "processed": processed})
}

// HandleAdminAPIBulkComments handles bulk comment operations (delete, approve, unapprove).
func (h *BlogHandler) HandleAdminAPIBulkComments(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req struct {
		Action string  `json:"action"` // delete, approve, unapprove
		IDs    []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	processed := 0
	for _, id := range req.IDs {
		var err error
		switch req.Action {
		case "delete":
			err = h.store.DeleteComment(r.Context(), id)
		case "approve":
			err = h.store.ApproveComment(r.Context(), id)
		case "unapprove":
			err = h.store.UnapproveComment(r.Context(), id)
		}
		if err == nil {
			processed++
		}
	}

	h.logger.Info("bulk comment operation", "action", req.Action, "count", processed)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "processed": processed})
}

// HandleAdminAPICreateDraftShare creates a secret preview link for a draft post.
func (h *BlogHandler) HandleAdminAPICreateDraftShare(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	slug := r.PathValue("slug")
	_, ok := h.loader.PostBySlugAdmin(slug)
	if !ok {
		jsonError(w, "Post not found", http.StatusNotFound)
		return
	}

	tokenBytes := make([]byte, 16)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)
	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 days

	share, err := h.store.CreateDraftShare(r.Context(), slug, token, expiresAt)
	if err != nil {
		jsonError(w, "Failed to create share link", http.StatusInternalServerError)
		return
	}

	site := h.cfg.GetSite()
	shareURL := site.BaseURL + "/preview/" + token

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":      token,
		"url":        shareURL,
		"expires_at": share.ExpiresAt,
	})
}

// HandleAdminAPIDeleteDraftShare revokes a draft share link.
func (h *BlogHandler) HandleAdminAPIDeleteDraftShare(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	slug := r.PathValue("slug")
	_ = h.store.DeleteDraftShare(r.Context(), slug)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleAdminAPIUploadAvatar handles avatar image upload for the site author.
func (h *BlogHandler) HandleAdminAPIUploadAvatar(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // 5MB max
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		jsonError(w, "File too large (max 5MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		jsonError(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate content type by magic bytes
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	contentType := http.DetectContentType(buf[:n])
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/gif" && contentType != "image/webp" {
		jsonError(w, "Only JPEG, PNG, GIF, or WebP images are allowed", http.StatusBadRequest)
		return
	}
	if seeker, ok := file.(io.ReadSeeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	// Determine extension from content type
	ext := ".jpg"
	switch contentType {
	case "image/png":
		ext = ".png"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	}

	// Save to content/static/
	staticDir := filepath.Join(h.contentDir, "static")
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		jsonError(w, "Failed to create static directory", http.StatusInternalServerError)
		return
	}

	filename := "profile" + ext
	destPath := filepath.Join(staticDir, filename)
	dest, err := os.Create(destPath)
	if err != nil {
		jsonError(w, "Failed to save avatar", http.StatusInternalServerError)
		return
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		jsonError(w, "Failed to write avatar", http.StatusInternalServerError)
		return
	}

	avatarURL := "/static/" + filename
	h.logger.Info("avatar uploaded", "file", filename, "size", header.Size, "type", contentType)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url":      avatarURL,
		"filename": filename,
		"status":   "ok",
	})
}

// HandleAdminAPIExport creates a ZIP archive of content + config directories.
func (h *BlogHandler) HandleAdminAPIExport(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="skriva-export.zip"`)

	zw := newZipWriter(w)
	defer zw.Close()

	// Walk content directory
	contentBase := h.contentDir
	_ = filepath.WalkDir(contentBase, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		relPath, _ := filepath.Rel(contentBase, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}
		fw, err := zw.Create("content/" + filepath.ToSlash(relPath))
		if err != nil {
			return nil
		}
		fw.Write(data)
		return nil
	})

	// Walk config directory
	configDir := h.cfg.GetConfigDir()
	_ = filepath.WalkDir(configDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		// Skip the SQLite database (it can be large and is regenerated)
		// Skip secrets.yaml (contains admin password hash, session secret, AP keys)
		if strings.HasSuffix(d.Name(), ".db") || strings.HasSuffix(d.Name(), ".db-wal") || strings.HasSuffix(d.Name(), ".db-shm") {
			return nil
		}
		if d.Name() == "secrets.yaml" {
			return nil
		}
		// Skip smtp.yaml (contains SMTP password in plaintext)
		if d.Name() == "smtp.yaml" {
			return nil
		}
		// Skip certificate files
		if strings.HasSuffix(d.Name(), ".pem") || strings.HasSuffix(d.Name(), ".key") {
			return nil
		}
		relPath, _ := filepath.Rel(configDir, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		fw, err := zw.Create("config/" + filepath.ToSlash(relPath))
		if err != nil {
			return nil
		}
		fw.Write(data)
		return nil
	})

	h.logger.Info("export ZIP created")
}

// HandleAdminAPIWebhooks lists all webhooks.
func (h *BlogHandler) HandleAdminAPIWebhooks(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	hooks, err := h.store.AllWebhooks(r.Context())
	if err != nil {
		jsonError(w, "Failed to load webhooks", http.StatusInternalServerError)
		return
	}
	if hooks == nil {
		hooks = []store.Webhook{}
	}
	// Redact secrets — never expose webhook signing secrets via API
	for i := range hooks {
		if hooks[i].Secret != "" {
			hooks[i].Secret = "••••••••"
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hooks)
}

// HandleAdminAPICreateWebhook registers a new webhook.
func (h *BlogHandler) HandleAdminAPICreateWebhook(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req struct {
		Event  string `json:"event"`
		URL    string `json:"url"`
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	validEvents := map[string]bool{
		"post.created": true, "post.updated": true, "post.deleted": true,
		"comment.created": true, "subscriber.new": true,
	}
	if !validEvents[req.Event] {
		jsonError(w, "Invalid event type. Valid: post.created, post.updated, post.deleted, comment.created, subscriber.new", http.StatusBadRequest)
		return
	}

	// Validate URL
	if _, err := url.Parse(req.URL); err != nil || req.URL == "" {
		jsonError(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	// Validate URL — must pass SSRF check
	if err := validateExternalURL(req.URL); err != nil {
		jsonError(w, "URL points to private/internal address: "+err.Error(), http.StatusBadRequest)
		return
	}

	hook, err := h.store.AddWebhook(r.Context(), req.Event, req.URL, req.Secret)
	if err != nil {
		jsonError(w, "Failed to create webhook", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(hook)
}

// HandleAdminAPIDeleteWebhook removes a webhook.
func (h *BlogHandler) HandleAdminAPIDeleteWebhook(w http.ResponseWriter, r *http.Request) {
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
		jsonError(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteWebhook(r.Context(), id); err != nil {
		jsonError(w, "Failed to delete webhook", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// FireWebhook sends a webhook payload to all registered endpoints for an event.
func (h *BlogHandler) FireWebhook(event string, payload interface{}) {
	hooks, err := h.store.WebhooksByEvent(context.Background(), event)
	if err != nil || len(hooks) == 0 {
		return
	}

	body, err := json.Marshal(map[string]interface{}{
		"event":     event,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"data":      payload,
	})
	if err != nil {
		return
	}

	client := safeHTTPClient()

	for _, hook := range hooks {
		go func(hookURL, secret string) {
			req, err := http.NewRequest("POST", hookURL, strings.NewReader(string(body)))
			if err != nil {
				h.logger.Error("webhook request error", "url", hookURL, "error", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "Skriva-Webhook/"+blog.Version)

			// HMAC signature if secret is set
			if secret != "" {
				mac := hmac.New(sha256.New, []byte(secret))
				mac.Write(body)
				sig := hex.EncodeToString(mac.Sum(nil))
				req.Header.Set("X-Webhook-Signature", "sha256="+sig)
			}

			resp, err := client.Do(req)
			if err != nil {
				h.logger.Error("webhook delivery failed", "url", hookURL, "error", err)
				return
			}
			_ = resp.Body.Close()
			h.logger.Info("webhook delivered", "event", event, "url", hookURL, "status", resp.StatusCode)
		}(hook.URL, hook.Secret)
	}
}

// HandleAdminAPITokens lists all API tokens.
func (h *BlogHandler) HandleAdminAPITokens(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	tokens, err := h.store.AllAPITokens(r.Context())
	if err != nil {
		jsonError(w, "Failed to load tokens", http.StatusInternalServerError)
		return
	}
	if tokens == nil {
		tokens = []store.APIToken{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

// HandleAdminAPICreateToken generates a new API token.
func (h *BlogHandler) HandleAdminAPICreateToken(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req struct {
		Name   string `json:"name"`
		Scopes string `json:"scopes"` // "read", "read,write", "read,write,admin"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "Name is required", http.StatusBadRequest)
		return
	}
	if req.Scopes == "" {
		req.Scopes = "read"
	}

	// Generate a random token with sk_ prefix
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	rawToken := "sk_" + hex.EncodeToString(tokenBytes)

	// Store the SHA-256 hash (never store the raw token)
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(rawToken)))

	token, err := h.store.AddAPIToken(r.Context(), req.Name, tokenHash, req.Scopes)
	if err != nil {
		jsonError(w, "Failed to create token", http.StatusInternalServerError)
		return
	}

	// Return the raw token ONLY on creation — it's never retrievable again
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     token.ID,
		"name":   token.Name,
		"scopes": token.Scopes,
		"token":  rawToken, // Only shown once!
	})
}

// HandleAdminAPIDeleteToken revokes an API token.
func (h *BlogHandler) HandleAdminAPIDeleteToken(w http.ResponseWriter, r *http.Request) {
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
		jsonError(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteAPIToken(r.Context(), id); err != nil {
		jsonError(w, "Failed to delete token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleMetrics returns basic Prometheus-compatible metrics.
func (h *BlogHandler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()

	// Gather metrics from store
	var totalViews, totalComments, totalSubscribers int64
	h.store.DB().QueryRowContext(ctx, "SELECT COALESCE(SUM(count),0) FROM page_views").Scan(&totalViews)
	h.store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM comments").Scan(&totalComments)
	h.store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM subscribers WHERE confirmed=1").Scan(&totalSubscribers)

	totalPosts := int64(len(h.loader.AllPosts()))
	publishedPosts := int64(len(h.loader.Posts()))
	followerCount, _ := h.store.ActivityPubFollowerCount(ctx)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP skriva_posts_total Total number of posts.\n")
	fmt.Fprintf(w, "# TYPE skriva_posts_total gauge\n")
	fmt.Fprintf(w, "skriva_posts_total %d\n", totalPosts)
	fmt.Fprintf(w, "# HELP skriva_posts_published Published posts.\n")
	fmt.Fprintf(w, "# TYPE skriva_posts_published gauge\n")
	fmt.Fprintf(w, "skriva_posts_published %d\n", publishedPosts)
	fmt.Fprintf(w, "# HELP skriva_views_total Total page views.\n")
	fmt.Fprintf(w, "# TYPE skriva_views_total counter\n")
	fmt.Fprintf(w, "skriva_views_total %d\n", totalViews)
	fmt.Fprintf(w, "# HELP skriva_comments_total Total comments.\n")
	fmt.Fprintf(w, "# TYPE skriva_comments_total counter\n")
	fmt.Fprintf(w, "skriva_comments_total %d\n", totalComments)
	fmt.Fprintf(w, "# HELP skriva_subscribers_confirmed Confirmed newsletter subscribers.\n")
	fmt.Fprintf(w, "# TYPE skriva_subscribers_confirmed gauge\n")
	fmt.Fprintf(w, "skriva_subscribers_confirmed %d\n", totalSubscribers)
	fmt.Fprintf(w, "# HELP skriva_activitypub_followers Fediverse followers.\n")
	fmt.Fprintf(w, "# TYPE skriva_activitypub_followers gauge\n")
	fmt.Fprintf(w, "skriva_activitypub_followers %d\n", followerCount)
}

// HandleAdminSettings serves the settings page.
func (h *BlogHandler) HandleAdminSettings(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	h.serveAdminHTML(w, adminSettingsHTML)
}

// HandleAdminAPIGetSettings returns the current site config as JSON.
func (h *BlogHandler) HandleAdminAPIGetSettings(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	site := h.cfg.GetSite()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(site)
}

// HandleAdminAPIUpdateSettings updates site configuration from admin UI.
func (h *BlogHandler) HandleAdminAPIUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var site config.SiteConfig
	if err := json.NewDecoder(r.Body).Decode(&site); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if site.Title == "" {
		jsonError(w, "Title is required", http.StatusBadRequest)
		return
	}

	oldSite := h.cfg.GetSite()

	if err := h.cfg.UpdateSite(site); err != nil {
		h.logger.Error("updating site config", "error", err)
		jsonError(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	h.logger.Info("site settings updated via admin")
	h.audit(r, "settings.updated", "Site settings updated")

	// If theme changed, reload templates
	if site.Theme != oldSite.Theme {
		if err := h.engine.LoadTemplates(); err != nil {
			h.logger.Error("reloading templates after theme change", "error", err)
			// Revert to old theme
			_ = h.cfg.UpdateSite(oldSite)
			jsonError(w, "Failed to load new theme: "+err.Error(), http.StatusBadRequest)
			return
		}
		h.logger.Info("theme changed", "from", oldSite.Theme, "to", site.Theme)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleAdminAPIListThemes returns available themes.
func (h *BlogHandler) HandleAdminAPIListThemes(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	themes, err := config.ListThemes(h.contentDir, h.engine.EmbeddedThemes())
	if err != nil {
		h.logger.Error("listing themes", "error", err)
		jsonError(w, "Failed to list themes", http.StatusInternalServerError)
		return
	}

	// Mark active theme
	site := h.cfg.GetSite()
	type themeWithActive struct {
		config.ThemeInfo
		Active bool `json:"active"`
	}

	result := make([]themeWithActive, len(themes))
	for i, t := range themes {
		result[i] = themeWithActive{ThemeInfo: t, Active: t.ID == site.Theme}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleAdminAPIPreviewTheme returns a preview URL for a theme without changing the active theme.
func (h *BlogHandler) HandleAdminAPIPreviewTheme(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	themeID := r.URL.Query().Get("theme")
	if themeID == "" {
		jsonError(w, "Theme ID required", http.StatusBadRequest)
		return
	}

	// Verify the theme exists
	themes, err := config.ListThemes(h.contentDir, h.engine.EmbeddedThemes())
	if err != nil {
		jsonError(w, "Failed to list themes", http.StatusInternalServerError)
		return
	}

	found := false
	for _, t := range themes {
		if t.ID == themeID {
			found = true
			break
		}
	}
	if !found {
		jsonError(w, "Theme not found", http.StatusNotFound)
		return
	}

	// Return the CSS preview URL (admin can load theme CSS in an iframe/preview)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"theme_id": themeID,
		"css_url":  "/admin/api/theme-preview-css?theme=" + themeID,
	})
}

// HandleAdminAPIThemePreviewCSS serves a full HTML preview page styled with the theme's CSS.
func (h *BlogHandler) HandleAdminAPIThemePreviewCSS(w http.ResponseWriter, r *http.Request) {
	if !h.isAuthenticated(r) {
		http.NotFound(w, r)
		return
	}

	// Override X-Frame-Options to allow same-origin embedding in admin settings
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")

	themeID := r.URL.Query().Get("theme")
	if themeID == "" || strings.Contains(themeID, "..") || strings.ContainsAny(themeID, "/\\") {
		http.NotFound(w, r)
		return
	}

	// Read theme CSS (filesystem first, then embedded)
	readThemeFile := func(relPath string) ([]byte, error) {
		fsPath := filepath.Join(h.contentDir, "themes", themeID, relPath)
		cleaned := filepath.Clean(fsPath)
		themesBase := filepath.Join(h.contentDir, "themes")
		if strings.HasPrefix(cleaned, themesBase) {
			if data, err := os.ReadFile(cleaned); err == nil {
				return data, nil
			}
		}
		return fs.ReadFile(h.engine.EmbeddedThemes(), "themes/"+themeID+"/"+relPath)
	}

	themeCSS, err := readThemeFile("css/theme.css")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	syntaxCSS, _ := readThemeFile("css/syntax.css")

	site := h.cfg.GetSite()
	title := html.EscapeString(site.Title)
	if title == "" {
		title = "My Blog"
	}

	// Sanitize CSS to prevent style tag breakout (XSS via malicious theme CSS)
	safeThemeCSS := strings.ReplaceAll(string(themeCSS), "</style>", "")
	safeSyntaxCSS := strings.ReplaceAll(string(syntaxCSS), "</style>", "")

	// Serve a self-contained HTML preview
	preview := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>` + safeThemeCSS + `</style>
<style>` + safeSyntaxCSS + `</style>
</head>
<body>
<header class="site-header">
<div class="container">
<div class="site-brand">
<a href="#" class="site-title">` + title + `</a>
<p class="site-tagline">Theme Preview</p>
</div>
<nav class="site-nav">
<a href="#" class="active">Home</a>
<a href="#">Archive</a>
<a href="#">RSS</a>
</nav>
</div>
</header>
<main class="main">
<div class="container">
<article class="post-card">
<div class="post-card-content">
<h2 class="post-card-title"><a href="#">Hello World</a></h2>
<div class="post-card-meta"><time>Jan 1, 2026</time> · <span>3 min read</span></div>
<p class="post-card-excerpt">A sample blog post to preview this theme. The quick brown fox jumps over the lazy dog.</p>
</div>
</article>
<article class="post-card">
<div class="post-card-content">
<h2 class="post-card-title"><a href="#">Building a Blog Engine</a></h2>
<div class="post-card-meta"><time>Feb 14, 2026</time> · <span>8 min read</span></div>
<p class="post-card-excerpt">Exploring Go, SQLite, and embedded themes to create a lightweight personal publishing platform.</p>
</div>
</article>
</div>
</main>
<footer class="site-footer">
<div class="container">
<div class="footer-bottom"><p>&copy; 2026 ` + title + `</p></div>
</div>
</footer>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprint(w, preview)
}
