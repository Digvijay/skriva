// Package content handles loading, parsing, and watching blog posts and pages
// from the filesystem. Posts are markdown files with YAML frontmatter.
package content

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Post represents a blog post parsed from a markdown file with YAML frontmatter.
type Post struct {
	Title       string    `yaml:"title"`
	Slug        string    `yaml:"slug"`
	Date        time.Time `yaml:"date"`
	Tags        []string  `yaml:"tags"`
	Description string    `yaml:"description"`
	Image       string    `yaml:"image"`
	ImageCredit *Credit   `yaml:"image_credit"`
	Featured    bool      `yaml:"featured"`
	Draft       bool      `yaml:"draft"`
	TOC         bool      `yaml:"toc"`
	Series      string    `yaml:"series"`
	SeriesOrder int       `yaml:"series_order"`

	// Computed fields (not from frontmatter)
	Content     string `yaml:"-"`
	HTML        string `yaml:"-"`
	ReadingTime int    `yaml:"-"`
	WordCount   int    `yaml:"-"`
	Dir         string `yaml:"-"`
	Permalink   string `yaml:"-"`
}

// Credit represents attribution for a featured image.
type Credit struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// Page represents a static page (e.g., about, uses).
type Page struct {
	Title       string `yaml:"title"`
	Slug        string `yaml:"slug"`
	Description string `yaml:"description"`

	Content   string `yaml:"-"`
	HTML      string `yaml:"-"`
	Dir       string `yaml:"-"`
	Permalink string `yaml:"-"`
}

// TagInfo holds a tag name and its post count.
type TagInfo struct {
	Name  string
	Slug  string
	Count int
}

// Loader manages loading and caching blog content from the filesystem.
type Loader struct {
	contentDir string
	logger     *slog.Logger

	mu    sync.RWMutex
	posts []Post
	pages []Page
	tags  []TagInfo

	// Index for fast lookups
	postBySlug map[string]*Post
	pageBySlug map[string]*Page
	postsByTag map[string][]Post
}

// NewLoader creates a new content loader and performs the initial scan.
func NewLoader(contentDir string, logger *slog.Logger) (*Loader, error) {
	l := &Loader{
		contentDir: contentDir,
		logger:     logger,
		postBySlug: make(map[string]*Post),
		pageBySlug: make(map[string]*Page),
		postsByTag: make(map[string][]Post),
	}

	if err := l.Reload(); err != nil {
		return nil, fmt.Errorf("initial content load: %w", err)
	}

	logger.Info("content loaded",
		"posts", len(l.posts),
		"pages", len(l.pages),
		"tags", len(l.tags),
	)

	return l, nil
}

// Reload rescans the filesystem and rebuilds all indexes.
func (l *Loader) Reload() error {
	posts, err := l.loadPosts()
	if err != nil {
		return fmt.Errorf("loading posts: %w", err)
	}

	pages, err := l.loadPages()
	if err != nil {
		return fmt.Errorf("loading pages: %w", err)
	}

	// Sort posts by date descending (newest first)
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})

	// Build indexes
	postBySlug := make(map[string]*Post, len(posts))
	pageBySlug := make(map[string]*Page, len(pages))
	postsByTag := make(map[string][]Post)
	tagCount := make(map[string]int)

	for i := range posts {
		postBySlug[posts[i].Slug] = &posts[i]
		for _, tag := range posts[i].Tags {
			slug := slugify(tag)
			postsByTag[slug] = append(postsByTag[slug], posts[i])
			tagCount[slug]++
		}
	}

	for i := range pages {
		pageBySlug[pages[i].Slug] = &pages[i]
	}

	// Build sorted tag list
	var tags []TagInfo
	for slug, count := range tagCount {
		// Find original tag name from the first post that has it
		name := slug
		for i := range posts {
			for _, t := range posts[i].Tags {
				if slugify(t) == slug {
					name = t
					break
				}
			}
			if name != slug {
				break
			}
		}
		tags = append(tags, TagInfo{Name: name, Slug: slug, Count: count})
	}
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	// Swap atomically
	l.mu.Lock()
	l.posts = posts
	l.pages = pages
	l.tags = tags
	l.postBySlug = postBySlug
	l.pageBySlug = pageBySlug
	l.postsByTag = postsByTag
	l.mu.Unlock()

	return nil
}

// Posts returns all published posts sorted by date descending.
// Excludes drafts and posts with future dates (scheduled posts).
func (l *Loader) Posts() []Post {
	l.mu.RLock()
	defer l.mu.RUnlock()

	now := time.Now()
	var published []Post
	for i := range l.posts {
		if !l.posts[i].Draft && !l.posts[i].Date.After(now) {
			published = append(published, l.posts[i])
		}
	}
	return published
}

