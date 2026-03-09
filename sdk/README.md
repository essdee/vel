<h1 align="center">⚡ Vel SDK</h1>

<p align="center">
  <strong>Operational scripts that power dashboard features.</strong>
</p>

---

## What is this?

The `sdk/` directory contains scripts that Vel calls from API endpoints. They bridge the dashboard to external systems — things like restarting services, fetching usage data, or running diagnostics.

Apps and panels reference these scripts by path. Vel's server resolves them at `{velRoot}/sdk/`.

## Directory Structure

```
sdk/
├── README.md           ← you are here
└── openclaw/           ← OpenClaw-specific scripts
    ├── restart.sh      ← restart the gateway service
    └── claude-usage-poll.sh  ← fetch Claude Max usage via OAuth API
```

## Scripts

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

## Related: `deploy.sh`

The deploy script lives at the Vel root (not in `sdk/`) because it's a user-copied template:

```bash
cp deploy.sh.example deploy.sh && chmod +x deploy.sh
```

**Called by:** `POST /api/updates/apply` (requires auth)

**Used in:** Velboard's `updates` panel → Deploy button

**What it does:** `git pull` → `go run . build` → `systemctl restart`

---

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
