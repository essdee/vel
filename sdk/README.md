<h1 align="center">⚡ Vel SDK</h1>

<p align="center">
  <strong>Operational scripts that power dashboard features.</strong>
</p>

---

## What is this?

The `sdk/` directory contains scripts that Vel calls from API endpoints. They bridge the dashboard to external systems — things like restarting services, deploying updates, or fetching usage data.

Apps and panels reference these scripts by path. Vel's server resolves them at `{velRoot}/sdk/`.

## Directory Structure

```
sdk/
├── README.md                  ← you are here
├── openclaw/                  ← OpenClaw gateway scripts
│   ├── restart.sh             ← restart the gateway service
│   └── claude-usage-poll.sh   ← fetch Claude Max usage via OAuth API
└── vel/                       ← Vel framework scripts
    └── deploy.sh              ← pull, build, restart
```

## Scripts

### `vel/deploy.sh`

Pulls latest code, builds, and restarts the Vel dashboard service.

**Called by:** `POST /api/updates/apply` (requires auth)

**Used in:** Velboard's `updates` panel → Deploy button

**What it does:**
1. Auto-detects the systemd service running from this directory
2. `git pull --ff-only` on the framework and each app in `apps/`
3. `go run . build`
4. `systemctl restart {service}`
5. Verifies the service came up

**Requirements:**
- Go installed
- Vel running as a systemd service (system or user)
- `sudo` access for `systemctl restart` (system services)

---

### `openclaw/restart.sh`

Restarts the OpenClaw gateway via `systemctl --user`.

**Called by:** `POST /api/gateway/restart` (requires full auth, 60s rate limit)

**Used in:** Velboard's `openclaw-status` panel → ⟳ Restart button

**What it does:**
1. Sets up the user systemd bus (`XDG_RUNTIME_DIR`, `DBUS_SESSION_BUS_ADDRESS`)
2. Runs `systemctl --user restart openclaw-gateway.service`
3. Waits 3 seconds, verifies the service is active
4. Reports success with PID or failure with status output

**Requirements:**
- OpenClaw installed as a user systemd service (`openclaw-gateway.service`)
- Running as the same user that owns the service

---

### `openclaw/claude-usage-poll.sh`

Fetches Claude Max subscription usage (5-hour and 7-day windows) from the Anthropic OAuth API.

**Called by:** `POST /api/usage/refresh` (requires auth)

**Used in:** Velboard's `claude-usage` panel → ↻ Refresh button, and by the `claude-usage-monitor` cron skill

**What it does:**
1. Reads OAuth credentials from `~/.claude/.credentials.json`
2. Refreshes the access token if expired
3. Calls `https://api.anthropic.com/api/oauth/usage`
4. Writes formatted JSON to `{workspace}/claude-usage.json`

**Requirements:**
- Claude Code CLI authenticated via `claude login` (NOT `setup-token` — needs `user:profile` scope)
- `jq` installed
- Output path: `$OPENCLAW_WORKSPACE/claude-usage.json` (or `~/.openclaw/workspace/claude-usage.json`)

**Environment variables:**
| Variable | Default | Description |
|---|---|---|
| `CLAUDE_USAGE_OUTPUT` | `{workspace}/claude-usage.json` | Output file path |
| `CLAUDE_CREDENTIALS_FILE` | `~/.claude/.credentials.json` | OAuth credentials |
| `OPENCLAW_WORKSPACE` | `~/.openclaw/workspace` | Workspace root |

---

## How apps use SDK scripts

Apps don't call SDK scripts directly. Instead, Vel's server exposes API endpoints that run them:

| Script | API Endpoint | Method | Auth |
|---|---|---|---|
| `vel/deploy.sh` | `/api/updates/apply` | POST | Full |
| `openclaw/restart.sh` | `/api/gateway/restart` | POST | Full (60s rate limit) |
| `openclaw/claude-usage-poll.sh` | `/api/usage/refresh` | POST | Full |

Panel UI components call these endpoints via `fetch()`. The server captures stdout/stderr and returns it as JSON:

```json
{ "ok": true, "output": "✓ Gateway restarted successfully (PID: 12345)" }
```

Or on failure:

```json
{ "ok": false, "error": "exit status 1", "output": "ERROR: service not found" }
```

## Adding new scripts

1. Create your script in `sdk/{namespace}/`
2. Add an API endpoint in `internal/server/server.go` that calls it
3. Add a UI trigger in the relevant panel
4. Document it here

Scripts should:
- Use `#!/usr/bin/env bash` and `set -euo pipefail`
- Output to stdout/stderr (the API endpoint captures both)
- Exit 0 on success, non-zero on failure
- Set up their own `PATH` and environment (they run from the Vel server process)
