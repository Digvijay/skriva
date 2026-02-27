# Changelog

All notable changes to Skriva will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] — 2026-02-27

### 🎉 Initial Release

The first public release of Skriva — a lightweight, single-binary personal blog engine written in Go.

### Added

**Core**

- Single Go binary (~26MB) with embedded themes, admin UI, and all assets
- Two-volume architecture: `/data/content` + `/data/config`
- 4 bundled themes: Classic, Newsletter, GitHub, Editorial (all with light/dark/auto)
- Markdown posts with YAML frontmatter, GFM tables, syntax highlighting (Chroma)
- Full-text search, tags, related posts, scheduled posts, post series
- Table of contents (opt-in per post), reading progress bar, lazy loading
- Image galleries with lightbox, image optimization (auto-resize >1920px), WebP conversion
- Draft sharing with secret preview links (7-day expiry)
- Revision history with one-click restore
- Static pages support

**Admin**

- Full admin dashboard with stats, post/comment management
- Live-preview Markdown editor with 500ms debounce
- Comment moderation (approve/unapprove/delete), honeypot anti-spam
- Reactions (like/dislike) with fingerprint deduplication
- Page view tracking (deduplicated by IP per day)
- Bulk operations for posts and comments
- One-click ZIP export of all content + config
- Unsplash image search integration
- Avatar upload

**Newsletter**

- Compose in Markdown with live preview
- Send immediately or schedule for future delivery
- A/B subject line testing
- Open and click tracking analytics
- Per-subscriber send tracking with retry for failed deliveries
- SMTP preflight verification
- Subscriber management with double opt-in

**Security** _(hardened through 3 rounds of red-team auditing — 27 vulnerabilities found and fixed)_

- bcrypt password hashing
- TOTP 2FA with QR code provisioning and single-use code tracking
- WebAuthn passkey authentication (challenge-matched sessions)
- CSRF protection on all state-changing endpoints
- IP lockout: 10 failed login attempts → 15-minute ban
- Persistent audit trail (14-day retention, 24-hour deletion floor)
- `sanitizeUntrustedHTML()` multi-pass sanitizer for all external content
- `safeHTTPClient()` with DNS resolution + redirect validation on all outbound HTTP
- `validateExternalURL()` blocks private IPs, cloud metadata, resolves hostnames
- `hasScope()` exact word matching for IndieAuth/API token scopes
- HSTS (2 years + preload), CSP, X-Frame-Options, Referrer-Policy
- Rate limiting: login (5/min), comments (3/min), AP inbox (10/min), webmention (10/min)
- Session tokens: HMAC-SHA256, HttpOnly + Secure + SameSite=Strict
- Password change rotates `session_secret` (instant token revocation)
- 39 dedicated security regression tests

**Fediverse & IndieWeb**

- ActivityPub: Webfinger, Actor, Inbox (Follow/Undo/Create), Outbox, Followers
- HTTP Signatures (sign + verify with digest enforcement)
- Webmention: send and receive per W3C spec
- IndieAuth: authorization + token endpoints with mandatory PKCE S256
- Micropub: create, update, delete posts + media upload endpoint
- Micropub queries: `?q=config`, `?q=source`, `?q=category`, `?q=syndicate-to`
- Independent enable/disable toggles for all 4 protocols
- Follower notification on new posts
- Outgoing webmentions on publish

**Operations**

- Docker image based on `gcr.io/distroless/static:nonroot` (read-only, non-root)
- Auto-TLS via Let's Encrypt with `BLOG_TLS_DOMAIN` env var
- Prometheus metrics at `/metrics` (admin-authenticated)
- Health check at `/healthz` (includes version)
- Versioned database migrations with `blog migrate status/rollback` CLI
- Hot-reload of content and config files via fsnotify
- RSS feed, sitemap, robots.txt auto-generation
- SEO: Open Graph, Twitter Cards, JSON-LD, canonical URLs, auto OG images
- Cookie consent banner (GDPR)
- Themed error pages (500s render through active theme)
- i18n with 6 languages (en, es, fr, de, ja, zh)
- ETags and in-memory LRU page cache

**Ecosystem**

- Ghost JSON import (preserves slugs, dates, tags, images)
- WordPress WXR XML import
- Webhook support with HMAC signatures
- `sk_`-prefixed API tokens for headless CMS access
- Hook-based plugin system (post lifecycle, custom routes, template functions)
- GitHub Actions CI/CD pipeline (test, vet, govulncheck, multi-arch Docker build)
- VitePress documentation site at [skriva.digvijay.dev](https://skriva.digvijay.dev)

[0.1.0]: https://github.com/Digvijay/skriva/releases/tag/v0.1.0
