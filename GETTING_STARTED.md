# Getting Started with Skriva

This guide walks you through three ways to run Skriva — from local development to production deployment.

```
┌─────────────────────────────────────────────────────────┐
│                    Choose Your Path                      │
├──────────────────┬──────────────────┬────────────────────┤
│   🖥️ Local        │   ☁️ Azure        │   🏠 Self-Host     │
│   (5 min)        │   (15 min)       │   (10 min)         │
│                  │                  │                    │
│   Docker on      │   Container Apps │   Any machine +    │
│   your machine   │   + Custom Domain│   Cloudflare       │
│                  │                  │   Tunnel           │
└──────────────────┴──────────────────┴────────────────────┘
```

---

## Option 1: Run Locally with Docker (5 minutes)

The fastest way to try Skriva.

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) installed

### Step 1: Create directories & config

```bash
mkdir -p skriva/data/content/posts skriva/data/config
cd skriva

# Create site config
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

### Step 2: Run the container

```bash
docker run -d --name blog \
  --read-only \
  -v $(pwd)/data/content:/data/content \
  -v $(pwd)/data/config:/data/config \
  -p 8080:8080 \
  ghcr.io/digvijay/skriva:latest
```

### Step 3: Open your blog

```
┌──────────────────────────────────────┐
│  🌐 Blog:      http://localhost:8080 │
│  🔧 Admin:     http://localhost:8080 │
│                 /admin/              │
│  📡 RSS:       http://localhost:8080 │
│                 /rss.xml             │
│  📊 Metrics:   http://localhost:8080 │
│                 /metrics             │
└──────────────────────────────────────┘
```

### Step 4: Set an admin password

```bash
# Generate a bcrypt hash (using Docker)
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

### Step 5: Write your first post

1. Click **"New Post"** in the admin sidebar (or go to `/admin/editor`)
2. Type a title — the slug auto-generates
3. Write in Markdown — use the floating toolbar for formatting
4. Click the **⚙ gear icon** to set tags, excerpt, featured image
5. Click the green **Publish** button

```
┌──────────────────────────────────────────────────────┐
│  ← Posts              Preview  ⚙  [ Publish ]        │
├──────────────────────────────────────────────────────┤
│                                                      │
│  My First Post                                       │
│  ─────────────                                       │
│                                                      │
│  Welcome to my blog! This is written in              │
│  **Markdown** with live preview support.             │
│                                                      │
│  ## Getting Started                                  │
│                                                      │
│  Here's what I learned today...                      │
│                                                      │
└──────────────────────────────────────────────────────┘
```

---

## Option 2: Deploy to Azure Container Apps (15 minutes)

Production deployment with HTTPS, custom domain, and zero server management.

### Architecture

```
┌─────────┐     ┌────────────────────────┐     ┌──────────────┐
│ Browser  │────▶│  Azure Container Apps  │────▶│   Skriva     │
│          │◀────│  (managed HTTPS)       │◀────│   Container  │
└─────────┘     └────────────────────────┘     └──────┬───────┘
                                                      │
                                               ┌──────┴───────┐
                                               │ Azure Files  │
                                               │ ┌──────────┐ │
                                               │ │ /content  │ │
                                               │ │ /config   │ │
                                               │ └──────────┘ │
                                               └──────────────┘
```

### Prerequisites

- [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli) installed
- An Azure subscription
- A custom domain (optional but recommended)

### Step 1: Set up Azure resources

```bash
# Variables
RESOURCE_GROUP="blog-rg"
LOCATION="eastus"
ENVIRONMENT="blog-env"
STORAGE_ACCOUNT="blogdata$(openssl rand -hex 4)"

# Login
az login

# Create resource group
az group create --name $RESOURCE_GROUP --location $LOCATION

# Create storage account + file shares for persistent data
az storage account create \
  --name $STORAGE_ACCOUNT \
  --resource-group $RESOURCE_GROUP \
  --location $LOCATION \
  --sku Standard_LRS

STORAGE_KEY=$(az storage account keys list \
  --account-name $STORAGE_ACCOUNT \
  --resource-group $RESOURCE_GROUP \
  --query '[0].value' -o tsv)

az storage share create --name blog-content --account-name $STORAGE_ACCOUNT
az storage share create --name blog-config  --account-name $STORAGE_ACCOUNT
```

