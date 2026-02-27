# Skriva Theme Development Guide

This guide explains how to create a custom theme for Skriva, step by step.

## Theme Structure

Every theme lives in a folder under `themes/` (embedded in binary) or `{contentDir}/themes/` (user-installed, takes priority over bundled).

```
themes/my-theme/
├── theme.yaml              # Theme metadata
├── css/
│   ├── theme.css           # Main stylesheet
│   └── syntax.css          # Syntax highlighting overrides (optional)
└── templates/
    ├── base.html           # HTML shell (head, body, nav, footer)
    ├── home.html           # Homepage with paginated posts
    ├── post.html           # Single post page
    ├── page.html           # Static page
    ├── tag.html            # Posts filtered by tag
    ├── tags.html           # All tags index page
    ├── archive.html        # Chronological archive
    ├── search.html         # Search page
    ├── 404.html            # Not found page
    └── partials/
        ├── head.html       # Extra <head> content (JSON-LD, etc.)
        ├── nav.html        # Site header/navigation
        ├── post-card.html  # Post summary card (used in lists)
        ├── comments.html   # Comment section with form
        ├── reactions.html  # Like/dislike buttons
        ├── share.html      # Social share buttons
        └── footer.html     # Site footer + cookie consent
```

## Step 1: Create `theme.yaml`

```yaml
name: "My Theme"
description: "A brief description of your theme."
author: "Your Name"
version: "1.0.0"
layout: "single-column" # or "two-column", for display in admin
```

## Step 2: Create `base.html`

This is the HTML shell. Every page template calls `{{template "base" .}}` to render inside it.

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
    {{if .Meta.Description}}
    <meta property="og:description" content="{{.Meta.Description}}" />
    {{end}}
    <meta property="og:type" content="{{.Meta.OGType}}" />
    {{if .Meta.CanonicalURL}}
    <meta property="og:url" content="{{.Meta.CanonicalURL}}" />
    {{end}} {{if .Meta.OGImage}}
    <meta property="og:image" content="{{.Meta.OGImage}}" />
    {{end}}

    <!-- Twitter Card -->
    <meta name="twitter:card" content="summary_large_image" />
    <meta name="twitter:title" content="{{.Meta.Title}}" />

    <link
      rel="alternate"
      type="application/rss+xml"
      title="{{.Site.Title}} RSS Feed"
      href="/rss.xml"
    />
    <link rel="stylesheet" href="/theme/css/theme.css?v={{cacheVer}}" />
    <link rel="stylesheet" href="/theme/css/syntax.css?v={{cacheVer}}" />
    {{template "head" .}}
  </head>
  <body>
    {{template "nav" .}}
    <main class="main">{{block "content" .}}{{end}}</main>
    {{template "footer" .}}

    <!-- Dark/light mode toggle script -->
    <script>
      (function () {
        const html = document.documentElement;
        const saved = localStorage.getItem("color-mode");
        if (saved) html.setAttribute("data-color-mode", saved);
        else if (window.matchMedia("(prefers-color-scheme: dark)").matches)
          html.setAttribute("data-color-mode", "dark");
        window.toggleColorMode = function () {
          const current = html.getAttribute("data-color-mode");
          const next = current === "dark" ? "light" : "dark";
          html.setAttribute("data-color-mode", next);
          localStorage.setItem("color-mode", next);
          document
            .querySelectorAll(".theme-toggle-icon")
            .forEach((el) => (el.textContent = next === "dark" ? "☀️" : "🌙"));
        };
        document.addEventListener("DOMContentLoaded", function () {
          const mode = html.getAttribute("data-color-mode");
          document
            .querySelectorAll(".theme-toggle-icon")
            .forEach((el) => (el.textContent = mode === "dark" ? "☀️" : "🌙"));
        });
      })();
    </script>
  </body>
