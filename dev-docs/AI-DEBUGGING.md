# AI Debugging Guide for Vel

> **This document is for AI agents building on or maintaining Vel.**
> Read this before debugging any issue. Follow it strictly.

## The One Rule

**Exhaust all automated investigation before involving the human.**

The human should never be asked to "try this link", "check if this works", or "see if the error changed" when the agent has the tools to verify it. The agent is on the server. The agent has access to logs, debug endpoints, curl, config files, source code, and the database. Use them.

## Debug Infrastructure Available

### Always Available (no flags needed)
- **Structured JSON logs**: `sudo journalctl -u vel --since "5 min ago" --no-pager`
- **Request IDs**: Every response has `X-Request-ID` header. Every log line has `request_id` field.
- **Source code**: Your Vel install directory
- **Config files**: `config.json`, `users.json` in your project root
- **bbolt database**: `data/sessions.db`
- **curl**: Test any endpoint locally (`http://localhost:3700/...`) or externally (`https://your-domain.example.com/...`)
- **DNS/networking**: `host`, `nslookup`, `curl -v`, `ss -tlnp`

### Debug Server (enabled by default)
Port: `localhost:6060` — always accessible from the server.
The debug server is enabled by default and binds to `127.0.0.1` only.
To disable: set `VEL_DEBUG=0` or `"debug": {"enabled": false}` in config.json.

| Endpoint | What it tells you |
|----------|------------------|
| `GET /debug/health` | Server up? Version? Uptime? Goroutines? |
| `GET /debug/routes` | All registered routes + middleware chains |
| `GET /debug/config` | Current config (secrets redacted) |
| `GET /debug/middleware` | Ordered middleware list |
| `GET /debug/sessions` | Active session count, age |

### AI Debug Server (when VEL_AI_DEBUG=1)
| Endpoint | What it tells you |
|----------|------------------|
| `GET /debug/request/:id` | Full log for a specific request (middleware chain, handler, identity, timing) |
| `GET /debug/errors/recent?n=10` | Last N error requests with full context |
| `GET /debug/diagnose` | Token-efficient overview: errors, slow requests, auth state, buffer stats |

### External Path Testing
If your server is behind a reverse proxy:
- **Your server**: `<server-ip>` (internal: `<internal-ip>`)
- **Proxy server**: `<proxy-ip>` (runs nginx, terminates TLS)
- **your-domain.example.com** → nginx → `<internal-ip>:<port>`

Always test both paths:
```bash
# Local (bypasses proxy)
curl -s http://localhost:3700/path

# External (through proxy, like a real user)
curl -s https://your-domain.example.com/path
```

If local works but external doesn't → proxy/DNS/TLS issue.
If both fail → server/code issue.

## Mandatory Debugging Workflow

When something doesn't work, follow this order. **Do NOT skip steps.**

### Step 1: Reproduce locally
```bash
# Can you reproduce the error yourself?
curl -v http://localhost:3700/<failing-path>
curl -v https://your-domain.example.com/<failing-path>
```
If you can reproduce it → proceed to Step 2.
If you can't → the issue is client-specific (browser, cookie, cache). Check Step 5.

### Step 2: Check the debug server
```bash
# What's the server's current state?
curl -s http://localhost:6060/debug/diagnose | python3 -m json.tool

# What happened with the specific request?
curl -s http://localhost:6060/debug/errors/recent?n=5 | python3 -m json.tool

# If you have the request ID:
curl -s http://localhost:6060/debug/request/<id> | python3 -m json.tool
```

### Step 3: Check the logs
```bash
# Recent errors
sudo journalctl -u vel --since "5 min ago" --no-pager | grep -i error

# Specific path
sudo journalctl -u vel --since "5 min ago" --no-pager | grep "/failing/path"

# Session/auth issues
sudo journalctl -u vel --since "5 min ago" --no-pager | grep -i "session\|auth\|identity"
```

