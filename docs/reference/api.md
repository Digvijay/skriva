# API Endpoints

## Public Endpoints

| Method | Path                    | Description                        |
| ------ | ----------------------- | ---------------------------------- |
| GET    | `/`                     | Homepage (paginated)               |
| GET    | `/{slug}`               | Single post or page                |
| GET    | `/tag/{tag}`            | Posts by tag                       |
| GET    | `/tags`                 | All tags index                     |
| GET    | `/archive`              | Chronological archive              |
| GET    | `/search`               | Full-text search                   |
| GET    | `/rss.xml`              | RSS feed                           |
| GET    | `/sitemap.xml`          | Sitemap                            |
| GET    | `/robots.txt`           | Robots file                        |
| GET    | `/healthz`              | Health check (JSON)                |
| GET    | `/metrics`              | Prometheus metrics (auth required) |
| GET    | `/og/{slug}`            | Auto-generated OG image            |
| GET    | `/favicon.ico`          | Favicon (custom or default SVG)    |
| GET    | `/favicon.svg`          | Favicon SVG variant                |
| POST   | `/api/comment/{slug}`   | Submit comment                     |
| POST   | `/api/reaction/{slug}`  | Like/dislike                       |
| POST   | `/api/subscribe`        | Newsletter subscribe               |
| GET    | `/api/unsubscribe`      | Newsletter unsubscribe             |
| GET    | `/api/newsletter/open`  | Open tracking pixel                |
| GET    | `/api/newsletter/click` | Click tracking redirect            |
| POST   | `/webmention`           | Receive webmention                 |
| GET    | `/preview/{token}`      | Draft preview                      |

## ActivityPub / IndieWeb

| Method | Path                                      | Description                   |
| ------ | ----------------------------------------- | ----------------------------- |
| GET    | `/.well-known/webfinger`                  | Webfinger discovery           |
| GET    | `/.well-known/oauth-authorization-server` | IndieAuth metadata            |
| GET    | `/activitypub/actor`                      | AP actor                      |
| POST   | `/activitypub/inbox`                      | AP inbox                      |
| GET    | `/activitypub/outbox`                     | AP outbox                     |
| GET    | `/activitypub/followers`                  | AP followers                  |
| GET    | `/indieauth/auth`                         | IndieAuth authorize           |
| POST   | `/indieauth/auth`                         | IndieAuth approve             |
| POST   | `/indieauth/token`                        | IndieAuth token exchange      |
| GET    | `/micropub`                               | Micropub query                |
| POST   | `/micropub`                               | Micropub create/update/delete |
| POST   | `/micropub/media`                         | Micropub media upload         |

## Admin API (authenticated)

### Pages

| Method | Path                   | Description        |
| ------ | ---------------------- | ------------------ |
| GET    | `/admin/`              | Dashboard          |
| GET    | `/admin/login`         | Login page         |
| GET    | `/admin/editor`        | New post editor    |
| GET    | `/admin/editor/{slug}` | Edit existing post |
| GET    | `/admin/settings`      | Settings page      |
| GET    | `/admin/newsletter`    | Newsletter editor  |

### Authentication

| Method | Path                                  | Description                 |
| ------ | ------------------------------------- | --------------------------- |
| POST   | `/admin/api/login`                    | Authenticate                |
| POST   | `/admin/api/logout`                   | End session                 |
| POST   | `/admin/api/password`                 | Change password             |
| GET    | `/admin/api/totp/status`              | TOTP 2FA status             |
| POST   | `/admin/api/totp/setup`               | Generate TOTP secret        |
| POST   | `/admin/api/totp/confirm`             | Verify & enable TOTP        |
| POST   | `/admin/api/totp/disable`             | Disable TOTP                |
| GET    | `/admin/api/passkeys`                 | List passkeys               |
| POST   | `/admin/api/passkeys/register/begin`  | Begin passkey registration  |
| POST   | `/admin/api/passkeys/register/finish` | Finish passkey registration |
| DELETE | `/admin/api/passkeys/{id}`            | Delete passkey              |
| POST   | `/admin/api/passkeys/login/begin`     | Begin passkey login         |
| POST   | `/admin/api/passkeys/login/finish`    | Finish passkey login        |

