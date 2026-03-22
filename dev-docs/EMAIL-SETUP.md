# Email Setup for Vel (Magic Link Delivery)

> **⚠️ This entire setup is OPTIONAL.** You do NOT need email or Himalaya to use Vel.
> Magic links work without email — the agent generates links via API and delivers them through Telegram or other channels.
> Email (via Himalaya) is only needed if you want **self-service magic link email delivery** (where a user enters their email on the login page and receives the link by email).
> Most deployments skip this entirely.

## How It Works

When a user submits their email on the login page (`/api/auth/magic-link/request`), Vel generates a magic link and tries to send it via email. If email isn't configured, it gracefully degrades — the link is generated but not delivered.

## Enabling Email Delivery

### 1. Install Himalaya CLI

[Himalaya](https://github.com/pimalaya/himalaya) is a lightweight CLI email client.

```bash
# Install via cargo
cargo install himalaya

# Or download binary from releases
# https://github.com/pimalaya/himalaya/releases
```

### 2. Configure Himalaya

Create `~/.config/himalaya/config.toml`:

```toml
[accounts.default]
email = "a.ram@essdee.fit"
display-name = "Vel Auth"

[accounts.default.backend]
type = "imap"
host = "imappro.zoho.in"
port = 993
login = "a.ram@essdee.fit"
passwd = { cmd = "echo 'YOUR_PASSWORD'" }

[accounts.default.message.send]
type = "smtp"
host = "smtp.zoho.in"
port = 465
login = "a.ram@essdee.fit"
passwd = { cmd = "echo 'YOUR_PASSWORD'" }
```

### 3. Configure Vel

In `config.json`:

```json
{
  "auth": {
    "providers": {
      "magic_link": {
        "enabled": true,
        "expiry_minutes": 15,
        "email": {
          "enabled": true,
          "method": "himalaya",
          "from": "a.ram@essdee.fit"
        }
      }
    }
  }
}
```

### 4. Test

```bash
# Test Himalaya directly
echo "Test email" | himalaya send --to "test@example.com" --subject "Test"

# Test via Vel API (requires admin auth)
curl -X POST http://localhost:3900/api/auth/magic-link/request \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com"}'
```

## Without Email

Magic links still work perfectly:
- **Agent-generated**: `POST /api/auth/magic-link` (admin API) → returns URL → agent sends via Telegram
- **Auth Settings panel**: Admin generates magic link from dashboard, copies URL manually
- **Login page**: Shows Telegram login option (no email needed)

## Notes

- IMAP host: use `imappro.zoho.in` (NOT `mail.essdee.net` — that times out)
- SMTP host: `smtp.zoho.in` port 465 (SSL)
- The `/api/auth/magic-link/request` endpoint never reveals whether an email exists (prevents enumeration)
- Rate limit: 5 magic links per hour per email
