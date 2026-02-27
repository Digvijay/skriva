# Skriva

[![Version](https://img.shields.io/badge/version-0.1.0-blue)](https://github.com/Digvijay/skriva/releases/tag/v0.1.0)
[![Go](https://img.shields.io/badge/go-1.25+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Docs](https://img.shields.io/badge/docs-skriva.digvijay.dev-purple)](https://skriva.digvijay.dev)

A lightweight, single-binary personal blog engine written in Go. Designed for simplicity, performance, and easy migration.

## Features

- **Single binary** (~26MB) — no runtime dependencies, no CDN, no external JS/CSS
- **Two volumes** — content (posts, media, themes) + config (settings, secrets, database)
- **4 bundled themes** — Classic, Newsletter, GitHub, Editorial — all with light/dark/auto toggle
- **Markdown posts** with YAML frontmatter, GFM tables, syntax highlighting (Chroma)
- **Full-text search** — `/search` page with multi-word matching
- **Tags** — `/tags` index page + paginated tag pages
- **Related posts** — automatically shown based on shared tags
- **Scheduled posts** — set a future date, auto-publishes when the time comes
- **Comments** — built-in with moderation (approve/unapprove/delete), honeypot anti-spam
- **Reactions** — like/dislike with fingerprint deduplication
- **Page views** — unique visitor tracking (deduplicated by IP per day)
- **Newsletter** — compose in Markdown, send to subscribers via SMTP, schedule for future delivery, subscriber management
- **Admin dashboard** — stats, paginated post/comment management, per-page filters
- **Live preview editor** — side-by-side markdown + rendered preview with 500ms debounce
- **Image optimization** — auto-resize uploaded images > 1920px wide
- **OG image auto-generation** — dynamic SVG Open Graph images for social sharing
- **Avatar upload** — upload author profile image from admin settings
- **Security** — CSRF, bcrypt passwords, TOTP 2FA, WebAuthn passkeys, IP rate limiting, IP lockout
- **Audit trail** — persistent admin action log viewable in Settings
- **SSRF protection** — all outbound HTTP clients validated against private IP ranges
- **HTML sanitization** — untrusted content sanitized on ingestion (ActivityPub, Webmention, Micropub)
- **IndieWeb toggles** — enable/disable ActivityPub, Webmention, IndieAuth, Micropub independently from admin settings
- **Unsplash integration** — search and use free images from the editor
- **RSS feed** — auto-generated at `/rss.xml`
- **SEO optimized** — sitemap, Open Graph, Twitter Cards, JSON-LD, canonical URLs
- **Cookie consent** — minimal GDPR-friendly banner
- **Health check** — `/healthz` endpoint for Docker/Kubernetes
- **Favicon** — custom from `/static/` or default emoji SVG
- **Hot-reload** — file changes detected and applied without restart
- **Ghost import** — migrate from Ghost with one command
- **WordPress import** — migrate from WordPress WXR exports
- **Read-only container** — distroless, non-root, all writes to volumes
- **Migration = copy two directories** — `scp` content + config, done
- **Table of contents** — opt-in per post via `toc: true` frontmatter, auto-generated from headings
- **Post series** — `series` + `series_order` frontmatter for multi-part guides with navigation
- **Lazy loading** — `loading="lazy"` auto-added to all images for faster page loads
- **Reading progress bar** — visual scroll indicator on post pages
- **Image galleries** — click-to-zoom lightbox on post images
- **Draft sharing** — secret preview links for unpublished posts (7-day expiry)
- **Revision history** — automatic versioning on every edit with one-click restore
- **Bulk operations** — select multiple posts/comments for batch publish/draft/delete
- **Export** — one-click ZIP download of all content + config from admin
- **Webhooks** — fire HTTP callbacks on post/comment/subscriber events with HMAC signatures
- **Prometheus metrics** — `/metrics` endpoint (admin-authenticated) for monitoring (views, posts, comments, followers)
- **ETags** — conditional responses with `304 Not Modified` for bandwidth savings
- **Themed error pages** — 500 errors render through the active theme (not raw text)
- **i18n** — `locale` setting with built-in translations (en/es/fr/de/ja/zh)
- **ActivityPub** — Fediverse integration: followers, outbox, replies as webmentions
- **Webmention** — send and receive webmentions per W3C spec
- **IndieAuth** — authorization + token endpoints with PKCE support
- **Micropub** — create, update, and delete posts from any Micropub client; media endpoint for file uploads
- **Micropub queries** — supports `?q=config`, `?q=source`, `?q=category`, `?q=syndicate-to`
- **Newsletter analytics** — open/click tracking, per-newsletter stats, A/B subject testing
- **Built-in HTTPS** — auto-TLS via Let's Encrypt with `BLOG_TLS_DOMAIN` env var
- **Response caching** — in-memory LRU page cache with automatic invalidation
- **WebP conversion** — auto-generate WebP for uploads, content negotiation for media
- **API tokens** — `sk_`-prefixed app passwords for headless CMS and API access
- **Plugin system** — hook-based registry for post lifecycle, custom routes, template functions
- **Integration tests** — httptest-based coverage for all major endpoints
- **Database migrations** — versioned schema with `blog migrate status/rollback` CLI
- **CI/CD pipeline** — GitHub Actions: test, vet, govulncheck, multi-arch Docker build + push to GHCR
- **SMTP preflight** — verify SMTP connection before newsletter send loop
- **Newsletter retry** — per-subscriber send tracking, retry only failed recipients
- **SSRF protection** — blocks outbound requests to private/reserved IPs, validates DNS and redirects
- **Security regression tests** — dedicated test suite covering XSS, SSRF, CSRF, auth, and data safety

> **📖 New to Skriva?** See the [Getting Started Guide](GETTING_STARTED.md) for step-by-step instructions on running locally, deploying to Azure Container Apps, or self-hosting with Cloudflare Tunnels.

## Quick Start

### Docker (recommended)

```bash
# Create data directories
mkdir -p data/content/posts data/content/themes data/config

# Create minimal config
cat > data/config/site.yaml << 'EOF'
title: "My Blog"
tagline: "A personal blog"
base_url: "http://localhost:8080"
theme: "classic"
posts_per_page: 10
author:
  name: "Your Name"
EOF

# Run
docker run -d --name blog \
  --read-only \
  -v $(pwd)/data/content:/data/content \
  -v $(pwd)/data/config:/data/config \
  -p 8080:8080 \
  ghcr.io/digvijay/skriva:latest
```

### Docker Compose

```bash
docker compose up -d
```

### From Source

```bash
go build -o blog ./cmd/blog
./blog serve --content ./data/content --config ./data/config
```

## Directory Structure

```
data/
├── content/                    # Volume 1: Content
│   ├── posts/
│   │   └── my-first-post/
│   │       ├── index.md        # Post with YAML frontmatter
│   │       └── media/          # Post-specific images/video
│   ├── pages/
│   │   └── about/
│   │       └── index.md
│   ├── themes/
│   │   └── classic/            # Theme override (optional)
│   └── static/                 # Global static files
└── config/                     # Volume 2: Config
    ├── site.yaml               # Site settings
    ├── secrets.yaml            # Admin password, API keys
    ├── smtp.yaml               # Email settings (optional)
    └── blog.db                 # SQLite database (auto-created)
```

## Post Format

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
---

Your markdown content here...
```

## Admin Dashboard

1. Set an admin password: create `data/config/secrets.yaml` with a bcrypt hash
2. Navigate to `/admin/`
3. Log in with your password
4. Create, edit, and delete posts; manage comments; view stats

## Ghost Migration

```bash
blog import ghost \
  --export /path/to/ghost-export.json \
  --images /path/to/ghost/content/images
```

This preserves slugs, dates, tags, featured status, and downloads/copies all images.

## Environment Variables

| Variable           | Description                     | Default         |
| ------------------ | ------------------------------- | --------------- |
| `BLOG_PORT`        | HTTP listen port                | `8080`          |
| `BLOG_CONTENT_DIR` | Content directory               | `/data/content` |
| `BLOG_CONFIG_DIR`  | Config directory                | `/data/config`  |
| `BLOG_LOG_LEVEL`   | Log level                       | `info`          |
| `BLOG_TLS_DOMAIN`  | Enable auto-TLS for this domain | _(disabled)_    |

## Performance

- Cold start: < 500ms
- Page render: < 10ms
- Memory: < 30MB idle
- Container image: < 30MB
- Lighthouse: > 95 all categories

## Tech Stack

| Component | Choice                                    |
| --------- | ----------------------------------------- |
| Language  | Go 1.25+                                  |
| Database  | SQLite via `modernc.org/sqlite` (pure Go) |
| Markdown  | goldmark + GFM + Chroma syntax            |
| Templates | `html/template` (stdlib)                  |
| Auth      | bcrypt + TOTP + WebAuthn passkeys         |
| Config    | YAML                                      |
| QR Codes  | `github.com/skip2/go-qrcode`              |
| Container | `gcr.io/distroless/static:nonroot`        |

## How Skriva Compares

|                | Skriva                                      | Ghost            | Hugo             | Bear Blog | jlelse/GoBlog        |
| -------------- | ------------------------------------------- | ---------------- | ---------------- | --------- | -------------------- |
| **Type**       | Dynamic, self-hosted                        | Dynamic          | Static generator | SaaS      | Dynamic, self-hosted |
| **Binary**     | 26MB                                        | ~400MB (Node)    | 90MB             | N/A       | ~30MB                |
| **Database**   | SQLite (embedded)                           | MySQL/PostgreSQL | None             | Managed   | SQLite               |
| **Themes**     | 4 + custom                                  | 30+              | 1000+            | 1         | None                 |
| **Admin UI**   | Full                                        | Excellent        | None             | Minimal   | Basic                |
| **Comments**   | Built-in + moderation                       | Integrations     | None             | No        | Webmention           |
| **Search**     | Built-in                                    | Built-in         | JS plugin        | No        | FTS5                 |
| **2FA**        | TOTP + Passkeys                             | TOTP             | N/A              | No        | TOTP + Passkeys      |
| **Newsletter** | Built-in + analytics                        | Paid feature     | No               | No        | No                   |
| **IndieWeb**   | Full (AP+WM+IA+MP)                          | —                | —                | —         | Full (AP+WM+IA+MP)   |
| **Plugins**    | Hook-based + webhooks                       | REST API         | Go modules       | 60,000+   | Limited              |
| **Security**   | TOTP+Passkeys+SSRF+CSP+audit log+IP lockout | TOTP             | N/A              | Plugins   | TOTP+Passkeys        |
| **Deployment** | `docker run` + auto-TLS                     | Docker + DB      | Build + CDN      | SaaS      | Docker               |
| **Migrations** | Versioned + rollback                        | Knex             | N/A              | PHP       | Manual               |
| **LOC**        | ~15,000                                     | ~100K+           | ~80K             | —         | ~30K                 |

**Skriva's niche**: Simpler than Ghost, more dynamic than Hugo, more features than Bear, better admin UX than jlelse/GoBlog — at 15K lines of Go you can understand every line.

## Roadmap

### Extensibility

- [x] **Webhook support** — fire HTTP callbacks on post/comment events
- [x] **API tokens** — app passwords for headless CMS / Micropub usage
- [x] **Plugin system** — hook-based plugin registry (post lifecycle, custom routes, template functions)
- [ ] **Custom template functions** — let themes register their own helpers

### Ecosystem

- [x] **WordPress importer** — WXR XML to markdown
- [x] **Export** — one-click ZIP download from admin
- [ ] **Theme marketplace** — curated gallery at skriva.dev/themes
- [ ] **Importers** — Medium, Substack, Hugo, Jekyll
- [ ] **CLI tools** — `skriva new post`, `skriva check-links`

### IndieWeb & Fediverse

- [x] **ActivityPub** — publish to Mastodon, receive replies as comments
- [x] **Webmention** — send and receive webmentions
- [x] **IndieAuth** — use your blog as your identity
- [x] **Micropub** — create posts via API from any Micropub client

### Production Hardening

- [x] **Structured error pages** — themed 500/503 pages
- [x] **Prometheus metrics** — `/metrics` endpoint for monitoring
- [x] **Integration tests** — httptest-based tests for all major endpoints
- [x] **Built-in HTTPS** — Let's Encrypt / ACME auto-TLS via `BLOG_TLS_DOMAIN`
- [x] **Database migration versioning** — `schema_migrations` table, `blog migrate status/rollback` CLI
- [x] **CI/CD pipeline** — GitHub Actions: test, vet, govulncheck, multi-arch Docker build + push to GHCR
- [ ] **Fuzz testing** — frontmatter parser, markdown renderer, search

### Content Features

- [x] **Post series** — "Part 1 of 3" with prev/next navigation
- [x] **Table of contents** — opt-in per post, auto-generated from headings
- [x] **Reading progress bar** — visual scroll indicator on long posts
- [x] **Image galleries** — lightbox-style image viewing
- [x] **Draft sharing** — secret preview links for unpublished posts
- [x] **Revision history** — track post edits with one-click restore
- [x] **i18n** — multi-language support (en/es/fr/de/ja/zh)
- [x] **Lazy loading** — native `loading="lazy"` on all images

### Performance

- [x] **ETags** — conditional responses with 304 Not Modified
- [x] **Response caching** — in-memory LRU page cache with invalidation on content reload
- [x] **Image WebP conversion** — auto-generate WebP alongside uploads (when cwebp available), content negotiation in media handler

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, coding standards, and PR guidelines.

## License

[MIT](LICENSE)
