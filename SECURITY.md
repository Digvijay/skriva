# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in Skriva, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please email: **security@digvijay.dev**

### What to include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response timeline

- **Acknowledgment**: Within 48 hours
- **Assessment**: Within 7 days
- **Fix release**: Within 30 days for critical issues

## Security Architecture

Skriva follows a defense-in-depth security model covering:

- **Authentication** — bcrypt passwords, TOTP 2FA, WebAuthn passkeys, IP lockout
- **Session management** — signed cookies, automatic rotation
- **Input/output** — HTML sanitization, template auto-escaping, CSRF protection
- **Network** — SSRF protection on all outbound HTTP clients, per-IP rate limiting, security headers (HSTS, CSP, etc.)
- **Data protection** — parameterized SQL, path traversal prevention, content-type validation
- **Federation** — HTTP Signature verification (ActivityPub), PKCE (IndieAuth), constant-time crypto
- **Audit** — persistent admin action log with configurable retention
- **Container** — distroless base image, read-only filesystem, non-root user, no CGO

The codebase includes a dedicated security regression test suite to prevent regressions:

```bash
go test -race ./...
```
