# Network & SSRF Protection

## SSRF Defense

All outbound HTTP requests are protected against SSRF:

- DNS resolution validates all resolved IPs against private/reserved ranges
- Redirect targets are validated at each hop
- Applies to all outbound HTTP clients (webmention, ActivityPub, webhooks, etc.)

## Rate Limiting

Per-IP sliding window rate limits are applied to all state-changing public endpoints, with additional limits on admin API endpoints and federation inboxes.

## Security Headers

Every response includes standard security headers: HSTS, CSP, X-Content-Type-Options, X-Frame-Options, Referrer-Policy, and Permissions-Policy.
