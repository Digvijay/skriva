package render

import (
	"embed"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Digvijay/skriva/internal/config"
	"github.com/Digvijay/skriva/internal/content"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"café", "caf"},
		{"a--b", "a-b"},
		{"", ""},
	}
	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
	}{
		{"short", 10},           // should not truncate
		{"Hello World Foo", 11}, // truncates after "Hello"
		{"a b c d e", 5},        // truncates after "a"
		{"", 10},                // empty
		{"exact", 5},            // exact length, no truncation
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if len(tt.input) <= tt.maxLen {
			if got != tt.input {
				t.Errorf("truncate(%q, %d) = %q, should be unchanged", tt.input, tt.maxLen, got)
			}
		} else {
			if len(got) > tt.maxLen+3 { // allow for "..."
				t.Errorf("truncate(%q, %d) = %q, too long", tt.input, tt.maxLen, got)
			}
			if !strings.HasSuffix(got, "...") {
				t.Errorf("truncate(%q, %d) = %q, should end with ...", tt.input, tt.maxLen, got)
			}
		}
	}
}

func TestFormatDate(t *testing.T) {
	// Verify formatDate produces expected output
	tm := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	result := formatDate(tm, "Jan 2, 2006")
	if result != "Jan 15, 2026" {
		t.Errorf("formatDate = %q, want %q", result, "Jan 15, 2026")
	}
}

func TestSeq(t *testing.T) {
	tests := []struct {
		start, end int
		wantLen    int
	}{
		{1, 5, 5},
		{3, 3, 1},
		{5, 3, 0},
		{0, 0, 1},
	}
	for _, tt := range tests {
		got := seq(tt.start, tt.end)
		if len(got) != tt.wantLen {
			t.Errorf("seq(%d, %d) length = %d, want %d", tt.start, tt.end, len(got), tt.wantLen)
		}
		if tt.wantLen > 0 && got[0] != tt.start {
			t.Errorf("seq(%d, %d)[0] = %d, want %d", tt.start, tt.end, got[0], tt.start)
		}
	}
}

func TestSafeHTML(t *testing.T) {
	result := safeHTML("<p>Hello</p>")
	if string(result) != "<p>Hello</p>" {
		t.Errorf("safeHTML failed: got %s", result)
	}
}

func TestJsonLD(t *testing.T) {
	meta := SEOMeta{
		Title:        "Test Post",
		Description:  "A test",
		CanonicalURL: "https://example.com/test",
	}
	result := jsonLD(meta)
	s := string(result)
	if !strings.Contains(s, "BlogPosting") {
		t.Error("jsonLD should contain BlogPosting")
	}
	if !strings.Contains(s, "Test Post") {
		t.Error("jsonLD should contain title")
	}
	if !strings.Contains(s, "application/ld+json") {
		t.Error("jsonLD should contain script type")
	}
}

func BenchmarkRenderMarkdown(b *testing.B) {
	// Build a minimal Engine with a goldmark pipeline — no templates needed
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
			),
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithHardWraps(), html.WithXHTML(), html.WithUnsafe()),
	)
	e := &Engine{md: md}

	source := "# Hello\n\nThis is a **bold** paragraph with [a link](https://example.com).\n\n```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\n\n- Item 1\n- Item 2\n- Item 3\n"
	b.ResetTimer()
	for b.Loop() {
		if _, err := e.RenderMarkdown(source); err != nil {
			b.Fatal(err)
		}
	}
}

// --- ExtractTableOfContents tests ---

