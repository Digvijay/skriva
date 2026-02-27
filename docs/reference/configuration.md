# Configuration

Skriva uses three YAML config files in the `config/` directory.

## site.yaml

Main site settings — all fields are editable from the admin UI.

```yaml
title: "My Blog"
tagline: "Thoughts and tutorials"
base_url: "https://blog.yourdomain.com"
theme: "classic"
locale: "en"
posts_per_page: 10

author:
  name: "Your Name"
  bio: "Writer, developer, thinker."
  avatar: "/static/profile.jpg"

social:
  github: "username"
  twitter: "username"
  linkedin: "username"
  youtube: "username"
  bluesky: "username.bsky.social"

footer:
  sections:
    - title: "Blog"
      links:
        - label: "Archive"
          url: "/archive"
        - label: "Tags"
          url: "/tags"

# IndieWeb toggles (all default to true)
activitypub_enabled: true
webmention_enabled: true
indieauth_enabled: true
micropub_enabled: true
```

## secrets.yaml

Sensitive settings — never committed to version control.

```yaml
admin_password: "$2y$10$..." # bcrypt hash
session_secret: "" # auto-generated on first run
unsplash_access_key: "" # optional, for Unsplash image search
totp_secret: "" # set when TOTP 2FA is enabled
activitypub_private_key: "" # auto-generated on first run
activitypub_public_key: "" # auto-generated on first run
```

## smtp.yaml

Email configuration for newsletters. Optional.

```yaml
enabled: false
host: "smtp.gmail.com"
port: 587
username: "you@gmail.com"
password: "app-password"
from: "blog@yourdomain.com"
```

## Supported Locales

Set `locale` in `site.yaml`:

| Code | Language |
| ---- | -------- |
| `en` | English  |
| `es` | Spanish  |
| `fr` | French   |
| `de` | German   |
| `ja` | Japanese |
| `zh` | Chinese  |
