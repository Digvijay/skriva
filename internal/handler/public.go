// Package handler implements all HTTP handlers for the blog,
// including public pages, API endpoints, and admin functionality.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	blog "github.com/Digvijay/skriva"
	"github.com/Digvijay/skriva/internal/config"
	"github.com/Digvijay/skriva/internal/content"
	"github.com/Digvijay/skriva/internal/plugin"
	"github.com/Digvijay/skriva/internal/render"
	"github.com/Digvijay/skriva/internal/store"
)

const (
	maxCommentLength  = 5000
	maxAuthorLength   = 100
	maxFileSize       = 20 << 20 // 20 MB
	sessionCookieName = "blog_session"
	sessionMaxAge     = 24 * time.Hour
	csrfTokenMaxAge   = 12 * time.Hour
)

// BlogHandler holds all dependencies for HTTP handlers.
type BlogHandler struct {
	cfg        *config.Config
	loader     *content.Loader
	store      *store.Store
	engine     *render.Engine
	logger     *slog.Logger
	contentDir string
	plugins    *plugin.HookRegistry
}

// NewBlogHandler creates a new handler with all dependencies injected.
func NewBlogHandler(
	cfg *config.Config,
	loader *content.Loader,
	store *store.Store,
	engine *render.Engine,
	logger *slog.Logger,
	contentDir string,
) *BlogHandler {
	return &BlogHandler{
		cfg:        cfg,
		loader:     loader,
		store:      store,
		engine:     engine,
		logger:     logger,
		contentDir: contentDir,
		plugins:    plugin.NewHookRegistry(logger),
	}
}

// Plugins returns the plugin hook registry for external registration.
func (h *BlogHandler) Plugins() *plugin.HookRegistry {
	return h.plugins
}

// HandleHome renders the homepage with paginated posts.
func (h *BlogHandler) HandleHome(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	posts := h.loader.Posts()

	// Pagination
	pageNum := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			pageNum = n
		}
	}

	perPage := site.PostsPerPage
	if perPage <= 0 {
		perPage = 10
	}
	totalPages := int(math.Ceil(float64(len(posts)) / float64(perPage)))
	if totalPages < 1 {
		totalPages = 1
	}

	start := (pageNum - 1) * perPage
	end := start + perPage
	if start > len(posts) {
		start = len(posts)
	}
	if end > len(posts) {
		end = len(posts)
	}

	pagePosts := posts[start:end]

	// Render markdown for each post (excerpt/preview)
	for i := range pagePosts {
		renderedHTML, err := h.engine.RenderMarkdown(pagePosts[i].Content)
		if err != nil {
			h.logger.Error("rendering post markdown", "slug", pagePosts[i].Slug, "error", err)
			continue
		}
		pagePosts[i].HTML = renderedHTML
	}

	pagination := &render.Pagination{
		CurrentPage: pageNum,
		TotalPages:  totalPages,
		HasPrev:     pageNum > 1,
		HasNext:     pageNum < totalPages,
	}
	if pagination.HasPrev {
		pagination.PrevURL = fmt.Sprintf("/?page=%d", pageNum-1)
	}
	if pagination.HasNext {
		pagination.NextURL = fmt.Sprintf("/?page=%d", pageNum+1)
	}

	data := &render.TemplateData{
		Site:        site,
		Posts:       pagePosts,
		Tags:        h.loader.Tags(),
		Pagination:  pagination,
		FediAddress: h.fediAddress(),
		Nav:         render.NavData{Path: "/", ActiveSection: "home"},
		Meta: render.SEOMeta{
			Title:        site.Title,
			Description:  site.Tagline,
			CanonicalURL: site.BaseURL + "/",
			OGType:       "website",
		},
	}

	h.renderTemplate(w, "home.html", data)
}

// HandlePostOrPage renders a single post or page by slug.
func (h *BlogHandler) HandlePostOrPage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	// Try post first
	if post, ok := h.loader.PostBySlug(slug); ok {
		h.renderPost(w, r, post)
		return
	}

	// Try page
	if page, ok := h.loader.PageBySlug(slug); ok {
		h.renderPage(w, r, page)
		return
	}

	h.render404(w, r)
}

