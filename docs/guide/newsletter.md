# Newsletter

Skriva includes a full newsletter system — compose in Markdown, send to subscribers via SMTP, with scheduling and analytics.

## Setup

Create `data/config/smtp.yaml`:

```yaml
enabled: true
host: "smtp.gmail.com" # or your SMTP provider
port: 587
username: "you@gmail.com"
password: "app-password" # use an app password, not your real password
from: "blog@yourdomain.com"
```

::: warning
For Gmail, create an [App Password](https://myaccount.google.com/apppasswords) — don't use your real password.
:::

## Features

- **Markdown editor** with live preview
- **Send immediately** or **schedule** for a future date/time
- **A/B subject testing** — test two subject lines, compare open rates
- **Open/click tracking** — pixel tracking for opens, link-rewriting for clicks
- **Per-subscriber retry** — only retry failed recipients, not the whole list
- **SMTP preflight** — verifies connection before sending

## Subscriber Management

- Visitors subscribe via the footer form on your blog
- Double opt-in via confirmation email (when SMTP is configured)
- Subscribers can unsubscribe via a link in every email
- Admin can view/delete subscribers at `/admin/newsletter`

## Sending Flow

1. Go to `/admin/newsletter`
2. Compose your newsletter in Markdown
3. Click **Preview** to see the rendered HTML email
4. Click **Send Now** or **Schedule** for later
5. Track opens and clicks in the analytics tab

## Newsletter Analytics

Each newsletter tracks:

- **Send count** — how many emails were sent
- **Open rate** — via tracking pixel
- **Click rate** — via link redirect tracking
- **Failed deliveries** — with retry capability