</html>
{{end}}
```

Key points:

- `{{define "base"}}...{{end}}` wraps the entire template
- `{{block "content" .}}{{end}}` is where page templates inject their content
- `{{template "nav" .}}` and `{{template "footer" .}}` include partials
- `{{template "head" .}}` includes extra head content (JSON-LD on post pages)
- `?v={{cacheVer}}` busts the browser CSS cache hourly

## Step 3: Create Page Templates

Each page template overrides the `content` block. Example `home.html`:

```html
{{template "base" .}} {{define "content"}}
<div class="container">
  {{if .Posts}}
  <section class="posts-list">
    {{range .Posts}} {{template "post-card" .}} {{end}}
  </section>

  {{if .Pagination}}
  <nav class="pagination">
    {{if .Pagination.HasPrev}}<a href="{{.Pagination.PrevURL}}">← Newer</a
    >{{end}} {{if .Pagination.HasNext}}<a href="{{.Pagination.NextURL}}"
      >Older →</a
    >{{end}}
  </nav>
  {{end}} {{else}}
  <p>No posts yet.</p>
  {{end}}
</div>
{{end}}
```

## Step 4: Template Data Available

Every template receives a `TemplateData` struct:

| Field              | Type           | Available In             | Description                                  |
| ------------------ | -------------- | ------------------------ | -------------------------------------------- |
| `.Site`            | `SiteConfig`   | All pages                | Title, tagline, author, social links, footer |
| `.Meta`            | `SEOMeta`      | All pages                | Title, description, canonical URL, OG data   |
| `.Nav`             | `NavData`      | All pages                | Current path, active section name            |
| `.Post`            | `*Post`        | `post.html`              | Single post with all fields                  |
| `.Posts`           | `[]Post`       | `home`, `tag`, `archive` | List of posts                                |
| `.Page`            | `*Page`        | `page.html`              | Static page                                  |
| `.Tags`            | `[]TagInfo`    | All pages                | All tags with post counts                    |
| `.Tag`             | `*TagInfo`     | `tag.html`               | Current tag being viewed                     |
| `.Stats`           | `*PostStats`   | `post.html`              | Views, likes, dislikes for current post      |
| `.Comments`        | `[]Comment`    | `post.html`              | Approved comments for current post           |
| `.Pagination`      | `*Pagination`  | `home.html`              | Current page, total, prev/next URLs          |
| `.CSRFToken`       | `string`       | All pages                | CSRF token for forms                         |
| `.Year`            | `int`          | All pages                | Current year (for copyright)                 |
| `.Error`           | `string`       | 404.html, error pages    | Error message (e.g. "Page not found")        |
| `.RelatedPosts`    | `[]Post`       | `post.html`              | Up to 3 posts with shared tags               |
| `.SearchQuery`     | `string`       | `search.html`            | Current search query                         |
| `.SearchResults`   | `[]Post`       | `search.html`            | Search results                               |
| `.Webmentions`     | `[]Webmention` | `post.html`              | Verified webmentions for current post        |
| `.FediAddress`     | `string`       | All pages                | Fediverse address (e.g. @blog@example.com)   |
| `.TableOfContents` | `[]TOCEntry`   | `post.html`              | Headings for ToC (when `toc: true`)          |
| `.SeriesPosts`     | `[]Post`       | `post.html`              | All posts in same series                     |

### Post fields

| Field          | Type        | Description                              |
| -------------- | ----------- | ---------------------------------------- |
| `.Title`       | `string`    | Post title                               |
| `.Slug`        | `string`    | URL slug                                 |
| `.Date`        | `time.Time` | Publication date                         |
| `.Tags`        | `[]string`  | Tag names                                |
| `.Description` | `string`    | SEO description / excerpt                |
| `.Image`       | `string`    | Featured image filename                  |
| `.ImageCredit` | `*Credit`   | Image attribution (`.Name`, `.URL`)      |
| `.Featured`    | `bool`      | Whether post is featured                 |
| `.Draft`       | `bool`      | Whether post is a draft                  |
| `.TOC`         | `bool`      | Whether to show table of contents        |
| `.Series`      | `string`    | Series name (for multi-part posts)       |
| `.SeriesOrder` | `int`       | Position in series (1, 2, 3...)          |
| `.Content`     | `string`    | Raw markdown content                     |
| `.HTML`        | `string`    | Rendered HTML (use `{{safeHTML .HTML}}`) |
| `.ReadingTime` | `int`       | Estimated minutes to read                |
| `.WordCount`   | `int`       | Word count                               |
| `.Permalink`   | `string`    | URL path (e.g., `/my-post`)              |

## Step 5: Available Template Functions

| Function      | Description            | Example                                   |
| ------------- | ---------------------- | ----------------------------------------- |
| `formatDate`  | Format `time.Time`     | `{{formatDate .Post.Date "Jan 2, 2006"}}` |
| `readingTime` | Reading time string    | `{{readingTime .Post}}` → `"3 min read"`  |
| `truncate`    | Truncate with ellipsis | `{{truncate .Description 160}}`           |
| `safeHTML`    | Mark trusted HTML      | `{{safeHTML .Post.HTML}}`                 |
| `slugify`     | Convert to URL slug    | `{{slugify .Tag.Name}}`                   |
| `currentYear` | Current year           | `{{currentYear}}` → `2026`                |
| `cacheVer`    | Cache bust version     | `{{cacheVer}}` (changes hourly)           |
| `hasPrefix`   | String prefix check    | `{{if hasPrefix .Nav.Path "/admin"}}`     |
| `i18n`        | Translated string      | `{{i18n "subscribe"}}` → locale-aware     |
| `lower`       | Lowercase              | `{{lower .Tag.Name}}`                     |
| `upper`       | Uppercase              | `{{upper .Post.Title}}`                   |
| `join`        | Join slice             | `{{join .Post.Tags ", "}}`                |
| `add`         | Add ints               | `{{add .Pagination.CurrentPage 1}}`       |
| `sub`         | Subtract ints          | `{{sub .Pagination.TotalPages 1}}`        |
| `seq`         | Int sequence           | `{{range seq 1 5}}...{{end}}`             |
| `jsonLD`      | JSON-LD script tag     | `{{jsonLD .Meta}}`                        |

## Step 6: CSS with Dark Mode

Use CSS custom properties scoped to `data-color-mode` attributes:

```css
/* Light tokens (default) */
:root,
[data-color-mode="light"] {
  --color-bg: #ffffff;
  --color-text: #24292f;
  --color-primary: #0969da;
  --color-border: #d0d7de;
  /* ... more tokens ... */
}

