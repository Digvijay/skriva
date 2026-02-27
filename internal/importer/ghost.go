// Package importer provides tools to import content from other blog platforms.
// Currently supports Ghost JSON export format.
package importer

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ghostExport represents the top-level Ghost JSON export structure.
type ghostExport struct {
	DB []ghostDB `json:"db"`
}

type ghostDB struct {
	Data ghostData `json:"data"`
}

type ghostData struct {
	Posts   []ghostPost    `json:"posts"`
	Tags    []ghostTag     `json:"tags"`
	PostTag []ghostPostTag `json:"posts_tags"`
}

type ghostPost struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Slug            string  `json:"slug"`
	HTML            string  `json:"html"`
	Mobiledoc       string  `json:"mobiledoc"`
	FeatureImage    string  `json:"feature_image"`
	Featured        int     `json:"featured"`
	Status          string  `json:"status"` // "published" or "draft"
	MetaDescription string  `json:"meta_description"`
	CreatedAt       string  `json:"created_at"`
	PublishedAt     *string `json:"published_at"`
}

type ghostTag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type ghostPostTag struct {
	PostID string `json:"post_id"`
	TagID  string `json:"tag_id"`
}

// GhostImporter handles importing posts from a Ghost JSON export.
type GhostImporter struct {
	exportPath string
	imagesDir  string
	outputDir  string
	logger     *slog.Logger
}

// NewGhostImporter creates a new importer from the given paths.
func NewGhostImporter(exportPath, imagesDir, outputDir string, logger *slog.Logger) *GhostImporter {
	return &GhostImporter{
		exportPath: exportPath,
		imagesDir:  imagesDir,
		outputDir:  outputDir,
		logger:     logger,
	}
}

// ImportResult contains the summary of an import operation.
type ImportResult struct {
	TotalFound   int
	Imported     int
	Skipped      int
	ImagesCopied int
	Errors       []string
}

// Run executes the Ghost import process.
func (g *GhostImporter) Run() (*ImportResult, error) {
	result := &ImportResult{}

	// Read and parse export file
	data, err := os.ReadFile(g.exportPath)
	if err != nil {
		return nil, fmt.Errorf("reading export file: %w", err)
	}

	var export ghostExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("parsing export JSON: %w", err)
	}

	if len(export.DB) == 0 {
		return nil, fmt.Errorf("export file contains no database entries")
	}

	db := export.DB[0].Data
	result.TotalFound = len(db.Posts)

	// Build tag lookup: tagID → tag name
	tagMap := make(map[string]string, len(db.Tags))
	for _, t := range db.Tags {
		tagMap[t.ID] = t.Name
	}

	// Build post→tags lookup
	postTags := make(map[string][]string)
	for _, pt := range db.PostTag {
		if name, ok := tagMap[pt.TagID]; ok {
			postTags[pt.PostID] = append(postTags[pt.PostID], name)
		}
	}

	// Ensure output directory exists
	postsDir := filepath.Join(g.outputDir, "posts")
	if err := os.MkdirAll(postsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating posts directory: %w", err)
	}

	for i := range db.Posts {
		post := &db.Posts[i]
		if post.Slug == "" {
			result.Skipped++
			continue
		}

		g.logger.Info("importing post", "slug", post.Slug, "title", post.Title)

		if err := g.importPost(*post, postTags[post.ID], postsDir, result); err != nil {
			errMsg := fmt.Sprintf("post %q: %v", post.Slug, err)
			result.Errors = append(result.Errors, errMsg)
			g.logger.Error("import failed", "slug", post.Slug, "error", err)
		} else {
			result.Imported++
		}
	}

	return result, nil
}

