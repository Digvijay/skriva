# Security Architecture

Skriva follows a defense-in-depth security model with multiple layers of protection.

## Key Security Areas

| Layer              | Summary                                                              |
| ------------------ | -------------------------------------------------------------------- |
| **Network**        | HSTS, SSRF protection, rate limiting                                 |
| **Authentication** | bcrypt passwords, TOTP 2FA, WebAuthn passkeys, IP lockout            |
| **Session**        | Signed cookies, automatic rotation on credential change              |
| **Authorization**  | CSRF protection, scoped API tokens                                   |
| **Input/Output**   | HTML sanitization, template auto-escaping, CSP headers               |
| **Data**           | Parameterized SQL, path traversal prevention, PII protection         |
| **File I/O**       | Content-type validation, safe directory traversal                    |
| **Federation**     | HTTP Signatures, PKCE enforcement, constant-time crypto              |
| **Audit**          | Persistent admin action log with retention policy                    |
| **Container**      | Distroless, read-only, non-root, pure Go (no CGO)                   |

## Security Testing

The project includes a dedicated security regression test suite:

```bash
go test -race ./...
```

Tests cover: XSS, SSRF, CSRF, authentication, authorization, cryptography, and data safety.
