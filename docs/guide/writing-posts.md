# Writing Posts

## Post Format

Every post lives in `data/content/posts/<slug>/index.md` with YAML frontmatter:

```markdown
---
title: "My First Post"
slug: "my-first-post"
date: 2024-01-15
tags: ["go", "blogging"]
description: "A short description for SEO"
image: "cover.jpg"
featured: false
draft: false
toc: true
series: "Building a Blog"
series_order: 1
---

Your markdown content here...
```

## Frontmatter Fields

| Field          | Type     | Required | Description                                     |
| -------------- | -------- | -------- | ----------------------------------------------- |
| `title`        | string   | Yes      | Post title                                      |
| `slug`         | string   | Yes      | URL slug (auto-generated from title)            |
| `date`         | date     | Yes      | Publication date (`YYYY-MM-DD`)                 |
| `tags`         | string[] | No       | Tag names                                       |
| `description`  | string   | No       | SEO description / excerpt                       |
| `image`        | string   | No       | Featured image filename (in `media/` subfolder) |
| `image_credit` | object   | No       | Attribution: `name` (string) and `url` (string) |
| `featured`     | bool     | No       | Show in featured section                        |
| `draft`        | bool     | No       | Hidden from public (visible in admin)           |
| `toc`          | bool     | No       | Show auto-generated table of contents           |
| `series`       | string   | No       | Series name for multi-part guides               |
| `series_order` | int      | No       | Position in series (1, 2, 3...)                 |

## Markdown Features

Skriva uses [goldmark](https://github.com/yuin/goldmark) with GFM extensions:

- **GFM tables** — pipe-based tables
- **Strikethrough** — `~~deleted~~`
- **Autolinks** — URLs auto-link
- **Syntax highlighting** — fenced code blocks with language tags (Chroma/Dracula)
- **Smart quotes** — typographic quotes, dashes, ellipsis
- **Heading IDs** — auto-generated for anchor links
- **Raw HTML** — allowed in admin-authored posts (for embeds)

## Media

Place images in `data/content/posts/<slug>/media/`:

```
posts/
  my-first-post/
    index.md
    media/
      cover.jpg
      diagram.png
```

Reference in markdown: `![Alt text](media/cover.jpg)`

Images are served at `/media/<slug>/<filename>`.

## Scheduled Posts

Set a future `date` — the post auto-publishes when the time arrives:

```yaml
date: 2025-12-25
draft: false
```

## Draft Sharing

Create a secret preview link for draft posts from the admin editor. Links expire after 7 days.

## Revision History

Every edit is automatically versioned. View and restore previous versions from the editor's revision history panel.