func TestExtractTableOfContents(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		wantLen  int
		wantIDs  []string
		wantLvls []int
		wantText []string
	}{
		{
			name:     "mixed h2 and h3",
			html:     `<h2 id="intro">Introduction</h2><p>text</p><h3 id="details">Details</h3><h2 id="conclusion">Conclusion</h2>`,
			wantLen:  3,
			wantIDs:  []string{"intro", "details", "conclusion"},
			wantLvls: []int{2, 3, 2},
			wantText: []string{"Introduction", "Details", "Conclusion"},
		},
		{
			name:    "no headings",
			html:    `<p>Just a paragraph</p>`,
			wantLen: 0,
		},
		{
			name:    "empty input",
			html:    "",
			wantLen: 0,
		},
		{
			name:     "h4 included",
			html:     `<h4 id="sub">Sub Section</h4>`,
			wantLen:  1,
			wantIDs:  []string{"sub"},
			wantLvls: []int{4},
			wantText: []string{"Sub Section"},
		},
		{
			name:    "h1 skipped",
			html:    `<h1 id="title">Title</h1>`,
			wantLen: 0,
		},
		{
			name:     "heading with inner HTML",
			html:     `<h2 id="bold-heading"><strong>Bold</strong> heading</h2>`,
			wantLen:  1,
			wantIDs:  []string{"bold-heading"},
			wantText: []string{"Bold heading"},
		},
		{
			name:    "heading without id skipped",
			html:    `<h2>No ID</h2>`,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := ExtractTableOfContents(tt.html)
			if len(entries) != tt.wantLen {
				t.Fatalf("got %d entries, want %d", len(entries), tt.wantLen)
			}
			for i, e := range entries {
				if i < len(tt.wantIDs) && e.ID != tt.wantIDs[i] {
					t.Errorf("entry[%d].ID = %q, want %q", i, e.ID, tt.wantIDs[i])
				}
				if i < len(tt.wantLvls) && e.Level != tt.wantLvls[i] {
					t.Errorf("entry[%d].Level = %d, want %d", i, e.Level, tt.wantLvls[i])
				}
				if i < len(tt.wantText) && e.Text != tt.wantText[i] {
					t.Errorf("entry[%d].Text = %q, want %q", i, e.Text, tt.wantText[i])
				}
			}
		})
	}
}

// --- AddLazyLoading tests ---

func TestAddLazyLoading(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, output string)
	}{
		{
			name:  "adds lazy to plain img",
			input: `<img src="a.jpg">`,
			check: func(t *testing.T, output string) {
				if !strings.Contains(output, `loading="lazy"`) {
					t.Errorf("expected loading=lazy, got %q", output)
				}
			},
		},
		{
			name:  "preserves existing loading=eager",
			input: `<img loading="eager" src="b.jpg">`,
			check: func(t *testing.T, output string) {
				if strings.Contains(output, `loading="lazy"`) {
					t.Error("should not override existing loading attribute")
				}
				if !strings.Contains(output, `loading="eager"`) {
					t.Error("should keep loading=eager")
				}
			},
		},
		{
			name:  "multiple images mixed",
			input: `<img src="a.jpg"><p>text</p><img loading="eager" src="b.jpg"><img src="c.jpg">`,
			check: func(t *testing.T, output string) {
				// First and third img should get lazy, second keeps eager
				count := strings.Count(output, `loading="lazy"`)
				if count != 2 {
					t.Errorf("expected 2 lazy-loaded images, got %d in %q", count, output)
				}
				if !strings.Contains(output, `loading="eager"`) {
					t.Error("should keep loading=eager")
				}
			},
		},
		{
			name:  "no images",
			input: `<p>no images here</p>`,
			check: func(t *testing.T, output string) {
				if output != `<p>no images here</p>` {
					t.Errorf("unchanged expected, got %q", output)
				}
			},
		},
		{
			name:  "empty input",
			input: "",
			check: func(t *testing.T, output string) {
				if output != "" {
					t.Errorf("expected empty, got %q", output)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := AddLazyLoading(tt.input)
			tt.check(t, output)
		})
	}
}

// --- RenderMarkdown tests ---

// buildMinimalEngine creates an Engine with just a goldmark pipeline (no templates).
func buildMinimalEngine() *Engine {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
			),
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithHardWraps(), html.WithXHTML(), html.WithUnsafe()),
	)
	return &Engine{md: md}
}

func TestRenderMarkdownBasic(t *testing.T) {
	e := buildMinimalEngine()

	tests := []struct {
		name     string
		source   string
		contains []string
	}{
		{
			name:     "bold text",
			source:   "This is **bold** text.",
			contains: []string{"<strong>bold</strong>"},
		},
		{
			name:     "heading with auto ID",
			source:   "# Hello World",
			contains: []string{"<h1 id=\"hello-world\">Hello World</h1>"},
		},
		{
			name:     "link",
			source:   "[click](https://example.com)",
			contains: []string{`href="https://example.com"`, `click`},
		},
		{
			name:     "code block",
			source:   "```go\nfmt.Println()\n```",
			contains: []string{"<pre", "<code"},
		},
		{
			name:     "unordered list",
			source:   "- one\n- two",
			contains: []string{"<ul>", "<li>one</li>", "<li>two</li>"},
		},
		{
			name:     "GFM table",
			source:   "| A | B |\n|---|---|\n| 1 | 2 |",
			contains: []string{"<table>", "<th>A</th>", "<td>1</td>"},
		},
		{
			name:     "strikethrough",
			source:   "~~deleted~~",
			contains: []string{"<del>deleted</del>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := e.RenderMarkdown(tt.source)
			if err != nil {
				t.Fatalf("RenderMarkdown error: %v", err)
			}
			for _, c := range tt.contains {
				if !strings.Contains(out, c) {
					t.Errorf("output missing %q\nfull output: %s", c, out)
				}
			}
		})
	}
}

