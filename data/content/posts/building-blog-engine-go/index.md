---
title: "Building a Blog Engine in Go"
slug: "building-blog-engine-go"
date: 2024-01-20T14:30:00Z
tags:
  - go
  - programming
  - docker
  - self-hosting
description: "Why I built a custom blog engine from scratch in Go, and what I learned along the way."
featured: false
draft: false
---

After years of using Ghost, WordPress, and various static site generators, I decided to build my own blog engine. Here's why.

## The Problem

Every blog platform comes with trade-offs:

| Platform    | Pros               | Cons                                    |
| ----------- | ------------------ | --------------------------------------- |
| WordPress   | Huge ecosystem     | Heavy, PHP, security nightmares         |
| Ghost       | Beautiful, Node.js | 1GB+ RAM, complex deployment            |
| Hugo/Jekyll | Fast, static       | No comments, no admin, rebuild required |
| Medium      | Zero maintenance   | No ownership, paywalls                  |

I wanted something that was:

1. **Lightweight** — Under 30MB container image
2. **Self-contained** — No external dependencies
3. **Portable** — Copy two folders to migrate
4. **Full-featured** — Comments, stats, admin dashboard
5. **Secure** — Read-only container, minimal attack surface

## The Solution: Go

Go was the perfect choice:

- **Single binary** — No runtime dependencies
- **Fast compilation** — Build in seconds
- **Great stdlib** — `net/http`, `html/template`, `embed`
- **Pure Go SQLite** — No CGO required with `modernc.org/sqlite`
- **Low memory** — 30MB idle vs Ghost's 300MB+

## Architecture

```
┌─────────────────────────────────┐
│         Docker Container         │
│  ┌───────────────────────────┐  │
│  │     Skriva Binary (~24MB)  │  │
│  │  ┌─────────┐ ┌─────────┐ │  │
│  │  │ Goldmark │ │ Chroma  │ │  │
│  │  │ (md→html)│ │(syntax) │ │  │
│  │  └─────────┘ └─────────┘ │  │
│  └───────────────────────────┘  │
│          │              │        │
│    ┌─────┴─────┐  ┌────┴────┐  │
│    │ /data/     │  │ /data/  │  │
│    │ content    │  │ config  │  │
│    └───────────┘  └─────────┘  │
└─────────────────────────────────┘
```

## Key Design Decisions

### Markdown Files as Posts

No database for content. Posts are Markdown files with YAML frontmatter, organized in directories:

```
content/posts/
├── hello-world/
│   ├── index.md
│   └── hero.jpg
├── building-blog-engine-go/
│   └── index.md
└── my-homelab/
    ├── index.md
    ├── rack-photo.jpg
    └── diagram.png
```

### SQLite for State

Comments, page views, and reactions go in SQLite — the only stateful data. WAL mode keeps reads fast during writes.

### Hot Reload

File changes trigger automatic reload via `fsnotify`. Edit a post, save, refresh — no restart needed.

## Performance

Early benchmarks are promising:

- **Cold start**: ~200ms
- **Page render**: ~3ms
- **Memory**: 25MB idle
- **Container image**: 18MB

## Lessons Learned

1. **Go's `embed.FS` is amazing** — Themes and admin UI baked into the binary
2. **Pure Go SQLite works** — `modernc.org/sqlite` is slower than CGO but perfectly adequate for a blog
3. **`html/template` is underrated** — Auto-escaping, template inheritance, custom functions
4. **Less is more** — Every dependency removed is a maintenance burden eliminated

The source code is on [GitHub](https://github.com/Digvijay/skriva). Give it a try!
