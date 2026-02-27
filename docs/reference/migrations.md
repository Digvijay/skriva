# Database Migrations

Skriva uses a versioned migration system with automatic application on startup.

## How it works

- Migrations are embedded in the Go binary
- A `schema_migrations` table tracks which versions have been applied
- On startup, any unapplied migrations run automatically
- Each migration has `Up` (apply) and `Down` (rollback) SQL

## CLI Commands

```bash
# Show current migration status
blog migrate status

# Rollback the last applied migration
blog migrate rollback
```

## Current Migrations

| Version | Name                    | Description                      |
| ------- | ----------------------- | -------------------------------- |
| 1       | `initial_schema`        | Comments, page views, reactions  |
| 2       | `comment_moderation`    | Approval column                  |
| 3       | `passkeys`              | WebAuthn passkey storage         |
| 4       | `unique_views`          | Deduplicated page view tracking  |
| 5       | `subscribers`           | Newsletter subscriber table      |
| 6       | `newsletters`           | Newsletter drafts and scheduling |
| 7       | `post_revisions`        | Revision history                 |
| 8       | `newsletter_analytics`  | Open/click event tracking        |
| 9       | `newsletter_ab_testing` | A/B subject line variants        |
| 10      | `draft_shares`          | Draft preview tokens             |
| 11      | `webmentions`           | Webmention storage               |
| 12      | `activitypub_followers` | AP follower tracking             |
| 13      | `webhooks`              | Webhook registration             |
| 14      | `api_tokens`            | API token storage                |
| 15      | `newsletter_sends`      | Per-subscriber send tracking     |
| 16      | `audit_log`             | Persistent admin audit trail     |
