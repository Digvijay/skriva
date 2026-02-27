// Package server provides the HTTP server with routing, middleware,
// and graceful shutdown support.
package server

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"github.com/Digvijay/skriva/internal/config"
	"github.com/Digvijay/skriva/internal/content"
	"github.com/Digvijay/skriva/internal/handler"
	"github.com/Digvijay/skriva/internal/render"
	"github.com/Digvijay/skriva/internal/store"
)

// Server wraps the HTTP server and all dependencies.
type Server struct {
	cfg         *config.Config
	loader      *content.Loader
	store       *store.Store
	engine      *render.Engine
	logger      *slog.Logger
	port        string
	contentDir  string
	configDir   string
	httpServer  *http.Server
	blogHandler *handler.BlogHandler
}

// New creates a new Server instance with all dependencies.
func New(
	cfg *config.Config,
	loader *content.Loader,
	store *store.Store,
	engine *render.Engine,
	logger *slog.Logger,
	port string,
	contentDir string,
	configDir string,
) *Server {
	return &Server{
		cfg:        cfg,
		loader:     loader,
		store:      store,
		engine:     engine,
		logger:     logger,
		port:       port,
		contentDir: contentDir,
		configDir:  configDir,
	}
}

// ListenAndServe starts the HTTP server and blocks until the context is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	rl := newRateLimiter()
	go rl.cleanup(ctx)

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	// Apply middleware stack
	var h http.Handler = mux
	h = s.etagMiddleware(h)
	h = s.rateLimitMiddleware(h, rl)
	h = s.maxBodyMiddleware(h)
	h = s.securityHeaders(h)
	h = s.requestLogger(h)
	h = s.recoverer(h)

	s.httpServer = &http.Server{
		Addr:         ":" + s.port,
		Handler:      h,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start content watcher
	watcher, err := content.NewWatcher(s.loader, s.logger)
	if err != nil {
		s.logger.Warn("content watcher failed to start", "error", err)
	} else {
		defer func() { _ = watcher.Close() }()
	}

	// Start config watcher
	if err := s.cfg.WatchForChanges(s.logger); err != nil {
		s.logger.Warn("config watcher failed to start", "error", err)
	}

	// Start newsletter scheduler (checks for due scheduled newsletters every minute)
	if s.blogHandler != nil {
		s.blogHandler.StartNewsletterScheduler(ctx)
		s.blogHandler.StartAuditLogCleanup(ctx)
	}

	// Start server in goroutine
	errCh := make(chan error, 1)

	tlsDomain := os.Getenv("BLOG_TLS_DOMAIN")
	if tlsDomain != "" {
		// Auto-TLS via Let's Encrypt
		certDir := filepath.Join(s.configDir, "certs")
		_ = os.MkdirAll(certDir, 0o700)

		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(tlsDomain),
			Cache:      autocert.DirCache(certDir),
		}

		s.httpServer.Addr = ":443"
		s.httpServer.TLSConfig = &tls.Config{
			GetCertificate: m.GetCertificate,
			MinVersion:     tls.VersionTLS12,
		}

		// HTTP→HTTPS redirect + ACME challenge on port 80
		go func() {
			redirect := &http.Server{
				Addr: ":80",
				Handler: m.HTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					target := "https://" + tlsDomain + r.URL.RequestURI()
					http.Redirect(w, r, target, http.StatusMovedPermanently)
				})),
			}
			s.logger.Info("HTTP→HTTPS redirect listening", "addr", ":80")
			_ = redirect.ListenAndServe()
		}()

		go func() {
			s.logger.Info("TLS server listening", "addr", ":443", "domain", tlsDomain)
			if err := s.httpServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				errCh <- fmt.Errorf("TLS server error: %w", err)
			}
			close(errCh)
		}()
	} else {
		go func() {
			s.logger.Info("server listening", "addr", s.httpServer.Addr)
			if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- fmt.Errorf("server error: %w", err)
			}
			close(errCh)
		}()
	}

	// Wait for shutdown signal or error
	select {
	case <-ctx.Done():
		s.logger.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	blog := handler.NewBlogHandler(s.cfg, s.loader, s.store, s.engine, s.logger, s.contentDir)

	// Store handler reference for scheduler startup
	s.blogHandler = blog

	// Public routes
	mux.HandleFunc("GET /{$}", blog.HandleHome)
	mux.HandleFunc("GET /tag/{tag}", blog.HandleTag)
	mux.HandleFunc("GET /tags", blog.HandleTags)
	mux.HandleFunc("GET /archive", blog.HandleArchive)
	mux.HandleFunc("GET /search", blog.HandleSearch)
	mux.HandleFunc("GET /rss.xml", blog.HandleRSS)
	mux.HandleFunc("GET /sitemap.xml", blog.HandleSitemap)
	mux.HandleFunc("GET /robots.txt", blog.HandleRobots)
	mux.HandleFunc("GET /favicon.ico", blog.HandleFavicon)
	mux.HandleFunc("GET /favicon.svg", blog.HandleFavicon)
	mux.HandleFunc("GET /healthz", blog.HandleHealthz)
	mux.HandleFunc("GET /metrics", blog.HandleMetrics)
	mux.HandleFunc("GET /og/{slug}", blog.HandleOGImage)

	// Static files
	mux.HandleFunc("GET /theme/", blog.HandleThemeAsset)
	mux.HandleFunc("GET /static/", blog.HandleStatic)
	mux.HandleFunc("GET /media/{slug}/{file...}", blog.HandleMedia)

	// API routes
	mux.HandleFunc("POST /api/comment/{slug}", blog.HandleCommentSubmit)
	mux.HandleFunc("POST /api/reaction/{slug}", blog.HandleReaction)
	mux.HandleFunc("POST /api/subscribe", blog.HandleNewsletterSubscribe)
	mux.HandleFunc("GET /api/unsubscribe", blog.HandleNewsletterUnsubscribe)

	// Newsletter tracking (public, no auth)
	mux.HandleFunc("GET /api/newsletter/open", blog.HandleNewsletterTrackOpen)
	mux.HandleFunc("GET /api/newsletter/click", blog.HandleNewsletterTrackClick)

	// Webmention endpoint (public)
	mux.HandleFunc("POST /webmention", blog.HandleWebmentionReceive)

	// Draft preview (public, token-based auth)
	mux.HandleFunc("GET /preview/{token}", blog.HandleDraftPreview)

	// ActivityPub & Webfinger endpoints
	mux.HandleFunc("GET /.well-known/webfinger", blog.HandleWebfinger)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", blog.HandleIndieAuthMetadata)
	mux.HandleFunc("GET /activitypub/actor", blog.HandleActivityPubActor)
	mux.HandleFunc("POST /activitypub/inbox", blog.HandleActivityPubInbox)
	mux.HandleFunc("GET /activitypub/outbox", blog.HandleActivityPubOutbox)
	mux.HandleFunc("GET /activitypub/followers", blog.HandleActivityPubFollowers)

	// IndieAuth endpoints
	mux.HandleFunc("GET /indieauth/auth", blog.HandleIndieAuthAuthorize)
	mux.HandleFunc("POST /indieauth/auth", blog.HandleIndieAuthAuthorize)
	mux.HandleFunc("POST /indieauth/token", blog.HandleIndieAuthToken)

	// Micropub endpoint
	mux.HandleFunc("GET /micropub", blog.HandleMicropub)
	mux.HandleFunc("POST /micropub", blog.HandleMicropub)
	mux.HandleFunc("POST /micropub/media", blog.HandleMicropubMedia)

	// Admin routes (authentication checked in handlers)
	mux.HandleFunc("GET /admin/", blog.HandleAdmin)
	mux.HandleFunc("GET /admin/login", blog.HandleAdminLogin)
	mux.HandleFunc("POST /admin/api/login", blog.HandleAdminAPILogin)
	mux.HandleFunc("POST /admin/api/logout", blog.HandleAdminAPILogout)
	mux.HandleFunc("GET /admin/editor", blog.HandleAdminEditor)
	mux.HandleFunc("GET /admin/editor/{slug}", blog.HandleAdminEditor)
	mux.HandleFunc("GET /admin/api/posts", blog.HandleAdminAPIPosts)
	mux.HandleFunc("POST /admin/api/posts", blog.HandleAdminAPICreatePost)
	mux.HandleFunc("PUT /admin/api/posts/{slug}", blog.HandleAdminAPIUpdatePost)
	mux.HandleFunc("DELETE /admin/api/posts/{slug}", blog.HandleAdminAPIDeletePost)
	mux.HandleFunc("POST /admin/api/posts/bulk", blog.HandleAdminAPIBulkPosts)
	mux.HandleFunc("POST /admin/api/comments/bulk", blog.HandleAdminAPIBulkComments)
	mux.HandleFunc("GET /admin/api/posts/{slug}/revisions", blog.HandleAdminAPIPostRevisions)
	mux.HandleFunc("POST /admin/api/revisions/{id}/restore", blog.HandleAdminAPIRestoreRevision)
	mux.HandleFunc("POST /admin/api/posts/{slug}/share", blog.HandleAdminAPICreateDraftShare)
	mux.HandleFunc("DELETE /admin/api/posts/{slug}/share", blog.HandleAdminAPIDeleteDraftShare)
	mux.HandleFunc("GET /admin/api/comments", blog.HandleAdminAPIComments)
	mux.HandleFunc("DELETE /admin/api/comments/{id}", blog.HandleAdminAPIDeleteComment)
	mux.HandleFunc("POST /admin/api/comments/{id}/approve", blog.HandleAdminAPIApproveComment)
	mux.HandleFunc("POST /admin/api/comments/{id}/unapprove", blog.HandleAdminAPIUnapproveComment)
	mux.HandleFunc("GET /admin/api/stats", blog.HandleAdminAPIStats)
	mux.HandleFunc("POST /admin/api/media/{slug}", blog.HandleAdminAPIUploadMedia)
	mux.HandleFunc("POST /admin/api/preview", blog.HandleAdminAPIPreviewMarkdown)
	mux.HandleFunc("POST /admin/api/avatar", blog.HandleAdminAPIUploadAvatar)
	mux.HandleFunc("GET /admin/api/unsplash", blog.HandleAdminAPIUnsplash)
	mux.HandleFunc("GET /admin/api/export", blog.HandleAdminAPIExport)

	// Admin: Webhooks
	mux.HandleFunc("GET /admin/api/webhooks", blog.HandleAdminAPIWebhooks)
	mux.HandleFunc("POST /admin/api/webhooks", blog.HandleAdminAPICreateWebhook)
	mux.HandleFunc("DELETE /admin/api/webhooks/{id}", blog.HandleAdminAPIDeleteWebhook)

	// Admin: API tokens
	mux.HandleFunc("GET /admin/api/tokens", blog.HandleAdminAPITokens)
	mux.HandleFunc("POST /admin/api/tokens", blog.HandleAdminAPICreateToken)
	mux.HandleFunc("DELETE /admin/api/tokens/{id}", blog.HandleAdminAPIDeleteToken)

	// Admin settings & themes
	mux.HandleFunc("GET /admin/settings", blog.HandleAdminSettings)
	mux.HandleFunc("GET /admin/api/settings", blog.HandleAdminAPIGetSettings)
	mux.HandleFunc("PUT /admin/api/settings", blog.HandleAdminAPIUpdateSettings)
	mux.HandleFunc("GET /admin/api/themes", blog.HandleAdminAPIListThemes)
	mux.HandleFunc("GET /admin/api/theme-preview", blog.HandleAdminAPIPreviewTheme)
	mux.HandleFunc("GET /admin/api/theme-preview-css", blog.HandleAdminAPIThemePreviewCSS)

	// TOTP 2FA setup
	mux.HandleFunc("GET /admin/api/totp/status", blog.HandleAdminAPITOTPStatus)
	mux.HandleFunc("POST /admin/api/password", blog.HandleAdminAPIChangePassword)
	mux.HandleFunc("POST /admin/api/totp/setup", blog.HandleAdminAPITOTPSetup)
	mux.HandleFunc("POST /admin/api/totp/confirm", blog.HandleAdminAPITOTPConfirm)
	mux.HandleFunc("POST /admin/api/totp/disable", blog.HandleAdminAPITOTPDisable)

	// Passkey (WebAuthn) routes
	mux.HandleFunc("GET /admin/api/passkeys", blog.HandleAdminAPIPasskeys)
	mux.HandleFunc("POST /admin/api/passkeys/register/begin", blog.HandleAdminAPIPasskeyBeginRegister)
	mux.HandleFunc("POST /admin/api/passkeys/register/finish", blog.HandleAdminAPIPasskeyFinishRegister)
	mux.HandleFunc("DELETE /admin/api/passkeys/{id}", blog.HandleAdminAPIDeletePasskey)
	mux.HandleFunc("POST /admin/api/passkeys/login/begin", blog.HandleAdminAPIPasskeyBeginLogin)
	mux.HandleFunc("POST /admin/api/passkeys/login/finish", blog.HandleAdminAPIPasskeyFinishLogin)

	// Newsletter admin routes
	mux.HandleFunc("GET /admin/newsletter", blog.HandleAdminNewsletter)
	mux.HandleFunc("GET /admin/api/newsletters", blog.HandleAdminAPINewsletters)
	mux.HandleFunc("POST /admin/api/newsletters", blog.HandleAdminAPICreateNewsletter)
	mux.HandleFunc("PUT /admin/api/newsletters/{id}", blog.HandleAdminAPIUpdateNewsletter)
	mux.HandleFunc("DELETE /admin/api/newsletters/{id}", blog.HandleAdminAPIDeleteNewsletter)
	mux.HandleFunc("POST /admin/api/newsletters/{id}/schedule", blog.HandleAdminAPIScheduleNewsletter)
	mux.HandleFunc("POST /admin/api/newsletters/{id}/unschedule", blog.HandleAdminAPIUnscheduleNewsletter)
	mux.HandleFunc("POST /admin/api/newsletters/{id}/send", blog.HandleAdminAPISendNewsletter)
	mux.HandleFunc("POST /admin/api/newsletters/preview", blog.HandleAdminAPINewsletterPreview)
	mux.HandleFunc("GET /admin/api/newsletters/{id}/analytics", blog.HandleAdminAPINewsletterAnalytics)
	mux.HandleFunc("POST /admin/api/newsletters/{id}/ab-test", blog.HandleAdminAPICreateABTest)
	mux.HandleFunc("POST /admin/api/newsletters/{id}/retry", blog.HandleAdminAPIRetryNewsletter)
	mux.HandleFunc("GET /admin/api/subscribers", blog.HandleAdminAPISubscribers)
	mux.HandleFunc("DELETE /admin/api/subscribers/{id}", blog.HandleAdminAPIDeleteSubscriber)

	// Admin: Fediverse & IndieWeb
	mux.HandleFunc("GET /admin/api/fediverse", blog.HandleAdminAPIFediverseStats)
	mux.HandleFunc("GET /admin/api/webmentions", blog.HandleAdminAPIWebmentions)
	mux.HandleFunc("DELETE /admin/api/webmentions/{id}", blog.HandleAdminAPIDeleteWebmention)
	mux.HandleFunc("DELETE /admin/api/followers/{id}", blog.HandleAdminAPIDeleteFollower)

	// Admin: Audit log
	mux.HandleFunc("GET /admin/api/audit-log", blog.HandleAdminAPIAuditLog)
	mux.HandleFunc("POST /admin/api/audit-log/clear", blog.HandleAdminAPIClearAuditLog)

	// Generate ActivityPub keys on startup if needed
	if err := blog.GenerateActivityPubKeys(); err != nil {
		s.logger.Warn("failed to generate ActivityPub keys", "error", err)
	}

	// Register plugin custom routes under /plugins/
	for path, handler := range blog.Plugins().Routes() {
		routePath := "GET /plugins/" + path
		mux.HandleFunc(routePath, func(w http.ResponseWriter, r *http.Request) {
			handler(w, r)
		})
	}

	// Catch-all: /{slug} must be last — matches posts and pages
	mux.HandleFunc("GET /{slug}", blog.HandlePostOrPage)
}

