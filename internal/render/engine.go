// Package render handles markdown-to-HTML conversion using goldmark,
// template loading/caching from theme directories, and syntax highlighting via chroma.
package render

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Digvijay/skriva/internal/config"
	"github.com/Digvijay/skriva/internal/content"
	"github.com/Digvijay/skriva/internal/store"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// SEOMeta holds computed SEO metadata for a page.
type SEOMeta struct {
	Title        string
	Description  string
	CanonicalURL string
	OGType       string
	OGImage      string
	JSONLDType   string
	JSONLDData   template.JS
}

// NavData holds navigation state for the current request.
type NavData struct {
	Path          string
	ActiveSection string
}

// Pagination holds page navigation data.
type Pagination struct {
	CurrentPage int
	TotalPages  int
	HasPrev     bool
	HasNext     bool
	PrevURL     string
	NextURL     string
	Pages       []int
}

// TemplateData is the data contract passed to all templates.
type TemplateData struct {
	Site            config.SiteConfig
	Meta            SEOMeta
	Nav             NavData
	Post            *content.Post
	Posts           []content.Post
	Page            *content.Page
	Tags            []content.TagInfo
	Tag             *content.TagInfo
	Stats           *store.PostStats
	Comments        []store.Comment
	Pagination      *Pagination
	CSRFToken       string
	Year            int
	Error           string
	RelatedPosts    []content.Post
	SearchQuery     string
	SearchResults   []content.Post
	Webmentions     []store.Webmention
	FediAddress     string // e.g. @blog@yourdomain.com
	TableOfContents []TOCEntry
	SeriesPosts     []content.Post // All posts in the same series
}

// Engine handles template rendering and markdown conversion.
type Engine struct {
	cfg            *config.Config
	logger         *slog.Logger
	md             goldmark.Markdown
	contentDir     string
	embeddedThemes embed.FS
	cache          *PageCache

	mu    sync.RWMutex
	tmpls map[string]*template.Template
	funcs template.FuncMap
}

// NewEngine creates a new render engine with markdown pipeline and template loading.
func NewEngine(contentDir string, cfg *config.Config, logger *slog.Logger, embeddedThemes embed.FS) (*Engine, error) {
	// Setup goldmark with extensions
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,         // GitHub Flavored Markdown (tables, strikethrough, autolinks)
			extension.Typographer, // Smart quotes, dashes, ellipsis
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
				highlighting.WithFormatOptions(),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // Auto-generate heading IDs for anchor links
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(), // Allow raw HTML in markdown (for embeds)
		),
	)

	e := &Engine{
		cfg:            cfg,
		logger:         logger,
		md:             md,
		contentDir:     contentDir,
		embeddedThemes: embeddedThemes,
		tmpls:          make(map[string]*template.Template),
		cache:          NewPageCache(200),
	}

	e.funcs = template.FuncMap{
		"formatDate":  formatDate,
		"readingTime": readingTime,
		"truncate":    truncate,
		"safeHTML":    safeHTML,
		"slugify":     slugify,
		"currentYear": func() int { return time.Now().Year() },
		"cacheVer":    func() int64 { return time.Now().Unix() / 3600 }, // changes hourly
		"hasPrefix":   strings.HasPrefix,
		"lower":       strings.ToLower,
		"upper":       strings.ToUpper,
		"join":        strings.Join,
		"add":         func(a, b int) int { return a + b },
		"sub":         func(a, b int) int { return a - b },
		"seq":         seq,
		"jsonLD":      jsonLD,
		"i18n":        func(key string) string { return cfg.Translate(key) },
	}

	if err := e.LoadTemplates(); err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}

	return e, nil
}

// RenderMarkdown converts markdown content to HTML.
// This is used for admin-authored content (posts, pages, newsletters)
// where raw HTML in markdown is intentionally allowed (for embeds etc.).
func (e *Engine) RenderMarkdown(source string) (string, error) {
	var buf bytes.Buffer
	if err := e.md.Convert([]byte(source), &buf); err != nil {
		return "", fmt.Errorf("converting markdown: %w", err)
	}
	return buf.String(), nil
}

// RenderMarkdownUntrusted converts markdown to HTML and then sanitizes the output
// to remove dangerous elements (script, iframe, event handlers, etc.).
// Use this for ALL content from external/untrusted sources: Micropub, ActivityPub,
// webmention content, or any future feature that accepts content from third parties.
//
// This is the security boundary: even though goldmark's html.WithUnsafe() passes
// raw HTML through, this wrapper ensures the output is safe for rendering.
func (e *Engine) RenderMarkdownUntrusted(source string, sanitizer func(string) string) (string, error) {
	rendered, err := e.RenderMarkdown(source)
	if err != nil {
		return "", err
	}
	return sanitizer(rendered), nil
}

