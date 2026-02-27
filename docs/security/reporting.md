# Reporting Vulnerabilities

**Do NOT open a public GitHub issue for security vulnerabilities.**

## Contact

Email: **security@digvijay.dev**

## What to include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

## Response timeline

| Stage          | Timeline                  |
| -------------- | ------------------------- |
| Acknowledgment | Within 48 hours           |
| Assessment     | Within 7 days             |
| Fix release    | Within 30 days (critical) |

## Scope

In scope:

- XSS, SSRF, CSRF, SQL injection
- Authentication/authorization bypass
- Session management issues
- Path traversal / file access
- Cryptographic weaknesses
- Federation protocol security (AP, IndieAuth, Micropub, Webmention)

Out of scope:

- Rate limiting thresholds (by design)
- Denial of service via legitimate traffic
- Social engineering
- Issues in dependencies (report upstream)

## Hall of Fame

We appreciate responsible disclosure. Reporters will be credited in the release notes (with permission).
