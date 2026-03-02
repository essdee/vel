# ⚡ Vel Framework — Agent Setup Guide

**For AI agents:** Step-by-step instructions to install and configure the Vel dashboard framework.

Vel is a modular dashboard framework written in Go. It provides:
- Telegram-based authentication
- A panel system (core + app panels)
- Data source polling (file-based)
- App discovery (`apps/` directory)
- Static shell with WebSocket live updates

---

## Step 0 — Ask the user

1. **Server domain** — Where will this be hosted? (e.g., `dashboard.example.com`)
2. **Telegram Bot Token** — Create one via [@BotFather](https://t.me/BotFather)
3. **Telegram User IDs** — Who should have dashboard access? (numeric IDs)
4. **Agent name & emoji** — For the landing page
5. **Port** — Default is `3700`
6. **Install directory** — Default: `/opt/vel`

---

## Step 1 — Prerequisites

```bash
# Go 1.24+
go version || {
  curl -LO https://go.dev/dl/go1.24.1.linux-amd64.tar.gz
  sudo tar -C /usr/local -xzf go1.24.1.linux-amd64.tar.gz
  export PATH=$PATH:/usr/local/go/bin
  echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
}
```

---

## Step 2 — Clone and build

```bash
git clone https://github.com/essdee/vel.git <install-dir>
cd <install-dir>
go build -o vel .
```

> **Note:** If you install apps with server-side Go code (e.g., VelBridge), you must rebuild using `go run . build` instead of `go build`. See Step 4.

---

## Step 3 — Configure

```bash
cp config.example.json config.json
```

Edit `config.json`:

```json
{
  "name": "<agent-name>",
  "emoji": "⚡",
  "subtitle": "Built with Vel",
  "role": "AI Agent",
  "accent": "#c9a84c",
  "accentName": "gold",
  "botUsername": "<bot-username>",
  "authUrl": "https://<domain>/auth/telegram/callback",
  "telegramLink": "https://t.me/<bot-username>",
  "allowedUsers": [<user-ids>],
  "port": <port>,
  "panels": {
    "order": [],
    "disabled": []
  }
}
```

Set the bot token and fetch the bot username (don't guess it):

```bash
echo "BOT_TOKEN=<token>" > .env

# Get the actual bot username from Telegram API
BOT_USERNAME=$(curl -s "https://api.telegram.org/bot<token>/getMe" | python3 -c "import sys,json; print(json.load(sys.stdin)['result']['username'])")
echo "Bot username: $BOT_USERNAME"
```

Use `$BOT_USERNAME` in the config below — do NOT guess the username format.

---

## Step 4 — Install apps (optional)

Vel discovers apps from the `apps/` directory. Each app is a subdirectory with an `app.json`.

```bash
cd <install-dir>/apps/

# Example: install Velboard (monitoring panels)
git clone https://github.com/karthikeyan5/velboard.git

# Example: install VelBridge (browser relay)
git clone https://github.com/karthikeyan5/velbridge.git
```

After adding apps, rebuild to include their server-side code:

```bash
cd <install-dir> && go run . build --mode=bypass
```

This scans apps for Go server packages, generates imports, and compiles a single binary. Use `--mode=bypass` to skip strict capability checks during initial setup.

---

## Step 5 — Test locally

```bash
cd <install-dir>
./vel
# Should print:
# [Config] Loaded config.json
# ┌─ App Report ──────────────
# │ Loaded: N
# ...
# [Server] Vel v0.1.0 running on http://0.0.0.0:<port>
```

Visit `http://localhost:<port>` to verify.

---

## Step 6 — systemd service

Create `/etc/systemd/system/vel.service`:

```ini
[Unit]
Description=Vel Dashboard
After=network.target

[Service]
Type=simple
WorkingDirectory=<install-dir>
ExecStart=<install-dir>/vel
EnvironmentFile=<install-dir>/.env
Restart=always
RestartSec=5
User=<username>
Environment=HOME=/home/<username>  # REQUIRED: systemd doesn't always set HOME correctly
Environment=WORKSPACE=/home/<username>/.openclaw/workspace  # REQUIRED: panels read data files from this directory

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable vel
sudo systemctl start vel
sudo systemctl status vel
```

---

## Step 7 — Expose to the internet

**⏸️ STOP — Ask the user how they want to expose the dashboard before proceeding.**

> How would you like to expose the dashboard to the internet?
>
> 1. **Nginx + Let's Encrypt** — if you already have nginx installed
> 2. **Caddy** — automatic HTTPS, simpler config
> 3. **Cloudflare Tunnel** — no open ports needed, zero-trust
> 4. **Direct / I already have a reverse proxy** — just tell me the port
> 5. **I don't know, guide me** — I'll check your setup and recommend the simplest option

**Wait for the user's answer before proceeding.** If they pick option 5, check what's already installed (`which nginx`, `which caddy`, `which cloudflared`) and recommend **Caddy** for simplicity if nothing is installed.

---

### Option A: Nginx + Let's Encrypt

```bash
sudo apt install -y nginx certbot python3-certbot-nginx
```

Create `/etc/nginx/sites-available/vel`:

```nginx
server {
    listen 80;
    server_name <domain>;

    location / {
        proxy_pass http://127.0.0.1:<port>;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

```bash
sudo ln -sf /etc/nginx/sites-available/vel /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d <domain>
```

### Option B: Caddy

```bash
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update && sudo apt install -y caddy
```

Add to `/etc/caddy/Caddyfile`:

```
<domain> {
    reverse_proxy localhost:<port>
}
```

Caddy handles HTTPS automatically. Restart:

```bash
sudo systemctl restart caddy
```

### Option C: Cloudflare Tunnel

See [Cloudflare Tunnel docs](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/). Create a tunnel pointing to `http://localhost:<port>`.

### Option D: Direct / Existing reverse proxy

Vel runs on `http://localhost:<port>`. Point your existing reverse proxy there.

> **⚠️ WebSocket support is required** for all reverse proxy options. Make sure your proxy forwards `Upgrade` and `Connection` headers — without this, live dashboard updates and relay connections will fail.

---

## Step 8 — Telegram Bot Setup

### A) Set the Menu Button (automated)

Run this to add a "📊 Dashboard" button to the bot's chat menu:

```bash
curl -s -X POST "https://api.telegram.org/bot<BOT_TOKEN>/setChatMenuButton" \
  -H "Content-Type: application/json" \
  -d '{
    "menu_button": {
      "type": "web_app",
      "text": "📊 Dashboard",
      "web_app": {"url": "https://<domain>/dashboard"}
    }
  }'
```

### B) Set the Login Widget Domain (manual — no API)

**⏸️ Ask the user to do this step manually and confirm when done:**

> Open **@BotFather** → `/mybots` → select your bot → **Bot Settings** → **Domain** → enter: `<domain>`
>
> This is required for the Telegram Login Widget to work. There is no API for this — it must be done in BotFather.
>
> Let me know when you've done this.

---

## Directory Structure

```
<install-dir>/
├── main.go              # Entry point
├── config.json          # User configuration
├── .env                 # BOT_TOKEN=...
├── core/
│   ├── public/          # Static assets (shell.html, sw.js)
│   ├── panels/          # Core panels (built-in)
│   └── vendor/          # Vendored JS libraries
├── internal/
│   ├── apps/            # App discovery
│   ├── auth/            # Telegram auth
│   ├── data/            # Data layer
│   ├── datasource/      # File polling data sources
│   ├── hooks/           # Event hooks
│   ├── panels/          # Panel registry
│   ├── schema/          # Panel manifest schema
│   └── server/          # HTTP server
├── apps/                # Installed apps (each with app.json)
│   ├── velboard/        # Monitoring panels
│   └── velbridge/        # Browser relay
└── custom/              # Custom static pages
```

## App Format

Each app needs an `app.json`:

```json
{
  "name": "myapp",
  "version": "1.0.0",
  "title": "My App",
  "description": "What it does",
  "panels": "panels",
  "server": {
    "package": "server"
  }
}
```

- `panels` — directory containing panel subdirectories (each with `manifest.json` + `ui.js`)
- `server.package` — Go package with an `init()` that calls `vel.RegisterApp()`

---

## Troubleshooting

- **"BOT_TOKEN required"** → Set `BOT_TOKEN` in `.env` or environment
- **No panels showing** → Check `config.json` `panels.order` includes panel IDs
- **App not discovered** → Verify `apps/<name>/app.json` exists and is valid JSON
- **WebSocket disconnects** → Ensure nginx has `Upgrade`/`Connection` proxy headers
- **Auth fails** → Verify domain matches `authUrl` and BotFather `/setdomain`