// --- Middleware ---

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		// HSTS: instruct browsers to always use HTTPS
		// Safe to set unconditionally — browsers ignore it on plain HTTP
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")

		// CSP: strict for public pages, relaxed for admin (needs inline scripts for editor)
		if isAdminPath(r.URL.Path) {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https://images.unsplash.com; connect-src 'self' https://api.unsplash.com; frame-src 'self'")
		} else {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-src https://www.youtube.com https://player.vimeo.com")
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusRecorder{ResponseWriter: w, status: 200}

		next.ServeHTTP(wrapped, r)

		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration", time.Since(start).String(),
			"remote", r.RemoteAddr,
		)
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				s.logger.Error("panic recovered", "error", err, "path", r.URL.Path)
				// Try to render a themed error page; fall back to plain text
				if s.blogHandler != nil {
					s.blogHandler.Render500(w, r)
				} else {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func isAdminPath(path string) bool {
	return len(path) >= 7 && path[:7] == "/admin/"
}

// --- Rate Limiter ---

// rateLimiter implements a sliding-window rate limiter per IP.
type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	maxIPs   int // cap on distinct IPs to prevent memory exhaustion
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		requests: make(map[string][]time.Time),
		maxIPs:   100000, // cap at 100k distinct IPs (~80MB worst case)
	}
}

