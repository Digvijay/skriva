# CLI Commands

## Serve

```bash
blog serve
```

Start the blog server. Configuration is via environment variables:

| Variable           | Description       | Default         |
| ------------------ | ----------------- | --------------- |
| `BLOG_CONTENT_DIR` | Content directory | `/data/content` |
| `BLOG_CONFIG_DIR`  | Config directory  | `/data/config`  |
| `BLOG_PORT`        | HTTP port         | `8080`          |
| `BLOG_LOG_LEVEL`   | Log level         | `info`          |
| `BLOG_TLS_DOMAIN`  | Auto-TLS domain   | _(disabled)_    |

See [Environment Variables](/reference/environment) for details.

## Import

### Ghost

```bash
blog import ghost --export <file> [--images <dir>]
```

Import posts from a Ghost JSON export.

### WordPress

```bash
blog import wordpress --export <file>
```

Import posts from a WordPress WXR XML export.

## Migrations

```bash
# Show status
blog migrate status

# Rollback last migration
blog migrate rollback
```