func (h *BlogHandler) renderPost(w http.ResponseWriter, r *http.Request, post *content.Post) {
	site := h.cfg.GetSite()
	ctx := r.Context()

	// Track page view — skip for authenticated admins, deduplicate by visitor per day
	if !h.isAuthenticated(r) {
		visitorHash := generateFingerprint(r) // hash of IP + User-Agent
		go func() {
			if err := h.store.TrackView(context.Background(), post.Slug, visitorHash); err != nil {
				h.logger.Error("tracking view", "slug", post.Slug, "error", err)
			}
		}()
	}

	// Render markdown
	renderedHTML, err := h.engine.RenderMarkdown(post.Content)
	if err != nil {
		h.logger.Error("rendering markdown", "slug", post.Slug, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Add lazy loading to images
	renderedHTML = render.AddLazyLoading(renderedHTML)
	post.HTML = renderedHTML

	// Extract table of contents from rendered HTML (only if author opted in)
	var toc []render.TOCEntry
	if post.TOC {
		toc = render.ExtractTableOfContents(renderedHTML)
	}

	// Load stats and comments
	stats, err := h.store.PostStatsForSlug(ctx, post.Slug)
	if err != nil {
		h.logger.Error("loading post stats", "slug", post.Slug, "error", err)
		stats = &store.PostStats{}
	}

	comments, err := h.store.CommentsByPost(ctx, post.Slug)
	if err != nil {
		h.logger.Error("loading comments", "slug", post.Slug, "error", err)
	}

	ogImage := ""
	if post.Image != "" {
		ogImage = site.BaseURL + "/media/" + post.Slug + "/" + post.Image
	} else {
		// Auto-generated OG image
		ogImage = site.BaseURL + "/og/" + post.Slug
	}

	// Related posts by shared tags
	related := h.loader.RelatedPosts(post, 3)

	// Load webmentions
	webmentions, err := h.store.WebmentionsByPost(ctx, post.Slug)
	if err != nil {
		h.logger.Error("loading webmentions", "slug", post.Slug, "error", err)
	}

	// Load series posts if post belongs to a series
	var seriesPosts []content.Post
	if post.Series != "" {
		seriesPosts = h.loader.SeriesPosts(post.Series)
	}

	// Compute Fediverse address
	fediAddress := h.fediAddress()

	data := &render.TemplateData{
		Site:            site,
		Post:            post,
		Stats:           stats,
		Comments:        comments,
		RelatedPosts:    related,
		Webmentions:     webmentions,
		FediAddress:     fediAddress,
		TableOfContents: toc,
		SeriesPosts:     seriesPosts,
		Tags:            h.loader.Tags(),
		Nav:             render.NavData{Path: post.Permalink, ActiveSection: "post"},
		Meta: render.SEOMeta{
			Title:        post.Title + " | " + site.Title,
			Description:  post.Description,
			CanonicalURL: site.BaseURL + post.Permalink,
			OGType:       "article",
			OGImage:      ogImage,
		},
	}

	h.renderTemplate(w, "post.html", data)
}

func (h *BlogHandler) renderPage(w http.ResponseWriter, r *http.Request, page *content.Page) {
	site := h.cfg.GetSite()

	renderedHTML, err := h.engine.RenderMarkdown(page.Content)
	if err != nil {
		h.logger.Error("rendering page markdown", "slug", page.Slug, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	page.HTML = renderedHTML

	data := &render.TemplateData{
		Site:        site,
		Page:        page,
		FediAddress: h.fediAddress(),
		Nav:         render.NavData{Path: page.Permalink, ActiveSection: "page"},
		Meta: render.SEOMeta{
			Title:        page.Title + " | " + site.Title,
			Description:  page.Description,
			CanonicalURL: site.BaseURL + page.Permalink,
			OGType:       "website",
		},
	}

	h.renderTemplate(w, "page.html", data)
}

// HandleTag renders posts filtered by tag with pagination.
func (h *BlogHandler) HandleTag(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	site := h.cfg.GetSite()

	allPosts := h.loader.PostsByTag(tag)

	// Pagination
	pageNum := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			pageNum = n
		}
	}
	perPage := site.PostsPerPage
	if perPage <= 0 {
		perPage = 10
	}
	totalPages := int(math.Ceil(float64(len(allPosts)) / float64(perPage)))
	if totalPages < 1 {
		totalPages = 1
	}
	start := (pageNum - 1) * perPage
	end := start + perPage
	if start > len(allPosts) {
		start = len(allPosts)
	}
	if end > len(allPosts) {
		end = len(allPosts)
	}
	posts := allPosts[start:end]

	for i := range posts {
		renderedHTML, err := h.engine.RenderMarkdown(posts[i].Content)
		if err != nil {
			h.logger.Error("rendering post markdown", "slug", posts[i].Slug, "error", err)
			continue
		}
		posts[i].HTML = renderedHTML
	}

	tagInfo := &content.TagInfo{Name: tag, Slug: tag, Count: len(allPosts)}
	pagination := &render.Pagination{
		CurrentPage: pageNum,
		TotalPages:  totalPages,
		HasPrev:     pageNum > 1,
		HasNext:     pageNum < totalPages,
	}
	if pagination.HasPrev {
		pagination.PrevURL = fmt.Sprintf("/tag/%s?page=%d", tag, pageNum-1)
	}
	if pagination.HasNext {
		pagination.NextURL = fmt.Sprintf("/tag/%s?page=%d", tag, pageNum+1)
	}

	data := &render.TemplateData{
		Site:        site,
		Posts:       posts,
		Tag:         tagInfo,
		Tags:        h.loader.Tags(),
		Pagination:  pagination,
		FediAddress: h.fediAddress(),
		Nav:         render.NavData{Path: "/tag/" + tag, ActiveSection: "tag"},
		Meta: render.SEOMeta{
			Title:        "Posts tagged \"" + tag + "\" | " + site.Title,
			Description:  fmt.Sprintf("All posts tagged with %s", tag),
			CanonicalURL: site.BaseURL + "/tag/" + tag,
			OGType:       "website",
		},
	}

	h.renderTemplate(w, "tag.html", data)
}

// HandleTags renders the tags index page listing all tags.
func (h *BlogHandler) HandleTags(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()

	data := &render.TemplateData{
		Site:        site,
		Tags:        h.loader.Tags(),
		FediAddress: h.fediAddress(),
		Nav:         render.NavData{Path: "/tags", ActiveSection: "tags"},
		Meta: render.SEOMeta{
			Title:        "Tags | " + site.Title,
			Description:  "Browse all tags",
			CanonicalURL: site.BaseURL + "/tags",
			OGType:       "website",
		},
	}

	h.renderTemplate(w, "tags.html", data)
}

// HandleArchive renders the chronological archive.
func (h *BlogHandler) HandleArchive(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()

	data := &render.TemplateData{
		Site:        site,
		Posts:       h.loader.Posts(),
		Tags:        h.loader.Tags(),
		FediAddress: h.fediAddress(),
		Nav:         render.NavData{Path: "/archive", ActiveSection: "archive"},
		Meta: render.SEOMeta{
			Title:        "Archive | " + site.Title,
			Description:  "All blog posts",
			CanonicalURL: site.BaseURL + "/archive",
			OGType:       "website",
		},
	}

	h.renderTemplate(w, "archive.html", data)
}

// HandleSearch renders search results.
func (h *BlogHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	query := r.URL.Query().Get("q")

	var results []content.Post
	if query != "" {
		results = h.loader.SearchPosts(query)
		// Render markdown excerpts for results
		for i := range results {
			renderedHTML, err := h.engine.RenderMarkdown(results[i].Content)
			if err == nil {
				results[i].HTML = renderedHTML
			}
		}
	}

	data := &render.TemplateData{
		Site:          site,
		SearchQuery:   query,
		SearchResults: results,
		Tags:          h.loader.Tags(),
		FediAddress:   h.fediAddress(),
		Nav:           render.NavData{Path: "/search", ActiveSection: "search"},
		Meta: render.SEOMeta{
			Title:        "Search | " + site.Title,
			Description:  "Search blog posts",
			CanonicalURL: site.BaseURL + "/search",
			OGType:       "website",
		},
	}

	h.renderTemplate(w, "search.html", data)
}

// HandleFavicon serves the favicon.
func (h *BlogHandler) HandleFavicon(w http.ResponseWriter, r *http.Request) {
	// Try content/static/favicon.ico first
	icoPath := filepath.Join(h.contentDir, "static", "favicon.ico")
	if _, err := os.Stat(icoPath); err == nil {
		http.ServeFile(w, r, icoPath)
		return
	}
	// Try content/static/favicon.svg
	svgPath := filepath.Join(h.contentDir, "static", "favicon.svg")
	if _, err := os.Stat(svgPath); err == nil {
		http.ServeFile(w, r, svgPath)
		return
	}
	// Serve a default inline SVG favicon (pen/blog icon)
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y=".9em" font-size="90">✍️</text></svg>`)
}

// HandleHealthz returns a simple health check.
func (h *BlogHandler) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","version":"%s"}`, blog.Version)
}

// HandleOGImage generates a dynamic Open Graph image for a post.
func (h *BlogHandler) HandleOGImage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	post, ok := h.loader.PostBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}

	site := h.cfg.GetSite()
	title := post.Title
	author := site.Author.Name
	dateStr := post.Date.Format("January 2, 2006")
	siteName := site.Title

	// Generate an SVG OG image (1200x630) — works everywhere, no image libs needed
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630">
<rect width="1200" height="630" fill="#0d1117"/>
<rect x="0" y="0" width="1200" height="8" fill="#58a6ff"/>
<text x="80" y="200" font-family="system-ui,-apple-system,sans-serif" font-size="52" font-weight="bold" fill="#e6edf3" textLength="1040" lengthAdjust="spacingAndGlyphs">%s</text>
<text x="80" y="420" font-family="system-ui,-apple-system,sans-serif" font-size="28" fill="#8b949e">%s</text>
<text x="80" y="470" font-family="system-ui,-apple-system,sans-serif" font-size="24" fill="#656d76">%s</text>
<text x="80" y="560" font-family="system-ui,-apple-system,sans-serif" font-size="22" fill="#58a6ff">%s</text>
</svg>`, html.EscapeString(title), html.EscapeString(author), html.EscapeString(dateStr), html.EscapeString(siteName))

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	fmt.Fprint(w, svg)
}

// HandleRSS serves the RSS feed.
func (h *BlogHandler) HandleRSS(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	posts := h.loader.Posts()

	// Limit to 20 most recent
	if len(posts) > 20 {
		posts = posts[:20]
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")

	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
<channel>
<title>%s</title>
<link>%s</link>
<description>%s</description>
<atom:link href="%s/rss.xml" rel="self" type="application/rss+xml"/>
<language>en-us</language>
`, escapeXML(site.Title), escapeXML(site.BaseURL), escapeXML(site.Tagline), escapeXML(site.BaseURL))

	for i := range posts {
		renderedHTML, _ := h.engine.RenderMarkdown(posts[i].Content)
		// Escape CDATA end sequences to prevent RSS injection
		safeHTML := strings.ReplaceAll(renderedHTML, "]]>", "]]]]><![CDATA[>")
		fmt.Fprintf(w, `<item>
<title>%s</title>
<link>%s%s</link>
<guid>%s%s</guid>
<pubDate>%s</pubDate>
<description><![CDATA[%s]]></description>
</item>
`, escapeXML(posts[i].Title), escapeXML(site.BaseURL), escapeXML(posts[i].Permalink),
			escapeXML(site.BaseURL), escapeXML(posts[i].Permalink),
			posts[i].Date.Format(time.RFC1123Z), safeHTML)
	}

	fmt.Fprint(w, "</channel>\n</rss>")
}

