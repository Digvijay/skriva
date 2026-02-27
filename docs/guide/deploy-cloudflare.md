# Self-Host with Cloudflare Tunnel

Run Skriva on any machine (Raspberry Pi, old laptop, NAS) and expose it securely via Cloudflare Tunnel — no port forwarding, no dynamic DNS, free HTTPS.

## Benefits

- ✅ No ports opened on your router
- ✅ Free HTTPS from Cloudflare
- ✅ DDoS protection included
- ✅ Works behind CGNAT (no static IP needed)
- ✅ Free tier is sufficient

## Prerequisites

- A domain name (pointed to Cloudflare nameservers)
- [Cloudflare account](https://dash.cloudflare.com/sign-up) (free tier)
- Docker installed

## Step 1: Run Skriva

```bash
mkdir -p skriva/data/content/posts skriva/data/config
cd skriva

cat > data/config/site.yaml << 'EOF'
title: "My Blog"
base_url: "https://blog.yourdomain.com"
theme: "editorial"
author:
  name: "Your Name"
EOF

docker run -d --name blog --read-only --restart unless-stopped \
  -v $(pwd)/data/content:/data/content \
  -v $(pwd)/data/config:/data/config \
  -p 8080:8080 ghcr.io/digvijay/skriva:latest
```

## Step 2: Create a Cloudflare Tunnel

1. Go to [Cloudflare Zero Trust](https://one.dash.cloudflare.com/) → Networks → Tunnels
2. Create a tunnel named `blog-tunnel`
3. Copy the tunnel token
4. Add a public hostname: `blog.yourdomain.com → http://localhost:8080`

## Step 3: Run the tunnel

```bash
docker run -d --name cloudflared --restart unless-stopped --network host \
  cloudflare/cloudflared:latest \
  tunnel --no-autoupdate run --token <YOUR_TUNNEL_TOKEN>
```

## Docker Compose (both services)

```yaml
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

## Cost

| Resource                   | Cost          |
| -------------------------- | ------------- |
| Cloudflare Tunnel + HTTPS  | Free          |
| Domain name                | ~$10/year     |
| Electricity (Raspberry Pi) | ~$3/year      |
| **Total**                  | **~$1/month** |