// RenderPage renders a named template with the given data.
func (e *Engine) RenderPage(name string, data *TemplateData) (string, error) {
	e.mu.RLock()
	tmpl, ok := e.tmpls[name]
	e.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("template %q not found", name)
	}

	// Always set the current year
	data.Year = time.Now().Year()

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("executing template %q: %w", name, err)
	}
	return buf.String(), nil
}

// LoadTemplates loads or reloads all templates from the active theme.
func (e *Engine) LoadTemplates() error {
	site := e.cfg.GetSite()
	themeName := site.Theme
	if themeName == "" {
		themeName = "classic"
	}

	// Look for theme in content dir first (user override), then fall back to embedded
	themeDir := filepath.Join(e.contentDir, "themes", themeName)
	useEmbedded := false
	if _, err := os.Stat(themeDir); os.IsNotExist(err) {
		// Check if theme exists in embedded FS
		if _, err := fs.Stat(e.embeddedThemes, "themes/"+themeName); err != nil {
			return fmt.Errorf("theme %q not found in content dir or embedded themes", themeName)
		}
		useEmbedded = true
		e.logger.Info("using embedded theme", "theme", themeName)
	}

	// readFile reads from filesystem or embedded FS depending on theme location
	readFile := func(relPath string) ([]byte, error) {
		if useEmbedded {
			return fs.ReadFile(e.embeddedThemes, "themes/"+themeName+"/"+relPath)
		}
		return os.ReadFile(filepath.Join(themeDir, relPath))
	}

	// readDir lists directory entries from filesystem or embedded FS
	readDir := func(relPath string) ([]fs.DirEntry, error) {
		if useEmbedded {
			return fs.ReadDir(e.embeddedThemes, "themes/"+themeName+"/"+relPath)
		}
		return os.ReadDir(filepath.Join(themeDir, relPath))
	}

	// Load partials first
	partials := make(map[string]string)
	partialEntries, err := readDir("templates/partials")
	if err == nil {
		for _, entry := range partialEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
				continue
			}
			partialContent, err := readFile("templates/partials/" + entry.Name())
			if err != nil {
				return fmt.Errorf("reading partial %q: %w", entry.Name(), err)
			}
			name := strings.TrimSuffix(entry.Name(), ".html")
			partials[name] = string(partialContent)
		}
	}

	// Load base template
	baseContent, err := readFile("templates/base.html")
	if err != nil {
		return fmt.Errorf("reading base.html: %w", err)
	}

	// Load each page template
	pageTemplates := []string{"home.html", "post.html", "page.html", "tag.html", "tags.html", "archive.html", "search.html", "404.html"}
	tmpls := make(map[string]*template.Template)

	for _, name := range pageTemplates {
		pageContent, err := readFile("templates/" + name)
		if err != nil {
			e.logger.Warn("template not found, skipping", "template", name)
			continue
		}

		// Parse: base + partials + page template
		tmpl := template.New("base.html").Funcs(e.funcs)

		// Parse base
		if _, err := tmpl.Parse(string(baseContent)); err != nil {
			return fmt.Errorf("parsing base.html: %w", err)
		}

		// Parse all partials
		for partialName, partialContent := range partials {
			if _, err := tmpl.New(partialName).Parse(partialContent); err != nil {
				return fmt.Errorf("parsing partial %q: %w", partialName, err)
			}
		}

		// Parse page template (overrides content block)
		if _, err := tmpl.New(name).Parse(string(pageContent)); err != nil {
			return fmt.Errorf("parsing template %q: %w", name, err)
		}

		tmpls[name] = tmpl
	}

	e.mu.Lock()
	e.tmpls = tmpls
	e.mu.Unlock()

	// Invalidate the page cache when templates change
	e.cache.Invalidate()

	e.logger.Info("templates loaded", "theme", themeName, "count", len(tmpls))
	return nil
}

// InvalidateCache clears the rendered page cache. Call after content changes.
func (e *Engine) InvalidateCache() {
	e.cache.Invalidate()
}

// CacheStats returns the number of cached pages.
func (e *Engine) CacheStats() int {
	return e.cache.Size()
}

// EmbeddedThemes returns the embedded themes FS.
func (e *Engine) EmbeddedThemes() embed.FS {
	return e.embeddedThemes
}

// ThemeDir returns the filesystem theme directory path and whether the theme is embedded.
// Used by the handler to serve theme CSS/assets from the right source.
func (e *Engine) ThemeDir(themeName string) (string, bool) {
	themeDir := filepath.Join(e.contentDir, "themes", themeName)
	if _, err := os.Stat(themeDir); os.IsNotExist(err) {
		// Check embedded
		if _, err := fs.Stat(e.embeddedThemes, "themes/"+themeName); err == nil {
			return "themes/" + themeName, true // embedded
		}
		return "", false
	}
	return themeDir, false // filesystem, not embedded
}