### Step 2: Upload initial config

```bash
# Create local config files first (see Option 1, Steps 1 & 4)

# Upload to Azure Files
az storage file upload \
  --share-name blog-config \
  --source data/config/site.yaml \
  --account-name $STORAGE_ACCOUNT

az storage file upload \
  --share-name blog-config \
  --source data/config/secrets.yaml \
  --account-name $STORAGE_ACCOUNT
```

### Step 3: Create Container Apps environment

```bash
# Create the environment
az containerapp env create \
  --name $ENVIRONMENT \
  --resource-group $RESOURCE_GROUP \
  --location $LOCATION

# Add storage mounts
az containerapp env storage set \
  --name $ENVIRONMENT \
  --resource-group $RESOURCE_GROUP \
  --storage-name blogstorage \
  --azure-file-account-name $STORAGE_ACCOUNT \
  --azure-file-account-key $STORAGE_KEY \
  --azure-file-share-name blog-content \
  --access-mode ReadWrite

az containerapp env storage set \
  --name $ENVIRONMENT \
  --resource-group $RESOURCE_GROUP \
  --storage-name blogconfig \
  --azure-file-account-name $STORAGE_ACCOUNT \
  --azure-file-account-key $STORAGE_KEY \
  --azure-file-share-name blog-config \
  --access-mode ReadWrite
```

### Step 4: Deploy the container

```bash
az containerapp create \
  --name blog \
  --resource-group $RESOURCE_GROUP \
  --environment $ENVIRONMENT \
  --image ghcr.io/digvijay/skriva:latest \
  --target-port 8080 \
  --ingress external \
  --min-replicas 1 \
  --max-replicas 1 \
  --cpu 0.25 \
  --memory 0.5Gi \
  --env-vars \
    BLOG_PORT=8080 \
    BLOG_CONTENT_DIR=/data/content \
    BLOG_CONFIG_DIR=/data/config
```

> **Note**: You'll need to add volume mounts via the Azure Portal or ARM template
> since `az containerapp create` volume mount support varies by CLI version.
> In the Portal: Container App → Containers → Edit → Volume mounts →
> Add `blogstorage` at `/data/content` and `blogconfig` at `/data/config`.

### Step 5: Get your URL

```bash
az containerapp show \
  --name blog \
  --resource-group $RESOURCE_GROUP \
  --query properties.configuration.ingress.fqdn -o tsv
```

Your blog is now live at `https://<app-name>.<region>.azurecontainerapps.io`!

### Step 6: Add a custom domain (optional)

```bash
# Add the custom domain
az containerapp hostname add \
  --name blog \
  --resource-group $RESOURCE_GROUP \
  --hostname blog.yourdomain.com

# Bind a managed certificate (free HTTPS!)
az containerapp hostname bind \
  --name blog \
  --resource-group $RESOURCE_GROUP \
  --hostname blog.yourdomain.com \
  --environment $ENVIRONMENT \
  --validation-method CNAME
```

Then add a CNAME record in your DNS:

```
┌─────────────────────────────────────────────────────────┐
│  DNS Configuration                                       │
│                                                          │
│  Type:  CNAME                                            │
│  Name:  blog                                             │
│  Value: <app-name>.<region>.azurecontainerapps.io        │
│  TTL:   300                                              │
└─────────────────────────────────────────────────────────┘
```

Update `base_url` in your `site.yaml`:

```yaml
base_url: "https://blog.yourdomain.com"
```

### Cost estimate

| Resource                          | Monthly cost        |
| --------------------------------- | ------------------- |
| Container Apps (0.25 vCPU, 0.5GB) | ~$5-10              |
| Azure Files (1 GB)                | ~$0.05              |
| Custom domain + HTTPS             | Free (managed cert) |
| **Total**                         | **~$5-10/month**    |

---

## Option 3: Self-Host from Home with Cloudflare Tunnel (10 minutes)

Run Skriva on any machine (Raspberry Pi, old laptop, NAS) and expose it securely via Cloudflare Tunnel — no port forwarding, no dynamic DNS, free HTTPS.