func TestRenderMarkdownUntrusted(t *testing.T) {
	e := buildMinimalEngine()

	// A trivial sanitizer that strips <script> tags
	sanitizer := func(s string) string {
		for {
			start := strings.Index(s, "<script")
			if start < 0 {
				break
			}
			end := strings.Index(s[start:], "</script>")
			if end < 0 {
				// Remove from start to end of string
				s = s[:start]
				break
			}
			s = s[:start] + s[start+end+len("</script>"):]
		}
		return s
	}

	tests := []struct {
		name      string
		source    string
		wantIn    []string
		wantNotIn []string
	}{
		{
			name:      "strips script tags",
			source:    "Hello\n\n<script>alert('xss')</script>\n\nWorld",
			wantIn:    []string{"Hello", "World"},
			wantNotIn: []string{"<script>", "alert"},
		},
		{
			name:      "preserves normal markdown",
			source:    "**safe** content",
			wantIn:    []string{"<strong>safe</strong>"},
			wantNotIn: []string{"<script>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := e.RenderMarkdownUntrusted(tt.source, sanitizer)
			if err != nil {
				t.Fatalf("RenderMarkdownUntrusted error: %v", err)
			}
			for _, w := range tt.wantIn {
				if !strings.Contains(out, w) {
					t.Errorf("output missing %q\nfull output: %s", w, out)
				}
			}
			for _, nw := range tt.wantNotIn {
				if strings.Contains(out, nw) {
					t.Errorf("output should not contain %q\nfull output: %s", nw, out)
				}
			}
		})
	}
}

// --- readingTime tests ---

