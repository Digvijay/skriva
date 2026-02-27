# How to Build a Skriva Theme — Step-by-Step Tutorial

This tutorial walks you through creating a complete Skriva theme from scratch. By the end, you'll have a working theme with light/dark mode, responsive layout, and all the features Skriva offers.

**Time required**: ~2 hours for a polished theme, ~30 minutes for a basic one.
**Prerequisites**: HTML, CSS. No Go, JavaScript frameworks, or build tools needed.

---

## Table of Contents

1. [Theme Structure](#1-theme-structure)
2. [Create `theme.yaml`](#2-create-themeyaml)
3. [Set Up Color Tokens](#3-set-up-color-tokens-themecss)
4. [Build `base.html`](#4-build-basehtml)
5. [Create Partials](#5-create-partials)
6. [Build Page Templates](#6-build-page-templates)
7. [Add Dark Mode](#7-add-dark-mode)
8. [Syntax Highlighting](#8-syntax-highlighting)
9. [Install & Activate](#9-install--activate)
10. [Available Template Data](#10-available-template-data)
11. [Tips & Best Practices](#11-tips--best-practices)

---

## 1. Theme Structure

Every theme lives in a single folder. Create this structure:

```
my-theme/
├── theme.yaml              # Required: theme metadata
├── css/
│   ├── theme.css           # Required: all your styles
│   └── syntax.css          # Optional: code highlighting overrides
└── templates/
    ├── base.html           # Required: HTML shell
    ├── home.html           # Required: homepage
    ├── post.html           # Required: single post
    ├── page.html           # Required: static page
    ├── tag.html            # Required: posts filtered by tag
    ├── tags.html           # Required: all tags listing
    ├── archive.html        # Required: chronological archive
    ├── search.html         # Required: search page
    ├── 404.html            # Required: not found page
    └── partials/
        ├── head.html       # Extra <head> content (JSON-LD)
        ├── nav.html        # Site header/navigation
        ├── post-card.html  # Post summary card (used in lists)
        ├── comments.html   # Comment section + form
        ├── reactions.html  # Like/dislike buttons
        ├── share.html      # Social share buttons
        └── footer.html     # Site footer
```

---

## 2. Create `theme.yaml`

```yaml
name: "My Theme"
description: "A clean, modern blog theme."
author: "Your Name"
version: "1.0.0"
layout: "single-column" # or "two-column" — shown in admin
```

---

## 3. Set Up Color Tokens (`theme.css`)

Skriva themes use CSS custom properties with `data-color-mode` for light/dark/auto switching:

```css
/* Light mode (default) */
:root,
[data-color-mode="light"] {
  --color-bg: #ffffff;
  --color-surface: #f8f9fa;
  --color-text: #1a1a2e;
  --color-muted: #6b7280;
  --color-primary: #2563eb;
  --color-border: #e5e7eb;
  --color-accent: #3b82f6;
  --font-sans:
    -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  --font-mono: "SF Mono", Consolas, monospace;
  --max-width: 1200px;
}

/* Dark mode */
[data-color-mode="dark"] {
  --color-bg: #0f172a;
  --color-surface: #1e293b;
  --color-text: #e2e8f0;
  --color-muted: #94a3b8;
  --color-primary: #60a5fa;
  --color-border: #334155;
  --color-accent: #818cf8;
}

/* Auto mode — follows OS preference */
@media (prefers-color-scheme: dark) {
  [data-color-mode="auto"] {
    --color-bg: #0f172a;
    --color-surface: #1e293b;
    --color-text: #e2e8f0;
    --color-muted: #94a3b8;
    --color-primary: #60a5fa;
    --color-border: #334155;
    --color-accent: #818cf8;
  }
}
```

**Key rule**: Use `var(--color-*)` everywhere. Never hardcode colors.

---

## 4. Build `base.html`

This is the HTML shell. Every page template renders inside it.

```html
{{define "base"}}<!DOCTYPE html>
<html lang="en" data-color-mode="auto">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{{.Meta.Title}}</title>
    {{if .Meta.Description}}
    <meta name="description" content="{{.Meta.Description}}" />
    {{end}} {{if .Meta.CanonicalURL}}
    <link rel="canonical" href="{{.Meta.CanonicalURL}}" />
    {{end}}

    <!-- Open Graph -->
    <meta property="og:title" content="{{.Meta.Title}}" />
    {{if .Meta.OGImage}}
    <meta property="og:image" content="{{.Meta.OGImage}}" />
    {{end}}

    <!-- RSS -->
    <link
      rel="alternate"
      type="application/rss+xml"
      title="RSS"
      href="/rss.xml"
    />

    <!-- Theme CSS -->
    <link rel="stylesheet" href="/theme/css/theme.css?v={{cacheVer}}" />
    <link rel="stylesheet" href="/theme/css/syntax.css?v={{cacheVer}}" />

    <!-- IndieWeb discovery -->
    <link rel="webmention" href="/webmention" />
    <link rel="micropub" href="/micropub" />
    <link rel="authorization_endpoint" href="/indieauth/auth" />
    <link rel="token_endpoint" href="/indieauth/token" />

    {{template "head" .}}
  </head>
  <body>
    {{template "nav" .}}
    <main class="main">{{block "content" .}}{{end}}</main>
    {{template "footer" .}}

    <!-- Dark/light mode toggle script -->
    <script>
      (function () {
        var html = document.documentElement;
        var saved = localStorage.getItem("color-mode");
        if (saved) html.setAttribute("data-color-mode", saved);
        window.toggleColorMode = function () {
          var current = html.getAttribute("data-color-mode");
          var next = current === "dark" ? "light" : "dark";
          html.setAttribute("data-color-mode", next);
          localStorage.setItem("color-mode", next);
        };
      })();
    </script>
  </body>
</html>
{{end}}
```

**Important points**:

- `{{define "base"}}...{{end}}` wraps everything
- `{{block "content" .}}{{end}}` is where page templates inject content
- `?v={{cacheVer}}` busts CSS cache hourly
- The dark mode script runs before body renders (no flash)

---

## 5. Create Partials

### `partials/head.html` — JSON-LD for SEO

```html
{{define "head"}} {{if .Post}}{{jsonLD .Meta}}{{end}} {{end}}
```

### `partials/nav.html` — Site header

```html
{{define "nav"}}
<header class="site-header">
  <div class="container">
    <a href="/" class="site-title">{{.Site.Title}}</a>
    <nav>
      <a href="/archive">{{i18n "archive"}}</a>
      <a href="/tags">{{i18n "tags"}}</a>
      <a href="/search">{{i18n "search"}}</a>
      <a href="/rss.xml">RSS</a>
      <button onclick="toggleColorMode()">🌓</button>
    </nav>
  </div>
</header>
{{end}}
```

### `partials/post-card.html` — Post summary

```html
{{define "post-card"}}
<article class="post-card">
  {{if .Image}}<img
    src="/media/{{.Slug}}/{{.Image}}"
    alt="{{.Title}}"
    loading="lazy"
  />{{end}}
  <div class="post-card-body">
    <h2><a href="{{.Permalink}}">{{.Title}}</a></h2>
    <div class="post-meta">
      <time>{{formatDate .Date "Jan 2, 2006"}}</time> · {{readingTime .}}
    </div>
    {{if .Description}}
    <p>{{truncate .Description 160}}</p>
    {{end}} {{if .Tags}}
    <div class="tags">
      {{range .Tags}}<a href="/tag/{{slugify .}}">{{.}}</a>{{end}}
    </div>
    {{end}}
  </div>
</article>
{{end}}
```

### `partials/comments.html` — Comment form with CSRF

```html
{{define "comments"}}
<section class="comments" id="comments">
  <h2>{{i18n "comments"}} ({{len .Comments}})</h2>

  {{range .Comments}}
  <div class="comment">
    <strong>{{.Author}}</strong>
    <time>{{formatDate .CreatedAt "Jan 2, 2006"}}</time>
    <p>{{.Content}}</p>
  </div>
  {{end}}

  <form action="/api/comment/{{.Post.Slug}}" method="POST">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <input type="text" name="author" placeholder="{{i18n "name"}}" required maxlength="100">
    <!-- Honeypot -->
    <div style="display:none"><input type="text" name="website" tabindex="-1"></div>
    <textarea name="content" placeholder="{{i18n "message"}}" required maxlength="5000"></textarea>
    <button type="submit">{{i18n "post_comment"}}</button>
  </form>
</section>
{{end}}
```

### `partials/reactions.html` — Like/dislike with CSRF

```html
{{define "reactions"}}
<div class="reactions">
  <button onclick="react('like','{{.Post.Slug}}')">
    👍 <span id="likes">{{if .Stats}}{{.Stats.Likes}}{{else}}0{{end}}</span>
  </button>
  <button onclick="react('dislike','{{.Post.Slug}}')">
    👎
    <span id="dislikes">{{if .Stats}}{{.Stats.Dislikes}}{{else}}0{{end}}</span>
  </button>
</div>
<script>
  async function react(type, slug) {
    var res = await fetch("/api/reaction/" + slug, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-CSRF-Token": "{{.CSRFToken}}",
      },
      body: JSON.stringify({ type: type }),
    });
    if (res.ok) {
      var d = await res.json();
      document.getElementById("likes").textContent = d.likes;
      document.getElementById("dislikes").textContent = d.dislikes;
    }
  }
</script>
{{end}}
```

### `partials/share.html` — Social sharing + Mastodon

```html
{{define "share"}}
<div class="share-buttons">
  <span>{{i18n "share"}}:</span>
  <a
    href="https://twitter.com/intent/tweet?url={{.Meta.CanonicalURL}}&text={{.Post.Title}}"
    target="_blank"
    >Twitter</a
  >
  <a
    href="https://www.linkedin.com/sharing/share-offsite/?url={{.Meta.CanonicalURL}}"
    target="_blank"
    >LinkedIn</a
  >
  <a
    href="https://bsky.app/intent/compose?text={{.Post.Title}}%20{{.Meta.CanonicalURL}}"
    target="_blank"
    >Bluesky</a
  >
  <button
    onclick="var i=prompt('Your Mastodon instance:');if(i)window.open('https://'+i.replace(/^https?:\/\//,'').replace(/\/$/,'')+'/share?text='+encodeURIComponent('{{.Post.Title}} {{.Meta.CanonicalURL}}'))"
  >
    Mastodon
  </button>
</div>
{{end}}
```

### `partials/footer.html` — Footer + subscribe + fedi address

```html
{{define "footer"}}
<footer class="site-footer">
  <div class="container">
    <div class="footer-grid">
      {{range .Site.Footer.Sections}}
      <div>
        <h3>{{.Title}}</h3>
        <ul>{{range .Links}}<li><a href="{{.URL}}">{{.Label}}</a></li>{{end}}</ul>
      </div>
      {{end}}
      <div>
        <h3>{{i18n "subscribe"}}</h3>
        <form action="/api/subscribe" method="POST">
          <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
          <input type="email" name="email" placeholder="{{i18n "your_email"}}" required>
          <button type="submit">{{i18n "subscribe"}}</button>
        </form>
      </div>
    </div>
    {{if .FediAddress}}<p class="fedi-address">Fediverse: <code>{{.FediAddress}}</code></p>{{end}}
    <p>&copy; {{currentYear}} {{.Site.Author.Name}}. {{i18n "powered_by"}} <a href="https://github.com/Digvijay/skriva">Skriva</a>.</p>
  </div>
</footer>
{{end}}
```

---

## 6. Build Page Templates

Each page template calls `{{template "base" .}}` and defines a `content` block.

### `home.html`

```html
{{template "base" .}} {{define "content"}}
<div class="container">
  {{if .Posts}}
  <div class="posts-grid">
    {{range .Posts}}{{template "post-card" .}}{{end}}
  </div>
  {{if .Pagination}}
  <nav class="pagination">
    {{if .Pagination.HasPrev}}<a href="{{.Pagination.PrevURL}}"
      >← {{i18n "newer"}}</a
    >{{end}}
    <span>{{.Pagination.CurrentPage}} / {{.Pagination.TotalPages}}</span>
    {{if .Pagination.HasNext}}<a href="{{.Pagination.NextURL}}"
      >{{i18n "older"}} →</a
    >{{end}}
  </nav>
  {{end}} {{else}}
  <p>No posts yet.</p>
  {{end}}
</div>
{{end}}
```

### `post.html`

```html
{{template "base" .}}
{{define "content"}}
<article class="post container">
  <header>
    <h1>{{.Post.Title}}</h1>
    <div class="post-meta">
      <time>{{formatDate .Post.Date "January 2, 2006"}}</time> · {{readingTime .Post}}
      {{range .Post.Tags}} <a href="/tag/{{slugify .}}">{{.}}</a>{{end}}
    </div>
  </header>

  {{if .Post.Image}}
  <img src="/media/{{.Post.Slug}}/{{.Post.Image}}" alt="{{.Post.Title}}" class="hero-image">
  {{end}}

  {{if .SeriesPosts}}
  <nav class="series-nav">
    <strong>Series: {{.Post.Series}}</strong>
    <ol>{{range .SeriesPosts}}<li{{if eq .Slug $.Post.Slug}} class="current"{{end}}><a href="{{.Permalink}}">{{.Title}}</a></li>{{end}}</ol>
  </nav>
  {{end}}

  {{if .TableOfContents}}
  <nav class="toc">
    <strong>Table of Contents</strong>
    <ul>{{range .TableOfContents}}<li class="toc-h{{.Level}}"><a href="#{{.ID}}">{{.Text}}</a></li>{{end}}</ul>
  </nav>
  {{end}}

  <div class="post-content">{{safeHTML .Post.HTML}}</div>

  {{template "share" .}}
  {{template "reactions" .}}

  {{if .RelatedPosts}}
  <section class="related">
    <h2>{{i18n "related_posts"}}</h2>
    <div class="posts-grid">{{range .RelatedPosts}}{{template "post-card" .}}{{end}}</div>
  </section>
  {{end}}

  {{template "comments" .}}

  {{if .Webmentions}}
  <section class="webmentions">
    <h2>Mentions from the Web</h2>
    {{range .Webmentions}}
    <div class="webmention">
      <strong>{{.AuthorName}}</strong> <span class="type">{{.MentionType}}</span>
      {{if .Content}}<p>{{truncate .Content 300}}</p>{{end}}
    </div>
    {{end}}
  </section>
  {{end}}
</article>
{{end}}
```

### Other pages (`tag.html`, `tags.html`, `search.html`, `archive.html`, `page.html`, `404.html`)

These follow the same pattern — `{{template "base" .}}` + `{{define "content"}}`. See the bundled `classic` theme for working examples.

---

## 7. Add Dark Mode

Already done! The CSS tokens in Step 3 + the script in `base.html` handle everything. Just make sure:

1. **All colors** use `var(--color-*)` — never hardcode
2. **Code blocks**: Use `:not([style])` to avoid overriding Chroma's inline styles:
   ```css
   pre:not([style]) {
     background: var(--color-surface);
   }
   ```
3. The toggle button calls `toggleColorMode()` (defined in base.html)

---

## 8. Syntax Highlighting

Create `css/syntax.css` to style code blocks. Skriva uses Chroma (Dracula theme) with inline styles, so your CSS only needs to handle the `<pre>` container:

```css
.post-content pre {
  border-radius: 8px;
  padding: 1rem;
  overflow-x: auto;
  font-size: 0.9rem;
  line-height: 1.6;
}
.post-content pre:not([style]) {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
}
.post-content code {
  font-family: var(--font-mono);
}
.post-content :not(pre) > code {
  background: var(--color-surface);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 0.85em;
}
```

---

## 9. Install & Activate

### Option A: User-installed theme

Copy your theme folder to `{contentDir}/themes/my-theme/`.

### Option B: Bundled theme

Place it in `themes/my-theme/` at the repo root (it gets embedded in the binary).

Then either:

- **Admin UI**: Go to Settings → Themes → click "Activate"
- **Config**: Set `theme: "my-theme"` in `data/config/site.yaml`

---

## 10. Available Template Data

### Every page gets:

| Field          | Type       | Description                                  |
| -------------- | ---------- | -------------------------------------------- |
| `.Site`        | SiteConfig | Title, tagline, author, social links, footer |
| `.Meta`        | SEOMeta    | Title, description, canonical URL, OG image  |
| `.Nav`         | NavData    | Current path, active section                 |
| `.Tags`        | []TagInfo  | All tags with post counts                    |
| `.CSRFToken`   | string     | For forms                                    |
| `.Year`        | int        | Current year                                 |
| `.FediAddress` | string     | e.g. `@blog@yourdomain.com`                  |
| `.Error`       | string     | Error message (404/500 pages)                |

### Post pages also get:

| Field              | Type         | Description                                    |
| ------------------ | ------------ | ---------------------------------------------- |
| `.Post`            | \*Post       | The post (Title, Slug, Date, Tags, HTML, etc.) |
| `.Stats`           | \*PostStats  | Views, likes, dislikes                         |
| `.Comments`        | []Comment    | Approved comments                              |
| `.RelatedPosts`    | []Post       | Up to 3 related posts                          |
| `.Webmentions`     | []Webmention | Received webmentions                           |
| `.TableOfContents` | []TOCEntry   | Headings (when `toc: true`)                    |
| `.SeriesPosts`     | []Post       | All posts in the same series                   |

### Template functions:

| Function      | Example                                   |
| ------------- | ----------------------------------------- |
| `formatDate`  | `{{formatDate .Post.Date "Jan 2, 2006"}}` |
| `readingTime` | `{{readingTime .Post}}` → "3 min read"    |
| `truncate`    | `{{truncate .Description 160}}`           |
| `safeHTML`    | `{{safeHTML .Post.HTML}}`                 |
| `slugify`     | `{{slugify .Tag.Name}}`                   |
| `i18n`        | `{{i18n "subscribe"}}` → locale-aware     |
| `cacheVer`    | `{{cacheVer}}` → hourly cache buster      |
| `jsonLD`      | `{{jsonLD .Meta}}` → JSON-LD script tag   |
| `currentYear` | `{{currentYear}}` → 2026                  |

---

## 11. Tips & Best Practices

1. **Start by copying** — clone the `classic` theme and modify. It's the simplest.
2. **Test both modes** — always check light AND dark mode.
3. **Mobile first** — use `@media (min-width: 768px)` for desktop styles.
4. **Respect inline styles** — Chroma syntax highlighting uses inline `style=`. Don't override `pre[style]`.
5. **Use `loading="lazy"`** — Skriva adds this automatically, but put it on your `<img>` tags in templates too.
6. **CSRF on all forms** — always include `<input type="hidden" name="csrf_token" value="{{.CSRFToken}}">`.
7. **Honeypot for comments** — include a hidden field `name="website"` to catch bots.
8. **Test with content** — create posts with images, tags, series, ToC, and code blocks to see everything working.

---

That's it! Your theme is ready. For a complete working example, see the `editorial` theme in `themes/editorial/`.
