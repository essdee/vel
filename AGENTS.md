# AGENTS.md — Vel Framework Agent Instructions

This file tells AI agents (Claude Code, Codex, etc.) how to work correctly with this vel deployment.

---

## Golden Rules

1. **Always run `vel verify` after deploying.** Never report "all good" without verify passing.
2. **Always run `vel verify` after making changes** that affect routes, auth, or app files.
3. **When verify fails, fix the issue, restart if needed, then re-verify.** Do not stop after one pass.
4. **When building a new app, write `verify.json`** with app-specific checks before calling it done.

---

## Workflow: Deploy → Verify → Report

```bash
# Build
go run . build

# Restart service (if needed)
sudo systemctl restart openclaw-dashboard-staging

# Verify
./vel verify --verbose

# If failed: read the log, fix, re-verify
./vel verify --json   # writes verify-log.json with full detail
cat verify-log.json
```

**Do not call a task complete until `./vel verify` exits 0.**

---

## Verify Commands

| Command | When to use |
|---------|-------------|
| `./vel verify` | Quick check — shows only failures |
| `./vel verify --verbose` | Full output — shows all checks including passed |
| `./vel verify --json` | Machine-readable output + writes `verify-log.json` |

---

## Layered Check System

Verify runs checks in layers. If an outer layer fails, inner layers are skipped:

| Layer | What it checks | Blocks if failed |
|-------|---------------|-----------------|
| Core (existing) | config.json, auth token, openclaw-cli, Telegram domain | — |
| 0 — Framework | Server reachable on configured port | Layers 1–3 |
| 1 — Auth probe | Auth mode works correctly (reject unauth, accept valid token) | Layer 2 |
| 2 — Endpoints | Key routes return expected responses | — |
| 3 — App verify.json | Per-app custom checks from `verify.json` | — |

---

## When Verify Fails

### Server not reachable (Layer 0)
```bash
# Check service status
sudo systemctl status openclaw-dashboard-staging

# Check port from config
grep '"port"' config.json

# Start if stopped
sudo systemctl start openclaw-dashboard-staging
```

### Auth probe failed (Layer 1)
- Check `auth.mode` and `auth.token` in `config.json`
- Token mode: ensure `auth.token` is set and the server is reading it
- The probe uses `/api/sources` — if that endpoint's auth changed, update the probe

### Endpoint 404 (Layer 2)
- Check `routes` in the app's `app.json`
- Ensure the app's static files or pages directory exists
- Re-run `go run . build` after adding routes

### App verify.json failure (Layer 3)
- Read the hint in the failure output
- Check the specific endpoint or file mentioned
- Fix the underlying issue (missing file, broken API, wrong path)

---

## Writing verify.json for New Apps

Every app should have a `verify.json` in its root directory. This is how agents and humans know the app is healthy after deploy.

### Schema

```json
{
  "checks": [
    {
      "type": "http_get",
      "path": "/myapp/api/status",
      "expect_status": 200,
      "expect_json_field": "status",
      "hint": "App status endpoint not responding. Check the server/register.go routes."
    },
    {
      "type": "file_exists",
      "path": "config.json",
      "relative_to": "app",
      "hint": "App config missing. Copy config.example.json to config.json."
    }
  ]
}
```

### Check types

**`http_get`** — Makes an authenticated HTTP GET request:
- `path` — URL path (e.g. `/myapp/api/status`)
- `expect_status` — expected HTTP status (default: 200)
- `expect_json_field` — if set, response body must be JSON containing this key
- Auth token is included automatically based on `config.json`

**`file_exists`** — Checks that a file or directory exists:
- `path` — path to check
- `relative_to`: `"app"` (app dir), `"root"` (vel root), `"workspace"` (~/.openclaw/workspace), `"absolute"`

---

## Common Mistakes to Watch For

### Auth mismatches
- If `auth.mode` is `"token"` but no `auth.token` is set → all API calls will fail
- If `auth.mode` is `"telegram"` but no `BOT_TOKEN` is set → Telegram login breaks
- After changing auth config: restart service + re-verify

### Missing files
- Data sources referenced in `app.json` must actually exist (verify checks `data:` entries)
- Panel manifests must be present (verify checks `panel:` entries)
- Verify.json `file_exists` checks use the `relative_to` field — wrong setting = false negatives

### Wrong paths
- App routes in `app.json` must match actual directory structure
- `static` routes serve from the named `dir` relative to the app directory
- `page` routes serve a single HTML file from the named `dir`

### Build not run after code changes
- Always run `go run . build` (or `./vel build`) after changing Go code in apps
- The binary `./vel` is what the service runs — source changes don't take effect until rebuilt

### Port mismatch
- The server reads `port` from `config.json`
- Verify reads the same field — if config and environment disagree, verify may check wrong port
- Check: `grep '"port"' config.json` and `echo $PORT`

---

## File Locations

| File | Purpose |
|------|---------|
| `config.json` | Main config (port, auth, panels) |
| `.env` | BOT_TOKEN and other secrets |
| `verify-log.json` | Written by `vel verify --json` — read this when debugging failures |
| `apps/*/verify.json` | Per-app health checks |
| `sdk/vel/deploy.sh` | Deploy script — runs verify automatically |
| `AGENTS.md` | This file |