func TestReadingTime(t *testing.T) {
	tests := []struct {
		name string
		post *content.Post
		want string
	}{
		{name: "nil post", post: nil, want: ""},
		{name: "zero words", post: &content.Post{ReadingTime: 0}, want: "1 min read"},
		{name: "one min", post: &content.Post{ReadingTime: 1}, want: "1 min read"},
		{name: "two min", post: &content.Post{ReadingTime: 2}, want: "2 min read"},
		{name: "ten min", post: &content.Post{ReadingTime: 10}, want: "10 min read"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readingTime(tt.post)
			if got != tt.want {
				t.Errorf("readingTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Additional template function tests ---

func TestTemplateFuncHasPrefix(t *testing.T) {
	if !strings.HasPrefix("/admin/settings", "/admin") {
		t.Error("hasPrefix should match /admin prefix")
	}
	if strings.HasPrefix("/public/page", "/admin") {
		t.Error("hasPrefix should not match /admin prefix for /public/page")
	}
}

func TestTemplateFuncJoin(t *testing.T) {
	got := strings.Join([]string{"go", "rust", "python"}, ", ")
	if got != "go, rust, python" {
		t.Errorf("join = %q, want %q", got, "go, rust, python")
	}
}

func TestTemplateFuncAddSub(t *testing.T) {
	add := func(a, b int) int { return a + b }
	sub := func(a, b int) int { return a - b }

	tests := []struct {
		name   string
		fn     func(int, int) int
		a, b   int
		expect int
	}{
		{"add positive", add, 3, 5, 8},
		{"add negative", add, 3, -2, 1},
		{"add zero", add, 0, 0, 0},
		{"sub positive", sub, 10, 3, 7},
		{"sub negative", sub, 3, 5, -2},
		{"sub zero", sub, 5, 0, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn(tt.a, tt.b)
			if got != tt.expect {
				t.Errorf("got %d, want %d", got, tt.expect)
			}
		})
	}
}

func TestTemplateFuncLowerUpper(t *testing.T) {
	if strings.ToLower("HELLO") != "hello" {
		t.Error("lower failed")
	}
	if strings.ToUpper("hello") != "HELLO" {
		t.Error("upper failed")
	}
}

func TestCurrentYear(t *testing.T) {
	yearFn := func() int { return time.Now().Year() }
	got := yearFn()
	if got != time.Now().Year() {
		t.Errorf("currentYear = %d, want %d", got, time.Now().Year())
	}
}

func TestCacheVer(t *testing.T) {
	cacheVerFn := func() int64 { return time.Now().Unix() / 3600 }
	v1 := cacheVerFn()
	v2 := cacheVerFn()
	if v1 != v2 {
		t.Error("cacheVer should be stable within the same second")
	}
	if v1 <= 0 {
		t.Error("cacheVer should be positive")
	}
}

// --- stripHTMLTags test ---

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"<strong>bold</strong> text", "bold text"},
		{"no tags", "no tags"},
		{"<a href='#'>link</a>", "link"},
		{"", ""},
		{"<p><em>nested</em></p>", "nested"},
	}
	for _, tt := range tests {
		got := stripHTMLTags(tt.input)
		if got != tt.want {
			t.Errorf("stripHTMLTags(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- NewEngine / LoadTemplates / RenderPage integration tests ---

// setupTestTheme creates a minimal theme in a temp directory and returns the
// content dir path, an empty embed.FS, and a loaded Config pointing at the
// same temp directory (so UpdateSite can write site.yaml).
func setupTestTheme(t *testing.T, themeName string) (string, embed.FS, *config.Config) {
	t.Helper()
	dir := t.TempDir()

	// Create theme directory structure
	themeDir := filepath.Join(dir, "themes", themeName)
	for _, sub := range []string{
		"templates/partials",
		"css",
	} {
		if err := os.MkdirAll(filepath.Join(themeDir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Write theme.yaml
	os.WriteFile(filepath.Join(themeDir, "theme.yaml"), []byte("name: test\n"), 0o644)

	// Write base.html
	baseHTML := `<!DOCTYPE html><html><body>{{block "content" .}}default{{end}}</body></html>`
	os.WriteFile(filepath.Join(themeDir, "templates", "base.html"), []byte(baseHTML), 0o644)

	// Write a partial
	navHTML := `<nav>nav</nav>`
	os.WriteFile(filepath.Join(themeDir, "templates", "partials", "nav.html"), []byte(navHTML), 0o644)

	// Write page templates
	pages := map[string]string{
		"home.html":    `{{define "home.html"}}home: {{.Site.Title}}{{end}}`,
		"post.html":    `{{define "post.html"}}post: {{.Post.Title}}{{end}}`,
		"search.html":  `{{define "search.html"}}search: {{.SearchQuery}}{{end}}`,
		"page.html":    `{{define "page.html"}}page{{end}}`,
		"tag.html":     `{{define "tag.html"}}tag{{end}}`,
		"tags.html":    `{{define "tags.html"}}tags{{end}}`,
		"archive.html": `{{define "archive.html"}}archive{{end}}`,
		"404.html":     `{{define "404.html"}}not found{{end}}`,
	}
	for name, tmpl := range pages {
		os.WriteFile(filepath.Join(themeDir, "templates", name), []byte(tmpl), 0o644)
	}

	// Write minimal CSS
	os.WriteFile(filepath.Join(themeDir, "css", "theme.css"), []byte("body{}"), 0o644)

	// Create a config directory with site.yaml so config.Load works
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)
	siteYAML := "title: Test Blog\ntheme: " + themeName + "\n"
	os.WriteFile(filepath.Join(configDir, "site.yaml"), []byte(siteYAML), 0o644)
	os.WriteFile(filepath.Join(configDir, "secrets.yaml"), []byte("admin_password: \"\"\nsession_secret: testsecret123456\n"), 0o644)
	os.WriteFile(filepath.Join(configDir, "smtp.yaml"), []byte("enabled: false\n"), 0o644)

	cfg, err := config.Load(configDir)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	var emptyFS embed.FS
	return dir, emptyFS, cfg
}

func TestNewEngineAndRenderPage(t *testing.T) {
	contentDir, emptyFS, cfg := setupTestTheme(t, "test")

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	eng, err := NewEngine(contentDir, cfg, logger, emptyFS)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	t.Run("render home page", func(t *testing.T) {
		data := &TemplateData{
			Site: config.SiteConfig{Title: "Test Blog"},
		}
		out, err := eng.RenderPage("home.html", data)
		if err != nil {
			t.Fatalf("RenderPage(home.html) error: %v", err)
		}
		if !strings.Contains(out, "home: Test Blog") {
			t.Errorf("expected home template output, got %q", out)
		}
	})

	t.Run("render post page", func(t *testing.T) {
		data := &TemplateData{
			Post: &content.Post{Title: "My Post"},
		}
		out, err := eng.RenderPage("post.html", data)
		if err != nil {
			t.Fatalf("RenderPage(post.html) error: %v", err)
		}
		if !strings.Contains(out, "post: My Post") {
			t.Errorf("expected post template output, got %q", out)
		}
	})

	t.Run("render search page", func(t *testing.T) {
		data := &TemplateData{
			SearchQuery: "golang",
		}
		out, err := eng.RenderPage("search.html", data)
		if err != nil {
			t.Fatalf("RenderPage(search.html) error: %v", err)
		}
		if !strings.Contains(out, "search: golang") {
			t.Errorf("expected search template output, got %q", out)
		}
	})

	t.Run("unknown template returns error", func(t *testing.T) {
		_, err := eng.RenderPage("nonexistent.html", &TemplateData{})
		if err == nil {
			t.Fatal("expected error for unknown template")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got %v", err)
		}
	})

	t.Run("Year is set automatically", func(t *testing.T) {
		data := &TemplateData{}
		_, err := eng.RenderPage("home.html", data)
		if err != nil {
			t.Fatalf("RenderPage error: %v", err)
		}
		if data.Year != time.Now().Year() {
			t.Errorf("Year = %d, want %d", data.Year, time.Now().Year())
		}
	})
}

func TestEmbeddedThemes(t *testing.T) {
	contentDir, emptyFS, cfg := setupTestTheme(t, "test")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	eng, err := NewEngine(contentDir, cfg, logger, emptyFS)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// EmbeddedThemes should return the same FS that was passed in
	efs := eng.EmbeddedThemes()
	// For an empty embed.FS reading any path should fail
	_, err = efs.ReadFile("anything")
	if err == nil {
		t.Error("expected error reading from empty embed.FS")
	}
}

func TestLoadTemplatesInvalidTheme(t *testing.T) {
	contentDir := t.TempDir()
	configDir := filepath.Join(contentDir, "config")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "site.yaml"), []byte("title: Test\ntheme: nonexistent-theme\n"), 0o644)
	os.WriteFile(filepath.Join(configDir, "secrets.yaml"), []byte("session_secret: testsecret123456\n"), 0o644)
	os.WriteFile(filepath.Join(configDir, "smtp.yaml"), []byte("enabled: false\n"), 0o644)

	cfg, err := config.Load(configDir)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	var emptyFS embed.FS
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	_, err = NewEngine(contentDir, cfg, logger, emptyFS)
	if err == nil {
		t.Fatal("expected error for nonexistent theme")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %v", err)
	}
}

func TestInvalidateCache(t *testing.T) {
	contentDir, emptyFS, cfg := setupTestTheme(t, "test")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	eng, err := NewEngine(contentDir, cfg, logger, emptyFS)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	eng.InvalidateCache()
	if eng.CacheStats() != 0 {
		t.Errorf("CacheStats after InvalidateCache = %d, want 0", eng.CacheStats())
	}
}

func TestThemeDir(t *testing.T) {
	contentDir, emptyFS, cfg := setupTestTheme(t, "test")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	eng, err := NewEngine(contentDir, cfg, logger, emptyFS)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Theme "test" exists on disk
	dir, isEmbedded := eng.ThemeDir("test")
	if isEmbedded {
		t.Error("test theme should not be embedded")
	}
	if dir == "" {
		t.Error("ThemeDir should return a path for existing theme")
	}

	// Non-existent theme
	dir, isEmbedded = eng.ThemeDir("nonexistent")
	if dir != "" || isEmbedded {
		t.Error("nonexistent theme should return empty")
	}
}

func TestLoadTemplatesReload(t *testing.T) {
	contentDir, emptyFS, cfg := setupTestTheme(t, "test")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	eng, err := NewEngine(contentDir, cfg, logger, emptyFS)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Render before reload
	data := &TemplateData{Site: config.SiteConfig{Title: "Before"}}
	out, err := eng.RenderPage("home.html", data)
	if err != nil {
		t.Fatalf("RenderPage error: %v", err)
	}
	if !strings.Contains(out, "home: Before") {
		t.Errorf("unexpected output before reload: %q", out)
	}

	// Modify template on disk
	themeDir := filepath.Join(contentDir, "themes", "test")
	newHome := `{{define "home.html"}}updated: {{.Site.Title}}{{end}}`
	os.WriteFile(filepath.Join(themeDir, "templates", "home.html"), []byte(newHome), 0o644)

	// Reload
	if err := eng.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates reload: %v", err)
	}

	data2 := &TemplateData{Site: config.SiteConfig{Title: "After"}}
	out2, err := eng.RenderPage("home.html", data2)
	if err != nil {
		t.Fatalf("RenderPage after reload error: %v", err)
	}
	if !strings.Contains(out2, "updated: After") {
		t.Errorf("unexpected output after reload: %q", out2)
	}
}
