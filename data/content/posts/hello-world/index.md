---
title: "Hello World — Welcome to My Blog"
slug: "hello-world"
date: 2024-01-15T10:00:00Z
tags:
  - general
  - blogging
  - go
description: "A first post to kick things off. Here's how this blog works and what you can expect."
featured: true
draft: false
---

Welcome to my blog! This is powered by [Skriva](https://github.com/Digvijay/skriva), a lightweight, single-binary blog engine written in Go.

## What Makes This Different?

Unlike heavy CMS platforms, this blog runs as a single container with two mapped volumes:

- **Content** — Markdown files, images, and themes
- **Config** — Site settings, secrets, and the SQLite database

That's it. No external databases, no CDN dependencies, no tracking scripts.

## Writing Posts

Every post is a Markdown file with YAML frontmatter:

```yaml
---
title: "My Post Title"
slug: "my-post-title"
date: 2024-01-15T10:00:00Z
tags:
  - go
  - blogging
description: "A short description for SEO and social cards."
featured: false
draft: false
---
```

Then write your content in standard Markdown with support for:

- **GFM tables** and task lists
- **Syntax highlighting** with Chroma
- **Images, audio, and video** in a per-post media folder
- **Smart typography** — curly quotes, em dashes, ellipses

## Code Highlighting

Here's a Go example:

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello from Skriva!")
}
```

And some Python too:

```python
def greet(name: str) -> str:
    return f"Hello, {name}!"

print(greet("World"))
```

## Migration

Moving to a new server? Copy two directories and run the container:

```bash
scp -r /srv/blog/content newserver:/srv/blog/content
scp -r /srv/blog/config newserver:/srv/blog/config
docker run -d --name blog \
  --read-only \
  -v /srv/blog/content:/data/content \
  -v /srv/blog/config:/data/config \
  -p 8080:8080 \
  ghcr.io/digvijay/skriva:latest
```

That's it. No database migrations, no export/import tools. Just files.

## What's Next?

I'll be writing about cloud architecture, homelabs, Go programming, and whatever else catches my interest. Stay tuned!
