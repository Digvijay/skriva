# Creating a Theme

## Theme Structure

```
themes/my-theme/
├── theme.yaml          # Theme metadata
├── css/
│   ├── theme.css       # Main stylesheet
│   └── syntax.css      # Syntax highlighting (optional)
└── templates/
    ├── base.html       # HTML shell
    ├── home.html       # Homepage
    ├── post.html       # Single post
    ├── page.html       # Static page
    ├── tag.html        # Posts by tag
    ├── tags.html       # All tags
    ├── archive.html    # Archive
    ├── search.html     # Search
    ├── 404.html        # Not found
    └── partials/
        ├── head.html       # Extra &lt;head&gt; content
        ├── nav.html        # Navigation
        ├── post-card.html  # Post summary card
        ├── comments.html   # Comments section
        ├── reactions.html  # Like/dislike
        ├── share.html      # Social sharing
        └── footer.html     # Footer + cookie consent
```

## Step 1: theme.yaml

```yaml
name: "My Theme"
description: "A custom theme for my blog"
author: "Your Name"
version: "1.0.0"
layout: "single-column"
```

## Step 2: base.html

```html
{{define "base"}}<!DOCTYPE html>
<html lang="en" data-color-mode="auto">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{{.Meta.Title}}</title>
    <link rel="stylesheet" href="/theme/css/theme.css?v={{cacheVer}}" />
    {{if .Site.WebmentionEnabled}}
    <link rel="webmention" href="/webmention" />
    {{end}} {{if .Site.MicropubEnabled}}
    <link rel="micropub" href="/micropub" />
    {{end}} {{if .Site.IndieAuthEnabled}}
    <link rel="authorization_endpoint" href="/indieauth/auth" />
    <link rel="token_endpoint" href="/indieauth/token" />
    {{end}} {{template "head" .}}
  </head>
  <body>
    {{template "nav" .}}
    <main>{{block "content" .}}{{end}}</main>
    {{template "footer" .}}
  </body>
</html>
{{end}}
```

::: tip
IndieWeb discovery links are conditional on the feature toggles from `site.yaml`.
:::

## Step 3: Dark Mode CSS

```css
:root,
[data-color-mode="light"] {
  --color-bg: #ffffff;
  --color-text: #24292f;
}
[data-color-mode="dark"] {
  --color-bg: #0d1117;
  --color-text: #e6edf3;
}
@media (prefers-color-scheme: dark) {
  [data-color-mode="auto"] {
    --color-bg: #0d1117;
    --color-text: #e6edf3;
  }
}
```

## Step 4: Install

Copy your theme folder to `data/content/themes/my-theme/`, then activate it in admin settings.

See [Template Data](/themes/template-data) and [Template Functions](/themes/functions) for the full API available to themes.