func (g *GhostImporter) importPost(post ghostPost, tags []string, postsDir string, result *ImportResult) error {
	// Create post directory
	postDir := filepath.Join(postsDir, post.Slug)
	if err := os.MkdirAll(postDir, 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Parse date
	date := time.Now()
	if post.PublishedAt != nil && *post.PublishedAt != "" {
		if t, err := time.Parse(time.RFC3339, *post.PublishedAt); err == nil {
			date = t
		} else if t, err := time.Parse("2006-01-02T15:04:05.000Z", *post.PublishedAt); err == nil {
			date = t
		}
	} else if post.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, post.CreatedAt); err == nil {
			date = t
		} else if t, err := time.Parse("2006-01-02T15:04:05.000Z", post.CreatedAt); err == nil {
			date = t
		}
	}

	// Convert HTML to markdown
	markdown := htmlToMarkdown(post.HTML)

	// Process feature image
	featureImage := ""
	if post.FeatureImage != "" {
		imgFilename, err := g.processImage(post.FeatureImage, postDir)
		if err != nil {
			g.logger.Warn("failed to process feature image", "url", post.FeatureImage, "error", err)
		} else {
			featureImage = imgFilename
			result.ImagesCopied++
		}
	}

	// Process inline images in content
	markdown, imageCount := g.processInlineImages(markdown, postDir)
	result.ImagesCopied += imageCount

	// Build frontmatter
	isDraft := post.Status != "published"
	isFeatured := post.Featured == 1

	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "title: %q\n", post.Title)
	fmt.Fprintf(&sb, "slug: %q\n", post.Slug)
	fmt.Fprintf(&sb, "date: %s\n", date.Format("2006-01-02"))

	if len(tags) > 0 {
		tagStrs := make([]string, len(tags))
		for i, t := range tags {
			tagStrs[i] = fmt.Sprintf("%q", t)
		}
		fmt.Fprintf(&sb, "tags: [%s]\n", strings.Join(tagStrs, ", "))
	}

	if post.MetaDescription != "" {
		fmt.Fprintf(&sb, "description: %q\n", post.MetaDescription)
	}
	if featureImage != "" {
		fmt.Fprintf(&sb, "image: %q\n", featureImage)
	}
	if isFeatured {
		sb.WriteString("featured: true\n")
	}
	if isDraft {
		sb.WriteString("draft: true\n")
	}
	sb.WriteString("---\n\n")
	sb.WriteString(markdown)

	// Write index.md
	indexPath := filepath.Join(postDir, "index.md")
	if err := os.WriteFile(indexPath, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("writing markdown file: %w", err)
	}

	return nil
}

// processImage downloads or copies an image to the post directory.
func (g *GhostImporter) processImage(imageURL, postDir string) (string, error) {
	mediaDir := filepath.Join(postDir, "media")
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return "", fmt.Errorf("creating media directory: %w", err)
	}

	filename := filepath.Base(imageURL)
	// Clean up query params
	if idx := strings.Index(filename, "?"); idx > 0 {
		filename = filename[:idx]
	}
	// Sanitize filename
	filename = sanitizeFilename(filename)

	destPath := filepath.Join(mediaDir, filename)

	// Try local images directory first
	if g.imagesDir != "" {
		// Ghost stores images relative to content/images/
		relPath := imageURL
		for _, prefix := range []string{"/content/images/", "__GHOST_URL__/content/images/", "content/images/"} {
			relPath = strings.TrimPrefix(relPath, prefix)
		}

		localPath := filepath.Join(g.imagesDir, relPath)
		if _, err := os.Stat(localPath); err == nil {
			return filename, copyFile(localPath, destPath)
		}
	}

	// Try downloading if it's a URL
	if strings.HasPrefix(imageURL, "http://") || strings.HasPrefix(imageURL, "https://") {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(imageURL)
		if err != nil {
			return "", fmt.Errorf("downloading image: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("download returned status %d", resp.StatusCode)
		}

		out, err := os.Create(destPath)
		if err != nil {
			return "", fmt.Errorf("creating file: %w", err)
		}
		defer out.Close()

		if _, err := io.Copy(out, resp.Body); err != nil {
			return "", fmt.Errorf("saving file: %w", err)
		}

		return filename, nil
	}

	return filename, fmt.Errorf("image not found locally: %s", imageURL)
}

// processInlineImages finds image references in markdown and downloads/copies them.
func (g *GhostImporter) processInlineImages(markdown, postDir string) (string, int) {
	imgRegex := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	count := 0

	result := imgRegex.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := imgRegex.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		alt := parts[1]
		src := parts[2]

		filename, err := g.processImage(src, postDir)
		if err != nil {
			g.logger.Warn("inline image not processed", "src", src, "error", err)
			return match
		}
		count++
		return fmt.Sprintf("![%s](media/%s)", alt, filename)
	})

	return result, count
}

