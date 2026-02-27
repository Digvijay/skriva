# What is Skriva?

**Skriva** (Swedish for "to write") is a lightweight, single-binary personal blog engine written in Go. It's designed for developers who want a full-featured blog with modern web standards, zero operational complexity, and complete ownership of their stack.

## Core Principles

- **Single binary** — The Go binary embeds default themes, admin UI, and all assets
- **Two volumes only** — `/data/content` (posts, pages, media, themes) and `/data/config` (settings, secrets, database)
- **Read-only container** — The container filesystem is read-only; all writes go to the two mapped volumes
- **No external dependencies** — No CDN, no external JS/CSS, no tracking. Everything is self-contained
- **Hot-reload** — File changes in content/config are detected and applied without restart
- **Migration = copy two directories** — `scp` the two volumes to a new server, run the container, done

## Features at a Glance

| Category       | Features                                                                                   |
| -------------- | ------------------------------------------------------------------------------------------ |
| **Content**    | Markdown + YAML frontmatter, GFM tables, syntax highlighting, TOC, series, scheduled posts |
| **Admin**      | Dashboard with stats, live-preview editor, comment moderation, bulk operations             |
| **Newsletter** | Compose in Markdown, schedule, A/B testing, open/click analytics, SMTP sending             |
| **Security**   | bcrypt + TOTP + WebAuthn, CSRF, rate limiting, IP lockout, SSRF protection, audit trail    |
| **IndieWeb**   | ActivityPub, Webmention, IndieAuth (PKCE), Micropub (CRUD + media)                         |
| **Themes**     | 4 bundled (Classic, Newsletter, GitHub, Editorial), custom theme support, dark mode        |
| **SEO**        | Sitemap, RSS, Open Graph, Twitter Cards, JSON-LD, canonical URLs, auto OG images           |
| **Ops**        | Docker, auto-TLS, Prometheus metrics, health checks, versioned DB migrations               |

## Tech Stack

| Component   | Choice                          | Rationale                                 |
| ----------- | ------------------------------- | ----------------------------------------- |
| Language    | Go 1.25+                        | Single binary, fast, stdlib HTTP          |
| HTTP Server | `net/http` (stdlib)             | No framework dependency                   |
| Database    | SQLite via `modernc.org/sqlite` | No CGO, embedded, portable                |
| Markdown    | goldmark + Chroma               | CommonMark compliant, syntax highlighting |
| Templating  | `html/template` (stdlib)        | Secure by default                         |
| Config      | YAML                            | Human-readable                            |
| Container   | `gcr.io/distroless/static`      | Minimal attack surface                    |

## Performance

| Metric          | Target              |
| --------------- | ------------------- |
| Cold start      | < 500ms             |
| Page render     | < 10ms              |
| Memory (idle)   | < 30MB              |
| Container image | < 30MB              |
| Lighthouse      | > 95 all categories |
