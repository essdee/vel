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
    },
    {
      "id": "nithin",
      "name": "Nithin",
      "email": "nithin@essdee.fit",
      "role": "user",
      "identities": [
        {
          "provider": "telegram",
          "provider_id": "2031224178"
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
        "GET /token-swap/api/usage",
        "GET /token-swap/api/status"
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
| `admin` | Full access. Bypasses all scope checks. Can generate magic links. |
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

**Creating keys:**
```bash
vel auth create-key --name "usage-share" --role viewer \
  --scope "GET /token-swap/api/usage" \
  --scope "GET /token-swap/api/status"
```

**Using keys:**
```bash
curl -H "Authorization: Bearer vel_ak_live_abc123..." https://example.com/token-swap/api/status
```

### 3. Magic Link

Passwordless login via one-time URL tokens.

**How it works:**
1. Admin generates a magic link (CLI or API)
2. User receives URL with `?ml_token=vel_ml_...`
3. User visits `/auth/magic?ml_token=vel_ml_...`
4. Provider SHA-256 hashes the token, validates against stored hash
5. Checks: not expired, not already used
6. Creates a session, marks token as used, redirects to dashboard

**Security:**
- Single-use (deleted after first use)
- Short expiry (15 minutes default, configurable)
- Stored as SHA-256 hash (plaintext never persisted)
- Rate limited: 5 requests per hour per user

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

### Examples

```json
{
  "scopes": ["GET /token-swap/api/usage", "GET /token-swap/api/status"]
}
```

This key can only read usage and status data from the token-swap app. Admin role always bypasses scope checks.

### Evaluation

Scopes are checked left-to-right. First matching scope grants access. No match = denied (403).

---

## Getting Auth Context in App Code

### From request handlers

```go
import "vel/internal/server"

func myHandler(w http.ResponseWriter, r *http.Request) {
    // Get the authenticated identity (nil if unauthenticated)
    identity := server.GetIdentity(r)
    if identity == nil {
        http.Error(w, "Unauthorized", 401)
        return
    }

    // Use identity fields
    fmt.Println(identity.UserID)   // "karthi"
    fmt.Println(identity.Name)     // "Karthikeyan"
    fmt.Println(identity.Role)     // "admin"
    fmt.Println(identity.Provider) // "telegram"
}
```

### Legacy compatibility

The old `auth.Check(r)` function still works and returns a `*auth.User` for backward compatibility. New code should use `server.GetIdentity(r)`.

### Middleware guards

```go
// Any authenticated user
mux.Handle("/protected", server.RequireAuth(myHandler))

// Admin only
mux.Handle("/admin", server.RequireAdmin(myHandler))

// Specific scope required
mux.Handle("/api/data", server.RequireScope("GET /api/data")(myHandler))
```

---

## Public vs Protected Routes

By default, all routes require authentication. The following paths are public (no auth required):

| Path | Purpose |
|------|---------|
| `/` | Root/landing page |
| `/login` | Login page |
| `/auth/login` | Login page (alias) |
| `/auth/magic` | Magic link validation endpoint |
| `/auth/telegram/callback` | Telegram login callback |
| `/auth/token` | Token auth endpoint |
| `/auth/dev` | Dev auth endpoint |
| `/auth/logout` | Logout endpoint |
| `/api/health` | Health check |
| `/api/auth` | Auth status API |
| `/api/auth/magic-link/request` | Public magic link request (sends email, never returns URL) |
| `/favicon.ico` | Favicon |
| `/robots.txt` | Robots file |
| `/public/*` | Public static assets |
| `/core/vendor/*` | Vendor libraries |
| `/custom/theme/*` | Theme assets |
| `/relay/*` | VelBridge relay (has its own auth) |

Everything else requires a valid session or API key.

---

## CLI Reference

### `vel auth create-key`

Generate a new API key.

```bash
vel auth create-key --name "my-key" --role viewer \
  --scope "GET /api/data" \
  --scope "GET /api/status"
```

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | Yes | Unique key identifier |
| `--role` | No | Role (default: "viewer") |
| `--scope` | No | Access scopes (repeatable; default: "*") |

Outputs the plaintext key. **Store it immediately — it cannot be retrieved later.**

### `vel auth list-keys`

List all configured API keys (tokens masked).

```bash
vel auth list-keys
```

### `vel auth revoke-key`

Remove an API key.

```bash
vel auth revoke-key --id usage-share
```

### `vel auth magic-link`

Generate a magic login link for a user.

```bash
vel auth magic-link --user karthi --expires 30
```

| Flag | Required | Description |
|------|----------|-------------|
| `--user` | Yes | User ID from users.json |
| `--expires` | No | Expiry in minutes (default: 15) |

### `vel auth list-users`

List all users from users.json.

```bash
vel auth list-users
```

### `vel auth add-user`

Add a new user to users.json.

```bash
vel auth add-user --id vignesh --name "Vignesh" --role user --telegram 37211163
```

| Flag | Required | Description |
|------|----------|-------------|
| `--id` | Yes | Canonical user ID |
| `--name` | Yes | Display name |
| `--role` | No | Role (default: "user") |
| `--email` | No | Email address |
| `--telegram` | No | Telegram user ID |

---

## API Endpoints

### `GET /api/auth`

Returns current auth status and identity (if authenticated).

### `POST /api/auth/magic-link`

**Requires:** Admin role

Generate a magic link and return the URL.

```json
// Request
{"user_id": "karthi", "expires_minutes": 30}

// Response
{"url": "https://example.com/auth/magic?ml_token=vel_ml_..."}
```

### `POST /api/auth/magic-link/request`

**Public** — request a magic link via email. Never returns the URL directly.

```json
// Request
{"email": "karthi@essdee.fit"}

// Response (always 200, even if email not found — prevents enumeration)
{"message": "If an account exists with that email, a login link has been sent"}
```

### `GET /auth/magic?ml_token=vel_ml_...`

**Public** — validates a magic link token, creates a session, and redirects to the dashboard.

### `GET /auth/login` / `GET /login`

**Public** — serves the login page with configured provider options:
- "Login with Telegram" button (if telegram provider enabled)
- "Login with Email" field + button (if magic link email enabled)

### `POST /auth/logout`

Destroys the current session and clears the cookie.

### `POST /auth/telegram/callback`

Handles Telegram Login Widget callback, verifies signature, creates session.

### `POST /auth/token`

Token-based authentication (sends `Authorization: tma <initData>`).

---

## Login Page

The framework serves a login page at `/login` (and `/auth/login`). It shows:

- **Telegram login:** If the telegram provider is enabled and `BOT_TOKEN` is set, shows a "Login with Telegram" button that opens the Telegram Login Widget
- **Email login:** If magic link email is enabled, shows an email field + "Send Login Link" button

After successful authentication (via any provider), the user is redirected to the page they originally requested (stored in `?redirect=` parameter).

---

## Migration from Old Auth

### What changed

| Old (legacy) | New |
|--------------|-----|
| `config.json` `allowedTelegramUsers` array | `users.json` with full user records |
| `config.json` `auth.mode` + `auth.token` | Provider-based auth (no master token) |
| `?token=` query parameter for API access | `Authorization: Bearer vel_ak_...` header |
| Scoped tokens in `config.json` `auth.tokens` | API keys in `users.json` `api_keys` |
| Cookie: signed `tg_user` with user data | Cookie: `vel_session` with session ID only |
| In-memory user state | bbolt server-side sessions |

### Automatic migration

On first startup with the new auth code:
1. If `users.json` does not exist but `config.json` has `allowedTelegramUsers`
2. Auto-generates `users.json` with all users as admin role
3. Prints a deprecation warning
4. Old config fields are ignored once `users.json` exists

### Manual migration steps

1. Create `users.json` with your users (see format above)
2. Remove `allowedTelegramUsers`, `auth.mode`, `auth.token` from `config.json`
3. If using scoped tokens, recreate them as API keys: `vel auth create-key`
4. Update any remote integrations from `?token=` to `Authorization: Bearer` header
5. Restart the service

### Backward compatibility

The legacy `auth.Check(r)` function continues to work alongside the new system. When the new auth system is active (users.json exists), both old and new auth paths are available during the transition period.
