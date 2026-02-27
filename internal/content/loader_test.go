package content

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"Go Programming", "go-programming"},
		{"with---multiple---dashes", "with-multiple-dashes"},
		{"UPPERCASE", "uppercase"},
		{"special!@#$chars", "specialchars"},
		{"already-slugified", "already-slugified"},
		{"", ""},
		{"  leading trailing  ", "leading-trailing"},
	}
	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCountWords(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"hello world", 2},
		{"one", 1},
		{"", 0},
		{"  spaces  between  words  ", 3},
		{"line\nbreaks\ncount", 3},
		{"tabs\tand\tspaces", 3},
	}
	for _, tt := range tests {
		got := countWords(tt.input)
		if got != tt.want {
			t.Errorf("countWords(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestSplitFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantFM   string
		wantBody string
		wantErr  bool
	}{
		{
			name:     "valid frontmatter",
			input:    "---\ntitle: Hello\n---\nBody text\n",
			wantFM:   "title: Hello\n",
			wantBody: "Body text\n",
			wantErr:  false,
		},
		{
			name:    "no frontmatter",
			input:   "Just body text",
			wantErr: true,
		},
		{
			name:    "empty file",
			input:   "",
			wantErr: true,
		},
		{
			name:    "no closing delimiter",
			input:   "---\ntitle: Hello\nBody text",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body, err := splitFrontmatter([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(fm) != tt.wantFM {
				t.Errorf("frontmatter = %q, want %q", string(fm), tt.wantFM)
			}
			if string(body) != tt.wantBody {
				t.Errorf("body = %q, want %q", string(body), tt.wantBody)
			}
		})
	}
}

// writePost creates a post file at {dir}/posts/{slug}/index.md
func writePost(t *testing.T, dir, slug, content string) {
	t.Helper()
	postDir := filepath.Join(dir, "posts", slug)
	if err := os.MkdirAll(postDir, 0o755); err != nil {
		t.Fatalf("creating post dir %s: %v", slug, err)
	}
	if err := os.WriteFile(filepath.Join(postDir, "index.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing post %s: %v", slug, err)
	}
}

// writePage creates a page file at {dir}/pages/{slug}.md
func writePage(t *testing.T, dir, slug, content string) {
	t.Helper()
	pagesDir := filepath.Join(dir, "pages")
	if err := os.MkdirAll(pagesDir, 0o755); err != nil {
		t.Fatalf("creating pages dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pagesDir, slug+".md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing page %s: %v", slug, err)
	}
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// setupTestLoader creates a Loader with a realistic set of posts and pages.
func setupTestLoader(t *testing.T) (*Loader, string) {
	t.Helper()
	dir := t.TempDir()

	writePost(t, dir, "hello-world", `---
title: "Hello World"
slug: "hello-world"
date: 2026-01-15
tags: ["go", "blog"]
description: "A hello world post"
draft: false
featured: false
---

This is the hello world body content.
`)

	writePost(t, dir, "go-testing", `---
title: "Go Testing"
slug: "go-testing"
date: 2026-01-20
tags: ["go", "testing"]
description: "A post about Go testing"
draft: false
featured: false
---

This is a post about testing in Go. It covers unit tests and benchmarks.
`)

	writePost(t, dir, "draft-post", `---
title: "Draft Post"
slug: "draft-post"
date: 2026-01-10
tags: ["go"]
description: "A draft post"
draft: true
featured: false
---

This draft is not published yet.
`)

	writePost(t, dir, "featured-post", `---
title: "Featured Post"
slug: "featured-post"
date: 2026-01-25
tags: ["blog", "featured"]
description: "A featured post"
draft: false
featured: true
---

This post is featured.
`)

	writePost(t, dir, "future-post", `---
title: "Future Post"
slug: "future-post"
date: 2030-01-01
tags: ["go"]
description: "A future scheduled post"
draft: false
featured: false
---

This post is scheduled for the future.
`)

	writePost(t, dir, "series-part-1", `---
title: "Series Part 1"
slug: "series-part-1"
date: 2026-01-05
tags: ["go", "series"]
description: "Part 1 of a series"
draft: false
featured: false
series: "go-basics"
series_order: 1
---

First part of the Go basics series.
`)

	writePost(t, dir, "series-part-2", `---
title: "Series Part 2"
slug: "series-part-2"
date: 2026-01-06
tags: ["go", "series"]
description: "Part 2 of a series"
draft: false
featured: false
series: "go-basics"
series_order: 2
---

Second part of the Go basics series.
`)

	writePage(t, dir, "about", `---
title: "About"
slug: "about"
description: "About page"
---

About page content here.
`)

	writePage(t, dir, "contact", `---
title: "Contact"
slug: "contact"
description: "Contact page"
---

Contact page content here.
`)

	loader, err := NewLoader(dir, newTestLogger())
	if err != nil {
		t.Fatalf("NewLoader() error: %v", err)
	}
	return loader, dir
}

func TestNewLoader(t *testing.T) {
	loader, dir := setupTestLoader(t)

	if loader == nil {
		t.Fatal("NewLoader() returned nil")
	}
	if loader.ContentDir() != dir {
		t.Errorf("ContentDir() = %q, want %q", loader.ContentDir(), dir)
	}

	// Should have loaded all posts (including drafts and future)
	all := loader.AllPosts()
	if len(all) != 7 {
		t.Errorf("AllPosts() count = %d, want 7", len(all))
	}
}

func TestNewLoaderEmptyDir(t *testing.T) {
	dir := t.TempDir()
	loader, err := NewLoader(dir, newTestLogger())
	if err != nil {
		t.Fatalf("NewLoader() with empty dir error: %v", err)
	}
	if len(loader.AllPosts()) != 0 {
		t.Errorf("expected 0 posts, got %d", len(loader.AllPosts()))
	}
	if len(loader.Tags()) != 0 {
		t.Errorf("expected 0 tags, got %d", len(loader.Tags()))
	}
}

func TestReload(t *testing.T) {
	loader, dir := setupTestLoader(t)

	initialCount := len(loader.AllPosts())

	// Add a new post
	writePost(t, dir, "new-post", `---
title: "New Post"
slug: "new-post"
date: 2026-02-01
tags: ["new"]
description: "A newly added post"
draft: false
featured: false
---

This post was added after initial load.
`)

	// Before reload, count is unchanged
	if got := len(loader.AllPosts()); got != initialCount {
		t.Errorf("before Reload(), AllPosts() = %d, want %d", got, initialCount)
	}

	if err := loader.Reload(); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}

	if got := len(loader.AllPosts()); got != initialCount+1 {
		t.Errorf("after Reload(), AllPosts() = %d, want %d", got, initialCount+1)
	}

	p, ok := loader.PostBySlug("new-post")
	if !ok {
		t.Fatal("PostBySlug('new-post') not found after Reload()")
	}
	if p.Title != "New Post" {
		t.Errorf("new post title = %q, want %q", p.Title, "New Post")
	}
}

func TestPosts(t *testing.T) {
	loader, _ := setupTestLoader(t)

	published := loader.Posts()

	// Should exclude draft-post and future-post
	// Published: featured-post (Jan 25), go-testing (Jan 20), hello-world (Jan 15),
	//            series-part-2 (Jan 6), series-part-1 (Jan 5)
	if len(published) != 5 {
		t.Fatalf("Posts() count = %d, want 5", len(published))
	}

	// Verify date-descending order
	for i := 1; i < len(published); i++ {
		if published[i].Date.After(published[i-1].Date) {
			t.Errorf("Posts() not sorted by date desc: %s (%v) before %s (%v)",
				published[i-1].Slug, published[i-1].Date,
				published[i].Slug, published[i].Date)
		}
	}

	// Verify drafts are excluded
	for _, p := range published {
		if p.Slug == "draft-post" {
			t.Error("Posts() should not include draft-post")
		}
		if p.Slug == "future-post" {
			t.Error("Posts() should not include future-post")
		}
	}
}

func TestAllPosts(t *testing.T) {
	loader, _ := setupTestLoader(t)

	all := loader.AllPosts()

	// Should include draft and future: 7 total
	if len(all) != 7 {
		t.Fatalf("AllPosts() count = %d, want 7", len(all))
	}

	slugs := make(map[string]bool)
	for _, p := range all {
		slugs[p.Slug] = true
	}
	if !slugs["draft-post"] {
		t.Error("AllPosts() should include draft-post")
	}
	if !slugs["future-post"] {
		t.Error("AllPosts() should include future-post")
	}

	// Verify date-descending order
	for i := 1; i < len(all); i++ {
		if all[i].Date.After(all[i-1].Date) {
			t.Errorf("AllPosts() not sorted by date desc: %s (%v) before %s (%v)",
				all[i-1].Slug, all[i-1].Date,
				all[i].Slug, all[i].Date)
		}
	}
}

func TestPostBySlug(t *testing.T) {
	loader, _ := setupTestLoader(t)

	t.Run("existing published post", func(t *testing.T) {
		p, ok := loader.PostBySlug("hello-world")
		if !ok || p == nil {
			t.Fatal("PostBySlug('hello-world') not found")
		}
		if p.Title != "Hello World" {
			t.Errorf("title = %q, want %q", p.Title, "Hello World")
		}
		if p.Permalink != "/hello-world" {
			t.Errorf("permalink = %q, want %q", p.Permalink, "/hello-world")
		}
	})

	t.Run("non-existing slug", func(t *testing.T) {
		_, ok := loader.PostBySlug("does-not-exist")
		if ok {
			t.Error("PostBySlug('does-not-exist') should return false")
		}
	})

	t.Run("draft excluded", func(t *testing.T) {
		_, ok := loader.PostBySlug("draft-post")
		if ok {
			t.Error("PostBySlug('draft-post') should return false for drafts")
		}
	})

	t.Run("future post excluded", func(t *testing.T) {
		_, ok := loader.PostBySlug("future-post")
		if ok {
			t.Error("PostBySlug('future-post') should return false for scheduled posts")
		}
	})
}

func TestPostBySlugAdmin(t *testing.T) {
	loader, _ := setupTestLoader(t)

	t.Run("published post", func(t *testing.T) {
		p, ok := loader.PostBySlugAdmin("hello-world")
		if !ok || p == nil {
			t.Fatal("PostBySlugAdmin('hello-world') not found")
		}
		if p.Title != "Hello World" {
			t.Errorf("title = %q, want %q", p.Title, "Hello World")
		}
	})

	t.Run("draft included", func(t *testing.T) {
		p, ok := loader.PostBySlugAdmin("draft-post")
		if !ok || p == nil {
			t.Fatal("PostBySlugAdmin('draft-post') should find draft")
		}
		if !p.Draft {
			t.Error("expected draft=true")
		}
	})

	t.Run("future post included", func(t *testing.T) {
		p, ok := loader.PostBySlugAdmin("future-post")
		if !ok || p == nil {
			t.Fatal("PostBySlugAdmin('future-post') should find future post")
		}
		if p.Title != "Future Post" {
			t.Errorf("title = %q, want %q", p.Title, "Future Post")
		}
	})

	t.Run("non-existing slug", func(t *testing.T) {
		_, ok := loader.PostBySlugAdmin("does-not-exist")
		if ok {
			t.Error("PostBySlugAdmin('does-not-exist') should return false")
		}
	})
}

func TestPageBySlug(t *testing.T) {
	loader, _ := setupTestLoader(t)

	t.Run("existing page", func(t *testing.T) {
		p, ok := loader.PageBySlug("about")
		if !ok || p == nil {
			t.Fatal("PageBySlug('about') not found")
		}
		if p.Title != "About" {
			t.Errorf("title = %q, want %q", p.Title, "About")
		}
		if p.Description != "About page" {
			t.Errorf("description = %q, want %q", p.Description, "About page")
		}
		if p.Permalink != "/about" {
			t.Errorf("permalink = %q, want %q", p.Permalink, "/about")
		}
	})

	t.Run("contact page", func(t *testing.T) {
		p, ok := loader.PageBySlug("contact")
		if !ok || p == nil {
			t.Fatal("PageBySlug('contact') not found")
		}
		if p.Title != "Contact" {
			t.Errorf("title = %q, want %q", p.Title, "Contact")
		}
	})

	t.Run("non-existing page", func(t *testing.T) {
		_, ok := loader.PageBySlug("nonexistent")
		if ok {
			t.Error("PageBySlug('nonexistent') should return false")
		}
	})
}

func TestPostsByTag(t *testing.T) {
	loader, _ := setupTestLoader(t)

	t.Run("go tag", func(t *testing.T) {
		posts := loader.PostsByTag("go")
		// Published with "go" tag: hello-world, go-testing, series-part-1, series-part-2
		// draft-post has "go" but is draft, future-post has "go" but is future
		if len(posts) != 4 {
			names := make([]string, len(posts))
			for i, p := range posts {
				names[i] = p.Slug
			}
			t.Fatalf("PostsByTag('go') count = %d, want 4, got: %v", len(posts), names)
		}
	})

	t.Run("blog tag", func(t *testing.T) {
		posts := loader.PostsByTag("blog")
		// Published with "blog" tag: hello-world, featured-post
		if len(posts) != 2 {
			t.Errorf("PostsByTag('blog') count = %d, want 2", len(posts))
		}
	})

	t.Run("nonexistent tag", func(t *testing.T) {
		posts := loader.PostsByTag("nonexistent")
		if len(posts) != 0 {
			t.Errorf("PostsByTag('nonexistent') count = %d, want 0", len(posts))
		}
	})

	t.Run("excludes drafts", func(t *testing.T) {
		posts := loader.PostsByTag("go")
		for _, p := range posts {
			if p.Draft {
				t.Errorf("PostsByTag should not include draft: %s", p.Slug)
			}
		}
	})
}

func TestTags(t *testing.T) {
	loader, _ := setupTestLoader(t)

	tags := loader.Tags()
	if len(tags) == 0 {
		t.Fatal("Tags() returned empty")
	}

	tagMap := make(map[string]int)
	for _, tag := range tags {
		tagMap[tag.Slug] = tag.Count
	}

	// "go" appears in: hello-world, go-testing, draft-post, future-post, series-part-1, series-part-2 = 6
	if count, ok := tagMap["go"]; !ok || count != 6 {
		t.Errorf("tag 'go' count = %d, want 6", count)
	}

	// "blog" appears in: hello-world, featured-post = 2
	if count, ok := tagMap["blog"]; !ok || count != 2 {
		t.Errorf("tag 'blog' count = %d, want 2", count)
	}

	// "testing" appears in: go-testing = 1
	if count, ok := tagMap["testing"]; !ok || count != 1 {
		t.Errorf("tag 'testing' count = %d, want 1", count)
	}

	// Verify alphabetical sort
	for i := 1; i < len(tags); i++ {
		if tags[i].Name < tags[i-1].Name {
			t.Errorf("Tags() not sorted alphabetically: %q before %q", tags[i-1].Name, tags[i].Name)
		}
	}
}

func TestFeaturedPosts(t *testing.T) {
	loader, _ := setupTestLoader(t)

	featured := loader.FeaturedPosts()
	if len(featured) != 1 {
		t.Fatalf("FeaturedPosts() count = %d, want 1", len(featured))
	}
	if featured[0].Slug != "featured-post" {
		t.Errorf("featured slug = %q, want %q", featured[0].Slug, "featured-post")
	}
	if !featured[0].Featured {
		t.Error("expected Featured=true")
	}
}

func TestContentDir(t *testing.T) {
	loader, dir := setupTestLoader(t)

	if got := loader.ContentDir(); got != dir {
		t.Errorf("ContentDir() = %q, want %q", got, dir)
	}
}

func TestSeriesPosts(t *testing.T) {
	loader, _ := setupTestLoader(t)

	t.Run("existing series", func(t *testing.T) {
		posts := loader.SeriesPosts("go-basics")
		if len(posts) != 2 {
			t.Fatalf("SeriesPosts('go-basics') count = %d, want 2", len(posts))
		}
		// Should be sorted by series_order
		if posts[0].SeriesOrder != 1 {
			t.Errorf("first post series_order = %d, want 1", posts[0].SeriesOrder)
		}
		if posts[1].SeriesOrder != 2 {
			t.Errorf("second post series_order = %d, want 2", posts[1].SeriesOrder)
		}
		if posts[0].Slug != "series-part-1" {
			t.Errorf("first post slug = %q, want %q", posts[0].Slug, "series-part-1")
		}
		if posts[1].Slug != "series-part-2" {
			t.Errorf("second post slug = %q, want %q", posts[1].Slug, "series-part-2")
		}
	})

	t.Run("empty series name", func(t *testing.T) {
		posts := loader.SeriesPosts("")
		if posts != nil {
			t.Errorf("SeriesPosts('') should return nil, got %d posts", len(posts))
		}
	})

	t.Run("nonexistent series", func(t *testing.T) {
		posts := loader.SeriesPosts("nonexistent")
		if len(posts) != 0 {
			t.Errorf("SeriesPosts('nonexistent') count = %d, want 0", len(posts))
		}
	})
}

func TestRelatedPosts(t *testing.T) {
	loader, _ := setupTestLoader(t)

	t.Run("related by tag", func(t *testing.T) {
		post, ok := loader.PostBySlug("hello-world")
		if !ok {
			t.Fatal("hello-world not found")
		}
		// hello-world has tags: go, blog
		// Should match: go-testing (go), featured-post (blog), series-part-1 (go), series-part-2 (go)
		related := loader.RelatedPosts(post, 10)
		if len(related) == 0 {
			t.Fatal("RelatedPosts() returned empty")
		}
		// All related posts should share at least one tag
		postTags := map[string]bool{"go": true, "blog": true}
		for _, rp := range related {
			hasShared := false
			for _, tag := range rp.Tags {
				if postTags[slugify(tag)] {
					hasShared = true
					break
				}
			}
			if !hasShared {
				t.Errorf("related post %q shares no tags with hello-world", rp.Slug)
			}
		}
		// Should not include self
		for _, rp := range related {
			if rp.Slug == "hello-world" {
				t.Error("RelatedPosts should not include the source post itself")
			}
		}
	})

	t.Run("limit results", func(t *testing.T) {
		post, _ := loader.PostBySlug("hello-world")
		related := loader.RelatedPosts(post, 2)
		if len(related) > 2 {
			t.Errorf("RelatedPosts(_, 2) returned %d, want <= 2", len(related))
		}
	})

	t.Run("nil post", func(t *testing.T) {
		related := loader.RelatedPosts(nil, 5)
		if related != nil {
			t.Error("RelatedPosts(nil, 5) should return nil")
		}
	})

	t.Run("post with no tags", func(t *testing.T) {
		noTags := &Post{Slug: "no-tags", Tags: nil}
		related := loader.RelatedPosts(noTags, 5)
		if related != nil {
			t.Error("RelatedPosts for post with no tags should return nil")
		}
	})
}

func TestSearchPosts(t *testing.T) {
	loader, _ := setupTestLoader(t)

	t.Run("search by title", func(t *testing.T) {
		results := loader.SearchPosts("Hello")
		if len(results) != 1 {
			t.Fatalf("SearchPosts('Hello') count = %d, want 1", len(results))
		}
		if results[0].Slug != "hello-world" {
			t.Errorf("result slug = %q, want %q", results[0].Slug, "hello-world")
		}
	})

	t.Run("search by description", func(t *testing.T) {
		results := loader.SearchPosts("featured")
		found := false
		for _, r := range results {
			if r.Slug == "featured-post" {
				found = true
				break
			}
		}
		if !found {
			t.Error("SearchPosts('featured') should find featured-post")
		}
	})

	t.Run("search by body content", func(t *testing.T) {
		results := loader.SearchPosts("benchmarks")
		if len(results) != 1 {
			t.Fatalf("SearchPosts('benchmarks') count = %d, want 1", len(results))
		}
		if results[0].Slug != "go-testing" {
			t.Errorf("result slug = %q, want %q", results[0].Slug, "go-testing")
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		results := loader.SearchPosts("HELLO")
		if len(results) != 1 {
			t.Fatalf("SearchPosts('HELLO') count = %d, want 1", len(results))
		}
	})

	t.Run("multi-word query", func(t *testing.T) {
		results := loader.SearchPosts("testing Go")
		found := false
		for _, r := range results {
			if r.Slug == "go-testing" {
				found = true
			}
		}
		if !found {
			t.Error("SearchPosts('testing Go') should find go-testing")
		}
	})

	t.Run("empty query", func(t *testing.T) {
		results := loader.SearchPosts("")
		if results != nil {
			t.Errorf("SearchPosts('') should return nil, got %d", len(results))
		}
	})

	t.Run("no match", func(t *testing.T) {
		results := loader.SearchPosts("zzzznotfound")
		if len(results) != 0 {
			t.Errorf("SearchPosts('zzzznotfound') count = %d, want 0", len(results))
		}
	})

	t.Run("excludes drafts", func(t *testing.T) {
		// "not published yet" appears only in draft-post body
		results := loader.SearchPosts("not published yet")
		for _, r := range results {
			if r.Slug == "draft-post" {
				t.Error("SearchPosts should exclude draft posts")
			}
		}
	})
}

func TestPostComputedFields(t *testing.T) {
	loader, _ := setupTestLoader(t)

	post, ok := loader.PostBySlug("hello-world")
	if !ok {
		t.Fatal("hello-world not found")
	}

	if post.WordCount == 0 {
		t.Error("WordCount should be > 0")
	}
	if post.ReadingTime < 1 {
		t.Error("ReadingTime should be >= 1")
	}
	if post.Content == "" {
		t.Error("Content should not be empty")
	}
	if post.Dir == "" {
		t.Error("Dir should not be empty")
	}
	if post.Permalink != "/hello-world" {
		t.Errorf("Permalink = %q, want %q", post.Permalink, "/hello-world")
	}
}

func TestSlugDerivedFromDir(t *testing.T) {
	dir := t.TempDir()

	// Create a post without a slug field; slug should be derived from dir name
	writePost(t, dir, "my-derived-slug", `---
title: "Derived Slug"
date: 2026-01-01
tags: ["test"]
description: "Test slug derivation"
draft: false
---

Content.
`)

	loader, err := NewLoader(dir, newTestLogger())
	if err != nil {
		t.Fatalf("NewLoader() error: %v", err)
	}

	p, ok := loader.PostBySlugAdmin("my-derived-slug")
	if !ok {
		t.Fatal("expected post with slug derived from directory name")
	}
	if p.Slug != "my-derived-slug" {
		t.Errorf("derived slug = %q, want %q", p.Slug, "my-derived-slug")
	}
}

func TestPageSlugDerivedFromFilename(t *testing.T) {
	dir := t.TempDir()

	// Create a page without a slug field
	writePage(t, dir, "my-page", `---
title: "My Page"
description: "Test page slug derivation"
---

Page content.
`)

	loader, err := NewLoader(dir, newTestLogger())
	if err != nil {
		t.Fatalf("NewLoader() error: %v", err)
	}

	p, ok := loader.PageBySlug("my-page")
	if !ok {
		t.Fatal("expected page with slug derived from filename")
	}
	if p.Slug != "my-page" {
		t.Errorf("derived slug = %q, want %q", p.Slug, "my-page")
	}
}

func TestPostMissingTitle(t *testing.T) {
	dir := t.TempDir()

	writePost(t, dir, "no-title", `---
slug: "no-title"
date: 2026-01-01
---

No title here.
`)

	loader, err := NewLoader(dir, newTestLogger())
	if err != nil {
		t.Fatalf("NewLoader() error: %v", err)
	}

	// Post with no title should be skipped
	_, ok := loader.PostBySlugAdmin("no-title")
	if ok {
		t.Error("post without title should be skipped during loading")
	}
}

func TestFeaturedExcludesDrafts(t *testing.T) {
	dir := t.TempDir()

	writePost(t, dir, "feat-draft", `---
title: "Featured Draft"
slug: "feat-draft"
date: 2026-01-01
tags: ["test"]
description: "Featured but draft"
draft: true
featured: true
---

Content.
`)

	loader, err := NewLoader(dir, newTestLogger())
	if err != nil {
		t.Fatalf("NewLoader() error: %v", err)
	}

	featured := loader.FeaturedPosts()
	for _, p := range featured {
		if p.Slug == "feat-draft" {
			t.Error("FeaturedPosts should not include draft posts")
		}
	}
}

func FuzzSplitFrontmatter(f *testing.F) {
	f.Add([]byte("---\ntitle: test\n---\nHello"))
	f.Add([]byte("no frontmatter"))
	f.Add([]byte("---\n\x00\xff\n---\n"))
	f.Add([]byte(""))
	f.Fuzz(func(t *testing.T, data []byte) {
		splitFrontmatter(data) // must not panic
	})
}