// allow checks whether the given IP is within the rate limit.
// limit is the max requests allowed within window duration.
func (rl *rateLimiter) allow(ip string, limit int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	// SECURITY: If we've hit the IP cap, reject new IPs to prevent memory exhaustion.
	// Existing tracked IPs continue to work normally.
	if _, exists := rl.requests[ip]; !exists && len(rl.requests) >= rl.maxIPs {
		return false
	}

	// Filter to only recent requests
	recent := rl.requests[ip][:0]
	for _, t := range rl.requests[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= limit {
		rl.requests[ip] = recent
		return false
	}

	rl.requests[ip] = append(recent, now)
	return true
}

// cleanup periodically removes stale entries to prevent memory growth.
func (rl *rateLimiter) cleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-10 * time.Minute)
			for ip, times := range rl.requests {
				var recent []time.Time
				for _, t := range times {
					if t.After(cutoff) {
						recent = append(recent, t)
					}
				}
				if len(recent) == 0 {
					delete(rl.requests, ip)
				} else {
					rl.requests[ip] = recent
				}
			}
			rl.mu.Unlock()
		}
	}
}

// maxBodyMiddleware enforces a default maximum request body size on all POST/PUT/DELETE
// requests. Individual handlers may set a lower limit (e.g., 64KB for login) which
// takes priority since MaxBytesReader wraps the existing reader. This prevents
// memory exhaustion from oversized request bodies sent by malicious clients.
const defaultMaxBodySize = 1 << 20 // 1MB default for JSON API bodies

