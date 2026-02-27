// Package importer — WordPress WXR (WordPress eXtended RSS) import support.
// Converts WordPress XML export files to Skriva markdown posts.
package importer

import (
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// WordPressExport represents the top-level WordPress WXR structure.
type WordPressExport struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Items []wpItem `xml:"item"`
	} `xml:"channel"`
}

type wpItem struct {
	Title       string       `xml:"title"`
	Link        string       `xml:"link"`
	PubDate     string       `xml:"pubDate"`
	PostName    string       `xml:"http://wordpress.org/export/1.2/ post_name"`
	PostType    string       `xml:"http://wordpress.org/export/1.2/ post_type"`
	Status      string       `xml:"http://wordpress.org/export/1.2/ status"`
	ContentHTML string       `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	ExcerptHTML string       `xml:"http://wordpress.org/export/1.2/excerpt/ encoded"`
	Categories  []wpCategory `xml:"category"`
}

type wpCategory struct {
	Domain string `xml:"domain,attr"`
	Name   string `xml:",chardata"`
}

// ImportWordPress reads a WordPress WXR XML export and creates markdown posts.
func ImportWordPress(exportPath, outputDir string, logger *slog.Logger) error {
	data, err := os.ReadFile(exportPath)
	if err != nil {
		return fmt.Errorf("reading WordPress export: %w", err)
	}

	var export WordPressExport
	if err := xml.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parsing WordPress XML: %w", err)
	}

	postsDir := filepath.Join(outputDir, "posts")
	if err := os.MkdirAll(postsDir, 0o755); err != nil {
		return fmt.Errorf("creating posts directory: %w", err)
	}

	imported := 0
	skipped := 0

	for i := range export.Channel.Items {
		item := &export.Channel.Items[i]
		// Only import posts (not pages, attachments, etc.)
		if item.PostType != "post" && item.PostType != "" {
			if item.PostType != "post" {
				skipped++
				continue
			}
		}

		// Skip drafts unless they have content
		if item.Status == "trash" {
			skipped++
			continue
		}

		slug := item.PostName
		if slug == "" {
			slug = slugifyWP(item.Title)
		}
		if slug == "" {
			skipped++
			continue
		}

		// Parse date
		date := time.Now()
		if item.PubDate != "" {
			if t, err := time.Parse("Mon, 02 Jan 2006 15:04:05 -0700", item.PubDate); err == nil {
				date = t
			} else if t, err := time.Parse(time.RFC1123Z, item.PubDate); err == nil {
				date = t
			}
		}

		// Extract tags (category domain="post_tag")
		var tags []string
		for _, cat := range item.Categories {
			if cat.Domain == "post_tag" {
				tags = append(tags, cat.Name)
			} else if cat.Domain == "category" && cat.Name != "Uncategorized" {
				tags = append(tags, cat.Name)
			}
		}

		// Convert HTML content to markdown
		content := wpHTMLToMarkdown(item.ContentHTML)
		if content == "" && item.Status == "draft" {
			skipped++
			continue
		}

		isDraft := item.Status == "draft"
		description := wpStripHTML(item.ExcerptHTML)
		if len(description) > 300 {
			description = description[:300]
		}

		// Build frontmatter
		tagsYAML := ""
		if len(tags) > 0 {
			quoted := make([]string, len(tags))
			for i, t := range tags {
				quoted[i] = fmt.Sprintf("%q", t)
			}
			tagsYAML = strings.Join(quoted, ", ")
		}

		frontmatter := fmt.Sprintf(`---
title: %q
slug: %q
date: %s
tags: [%s]
description: %q
draft: %v
---

%s`, item.Title, slug, date.Format("2006-01-02"), tagsYAML, description, isDraft, content)

		// Create post directory and write file
		postDir := filepath.Join(postsDir, slug)
		if err := os.MkdirAll(postDir, 0o755); err != nil {
			logger.Warn("failed to create post directory", "slug", slug, "error", err)
			continue
		}

		indexPath := filepath.Join(postDir, "index.md")
		if err := os.WriteFile(indexPath, []byte(frontmatter), 0o644); err != nil {
			logger.Warn("failed to write post", "slug", slug, "error", err)
			continue
		}

		imported++
		logger.Info("imported post", "slug", slug, "title", item.Title)
	}

	logger.Info("WordPress import complete", "imported", imported, "skipped", skipped)
	return nil
}

// wpHTMLToMarkdown converts WordPress HTML content to markdown.
// This is a simplified converter that handles the most common patterns.
func wpHTMLToMarkdown(html string) string {
	if html == "" {
		return ""
	}

	s := html

	// WordPress uses \n\n for paragraphs (not <p> tags sometimes)
	// But also uses <p> tags
	s = regexp.MustCompile(`<p[^>]*>`).ReplaceAllString(s, "\n\n")
	s = strings.ReplaceAll(s, "</p>", "\n\n")

	// Headings
	for i := 6; i >= 1; i-- {
		prefix := strings.Repeat("#", i)
		s = regexp.MustCompile(fmt.Sprintf(`<h%d[^>]*>`, i)).ReplaceAllString(s, fmt.Sprintf("\n\n%s ", prefix))
		s = strings.ReplaceAll(s, fmt.Sprintf("</h%d>", i), "\n\n")
	}

	// Bold and italic
	s = regexp.MustCompile(`<strong[^>]*>`).ReplaceAllString(s, "**")
	s = strings.ReplaceAll(s, "</strong>", "**")
	s = regexp.MustCompile(`<b[^>]*>`).ReplaceAllString(s, "**")
	s = strings.ReplaceAll(s, "</b>", "**")
	s = regexp.MustCompile(`<em[^>]*>`).ReplaceAllString(s, "*")
	s = strings.ReplaceAll(s, "</em>", "*")

	// Images (must come before <i> italic to avoid <img being matched as <i>)
	imgRe := regexp.MustCompile(`<img[^>]*src="([^"]*)"[^>]*alt="([^"]*)"[^>]*/?>`)
	s = imgRe.ReplaceAllString(s, "![$2]($1)")
	imgRe2 := regexp.MustCompile(`<img[^>]*src="([^"]*)"[^>]*/?>`)
	s = imgRe2.ReplaceAllString(s, "![]($1)")

	// Italic (after images so <img> is already consumed)
	s = regexp.MustCompile(`<i[ >][^>]*>`).ReplaceAllString(s, "*")
	s = strings.ReplaceAll(s, "<i>", "*")
	s = strings.ReplaceAll(s, "</i>", "*")

	// Links
	linkRe := regexp.MustCompile(`<a[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	s = linkRe.ReplaceAllString(s, "[$2]($1)")

	// Lists
	s = regexp.MustCompile(`<ul[^>]*>`).ReplaceAllString(s, "\n")
	s = strings.ReplaceAll(s, "</ul>", "\n")
	s = regexp.MustCompile(`<ol[^>]*>`).ReplaceAllString(s, "\n")
	s = strings.ReplaceAll(s, "</ol>", "\n")
	s = regexp.MustCompile(`<li[^>]*>`).ReplaceAllString(s, "- ")
	s = strings.ReplaceAll(s, "</li>", "\n")

	// Blockquotes
	s = regexp.MustCompile(`<blockquote[^>]*>`).ReplaceAllString(s, "\n> ")
	s = strings.ReplaceAll(s, "</blockquote>", "\n")

	// Code blocks
	s = regexp.MustCompile(`<pre[^>]*><code[^>]*>`).ReplaceAllString(s, "\n```\n")
	s = strings.ReplaceAll(s, "</code></pre>", "\n```\n")
	s = regexp.MustCompile(`<code[^>]*>`).ReplaceAllString(s, "`")
	s = strings.ReplaceAll(s, "</code>", "`")

	// Line breaks
	s = regexp.MustCompile(`<br\s*/?>`).ReplaceAllString(s, "\n")

	// Horizontal rules
	s = regexp.MustCompile(`<hr\s*/?>`).ReplaceAllString(s, "\n---\n")

	// Strip remaining HTML tags
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")

	// Decode common HTML entities
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#039;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&#8217;", "'")
	s = strings.ReplaceAll(s, "&#8216;", "'")
	s = strings.ReplaceAll(s, "&#8220;", `"`)
	s = strings.ReplaceAll(s, "&#8221;", `"`)
	s = strings.ReplaceAll(s, "&#8211;", "–")
	s = strings.ReplaceAll(s, "&#8212;", "—")
	s = strings.ReplaceAll(s, "&#8230;", "…")

	// Clean up excessive whitespace
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	s = strings.TrimSpace(s)

	return s
}

func wpStripHTML(s string) string {
	return strings.TrimSpace(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, ""))
}

func slugifyWP(s string) string {
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

// Ensure io import is used
var _ = io.Discard