/* Dark tokens */
[data-color-mode="dark"] {
  --color-bg: #0d1117;
  --color-text: #e6edf3;
  --color-primary: #58a6ff;
  --color-border: #30363d;
}

/* Auto mode — follow OS preference */
@media (prefers-color-scheme: dark) {
  [data-color-mode="auto"] {
    --color-bg: #0d1117;
    --color-text: #e6edf3;
    --color-primary: #58a6ff;
    --color-border: #30363d;
  }
}

/* Use variables everywhere */
body {
  background: var(--color-bg);
  color: var(--color-text);
}
```

### Syntax highlighting

Code blocks use Chroma (Dracula style) with inline styles. Your `pre` CSS must **not** override the inline `background-color`. Use `:not([style])` to only style plain `<pre>` blocks:

```css
.post-content pre {
  border-radius: 6px;
  padding: 1rem;
  overflow-x: auto;
}
.post-content pre:not([style]) {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
}
.post-content pre code {
  background: none;
  padding: 0;
}
```

## Step 7: Dark Mode Toggle Button

Add a toggle button to your `nav.html`:

```html
<button
  class="theme-toggle"
  onclick="toggleColorMode()"
  aria-label="Toggle color mode"
>
  <span class="theme-toggle-icon">🌙</span>
</button>
```

The toggle JS in `base.html` handles the state (see Step 2).

## Step 8: Required Partials

### `head.html`

```html
{{define "head"}} {{if .Post}}{{jsonLD .Meta}}{{end}} {{end}}
```

### `post-card.html`

Used by `home.html`, `tag.html`, and `search.html` to render post summaries:

```html
{{define "post-card"}}
<article class="post-card">
  <h2><a href="{{.Permalink}}">{{.Title}}</a></h2>
  <time>{{formatDate .Date "Jan 2, 2006"}}</time>
  <span>{{readingTime .}}</span>
  {{if .Description}}
  <p>{{truncate .Description 200}}</p>
  {{end}}