// AllPosts returns all posts including drafts (for admin).
func (l *Loader) AllPosts() []Post {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]Post, len(l.posts))
	copy(result, l.posts)
	return result
}

// PostBySlug returns a single post by its slug (excludes drafts and scheduled).
// Returns a copy of the post to prevent data races from concurrent mutations.
func (l *Loader) PostBySlug(slug string) (*Post, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	p, ok := l.postBySlug[slug]
	if ok && (p.Draft || p.Date.After(time.Now())) {
		return nil, false
	}
	if !ok {
		return nil, false
	}
	clone := *p
	return &clone, true
}

// PostBySlugAdmin returns a post by slug including drafts (for admin).
// Returns a copy of the post to prevent data races from concurrent mutations.
func (l *Loader) PostBySlugAdmin(slug string) (*Post, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	p, ok := l.postBySlug[slug]
	if !ok {
		return nil, false
	}
	clone := *p
	return &clone, true
}

// PageBySlug returns a static page by its slug.
// Returns a copy of the page to prevent data races from concurrent mutations.
func (l *Loader) PageBySlug(slug string) (*Page, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	p, ok := l.pageBySlug[slug]
	if !ok {
		return nil, false
	}
	clone := *p
	return &clone, true
}

// PostsByTag returns all published posts with the given tag slug.
func (l *Loader) PostsByTag(tagSlug string) []Post {
	l.mu.RLock()
	defer l.mu.RUnlock()

	now := time.Now()
	posts := l.postsByTag[tagSlug]
	var published []Post
	for i := range posts {
		if !posts[i].Draft && !posts[i].Date.After(now) {
			published = append(published, posts[i])
		}
	}
	return published
}

// Tags returns all tags with their post counts.
func (l *Loader) Tags() []TagInfo {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]TagInfo, len(l.tags))
	copy(result, l.tags)
	return result
}

// FeaturedPosts returns all featured published posts.
func (l *Loader) FeaturedPosts() []Post {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var featured []Post
	for i := range l.posts {
		if l.posts[i].Featured && !l.posts[i].Draft {
			featured = append(featured, l.posts[i])
		}
	}
	return featured
}

// ContentDir returns the content directory path.
func (l *Loader) ContentDir() string {
	return l.contentDir
}

// SeriesPosts returns all published posts in the same series, sorted by series_order.
func (l *Loader) SeriesPosts(seriesName string) []Post {
	if seriesName == "" {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()

	now := time.Now()
	var posts []Post
	for i := range l.posts {
		if l.posts[i].Series == seriesName && !l.posts[i].Draft && !l.posts[i].Date.After(now) {
			posts = append(posts, l.posts[i])
		}
	}
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].SeriesOrder < posts[j].SeriesOrder
	})
	return posts
}

// RelatedPosts returns up to n published posts that share tags with the given post.
func (l *Loader) RelatedPosts(post *Post, n int) []Post {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if post == nil || len(post.Tags) == 0 {
		return nil
	}

	// Build set of this post's tag slugs
	tagSet := make(map[string]bool, len(post.Tags))
	for _, t := range post.Tags {
		tagSet[slugify(t)] = true
	}

	// Score each post by shared tags
	type scored struct {
		post  Post
		score int
	}
	var candidates []scored
	for i := range l.posts {
		if l.posts[i].Slug == post.Slug || l.posts[i].Draft {
			continue
		}
		score := 0
		for _, t := range l.posts[i].Tags {
			if tagSet[slugify(t)] {
				score++
			}
		}
		if score > 0 {
			candidates = append(candidates, scored{l.posts[i], score})
		}
	}

	// Sort by score descending, then by date descending
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].post.Date.After(candidates[j].post.Date)
	})

	result := make([]Post, 0, n)
	for i := 0; i < len(candidates) && i < n; i++ {
		result = append(result, candidates[i].post)
	}
	return result
}

// SearchPosts performs a simple full-text search across post titles, descriptions, and content.
func (l *Loader) SearchPosts(query string) []Post {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if query == "" {
		return nil
	}

	q := strings.ToLower(strings.TrimSpace(query))
	words := strings.Fields(q)

	var results []Post
	for i := range l.posts {
		if l.posts[i].Draft {
			continue
		}
		// Search in title, description, content, and tags
		haystack := strings.ToLower(l.posts[i].Title + " " + l.posts[i].Description + " " + l.posts[i].Content + " " + strings.Join(l.posts[i].Tags, " "))
		matched := true
		for _, word := range words {
			if !strings.Contains(haystack, word) {
				matched = false
				break
			}
		}
		if matched {
			results = append(results, l.posts[i])
		}
	}
	return results
}