func (s *Server) maxBodyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			// File upload endpoints get a larger limit (handled in their own handlers)
			path := r.URL.Path
			if strings.HasPrefix(path, "/admin/api/media/") || path == "/admin/api/avatar" || path == "/micropub/media" {
				// These handlers set their own MaxBytesReader (20MB / 5MB)
				next.ServeHTTP(w, r)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, defaultMaxBodySize)
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware applies per-IP rate limits based on the request path and method.
func (s *Server) rateLimitMiddleware(next http.Handler, rl *rateLimiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only rate-limit state-changing requests
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		ip := extractIP(r)
		path := r.URL.Path

		var limit int
		var window time.Duration

		switch {
		case path == "/admin/api/login":
			limit, window = 5, time.Minute // 5 login attempts per minute
		case strings.HasPrefix(path, "/api/comment/"):
			limit, window = 3, time.Minute // 3 comments per minute
		case strings.HasPrefix(path, "/api/reaction/"):
			limit, window = 3, time.Minute // 3 reactions per minute (DB enforces one-per-post)
		case path == "/api/subscribe":
			limit, window = 3, time.Minute // 3 subscribe attempts per minute
		case path == "/activitypub/inbox":
			limit, window = 10, time.Minute // 10 AP inbox deliveries per minute
		case path == "/webmention":
			limit, window = 10, time.Minute // 10 webmentions per minute
		case strings.HasPrefix(path, "/indieauth/"):
			limit, window = 10, time.Minute // 10 IndieAuth requests per minute
		case strings.HasPrefix(path, "/micropub"):
			limit, window = 10, time.Minute // 10 Micropub requests per minute
		case strings.HasPrefix(path, "/admin/api/"):
			limit, window = 30, time.Minute // 30 admin API calls per minute
		default:
			next.ServeHTTP(w, r)
			return
		}

		if !rl.allow(ip, limit, window) {
			s.logger.Warn("rate limit exceeded", "ip", ip, "path", path)
			http.Error(w, "Too many requests. Please try again later.", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func extractIP(r *http.Request) string {
	// Do NOT trust X-Forwarded-For by default — it can be spoofed by direct clients.
	// Only the direct TCP connection's RemoteAddr is reliable without a trusted proxy list.
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// --- ETag Middleware ---

// etagMiddleware adds ETag headers and returns 304 Not Modified for matching responses.
// Only applies to GET requests for public HTML pages (not admin, not API).
func (s *Server) etagMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only cache GET requests for public pages
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path
		// Skip admin, API, static files, and streaming endpoints
		if isAdminPath(path) || strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/static/") ||
			strings.HasPrefix(path, "/theme/") || strings.HasPrefix(path, "/media/") ||
			path == "/metrics" || path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		// Capture the response body
		rec := &bodyRecorder{ResponseWriter: w, body: make([]byte, 0, 4096), status: 200}
		next.ServeHTTP(rec, r)

		// Only ETag successful HTML responses
		if rec.status == 200 && len(rec.body) > 0 {
			hash := sha256.Sum256(rec.body)
			etag := `"` + hex.EncodeToString(hash[:8]) + `"`
			w.Header().Set("ETag", etag)
			w.Header().Set("Cache-Control", "no-cache") // must revalidate

			if match := r.Header.Get("If-None-Match"); match == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		w.WriteHeader(rec.status)
		w.Write(rec.body)
	})
}

// bodyRecorder captures the response body for ETag computation.
type bodyRecorder struct {
	http.ResponseWriter
	body   []byte
	status int
}

func (br *bodyRecorder) WriteHeader(code int) {
	br.status = code
}

func (br *bodyRecorder) Write(b []byte) (int, error) {
	br.body = append(br.body, b...)
	return len(b), nil
}
