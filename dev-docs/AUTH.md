# Auth System Documentation

Vel uses a **session-based, multi-provider authentication system**. Users are defined in `users.json`, authenticated via providers (Telegram, API Key, Magic Link), and tracked with server-side sessions stored in bbolt.

---

## Architecture Overview

```
Request
  → SessionMiddleware (load session from vel_session cookie)
    → Session found & valid? Attach Identity to context
    → No session? Continue
  → AuthMiddleware (try each registered provider)
    → Provider matched? Authenticate, create session (except API keys), attach Identity
    → No match? Request is unauthenticated
  → RequireAuthPaths (enforce auth on non-public paths)
    → Public path? Pass through
    → Authenticated? Pass through
    → Browser request? Redirect to /login
    → API request? Return 401
```

### Key components

| Component | File | Purpose |
|-----------|------|---------|
| Provider interface | `internal/auth/types.go` | Extract + Authenticate contract |
| Identity | `internal/auth/types.go` | Normalized auth result (UserID, Name, Role, Scopes) |
| Session | `internal/auth/types.go` | Server-side session with Identity + expiry |
| AuthManager | `internal/auth/manager.go` | Coordinates providers, sessions, users |
| UserStore | `internal/auth/users.go` | Loads/watches/queries users.json |
| BoltSessionStore | `internal/auth/session_store.go` | bbolt-backed session persistence |
| MagicLinkStore | `internal/auth/magiclink.go` | Magic link token storage + validation |
| Middleware | `internal/server/authmiddleware.go` | Session, Auth, RequireAuth, RequireAdmin, RequireScope |

---

## Bootstrap Strategy

When setting up a new Vel deployment, you need at least one admin user to manage auth.

### Option 1: Telegram Bootstrap (Recommended)
1. Add your Telegram user ID to `users.json` with role "admin"
2. Start the server
3. Open the dashboard — Telegram auth will create your session
4. Manage everything from the Auth Settings panel

### Option 2: File Bootstrap
1. Edit `users.json` directly to add users and identities
2. The server auto-reloads the file every 30 seconds
3. No restart needed

### Option 3: Migration Bootstrap
If upgrading from old Vel auth (allowedTelegramUsers), the framework
auto-generates `users.json` from your existing config on first startup.
All migrated users get admin role.

---

## Configuration

### config.json auth section

