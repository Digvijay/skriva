---
layout: home

hero:
  name: Skriva
  text: Personal Blog Engine
  tagline: "v0.1.0 — A lightweight, single-binary blog engine written in Go. Self-hosted, secure, fediverse-native."
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: Changelog
      link: /changelog
    - theme: alt
      text: View on GitHub
      link: https://github.com/Digvijay/skriva

features:
  - icon: 🚀
    title: Single Binary
    details: ~26MB binary with embedded themes, admin UI, and all assets. No runtime dependencies, no CDN, no external JS/CSS.
  - icon: 🔒
    title: Hardened Security
    details: 3 rounds of red-team auditing. bcrypt + TOTP + WebAuthn passkeys, SSRF protection, IP lockout, persistent audit trail.
  - icon: 🌐
    title: Fediverse Native
    details: ActivityPub, Webmention, IndieAuth, and Micropub built-in. Your blog is a first-class fediverse citizen.
  - icon: 📬
    title: Built-in Newsletter
    details: Compose in Markdown, send via SMTP, schedule delivery, A/B subject testing, open/click analytics.
  - icon: 🎨
    title: 4 Bundled Themes
    details: Classic, Newsletter, GitHub, Editorial — all with light/dark/auto toggle. Create custom themes with Go templates.
  - icon: 📦
    title: Zero-Ops Migration
    details: Two directories. scp them to a new server, run the container, done. SQLite database included.
---

## How Skriva Compares

|                | Skriva               | Ghost            | Hugo             | WriteFreely  |
| -------------- | -------------------- | ---------------- | ---------------- | ------------ |
| **Type**       | Dynamic, self-hosted | Dynamic          | Static generator | Dynamic      |
| **Binary**     | 26MB                 | ~400MB (Node)    | 90MB             | ~30MB        |
| **Database**   | SQLite (embedded)    | MySQL/PostgreSQL | None             | MySQL/SQLite |
| **Admin UI**   | Full dashboard       | Excellent        | None             | Minimal      |
| **Newsletter** | Built-in + analytics | Paid feature     | No               | No           |
| **2FA**        | TOTP + Passkeys      | TOTP             | N/A              | No           |
| **IndieWeb**   | Full (AP+WM+IA+MP)   | Experimental AP  | —                | AP only      |
| **Deployment** | `docker run`         | Docker + DB      | Build + CDN      | Docker       |