### Architecture

```
┌─────────┐     ┌────────────────┐     ┌──────────────────────────────┐
│ Browser  │────▶│  Cloudflare    │────▶│  Your Home Network           │
│          │◀────│  Edge (HTTPS)  │◀────│                              │
└─────────┘     └────────────────┘     │  ┌──────────┐ ┌───────────┐  │
                    ▲                  │  │cloudflared│─│  Skriva   │  │
                    │                  │  │ (tunnel)  │ │ :8080     │  │
                    │ Encrypted tunnel │  └──────────┘ └───────────┘  │
                    └──────────────────│                              │
                                       └──────────────────────────────┘
```

**Key benefits:**

- ✅ No ports opened on your router
- ✅ Free HTTPS from Cloudflare
- ✅ DDoS protection included
- ✅ Works behind CGNAT (no static IP needed)
- ✅ Free tier is sufficient

### Prerequisites

- A domain name (pointed to Cloudflare nameservers)
- [Cloudflare account](https://dash.cloudflare.com/sign-up) (free tier works)
- Docker installed on your home machine

### Step 1: Run Skriva

```bash
# Same as Option 1
mkdir -p skriva/data/content/posts skriva/data/config
cd skriva

# Create your site.yaml (set base_url to your domain!)
cat > data/config/site.yaml << 'EOF'
title: "My Blog"
tagline: "Self-hosted from home"
base_url: "https://blog.yourdomain.com"
theme: "editorial"
posts_per_page: 10
author:
  name: "Your Name"
EOF

# Run Skriva
docker run -d --name blog \
  --read-only \
  --restart unless-stopped \
  -v $(pwd)/data/content:/data/content \
  -v $(pwd)/data/config:/data/config \
  -p 8080:8080 \
  ghcr.io/digvijay/skriva:latest
```

### Step 2: Install Cloudflare Tunnel

```bash
# Option A: Docker (recommended)
docker run -d --name cloudflared \
  --restart unless-stopped \
  --network host \
  cloudflare/cloudflared:latest \
  tunnel --no-autoupdate run --token <YOUR_TUNNEL_TOKEN>

# Option B: Binary
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o cloudflared
chmod +x cloudflared
sudo mv cloudflared /usr/local/bin/
```

### Step 3: Create a tunnel in Cloudflare Dashboard

1. Go to [Cloudflare Zero Trust](https://one.dash.cloudflare.com/) → **Networks** → **Tunnels**
2. Click **Create a tunnel**
3. Name it `blog-tunnel`
4. Copy the tunnel token (used in Step 2)
5. Add a **Public Hostname**:

```
┌─────────────────────────────────────────────────────────┐
│  Cloudflare Tunnel Configuration                         │
│                                                          │
│  Subdomain:  blog                                        │
│  Domain:     yourdomain.com                              │
│  Service:    http://localhost:8080                        │
│                                                          │
│  Result: blog.yourdomain.com → your Skriva instance      │
└─────────────────────────────────────────────────────────┘
```

### Step 4: Done!

Your blog is now live at `https://blog.yourdomain.com` with:

- Free HTTPS (Cloudflare managed)
- DDoS protection
- Global CDN caching
- No ports opened on your router

### Docker Compose (both services together)

```yaml
# docker-compose.yml
services:
  blog:
    image: ghcr.io/digvijay/skriva:latest
    read_only: true
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./data/content:/data/content
      - ./data/config:/data/config

  tunnel:
    image: cloudflare/cloudflared:latest
    restart: unless-stopped
    network_mode: host
    command: tunnel --no-autoupdate run --token ${CLOUDFLARE_TUNNEL_TOKEN}
```

```bash
# Save your token in .env
echo "CLOUDFLARE_TUNNEL_TOKEN=<your-token>" > .env

# Start both
docker compose up -d
```

### Cost

| Resource          | Cost                      |
| ----------------- | ------------------------- |
| Cloudflare Tunnel | Free                      |
| HTTPS certificate | Free (Cloudflare managed) |
| Domain name       | ~$10/year                 |
| Electricity       | ~$3/year (Raspberry Pi)   |
| **Total**         | **~$1/month**             |

---

## After Setup: Essential Configuration

### Set up admin password & 2FA

1. Generate a bcrypt hash for your password
2. Add it to `data/config/secrets.yaml`
3. Log in at `/admin/`
4. Go to **Settings → Security** to enable TOTP 2FA or register a Passkey

### Configure newsletter (optional)

Create `data/config/smtp.yaml`:

```yaml
enabled: true
host: "smtp.gmail.com" # or your SMTP provider
port: 587
username: "you@gmail.com"
password: "app-password" # use an app password, not your real password
from: "blog@yourdomain.com"
```

### Import from Ghost or WordPress

```bash
# Ghost
docker exec blog /blog import ghost --export /data/content/ghost-export.json

# WordPress
docker exec blog /blog import wordpress --export /data/content/wordpress-export.xml
```

### Check migration status

```bash
docker exec blog /blog migrate status
```

---

## Quick Reference

```
┌────────────────────────────────────────────────────────┐
│                    Skriva Quick Reference                │
├────────────────────────────────────────────────────────┤
│                                                        │
│  Blog:           http://localhost:8080                  │
│  Admin:          http://localhost:8080/admin/           │
│  Editor:         http://localhost:8080/admin/editor     │
│  Newsletter:     http://localhost:8080/admin/newsletter │
│  Settings:       http://localhost:8080/admin/settings   │
│                                                        │
│  RSS Feed:       /rss.xml                              │
│  Sitemap:        /sitemap.xml                          │
│  Health Check:   /healthz                              │
│  Metrics:        /metrics                              │
│  ActivityPub:    /activitypub/actor                    │
│  Webfinger:      /.well-known/webfinger                │
│                                                        │
│  Content dir:    /data/content (posts, pages, media)   │
│  Config dir:     /data/config (yaml, db, certs)        │
│                                                        │
│  Migrate:        blog migrate status                   │
│  Rollback:       blog migrate rollback                 │
│  Import Ghost:   blog import ghost --export <file>     │
│  Import WP:      blog import wordpress --export <file> │
│                                                        │
│  Env vars:                                             │
│    BLOG_PORT         (default: 8080)                   │
│    BLOG_CONTENT_DIR  (default: /data/content)          │
│    BLOG_CONFIG_DIR   (default: /data/config)           │
│    BLOG_LOG_LEVEL    (default: info)                   │
│    BLOG_TLS_DOMAIN   (enables auto-TLS)               │
│                                                        │
│  Keyboard shortcuts (editor):                          │
│    Ctrl+B  Bold      Ctrl+I  Italic                    │
│    Ctrl+K  Link      Ctrl+S  Save                      │
│                                                        │
└────────────────────────────────────────────────────────┘
```

---

## Troubleshooting

| Problem                | Solution                                                                |
| ---------------------- | ----------------------------------------------------------------------- |
| Can't log in           | Check `secrets.yaml` has a valid bcrypt hash. Restart the container.    |
| Posts not appearing    | Posts with `draft: true` or future dates won't show. Check frontmatter. |
| Images broken          | Verify the media is in `data/content/posts/<slug>/media/`.              |
| Theme not loading      | Check `theme:` in `site.yaml` matches a folder in `themes/`.            |
| Newsletter not sending | Check `smtp.yaml` is configured. Use an app password for Gmail.         |
| Container won't start  | Check logs: `docker logs blog`. Usually a YAML syntax error.            |
| Cloudflare tunnel down | Check `docker logs cloudflared`. Verify the token is correct.           |
| Azure deployment fails | Check volume mounts are connected. Run `az containerapp logs show`.     |

---

## Next Steps

- 📝 **Write your first post** — `/admin/editor`
- 🎨 **Try different themes** — Settings → Themes → Activate
- 📬 **Set up newsletter** — Configure SMTP, go to `/admin/newsletter`
- 🔒 **Enable 2FA** — Settings → Security → Set up TOTP
- 🌍 **Join the Fediverse** — Your blog is already an ActivityPub actor at `@blog@yourdomain.com`
- 📖 **Create a theme** — See [TUTORIAL.md](TUTORIAL.md) for step-by-step guide