// --- Template functions ---

func formatDate(t time.Time, layout string) string {
	return t.Format(layout)
}

func readingTime(p *content.Post) string {
	if p == nil {
		return ""
	}
	if p.ReadingTime <= 1 {
		return "1 min read"
	}
	return fmt.Sprintf("%d min read", p.ReadingTime)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Truncate at word boundary
	truncated := s[:maxLen]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > 0 {
		truncated = truncated[:lastSpace]
	}
	return truncated + "..."
}

func safeHTML(s string) template.HTML {
	return template.HTML(s) //nolint:gosec // Used only for pre-rendered trusted HTML
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

func seq(start, end int) []int {
	if start > end {
		return nil
	}
	s := make([]int, end-start+1)
	for i := range s {
		s[i] = start + i
	}
	return s
}

// jsonLD generates a JSON-LD script tag for structured data from SEO metadata.
func jsonLD(meta SEOMeta) template.HTML {
	ld := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "BlogPosting",
		"headline": meta.Title,
	}
	if meta.Description != "" {
		ld["description"] = meta.Description
	}
	if meta.CanonicalURL != "" {
		ld["url"] = meta.CanonicalURL
	}
	if meta.OGImage != "" {
		ld["image"] = meta.OGImage
	}

	data, err := json.Marshal(ld)
	if err != nil {
		return ""
	}
	return template.HTML(`<script type="application/ld+json">` + string(data) + `</script>`) //nolint:gosec
}

// --- Table of Contents ---

// TOCEntry represents a heading in the table of contents.
type TOCEntry struct {
	Level int
	ID    string
	Text  string
}

// ExtractTableOfContents parses headings from rendered HTML to build a ToC.
func ExtractTableOfContents(htmlContent string) []TOCEntry {
	var entries []TOCEntry
	// Match <h2 id="...">...</h2> and <h3 id="...">...</h3>
	i := 0
	for i < len(htmlContent) {
		// Find next heading tag
		idx := strings.Index(htmlContent[i:], "<h")
		if idx < 0 {
			break
		}
		pos := i + idx
		if pos+3 >= len(htmlContent) {
			break
		}

		level := htmlContent[pos+2]
		if level < '2' || level > '4' {
			i = pos + 3
			continue
		}

		// Extract id attribute
		tagEnd := strings.Index(htmlContent[pos:], ">")
		if tagEnd < 0 {
			break
		}
		tag := htmlContent[pos : pos+tagEnd]
		idStart := strings.Index(tag, `id="`)
		id := ""
		if idStart >= 0 {
			idStart += 4
			idEnd := strings.Index(tag[idStart:], `"`)
			if idEnd >= 0 {
				id = tag[idStart : idStart+idEnd]
			}
		}

		// Extract text content (strip inner HTML tags)
		contentStart := pos + tagEnd + 1
		closeTag := fmt.Sprintf("</h%c>", level)
		contentEnd := strings.Index(htmlContent[contentStart:], closeTag)
		if contentEnd < 0 {
			break
		}
		rawText := htmlContent[contentStart : contentStart+contentEnd]
		text := stripHTMLTags(rawText)

		if id != "" && text != "" {
			entries = append(entries, TOCEntry{
				Level: int(level - '0'),
				ID:    id,
				Text:  text,
			})
		}

		i = contentStart + contentEnd + len(closeTag)
	}
	return entries
}

// stripHTMLTags removes HTML tags from a string, returning plain text.
func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// --- Lazy Loading ---

// AddLazyLoading adds loading="lazy" to all <img> tags that don't already have it.
func AddLazyLoading(htmlContent string) string {
	// Replace <img that doesn't already have loading= with <img loading="lazy"
	result := htmlContent
	i := 0
	var out strings.Builder
	out.Grow(len(result) + 256)

	for i < len(result) {
		idx := strings.Index(result[i:], "<img ")
		if idx < 0 {
			out.WriteString(result[i:])
			break
		}
		out.WriteString(result[i : i+idx])
		// Find end of <img tag
		tagStart := i + idx
		tagEnd := strings.Index(result[tagStart:], ">")
		if tagEnd < 0 {
			out.WriteString(result[tagStart:])
			break
		}
		tag := result[tagStart : tagStart+tagEnd+1]
		if !strings.Contains(tag, "loading=") {
			// Insert loading="lazy" after <img
			tag = "<img loading=\"lazy\" " + tag[5:]
		}
		out.WriteString(tag)
		i = tagStart + tagEnd + 1
	}
	return out.String()
}