```json
{
  "auth": {
    "session": {
      "store": "bolt",
      "cookie_name": "vel_session",
      "max_age_hours": 168
    },
    "providers": {
      "telegram": {
        "enabled": true,
        "bot_token_env": "BOT_TOKEN"
      },
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

### Environment variables

| Variable | Purpose |
|----------|---------|
| `BOT_TOKEN` | Telegram bot token (in `.env` file) |

---

## users.json

The user database lives at `users.json` in the vel root directory. It's auto-reloaded every 30 seconds when modified.

### Format

```json
{
  "users": [
    {
      "id": "karthi",
      "name": "Karthikeyan",
      "email": "karthi@essdee.fit",
      "role": "admin",
      "identities": [
        {
          "provider": "telegram",
          "provider_id": "85720317"
        }
      ]
    }
  ],
  "api_keys": [
    {
      "id": "usage-share",
      "name": "usage-share",
      "key_hash": "sha256:...",
      "role": "viewer",
      "scopes": [
        "GET /token-swap/api/usage"
      ],
      "created_by": "karthi",
      "created_at": "2026-03-10T12:00:00Z"
    }
  ]
}
```

### Fields

**User record:**
| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Canonical user ID (e.g. "karthi") |
| `name` | Yes | Display name |
| `email` | No | Email address (used for magic link login) |
| `role` | Yes | `"admin"`, `"user"`, or `"viewer"` |
| `identities` | No | Provider links (telegram, etc.) |

**API key record:**
| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Unique key identifier |
| `name` | Yes | Human-readable name |
| `key_hash` | Yes | SHA-256 hash of the key (`sha256:hex...`) |
| `role` | Yes | Role for this key's Identity |
| `scopes` | No | Restricted access scopes (empty = full access for role) |
| `created_by` | No | Who created this key |
| `created_at` | No | ISO 8601 timestamp |

### Roles

| Role | Access |
|------|--------|
| `admin` | Full access. Bypasses all scope checks. Can manage users/keys via Admin API. |
| `user` | Authenticated access to all routes (subject to scope restrictions if via API key). |
| `viewer` | Read-only access (subject to scope restrictions if via API key). |

---

## Providers

### 1. Telegram

Authenticates Telegram Mini App users via HMAC-SHA256 signature verification.

**How it works:**
1. Client sends `Authorization: tma <initData>` or `X-Telegram-Init-Data: <initData>` header
2. Provider verifies HMAC-SHA256 signature using bot token
3. Checks `auth_date` is within 24 hours
4. Looks up Telegram user ID in `users.json` via `identities[].provider_id`
5. Creates a server-side session

**Configuration:** Enabled automatically when `BOT_TOKEN` is set in `.env`.

### 2. API Key

Authenticates machine-to-machine requests using bearer tokens.

**How it works:**
1. Client sends `Authorization: Bearer vel_ak_...` or `X-API-Key: vel_ak_...` header
2. Provider SHA-256 hashes the key, looks up hash in `users.json` `api_keys`
3. Returns Identity with the key's role and scopes
4. **No session is created** — every request is independently verified

**Key format:** `vel_ak_live_<hex>` (production) or `vel_ak_test_<hex>` (test)

**Creating keys:** Use the Admin API or Auth Settings panel (see below).

**Using keys:**
```bash
curl -H "Authorization: Bearer vel_ak_live_abc123..." https://example.com/api/auth/users
```

### 3. Magic Link

Passwordless login via one-time URL tokens.

**How it works:**
1. Admin generates a magic link via Admin API, Auth Settings panel, or `sdk/vel/magic-link.sh`
2. User receives URL with `?ml_token=vel_ml_...`
3. User visits `/auth/magic?ml_token=vel_ml_...`
4. Provider SHA-256 hashes the token, validates against stored hash
5. Checks: not expired, not already used
6. Creates a session, marks token as used, redirects to dashboard

**⚠️ Critical implementation details (for agents):**
- Query parameter is **`ml_token`** (NOT `token`)
- Magic links are stored in **`data/sessions.db`** (the session store's bbolt DB, NOT a separate `auth.db`)
- The MagicLinkStore shares the bbolt database with BoltSessionStore via `sessStore.DB()`
- bbolt requires exclusive access — you cannot write tokens while the server is running
- Use `sdk/vel/magic-link.sh <user_id> [staging|production]` which handles stop/generate/restart

**Agent workflow:**
```bash
# Recommended: use the helper script
sdk/vel/magic-link.sh karthi staging
# → outputs: https://w3-ram.ai.essd.ee/auth/magic?ml_token=vel_ml_...

# Alternative: via admin API (requires existing admin session/key)
curl -X POST http://localhost:3900/api/auth/magic-link \
  -H "Authorization: Bearer vel_ak_..." \
  -H "Content-Type: application/json" \
  -d '{"user_id": "karthi", "expires_minutes": 15}'
```

**Security:**
- Single-use (deleted after first use)
- Short expiry (15 minutes default, configurable)
- Stored as SHA-256 hash (plaintext never persisted)
- Rate limited: 5 requests per hour per user
- Telegram bot previews blocked (returns 204 for bot User-Agents)

---

## Session Management

### Storage

Sessions are stored server-side in **bbolt** (an embedded key-value database).

| Setting | Default | Description |
|---------|---------|-------------|
| Database file | `data/sessions.db` | bbolt database path |
| Cookie name | `vel_session` | HTTP cookie name |
| Cookie flags | HttpOnly, Secure, SameSite=Lax, Path=/ | Security settings |
| Max age | 168 hours (7 days) | Session lifetime |

### Session content

The cookie contains only the session ID. All user data (Identity, metadata) is stored server-side in bbolt.

### Cleanup

Expired sessions and used magic links are cleaned up automatically by the AuthManager's background cleanup routine.

---

## Scoped Access

API keys can be restricted to specific HTTP method + path combinations.

### Scope format

| Format | Matches |
|--------|---------|
| `"*"` | Everything |
| `"GET /path/*"` | GET requests with path prefix |
| `"/path/*"` | Any method with path prefix |
| `"GET /path"` | GET on exact path |
| `"/path"` | Any method on exact path |

### Evaluation

Scopes are checked left-to-right. First matching scope grants access. No match = denied (403). Admin role always bypasses scope checks.

---

## Admin API Reference

All admin endpoints require the `admin` role (via session or API key).

### `GET /api/auth/users`

List all users.

```json
// Response
{
  "users": [
    {"id": "karthi", "name": "Karthikeyan", "email": "karthi@essdee.fit", "role": "admin", "identities": [...]}
  ]
}
```

### `POST /api/auth/users`

Add a new user.

```json
// Request
{
  "id": "vignesh",
  "name": "Vignesh",
  "role": "user",
  "email": "vignesh@example.com",
  "identities": [{"provider": "telegram", "provider_id": "37211163"}]
}

