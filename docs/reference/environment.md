# Environment Variables

| Variable           | Description                                  | Default         |
| ------------------ | -------------------------------------------- | --------------- |
| `BLOG_PORT`        | HTTP listen port                             | `8080`          |
| `BLOG_CONTENT_DIR` | Content directory path                       | `/data/content` |
| `BLOG_CONFIG_DIR`  | Config directory path                        | `/data/config`  |
| `BLOG_LOG_LEVEL`   | Log level (`debug`, `info`, `warn`, `error`) | `info`          |
| `BLOG_TLS_DOMAIN`  | Enable auto-TLS for this domain              | _(disabled)_    |

## Examples

```bash
# Custom port
docker run -e BLOG_PORT=3000 -p 3000:3000 ...

# Debug logging
docker run -e BLOG_LOG_LEVEL=debug ...

# Auto-TLS (HTTPS)
docker run -e BLOG_TLS_DOMAIN=blog.example.com -p 443:443 -p 80:80 ...
```

When `BLOG_TLS_DOMAIN` is set, Skriva automatically:

- Obtains a TLS certificate from Let's Encrypt
- Redirects HTTP (port 80) to HTTPS (port 443)
- Renews the certificate before expiry
- Sets HSTS headers for browser enforcement
