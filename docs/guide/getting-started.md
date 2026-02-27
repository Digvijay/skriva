# Getting Started

The fastest way to try Skriva is with Docker. You'll have a working blog in under 5 minutes.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) installed

## Step 1: Create directories & config

```bash
mkdir -p skriva/data/content/posts skriva/data/config
cd skriva

cat > data/config/site.yaml << 'EOF'
title: "My Blog"
tagline: "Thoughts, tutorials, and projects"
base_url: "http://localhost:8080"
theme: "classic"
posts_per_page: 10
author:
  name: "Your Name"
  bio: "Writer, developer, thinker."
footer:
  sections:
    - title: "Blog"
      links:
        - label: "Archive"
          url: "/archive"
        - label: "Tags"
          url: "/tags"
EOF
```

## Step 2: Run the container

```bash
docker run -d --name blog \
  --read-only \
  -v $(pwd)/data/content:/data/content \
  -v $(pwd)/data/config:/data/config \
  -p 8080:8080 \
  ghcr.io/digvijay/skriva:latest
```

## Step 3: Open your blog

- **Blog**: [http://localhost:8080](http://localhost:8080)
- **Admin**: [http://localhost:8080/admin/](http://localhost:8080/admin/)
- **RSS**: [http://localhost:8080/rss.xml](http://localhost:8080/rss.xml)

## Step 4: Set an admin password

```bash
# Generate a bcrypt hash
docker run --rm -it alpine sh -c \
  "apk add --no-cache apache2-utils && htpasswd -nbBC 10 '' 'YourPassword123!' | cut -d: -f2"

# Create secrets.yaml with the hash
cat > data/config/secrets.yaml << 'EOF'
admin_password: "$2y$10$..."   # paste your hash here
session_secret: ""              # auto-generated on first run
EOF

# Restart to pick up the new password
docker restart blog
```

Now go to `http://localhost:8080/admin/` and log in!

## Step 5: Write your first post

1. Click **"New Post"** in the admin dashboard
2. Type a title — the slug auto-generates
3. Write in Markdown with live preview
4. Set tags, description, and featured image
5. Click **Publish**

## Next Steps

- [Deploy to production](/guide/deploy-docker) with Docker
- [Enable 2FA](/security/authentication) in Settings → Security
- [Set up a newsletter](/guide/newsletter) with SMTP
- [Join the Fediverse](/guide/fediverse) — your blog is already an ActivityPub actor
- [Create a custom theme](/themes/creating)
