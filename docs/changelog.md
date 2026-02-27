---
outline: deep
---

# Changelog

All notable changes to Skriva will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] — 2026-02-27 {#v0-1-0}

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
- Image galleries with lightbox, image optimization, WebP conversion
- Draft sharing with secret preview links (7-day expiry)
- Revision history with one-click restore

**Admin**

- Full admin dashboard with stats, post/comment management
- Live-preview Markdown editor
- Comment moderation, reactions, page view tracking
- Bulk operations, ZIP export, Unsplash integration

**Newsletter**

- Compose in Markdown, send or schedule, A/B testing
- Open/click analytics, per-subscriber retry
- SMTP preflight verification

**Security** _(3 rounds of red-team auditing — 27 vulnerabilities found and fixed)_

- bcrypt + TOTP 2FA + WebAuthn passkeys
- CSRF, IP lockout, persistent audit trail
- Multi-pass HTML sanitizer for untrusted content
- SSRF protection (DNS resolution + redirect validation)
- 39 dedicated security regression tests

**Fediverse & IndieWeb**

- ActivityPub (Webfinger, Actor, Inbox/Outbox, HTTP Signatures)
- Webmention (send + receive, W3C spec)
- IndieAuth (mandatory PKCE S256)
- Micropub (CRUD + media + queries)
- Independent enable/disable toggles

**Operations**

- Distroless Docker image (read-only, non-root)
- Auto-TLS via Let's Encrypt
- Prometheus metrics, health checks, versioned DB migrations
- Ghost + WordPress importers
- Webhooks, API tokens, plugin system
- CI/CD pipeline with GitHub Actions

See the [full changelog on GitHub](https://github.com/Digvijay/skriva/blob/main/CHANGELOG.md).