// Response
{"ok": true, "user": {...}}
```

### `DELETE /api/auth/users?id=vignesh`

Remove a user.

```json
// Response
{"ok": true}
```

### `GET /api/auth/keys`

List all API keys (hashes never exposed).

```json
// Response
{
  "keys": [
    {"id": "usage-share", "name": "usage-share", "role": "viewer", "scopes": [...], "created_at": "..."}
  ]
}
```

### `POST /api/auth/keys`

Create a new API key. The plaintext key is returned **once** — it cannot be retrieved later.

```json
// Request
{"name": "my-key", "role": "viewer", "scopes": ["GET /api/health"]}

// Response
{"ok": true, "key": "vel_ak_live_...", "id": "my-key"}
```

### `DELETE /api/auth/keys?id=my-key`

Revoke an API key.

```json
// Response
{"ok": true}
```

### `POST /api/auth/magic-link`

Generate a magic login link (admin only).

```json
// Request
{"user_id": "karthi", "expires_minutes": 30}

// Response
{"ok": true, "url": "https://example.com/auth/magic?ml_token=vel_ml_..."}
```

### `POST /api/auth/magic-link/request`

**Public** — request a magic link via email. Never returns the URL directly (prevents enumeration).

```json
// Request
{"email": "karthi@essdee.fit"}

// Response (always same format)
{"ok": true, "message": "If that email is registered, a login link was sent."}
```

---

## Auth Settings Panel

The **Auth Settings** panel (🔐) in the dashboard provides a GUI for all admin auth operations:

- **Users**: View, add, and delete users
- **API Keys**: View, create (with one-time key display), and revoke keys
- **Magic Links**: Generate login links for any user

The panel is admin-only — non-admin users will see an error.

---

## Public vs Protected Routes

By default, all routes require authentication. The following paths are public:

| Path | Purpose |
|------|---------|
| `/` | Root/landing page |
| `/login` | Login page |
| `/auth/login` | Login page (alias) |
| `/auth/magic` | Magic link validation |
| `/auth/telegram/callback` | Telegram login callback |
| `/auth/token` | Token auth endpoint |
| `/auth/dev` | Dev auth endpoint |
| `/auth/logout` | Logout |
| `/api/health` | Health check |
| `/api/auth` | Auth status |
| `/api/auth/magic-link/request` | Public magic link request |
| `/public/*` | Public static assets |
| `/core/vendor/*` | Vendor libraries |
| `/custom/theme/*` | Theme assets |
| `/relay/*` | VelBridge relay |

Everything else requires a valid session or API key.

---

## Getting Auth Context in App Code

### From request handlers

```go
import vel "vel/pkg/vel"

func myHandler(w http.ResponseWriter, r *http.Request) {
    identity := vel.GetIdentity(r)
    if identity == nil {
        http.Error(w, "Unauthorized", 401)
        return
    }
    fmt.Println(identity.UserID)   // "karthi"
    fmt.Println(identity.Name)     // "Karthikeyan"
    fmt.Println(identity.Role)     // "admin"
    fmt.Println(identity.Provider) // "telegram"
}
```

> **Note:** The middleware guards below use `internal/server` — they are for framework contributors only. App developers should use `vel.Check(r)` and `vel.IsAdmin(r)` from the public API.

### Middleware guards (framework-internal)

```go
mux.Handle("/protected", server.RequireAuth(myHandler))
mux.Handle("/admin", server.RequireAdmin(myHandler))
mux.Handle("/api/data", server.RequireScope("GET /api/data")(myHandler))
```

---

## Migration from Old Auth

### Automatic migration

On first startup with the new auth code:
1. If `users.json` does not exist but `config.json` has `allowedTelegramUsers`
2. Auto-generates `users.json` with all users as admin role
3. Old config fields are ignored once `users.json` exists

### Manual migration

1. Create `users.json` with your users
2. Remove `allowedTelegramUsers`, `auth.mode`, `auth.token` from `config.json`
3. Recreate scoped tokens as API keys via Admin API
4. Update integrations from `?token=` to `Authorization: Bearer` header
5. Restart the service
