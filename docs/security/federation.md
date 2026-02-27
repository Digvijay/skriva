# Federation Security

## ActivityPub

- Inbox requires HTTP Signatures on all incoming activities
- All remote content is sanitized before storage

## IndieAuth

- Mandatory PKCE S256 on all authorization flows
- Constant-time token verification
- Exact-match scope checking

## Webmention

- Inbound: source URLs validated, content sanitized, rate-limited
- Outbound: SSRF-safe HTTP client, only for published posts

## Feature Toggles

All four protocols (ActivityPub, Webmention, IndieAuth, Micropub) can be independently enabled or disabled from **Admin → Settings → IndieWeb**.