### Posts

| Method | Path                                | Description        |
| ------ | ----------------------------------- | ------------------ |
| GET    | `/admin/api/posts`                  | List all posts     |
| POST   | `/admin/api/posts`                  | Create post        |
| PUT    | `/admin/api/posts/{slug}`           | Update post        |
| DELETE | `/admin/api/posts/{slug}`           | Delete post        |
| POST   | `/admin/api/posts/bulk`             | Bulk operations    |
| GET    | `/admin/api/posts/{slug}/revisions` | List revisions     |
| POST   | `/admin/api/revisions/{id}/restore` | Restore revision   |
| POST   | `/admin/api/posts/{slug}/share`     | Create draft share |
| DELETE | `/admin/api/posts/{slug}/share`     | Revoke draft share |
| POST   | `/admin/api/preview`                | Render markdown    |
| POST   | `/admin/api/media/{slug}`           | Upload media       |
| POST   | `/admin/api/avatar`                 | Upload avatar      |
| GET    | `/admin/api/unsplash`               | Search Unsplash    |

### Comments

| Method | Path                                 | Description     |
| ------ | ------------------------------------ | --------------- |
| GET    | `/admin/api/comments`                | List comments   |
| POST   | `/admin/api/comments/{id}/approve`   | Approve         |
| POST   | `/admin/api/comments/{id}/unapprove` | Unapprove       |
| DELETE | `/admin/api/comments/{id}`           | Delete          |
| POST   | `/admin/api/comments/bulk`           | Bulk operations |

### Newsletter

| Method | Path                                     | Description          |
| ------ | ---------------------------------------- | -------------------- |
| GET    | `/admin/api/newsletters`                 | List newsletters     |
| POST   | `/admin/api/newsletters`                 | Create draft         |
| PUT    | `/admin/api/newsletters/{id}`            | Update draft         |
| DELETE | `/admin/api/newsletters/{id}`            | Delete newsletter    |
| POST   | `/admin/api/newsletters/{id}/schedule`   | Schedule send        |
| POST   | `/admin/api/newsletters/{id}/unschedule` | Back to draft        |
| POST   | `/admin/api/newsletters/{id}/send`       | Send immediately     |
| POST   | `/admin/api/newsletters/{id}/retry`      | Retry failed sends   |
| POST   | `/admin/api/newsletters/preview`         | Preview markdown     |
| GET    | `/admin/api/newsletters/{id}/analytics`  | Open/click analytics |
| POST   | `/admin/api/newsletters/{id}/ab-test`    | Create A/B variant   |
| GET    | `/admin/api/subscribers`                 | List subscribers     |
| DELETE | `/admin/api/subscribers/{id}`            | Remove subscriber    |

### Settings & Admin

| Method | Path                           | Description        |
| ------ | ------------------------------ | ------------------ |
| GET    | `/admin/api/settings`          | Get site config    |
| PUT    | `/admin/api/settings`          | Update site config |
| GET    | `/admin/api/stats`             | Dashboard stats    |
| GET    | `/admin/api/themes`            | List themes        |
| GET    | `/admin/api/theme-preview`     | Theme preview info |
| GET    | `/admin/api/theme-preview-css` | Theme preview page |
| GET    | `/admin/api/audit-log`         | Audit trail        |
| POST   | `/admin/api/audit-log/clear`   | Clear old entries  |
| GET    | `/admin/api/export`            | ZIP export         |
| GET    | `/admin/api/fediverse`         | Fediverse stats    |
| GET    | `/admin/api/webmentions`       | List webmentions   |
| DELETE | `/admin/api/webmentions/{id}`  | Delete webmention  |
| DELETE | `/admin/api/followers/{id}`    | Remove AP follower |
| GET    | `/admin/api/webhooks`          | List webhooks      |
| POST   | `/admin/api/webhooks`          | Create webhook     |
| DELETE | `/admin/api/webhooks/{id}`     | Delete webhook     |
| GET    | `/admin/api/tokens`            | List API tokens    |
| POST   | `/admin/api/tokens`            | Create API token   |
| DELETE | `/admin/api/tokens/{id}`       | Revoke API token   |