### Step 4: Check the full request path
```bash
# DNS: does the domain resolve to the right server?
host your-domain.example.com

# Your server's IP
curl -s ifconfig.me

# Are they the same? If not, there's a proxy.

# Is the service running?
systemctl status vel

# Is the port listening?
ss -tlnp | grep 3700

# What does the proxy see? (check nginx logs on proxy server if accessible)
```

### Step 5: Check client-specific issues
Before asking the human to do anything, check these yourself:

**Cookie/session issues:**
```bash
# Is there a valid session for this user?
curl -s http://localhost:6060/debug/sessions

# Check the session store directly
# (Can't open bbolt while server runs — use debug endpoints instead)
```

**Token/auth issues:**
```bash
# Is the token valid? Check via API
curl -s http://localhost:3700/api/auth/users -H "Authorization: Bearer <token>"

# Are there rate limits active?
curl -s http://localhost:6060/debug/diagnose | python3 -c "import json,sys; print(json.dumps(json.load(sys.stdin).get('auth_summary',{}), indent=2))"
```

**Browser-specific issues:**
- Telegram link preview consuming tokens → check for bot User-Agents in logs
- Cached responses → check `Cache-Control` headers
- Cookie domain mismatch → check `config.json` authUrl matches the domain being accessed
- Mixed content / Secure flag → check if request arrives as HTTP vs HTTPS

### Step 6: Check the code
```bash
# Only after Steps 1-5 fail to identify the issue
# Read the relevant handler
grep -A 30 "func.*HandleXxx" internal/server/server.go

# Check middleware chain
grep -n "isPublicPath\|RequireAuth" internal/server/authmiddleware.go

# Check for type mismatches, import issues
grep -rn "contextKey\|identityKey" pkg/ internal/
```

### Step 7: Fix and verify BEFORE telling the human
```bash
# Make the fix
# Build
go run . build

# Restart
sudo systemctl restart vel

# Verify the fix yourself
curl -v http://localhost:3700/<previously-failing-path>
curl -v https://your-domain.example.com/<previously-failing-path>

# Run verify
./vel verify

# Check debug server confirms fix
curl -s http://localhost:6060/debug/errors/recent?n=5

# ONLY THEN tell the human it's fixed
```

## Common Gotchas

### Context key types in Go
Two packages defining `type contextKey string` with the same value creates **different types**. `context.Value()` matches on type+value. Always use a shared constant or plain string key across packages.

### Single-use tokens
Magic links are single-use. If you test a token with curl, it's consumed. Generate a separate token for testing.

### Rate limits
Magic link creation: 5/hour/user (in-memory, resets on server restart).
Don't burn through tokens during debugging.

### Telegram link preview
Telegram fetches URLs for preview cards, consuming single-use tokens. The bot filter (returns 204 for bot User-Agents) handles this, but be aware.

### Config domain mismatch
`authUrl` in config.json determines the domain in magic link URLs. Staging config must use staging domain. Production config must use production domain.

### Proxy path
External requests go through nginx on a different server. `curl localhost:3700` and `curl https://your-domain.example.com` hit the same code but take different network paths. Always test both.

### Cookie Secure flag
`vel_session` has `Secure: true`. Browser only sends it over HTTPS. Server receives HTTP (nginx strips TLS). This is normal — `Secure` controls the browser, not the server.

## What Good Debugging Looks Like

### Bad:
1. Generate magic link → send to human → "try this" → human reports error
2. Add print statements → rebuild → "try again" → still broken
3. Generate another link → test with curl (consuming it) → "try this new one" → human tries consumed token
4. Guess it's nginx caching → "check your nginx config" → not the issue
5. Eventually find 3 stacked bugs after 45 minutes and 8+ human interactions

### Good:
1. Generate magic link → test via `curl https://your-domain.example.com/auth/magic?ml_token=...` myself
2. See 401 → check `/debug/request/:id` → see middleware consumed token
3. Fix middleware → rebuild → test again myself → see 302
4. Check `/debug/errors/recent` → see TelegramBot UA on prior request → add bot filter
5. Test by sending URL in test message → verify bot gets 204, normal request gets 302
6. Tell human: "Fixed. Here's your link." — ONE interaction

**The human's time is the most expensive resource. Protect it.**
