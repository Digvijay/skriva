# Docker Deployment

## Basic Docker Run

```bash
docker run -d --name blog \
  --read-only \
  -v /srv/blog/content:/data/content \
  -v /srv/blog/config:/data/config \
  -p 8080:8080 \
  ghcr.io/digvijay/skriva:latest
```

## With Auto-TLS (HTTPS)

```bash
docker run -d --name blog \
  --read-only \
  -v /srv/blog/content:/data/content \
  -v /srv/blog/config:/data/config \
  -p 443:443 -p 80:80 \
  -e BLOG_TLS_DOMAIN=blog.yourdomain.com \
  ghcr.io/digvijay/skriva:latest
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
    environment:
      - BLOG_PORT=8080
      - BLOG_LOG_LEVEL=info
```

## Container Security

Skriva's Docker image is built on `gcr.io/distroless/static:nonroot`:

- **Read-only filesystem** — container root is immutable
- **Non-root user** — runs as `nonroot` (UID 65532)
- **No shell** — distroless has no shell, package manager, or utilities
- **Minimal attack surface** — only the Go binary and CA certificates
- **No CGO** — pure Go build, no C library dependencies

## Updating

```bash
docker pull ghcr.io/digvijay/skriva:latest
docker stop blog && docker rm blog
# Re-run the docker run command above
```

Your data is safe in the mounted volumes — the container is stateless.