// HandleSitemap serves the sitemap XML.
func (h *BlogHandler) HandleSitemap(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	posts := h.loader.Posts()

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")

	fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
`)

	// Homepage
	fmt.Fprintf(w, "<url><loc>%s/</loc><changefreq>daily</changefreq><priority>1.0</priority></url>\n", escapeXML(site.BaseURL))

	// Posts
	for i := range posts {
		fmt.Fprintf(w, "<url><loc>%s%s</loc><lastmod>%s</lastmod><changefreq>monthly</changefreq><priority>0.8</priority></url>\n",
			escapeXML(site.BaseURL), escapeXML(posts[i].Permalink), posts[i].Date.Format("2006-01-02"))
	}

	// Tags
	for _, t := range h.loader.Tags() {
		fmt.Fprintf(w, "<url><loc>%s/tag/%s</loc><changefreq>weekly</changefreq><priority>0.5</priority></url>\n",
			escapeXML(site.BaseURL), escapeXML(t.Slug))
	}

	fmt.Fprint(w, "</urlset>")
}

// HandleRobots serves robots.txt.
func (h *BlogHandler) HandleRobots(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nDisallow: /admin/\n\nSitemap: %s/sitemap.xml\n", site.BaseURL)
}

// HandleThemeAsset serves CSS and other assets from the active theme.
func (h *BlogHandler) HandleThemeAsset(w http.ResponseWriter, r *http.Request) {
	site := h.cfg.GetSite()
	// Strip /theme/ prefix
	relPath := strings.TrimPrefix(r.URL.Path, "/theme/")
	if relPath == "" || strings.Contains(relPath, "..") {
		http.NotFound(w, r)
		return
	}

	// Try filesystem first
	filePath := filepath.Join(h.contentDir, "themes", site.Theme, relPath)
	cleaned := filepath.Clean(filePath)
	themeBase := filepath.Join(h.contentDir, "themes", site.Theme)
	if strings.HasPrefix(cleaned, themeBase) {
		if _, err := os.Stat(cleaned); err == nil {
			http.ServeFile(w, r, cleaned)
			return
		}
	}

	// Fall back to embedded FS
	embeddedPath := "themes/" + site.Theme + "/" + relPath
	data, err := fs.ReadFile(h.engine.EmbeddedThemes(), embeddedPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Set content type based on extension
	switch {
	case strings.HasSuffix(relPath, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(relPath, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case strings.HasSuffix(relPath, ".svg"):
		w.Header().Set("Content-Type", "image/svg+xml")
	case strings.HasSuffix(relPath, ".png"):
		w.Header().Set("Content-Type", "image/png")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(data)
}

// HandleStatic serves static files from content/static/.
func (h *BlogHandler) HandleStatic(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimPrefix(r.URL.Path, "/static/")
	if relPath == "" || strings.Contains(relPath, "..") {
		http.NotFound(w, r)
		return
	}

	filePath := filepath.Join(h.contentDir, "static", relPath)
	cleaned := filepath.Clean(filePath)
	staticBase := filepath.Join(h.contentDir, "static")
	if !strings.HasPrefix(cleaned, staticBase) {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, cleaned)
}

// HandleMedia serves media files from a post's directory.
func (h *BlogHandler) HandleMedia(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	file := r.PathValue("file")
	if slug == "" || file == "" || strings.Contains(file, "..") {
		http.NotFound(w, r)
		return
	}

	// Find post directory
	post, ok := h.loader.PostBySlug(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}

	filePath := filepath.Join(post.Dir, file)
	cleaned := filepath.Clean(filePath)
	if !strings.HasPrefix(cleaned, filepath.Clean(post.Dir)) {
		http.NotFound(w, r)
		return
	}

	// WebP content negotiation: if browser accepts WebP and a .webp version exists, serve it
	if strings.Contains(r.Header.Get("Accept"), "image/webp") {
		ext := filepath.Ext(cleaned)
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
			webpPath := cleaned + ".webp"
			if _, err := os.Stat(webpPath); err == nil {
				w.Header().Set("Content-Type", "image/webp")
				w.Header().Set("Vary", "Accept")
				http.ServeFile(w, r, webpPath)
				return
			}
		}
	}

	http.ServeFile(w, r, cleaned)
}

// HandleCommentSubmit processes a new comment submission.
func (h *BlogHandler) HandleCommentSubmit(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	// Verify post exists
	if _, ok := h.loader.PostBySlug(slug); !ok {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// CSRF validation
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		http.Error(w, "Invalid or expired form token. Please reload and try again.", http.StatusForbidden)
		return
	}

	// Honeypot field — if filled, it's a bot
	if r.FormValue("website") != "" {
		// Silently accept but don't save
		http.Redirect(w, r, "/"+slug+"#comments", http.StatusSeeOther)
		return
	}

	author := strings.TrimSpace(r.FormValue("author"))
	email := strings.TrimSpace(r.FormValue("email"))
	body := strings.TrimSpace(r.FormValue("content"))

	// Validate
	if author == "" || len(author) > maxAuthorLength {
		http.Error(w, "Invalid author name", http.StatusBadRequest)
		return
	}
	if body == "" || len(body) > maxCommentLength {
		http.Error(w, "Comment must be between 1 and 5000 characters", http.StatusBadRequest)
		return
	}

	comment := &store.Comment{
		PostSlug: slug,
		Author:   author,
		Email:    email,
		Content:  body,
	}

	if err := h.store.AddComment(r.Context(), comment); err != nil {
		h.logger.Error("saving comment", "slug", slug, "error", err)
		http.Error(w, "Failed to save comment", http.StatusInternalServerError)
		return
	}

	h.logger.Info("comment added", "slug", slug, "author", author)
	http.Redirect(w, r, "/"+slug+"#comments", http.StatusSeeOther)
}

// HandleReaction processes a like/dislike.
func (h *BlogHandler) HandleReaction(w http.ResponseWriter, r *http.Request) {
	// CSRF validation (from X-CSRF-Token header)
	if !h.validateCSRFToken(csrfFromRequest(r)) {
		jsonError(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	slug := r.PathValue("slug")

	var req struct {
		Type string `json:"type"` // "like" or "dislike"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Type != "like" && req.Type != "dislike" {
		http.Error(w, "Invalid reaction type", http.StatusBadRequest)
		return
	}

	// Generate fingerprint from IP + User-Agent
	fingerprint := generateFingerprint(r)

	added, err := h.store.AddReaction(r.Context(), slug, req.Type, fingerprint)
	if err != nil {
		h.logger.Error("adding reaction", "slug", slug, "error", err)
		http.Error(w, "Failed to save reaction", http.StatusInternalServerError)
		return
	}

	likes, dislikes, _ := h.store.ReactionsByPost(r.Context(), slug)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"added":    added,
		"likes":    likes,
		"dislikes": dislikes,
	})
}

// fediAddress computes the fediverse address from the site base URL.
// Returns empty string if ActivityPub is disabled.
func (h *BlogHandler) fediAddress() string {
	site := h.cfg.GetSite()
	if !site.ActivityPubEnabled {
		return ""
	}
	if site.BaseURL != "" {
		if u, err := url.Parse(site.BaseURL); err == nil && u.Host != "" {
			return "@blog@" + u.Host
		}
	}
	return ""
}

// HandleDraftPreview renders a draft post using a secret share token.
func (h *BlogHandler) HandleDraftPreview(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	share, err := h.store.GetDraftShare(r.Context(), token)
	if err != nil {
		http.Error(w, "Invalid or expired share link", http.StatusGone)
		return
	}

	post, ok := h.loader.PostBySlugAdmin(share.PostSlug)
	if !ok {
		http.NotFound(w, r)
		return
	}

	site := h.cfg.GetSite()
	renderedHTML, _ := h.engine.RenderMarkdown(post.Content)
	post.HTML = renderedHTML

	data := &render.TemplateData{
		Site: site,
		Post: post,
		Meta: render.SEOMeta{
			Title:       "[PREVIEW] " + post.Title,
			Description: post.Description,
			OGType:      "article",
		},
		Nav: render.NavData{Path: "/" + post.Slug},
	}

	output, err := h.engine.RenderPage("post.html", data)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Robots-Tag", "noindex")
	fmt.Fprint(w, output)
}
