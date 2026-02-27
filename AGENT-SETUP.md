# Agent Setup Instructions — Vel

**For AI agents:** Follow these steps when a user asks you to set up a Vel app.

---

## Step 0 — Clone

```bash
git clone https://github.com/essdee/vel.git
cd vel
```

## Step 1 — Ask the user

Send a single message asking:

> I'll set up your Vel app! I need a few details:
>
> 1. **App name** — What should it be called?
> 2. **Emoji** — Pick an emoji for the avatar (e.g. 🤖, ⚡)
> 3. **Subtitle** — A short tagline
> 4. **Accent color** — 🟡 Gold (#c9a84c), 🔴 Red (#e94560), 🔵 Cyan (#00d2ff), 🟣 Purple (#a855f7), 🟢 Green (#22c55e), or custom hex
> 5. **Domain** — Where it'll be hosted
> 6. **Telegram bot token** — From @BotFather
> 7. **Allowed user IDs** — Who can access the dashboard

## Step 2 — Write config.json

```json
{
  "name": "<name>",
  "emoji": "<emoji>",
  "subtitle": "<subtitle>",
  "accent": "<accent_hex>",
  "accentName": "custom",
  "botUsername": "<bot_username>",
  "authUrl": "https://<domain>/auth/telegram/callback",
  "telegramLink": "https://t.me/<bot_username>",
  "allowedUsers": [<user_ids>],
  "port": 3700,
  "panels": { "order": [], "disabled": [] },
  "routes": {},
  "plugins": []
}
```

## Step 3 — Write .env

```bash
echo "BOT_TOKEN=<bot_token>" > .env
chmod 600 .env
```

## Step 4 — Build

```bash
go build -o vel .
```

## Step 5 — Nginx reverse proxy

```bash
sudo tee /etc/nginx/sites-available/<domain> > /dev/null << 'EOF'
server {
    listen 80;
    server_name <domain>;

    location / {
        proxy_pass http://127.0.0.1:3700;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 86400;
    }
}
EOF

sudo ln -sf /etc/nginx/sites-available/<domain> /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

## Step 6 — Systemd service

```bash
sudo tee /etc/systemd/system/vel.service > /dev/null << EOF
[Unit]
Description=Vel App
After=network.target

[Service]
Type=simple
User=$(whoami)
WorkingDirectory=$(pwd)
EnvironmentFile=$(pwd)/.env
ExecStart=$(pwd)/vel
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now vel.service
```

## Step 7 — Verify

```bash
sleep 2 && curl -s http://localhost:3700/api/health
```

## Step 8 — Tell the user

> ✅ Your Vel app is live at `https://<domain>`
>
> **One thing to do manually** — open @BotFather:
> 1. `/mybots` → select your bot → `Bot Settings` → `Domain` → enter: `<domain>`
> 2. Optionally set Menu Button: `📊 Dashboard` → `https://<domain>/dashboard`

## Error Handling

- `go build` fails → check Go version (needs 1.22+)
- Port 3700 in use → change `port` in config.json
- Service fails → `journalctl -u vel -f`
- Telegram "Bot domain invalid" → BotFather domain not set
