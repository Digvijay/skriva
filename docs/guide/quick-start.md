# Quick Start

Three-command setup:

```bash
# 1. Create config
mkdir -p data/content/posts data/config
cat > data/config/site.yaml << 'EOF'
title: "My Blog"
base_url: "http://localhost:8080"
theme: "classic"
author:
  name: "Your Name"
EOF

# 2. Run
docker run -d --name blog --read-only \
  -v $(pwd)/data/content:/data/content \
  -v $(pwd)/data/config:/data/config \
  -p 8080:8080 ghcr.io/digvijay/skriva:latest

# 3. Open
open http://localhost:8080
```

## Docker Compose

```yaml
services:
  blog:
    image: ghcr.io/digvijay/skriva:latest
    read_only: true
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./data/content:/data/content
      - ./data/config:/data/config
```

```bash
docker compose up -d
```

## From Source

```bash
go build -o blog ./cmd/blog
BLOG_CONTENT_DIR=./data/content BLOG_CONFIG_DIR=./data/config ./blog serve
```

## Directory Structure

```
data/
├── content/                    # Volume 1: Content
│   ├── posts/
│   │   └── my-first-post/
│   │       ├── index.md        # Post with YAML frontmatter
│   │       └── media/          # Post-specific images
│   ├── pages/
│   │   └── about/
│   │       └── index.md
│   ├── themes/                 # Custom themes (optional)
│   └── static/                 # Global static files
└── config/                     # Volume 2: Config
    ├── site.yaml               # Site settings
    ├── secrets.yaml            # Admin password, API keys
    ├── smtp.yaml               # Email settings (optional)
    └── blog.db                 # SQLite database (auto-created)
```
