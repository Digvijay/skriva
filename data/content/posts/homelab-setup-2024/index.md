---
title: "My Homelab Setup in 2024"
slug: "homelab-setup-2024"
date: 2024-03-10
tags: ["homelab", "docker", "self-hosting", "testing"]
description: "A tour of my current homelab — from hardware to the full Docker stack running on it."
image: ""
featured: false
draft: false
---



I've been running a homelab for a few years now, and 2024 is when I finally feel like the setup is "done" (famous last words). Here's a look at the hardware and software stack.

## Hardware

My main server is a used Dell OptiPlex Micro with:

- **CPU**: Intel Core i5-12500T (6 cores, 35W TDP)
- **RAM**: 32GB DDR4
- **Storage**: 1TB NVMe + 4TB external HDD for backups
- **Network**: 2.5GbE via USB adapter

It sits on a shelf in my closet, draws about 15W idle, and handles everything I throw at it.

## Software Stack

Everything runs in Docker containers managed with Docker Compose. Here's the key services:

```yaml
services:
  traefik:
    image: traefik:v3.0
    ports:
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./traefik:/etc/traefik

  blog:
    image: ghcr.io/digvijay/skriva:latest
    read_only: true
    volumes:
      - ./blog/content:/data/content
      - ./blog/config:/data/config
    labels:
      - "traefik.http.routers.blog.rule=Host(`digvijay.dev`)"

  vaultwarden:
    image: vaultwarden/server:latest
    volumes:
      - ./vaultwarden:/data

  monitoring:
    image: grafana/grafana:latest
    volumes:
      - grafana-data:/var/lib/grafana
```

## Networking

I use Tailscale for remote access instead of exposing anything directly. Combined with Traefik as the reverse proxy, I get:

1. **HTTPS everywhere** via Let's Encrypt
2. **Zero-trust access** to internal services
3. **Automatic routing** based on hostnames

## Backups

Backups are critical. My strategy is simple:

- **Daily**: Restic snapshots to the 4TB drive
- **Weekly**: Restic to Backblaze B2
- **Monthly**: Full tar archive to a separate USB drive

The backup script is just a cron job:

```bash
#!/bin/bash
set -euo pipefail

BACKUP_DIRS="/srv/docker /home/digvijay/scripts"
RESTIC_REPO="/mnt/backup/restic"

restic backup $BACKUP_DIRS \
  --repo "$RESTIC_REPO" \
  --tag "daily" \
  --exclude-caches

restic forget \
  --repo "$RESTIC_REPO" \
  --keep-daily 7 \
  --keep-weekly 4 \
  --keep-monthly 6 \
  --prune
```

## What's Next?

I'm looking at adding a dedicated NAS (probably a Synology DS423+) and experimenting with running local LLMs on a GPU box. But for now, this little OptiPlex handles everything perfectly.

The total cost of running this setup? About **$3/month** in electricity. Not bad for a full self-hosted stack.