func (l *Loader) loadPosts() ([]Post, error) {
	postsDir := filepath.Join(l.contentDir, "posts")
	if _, err := os.Stat(postsDir); os.IsNotExist(err) {
		l.logger.Warn("posts directory does not exist, creating", "dir", postsDir)
		if err := os.MkdirAll(postsDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating posts directory: %w", err)
		}
		return nil, nil
	}

	var posts []Post

	err := filepath.WalkDir(postsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() != "index.md" {
			return nil
		}

		post, err := l.parsePost(path)
		if err != nil {
			l.logger.Warn("skipping invalid post", "path", path, "error", err)
			return nil
		}
		posts = append(posts, *post)
		return nil
	})

	return posts, err
}

func (l *Loader) loadPages() ([]Page, error) {
	pagesDir := filepath.Join(l.contentDir, "pages")
	if _, err := os.Stat(pagesDir); os.IsNotExist(err) {
		l.logger.Warn("pages directory does not exist, creating", "dir", pagesDir)
		if err := os.MkdirAll(pagesDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating pages directory: %w", err)
		}
		return nil, nil
	}

	var pages []Page

	entries, err := os.ReadDir(pagesDir)
	if err != nil {
		return nil, fmt.Errorf("reading pages directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		page, err := l.parsePage(filepath.Join(pagesDir, entry.Name()))
		if err != nil {
			l.logger.Warn("skipping invalid page", "path", entry.Name(), "error", err)
			continue
		}
		pages = append(pages, *page)
	}

	return pages, nil
}

func (l *Loader) parsePost(path string) (*Post, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading post file: %w", err)
	}

	frontmatter, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	var post Post
	if err := yaml.Unmarshal(frontmatter, &post); err != nil {
		return nil, fmt.Errorf("parsing post YAML: %w", err)
	}

	// Derive slug from directory name if not set
	dir := filepath.Dir(path)
	dirName := filepath.Base(dir)
	if post.Slug == "" {
		post.Slug = dirName
		// Strip date prefix if present (e.g., "2024-09-20-homelab-start" → "homelab-start")
		parts := strings.SplitN(dirName, "-", 4)
		if len(parts) >= 4 {
			if _, err := time.Parse("2006-01-02", strings.Join(parts[:3], "-")); err == nil {
				post.Slug = parts[3]
			}
		}
	}

	post.Content = string(body)
	post.Dir = dir
	post.Permalink = "/" + post.Slug
	post.WordCount = countWords(string(body))
	post.ReadingTime = int(math.Ceil(float64(post.WordCount) / 200.0))
	if post.ReadingTime < 1 {
		post.ReadingTime = 1
	}

	if post.Title == "" {
		return nil, fmt.Errorf("post %q has no title", path)
	}

	return &post, nil
}

func (l *Loader) parsePage(path string) (*Page, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading page file: %w", err)
	}

	frontmatter, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	var page Page
	if err := yaml.Unmarshal(frontmatter, &page); err != nil {
		return nil, fmt.Errorf("parsing page YAML: %w", err)
	}

	// Derive slug from filename if not set
	if page.Slug == "" {
		name := filepath.Base(path)
		page.Slug = strings.TrimSuffix(name, ".md")
	}

	page.Content = string(body)
	page.Dir = filepath.Dir(path)
	page.Permalink = "/" + page.Slug

	return &page, nil
}

// splitFrontmatter separates YAML frontmatter from markdown body.
// Expects the file to start with "---\n" and end the frontmatter with "---\n".
func splitFrontmatter(data []byte) ([]byte, []byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	// First line must be "---"
	if !scanner.Scan() {
		return nil, nil, fmt.Errorf("empty file")
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return nil, nil, fmt.Errorf("file does not start with frontmatter delimiter")
	}

	var frontmatter bytes.Buffer
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			found = true
			break
		}
		frontmatter.WriteString(line)
		frontmatter.WriteByte('\n')
	}

	if !found {
		return nil, nil, fmt.Errorf("no closing frontmatter delimiter found")
	}

	// Rest is the body
	var body bytes.Buffer
	for scanner.Scan() {
		body.WriteString(scanner.Text())
		body.WriteByte('\n')
	}

	return frontmatter.Bytes(), body.Bytes(), scanner.Err()
}

func countWords(s string) int {
	count := 0
	inWord := false
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
		i += size
	}
	return count
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
	// Collapse multiple hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