</article>
{{end}}
```

### `comments.html`

Must include a CSRF token in the form:

```html
{{define "comments"}}
<section id="comments">
  <!-- Display existing comments -->
  {{range .Comments}}
  <div>
    <strong>{{.Author}}</strong>
    <time>{{formatDate .CreatedAt "Jan 2, 2006"}}</time>
    <p>{{.Content}}</p>
  </div>
  {{end}}

  <!-- Comment form -->
  <form action="/api/comment/{{.Post.Slug}}" method="POST">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}" />
    <input type="text" name="author" required maxlength="100" />
    <!-- Honeypot for bots -->
    <div style="display:none">
      <input type="text" name="website" tabindex="-1" />
    </div>
    <textarea name="content" required maxlength="5000"></textarea>
    <button type="submit">Post Comment</button>
  </form>
</section>
{{end}}
```

### `reactions.html`

Must include a CSRF token header in the fetch call:

```html
{{define "reactions"}}
<div>
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
    const res = await fetch("/api/reaction/" + slug, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-CSRF-Token": "{{.CSRFToken}}",
      },
      body: JSON.stringify({ type: type }),
    });
    if (res.ok) {
      const d = await res.json();
      document.getElementById("likes").textContent = d.likes;
      document.getElementById("dislikes").textContent = d.dislikes;
    }
  }
</script>
{{end}}
```

### `footer.html`

Should include the cookie consent banner:

```html
{{define "footer"}}
<footer>
  <!-- Newsletter subscribe form -->
  <section>
    <h3>Subscribe</h3>
    <p>Get new posts delivered to your inbox.</p>
    <form action="/api/subscribe" method="POST">
      <input type="hidden" name="csrf_token" value="{{.CSRFToken}}" />
      <input type="email" name="email" placeholder="you@example.com" required />
      <!-- Honeypot for bots -->
      <div style="display:none">
        <input type="text" name="website" tabindex="-1" />
      </div>
      <button type="submit">Subscribe</button>
    </form>
  </section>

  <p>
    &copy; {{currentYear}} {{.Site.Author.Name}}. Powered by
    <a href="https://github.com/Digvijay/skriva">Skriva</a>.
  </p>
</footer>
<div
  id="cookieConsent"
  style="display:none;position:fixed;bottom:0;left:0;right:0;background:#24292f;color:#fff;padding:.75rem;font-size:.85rem;text-align:center;z-index:9999"
>
  This site uses cookies for sessions and analytics.
  <a href="#" style="color:#58a6ff" onclick="acceptCookies();return false"
    >Got it</a
  >
</div>
<script>
  if (!localStorage.getItem("cookie_consent")) {
    document.getElementById("cookieConsent").style.display = "block";
  }
  function acceptCookies() {
    localStorage.setItem("cookie_consent", "1");
    document.getElementById("cookieConsent").style.display = "none";
  }
</script>
{{end}}
```

The subscribe form posts to `/api/subscribe` which handles email validation, confirmation (via SMTP or auto-confirm), and unsubscribe tokens automatically. Admin can then compose and send newsletters to all confirmed subscribers from `/admin/newsletter`.

## Step 9: Install Your Theme

1. Copy your theme folder to `{contentDir}/themes/my-theme/`
2. Go to Admin → Settings → Themes tab
3. Your theme will appear in the list — click "Activate"

Alternatively, set `theme: "my-theme"` in `data/config/site.yaml`.

User-installed themes in `{contentDir}/themes/` take priority over bundled themes with the same name.

## Bundled Themes Reference

| Theme        | Default Mode | Modes               | Layout        |
| ------------ | ------------ | ------------------- | ------------- |
| `classic`    | Light        | Light + Dark + Auto | Single-column |
| `newsletter` | Dark         | Light + Dark + Auto | Two-column    |
| `github`     | Auto (OS)    | Light + Dark + Auto | Single-column |

All three bundled themes serve as working examples. Read their source in `themes/` for real-world patterns.
