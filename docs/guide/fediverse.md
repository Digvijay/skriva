# Fediverse & IndieWeb

Skriva is a first-class fediverse and IndieWeb citizen. All four protocols are built-in and can be independently toggled from admin settings.

## ActivityPub

Your blog is an ActivityPub actor at `@blog@yourdomain.com`. Mastodon, Misskey, Pleroma, and other fediverse users can follow your blog and see new posts in their timeline.

- **Webfinger discovery** at `/.well-known/webfinger`
- **Actor endpoint** at `/activitypub/actor`
- **Inbox** receives Follow, Undo, Create (replies)
- **Outbox** serves published posts as activities
- **Followers** collection
- **HTTP Signatures** on all outbound deliveries
- **Automatic notification** — followers are notified when you publish

## Webmention

Send and receive [webmentions](https://www.w3.org/TR/webmention/) per the W3C spec:

- **Receive** — `POST /webmention` accepts source/target
- **Verify** — async verification that source links to target
- **Display** — verified webmentions shown on post pages
- **Send** — outgoing webmentions sent to linked pages when you publish
- **Admin** — manage webmentions from the dashboard

## IndieAuth

Your blog is an [IndieAuth](https://indieauth.spec.indieweb.org/) provider:

- **Authorization endpoint** at `/indieauth/auth`
- **Token endpoint** at `/indieauth/token`
- **Mandatory PKCE** (S256) for all flows
- **OAuth server metadata** at `/.well-known/oauth-authorization-server`

Use your blog URL as your identity to sign in to IndieWeb services.

## Micropub

Create, update, and delete posts from any [Micropub](https://micropub.spec.indieweb.org/) client (Quill, Indigenous, etc.):

- **Create** posts with title, content, tags
- **Update** with replace/add/delete operations
- **Delete** posts
- **Media upload** at `/micropub/media`
- **Queries** — `?q=config`, `?q=source`, `?q=category`, `?q=syndicate-to`

## Configuration

All four protocols are enabled by default. Toggle them individually in **Admin → Settings → IndieWeb**:

```yaml
# site.yaml
activitypub_enabled: true
webmention_enabled: true
indieauth_enabled: true
micropub_enabled: true
```

When a protocol is disabled:

- Its endpoints return 404
- Discovery `<link>` tags are removed from theme `<head>`
- The fediverse address is hidden from footers
