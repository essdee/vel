# AGENTS.md — Vel Framework Agent Instructions

This file tells AI agents (Claude Code, Codex, etc.) how to work correctly with this vel deployment.

---

## Golden Rules

1. **Always run `vel verify` after deploying.** Never report "all good" without verify passing.
2. **Always run `vel verify` after making changes** that affect routes, auth, or app files.
3. **When verify fails, fix the issue, restart if needed, then re-verify.** Do not stop after one pass.
4. **When building a new app, write `verify.json`** with app-specific checks before calling it done.
5. **When building a new app, create `testdata/default/` and `testdata/empty/`** with fixture data for all file-based data sources.
6. **Run `vel test` before committing** to ensure fixture-based integration checks pass.
7. **If an app has no `testdata/`, `vel test` will warn** but not fail — add fixtures when possible.

---

## Workflow: Deploy → Verify → Report

```bash
# Build
go run . build

# Restart service (if needed)
sudo systemctl restart vel

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
sudo systemctl status vel

# Check port from config
grep '"port"' config.json

# Start if stopped
sudo systemctl start vel
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

## Test Mode & Fixtures

### Running tests
```bash
./vel test                          # Run all fixture-based integration tests
./vel start --test-mode             # Start server with testdata/default/ fixtures
./vel start --test-mode --fixture=empty  # Use empty fixture set
./vel start --demo                  # Shortcut: --test-mode --fixture=demo
```

### When to use test mode
- Development and local testing without real data files
- Demo environments
- CI checks via `vel test` (exits 0 on pass, 1 on failure)

### Adding fixtures for a new app

If your app has file-based data sources in `app.json`, create fixture files:

```
apps/my-app/
  testdata/
    default/          ← realistic sample data (required)
      my-data.json
    empty/            ← zero-state data (required)
      my-data.json
```

The fixture filename must match the **basename** of the source path.
For example: `"path": "~/.openclaw/workspace/my-data.json"` → fixture is `testdata/default/my-data.json`.

Apps without file-based data sources still get a `testdata/default/README.md` noting no fixtures are needed.

---

## Authentication

Vel uses **session-based, multi-provider auth** with users defined in `users.json`. See `docs/AUTH.md` for full details.

### How it works

1. **Users** are defined in `users.json` (in the vel root directory)
2. **Providers** authenticate requests: Telegram (HMAC-SHA256), API Key (Bearer token), Magic Link (one-time URL)
3. **Sessions** are stored server-side in bbolt (`data/sessions.db`), referenced by `vel_session` cookie
4. **All routes are protected by default** — only explicitly public paths (like `/login`, `/api/health`, `/public/*`) skip auth

### Getting auth context in handlers

```go
import "vel/internal/server"

// Get the authenticated identity
identity := server.GetIdentity(r)
if identity != nil {
    identity.UserID   // "admin"
    identity.Name     // "Admin User"
    identity.Role     // "admin", "user", "viewer"
    identity.Provider // "telegram", "api_key", "magic_link"
}

// Legacy compatibility (still works)
user := auth.Check(r)
```

### Middleware guards

```go
server.RequireAuth(handler)           // any authenticated user
server.RequireAdmin(handler)          // admin role only
server.RequireScope("GET /api/*")(handler)  // specific scope
```

### users.json format

```json
{
  "users": [
    {
      "id": "admin",
      "name": "Admin User",
      "email": "user@example.com",
      "role": "admin",
      "identities": [{"provider": "telegram", "provider_id": "123456789"}]
    }
  ],
  "api_keys": [
    {
      "id": "usage-share",
      "name": "usage-share",
      "key_hash": "sha256:...",
      "role": "viewer",
      "scopes": ["GET /myapp/api/data"]
    }
  ]
}
```

### CLI commands

```bash
vel auth create-key --name "my-key" --role viewer --scope "GET /api/data"
vel auth list-keys
vel auth revoke-key --id my-key
vel auth magic-link --user admin --expires 30
vel auth list-users
vel auth add-user --id newuser --name "New User" --role user --telegram 987654321
```

### API keys for cross-site access

Instead of `?token=` query params, use scoped API keys with Bearer auth:

```bash
# Create a key with specific scopes
vel auth create-key --name usage-share --role viewer \
  --scope "GET /myapp/api/usage" \
  --scope "GET /myapp/api/status"

# Use it
curl -H "Authorization: Bearer vel_ak_live_..." https://example.com/myapp/api/status
```

---

## Common Mistakes to Watch For

### Auth mismatches
- If `BOT_TOKEN` is set but user's Telegram ID isn't in `users.json` → they can't log in
- If `users.json` doesn't exist → auto-migrated from legacy config on first startup
- API keys must use `Authorization: Bearer vel_ak_...` header (not `?token=` query params)
- After changing auth config or `users.json`: restart service + re-verify

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
| `users.json` | User database (identities, API keys, roles) |
| `data/sessions.db` | Server-side session store (bbolt) |
| `verify-log.json` | Written by `vel verify --json` — read this when debugging failures |
| `apps/*/verify.json` | Per-app health checks |
| `sdk/vel/deploy.sh` | Deploy script — runs verify automatically |
| `docs/AUTH.md` | Comprehensive auth documentation |
| `AGENTS.md` | This file |