// htmlToMarkdown converts HTML content to markdown.
// This is a pragmatic converter focused on Ghost's output patterns.
func htmlToMarkdown(html string) string {
	if html == "" {
		return ""
	}

	s := html

	// Remove common Ghost cruft
	s = regexp.MustCompile(`<!--.*?-->`).ReplaceAllString(s, "")

	// Headings
	for i := 6; i >= 1; i-- {
		tag := fmt.Sprintf("h%d", i)
		prefix := strings.Repeat("#", i)
		re := regexp.MustCompile(`(?i)<` + tag + `[^>]*>(.*?)</` + tag + `>`)
		s = re.ReplaceAllString(s, "\n"+prefix+" $1\n")
	}

	// Bold, italic
	s = regexp.MustCompile(`(?i)<strong[^>]*>(.*?)</strong>`).ReplaceAllString(s, "**$1**")
	s = regexp.MustCompile(`(?i)<b[^>]*>(.*?)</b>`).ReplaceAllString(s, "**$1**")
	s = regexp.MustCompile(`(?i)<em[^>]*>(.*?)</em>`).ReplaceAllString(s, "*$1*")
	s = regexp.MustCompile(`(?i)<i[^>]*>(.*?)</i>`).ReplaceAllString(s, "*$1*")

	// Code
	s = regexp.MustCompile(`(?i)<code[^>]*>(.*?)</code>`).ReplaceAllString(s, "`$1`")

	// Code blocks (pre > code)
	preRe := regexp.MustCompile(`(?is)<pre[^>]*><code[^>]*(?:class="language-(\w+)")?[^>]*>(.*?)</code></pre>`)
	s = preRe.ReplaceAllStringFunc(s, func(match string) string {
		parts := preRe.FindStringSubmatch(match)
		lang := ""
		code := match
		if len(parts) > 1 {
			lang = parts[1]
		}
		if len(parts) > 2 {
			code = parts[2]
		}
		code = unescapeHTML(code)
		return "\n```" + lang + "\n" + strings.TrimSpace(code) + "\n```\n"
	})

	// Links
	s = regexp.MustCompile(`(?i)<a[^>]*href="([^"]*)"[^>]*>(.*?)</a>`).ReplaceAllString(s, "[$2]($1)")

	// Images
	s = regexp.MustCompile(`(?i)<img[^>]*src="([^"]*)"[^>]*alt="([^"]*)"[^>]*/?\s*>`).ReplaceAllString(s, "![$2]($1)")
	s = regexp.MustCompile(`(?i)<img[^>]*src="([^"]*)"[^>]*/?\s*>`).ReplaceAllString(s, "![]($1)")

	// Lists
	s = regexp.MustCompile(`(?i)<li[^>]*>\s*(.*?)\s*</li>`).ReplaceAllString(s, "- $1")
	s = regexp.MustCompile(`(?i)</?[ou]l[^>]*>`).ReplaceAllString(s, "\n")

	// Blockquotes
	bqRe := regexp.MustCompile(`(?is)<blockquote[^>]*>(.*?)</blockquote>`)
	s = bqRe.ReplaceAllStringFunc(s, func(match string) string {
		inner := bqRe.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		content := strings.TrimSpace(inner[1])
		content = regexp.MustCompile(`(?i)</?p[^>]*>`).ReplaceAllString(content, "")
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			lines[i] = "> " + strings.TrimSpace(line)
		}
		return "\n" + strings.Join(lines, "\n") + "\n"
	})

	// Horizontal rules
	s = regexp.MustCompile(`(?i)<hr\s*/?\s*>`).ReplaceAllString(s, "\n---\n")

	// Line breaks
	s = regexp.MustCompile(`(?i)<br\s*/?\s*>`).ReplaceAllString(s, "\n")

	// Paragraphs
	s = regexp.MustCompile(`(?i)<p[^>]*>`).ReplaceAllString(s, "\n")
	s = regexp.MustCompile(`(?i)</p>`).ReplaceAllString(s, "\n")

	// Figures (Ghost uses these for images)
	s = regexp.MustCompile(`(?i)</?figure[^>]*>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?i)<figcaption[^>]*>(.*?)</figcaption>`).ReplaceAllString(s, "*$1*")

	// Divs, spans (strip)
	s = regexp.MustCompile(`(?i)</?(?:div|span|section|article|aside|header|footer|main|nav)[^>]*>`).ReplaceAllString(s, "")

	// Iframes (YouTube, Vimeo embeds)
	s = regexp.MustCompile(`(?i)<iframe[^>]*src="([^"]*)"[^>]*>.*?</iframe>`).ReplaceAllString(s, "\n[$1]($1)\n")

	// Strip remaining HTML tags
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")

	// Unescape HTML entities
	s = unescapeHTML(s)

	// Clean up excessive whitespace
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	s = strings.TrimSpace(s)

	return s
}

func unescapeHTML(s string) string {
	replacements := map[string]string{
		"&amp;":  "&",
		"&lt;":   "<",
		"&gt;":   ">",
		"&quot;": "\"",
		"&#39;":  "'",
		"&nbsp;": " ",
		"&#x27;": "'",
		"&#x2F;": "/",
	}
	for entity, char := range replacements {
		s = strings.ReplaceAll(s, entity, char)
	}
	return s
}

func sanitizeFilename(name string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying data: %w", err)
	}

	return nil
}
