<h1 align="center">⚡ Vel Testing & Verification</h1>

<p align="center">
  <strong>Why things never break — and if they do, the agent fixes them before you notice.</strong>
</p>

---

## The Two Commands

Vel has two distinct quality gates. They serve different purposes, run at different times, and catch different problems.

| | `vel test` | `vel verify` |
|---|---|---|
| **Purpose** | Is the code correct? | Does the deployment work? |
| **When** | Development, before commit, in CI | After every deploy, on demand |
| **What it checks** | Logic, edge cases, auth rules, code coverage | Server reachable, pages load, APIs return data, files exist |
| **Catches** | Bugs in code | Environment mismatches, missing config, broken wiring |
| **Speed** | Seconds (unit tests) | Seconds (HTTP checks) |
| **Who runs it** | Developer or CI pipeline | `deploy.sh` (automatically) or agent |

**They are not interchangeable.** Tests that pass locally can still fail in production because of a missing config file, wrong auth mode, or a file path that doesn't exist on the server. That's what verify catches.

---

## Why Vel Verify Exists

Vel is an **AI-agent-first framework**. Apps are built and deployed by AI agents, not just humans. This changes the quality equation:

1. **Agents make predictable mistakes.** Wrong file paths, config mismatches, forgotten dependencies — the same categories of error, repeatedly. Verify is designed to catch exactly these.

2. **Agents can fix what they find.** Unlike a human CI pipeline that just reports red/green, an AI agent that runs `vel verify` and sees a failure can **read the hint, understand the problem, and fix it** — in the same turn, without human intervention.

3. **"All good" must mean all good.** When an agent deploys and reports success, the user trusts that. Verify is the gate that makes that trust earned. No verify pass = no "all good."

4. **The user should never discover broken deployments.** Every issue the user finds manually is a failure of the system. Verify exists to make that impossible for obvious problems.

This is the Vel guarantee: **if it's deployed and verified, it works.**

---

## `vel test` — Code Correctness

Standard Go tests. Run during development and in CI.

```bash
vel test           # Run all tests
vel test ./apps/token-swap/...  # Test one app
```

### What belongs in `vel test`

- Auth logic: wrong token returns 403, valid token returns 200
- API handlers: correct response format, error handling, edge cases
- Build system: manifests parse correctly, routes register properly
- Data logic: config read/write, file operations
- Code coverage for critical paths

### When to run

- Before every commit (locally)
- On every push (CI via GitHub Actions)
- Before tagging a release

### Key principle

Tests verify the **code does what it should**. They run against test fixtures and mocks, not live servers. If `vel test` passes, the logic is correct.

---

## `vel verify` — Deployment Sanity

Live HTTP checks against the running server. Verifies the deployment works from a user's perspective.

```bash
vel verify                    # Verify current deployment
vel verify --verbose          # Show all checks (not just failures)
vel verify --json             # Output structured JSON
vel verify --fix              # Auto-fix known issues where possible
```

### What `vel verify` checks

**Layer 0: Framework**
- Server is running and reachable
- `/api/health` returns 200
- Port is open and responding

**Layer 1: Auth**
- Can the configured auth method access the dashboard?
- One request with the configured token/credentials
- If this fails → all downstream checks skip (they'd all fail with 403)

**Layer 2: App endpoints**
- Each app's key API returns data (not errors)
- e.g., `GET /token-swap/api/status` → has `"tokens"` field
- e.g., `GET /api/sources` → returns JSON array

**Layer 3: App UI**
- Each app's main page loads
- Returns HTML, not 403/500/blank
- e.g., `GET /token-swap/` → contains `<html`
- e.g., `GET /dashboard` → contains `<html`

**Layer 4: Files**
- Required config files exist and are valid JSON
- Required data files are present and readable

### Layered blocking

If a layer fails, dependent layers are **skipped** — not run. This prevents noise:

```
✗ Layer 1: Auth failed — token rejected
  → Hint: Check auth.token in config.json
⊘ Layer 2: Skipped (blocked by auth failure)
⊘ Layer 3: Skipped (blocked by auth failure)
✓ Layer 4: Files OK
```

One root cause, one error, one hint. Not 50 cascading 403s.

### Every failure has a hint

```
✗ GET /token-swap/api/status → 403 Unauthorized
  → Hint: Auth mode is "telegram" (auto-detected from BOT_TOKEN)
    but no token is configured for browser access.
    Fix: Add "token" to auth section in config.json.
```

Hints are written for AI agents — specific enough that an agent can read the hint and fix the problem without asking the user.

### Output

Terminal (default):
```
⚡ Vel Verify

  Framework
    ✓ Server reachable (http://localhost:3700)
    ✓ /api/health → 200

  Auth
    ✓ Token auth → 200

  Endpoints
    ✓ /token-swap/api/status → 200 (has "tokens")
    ✓ /api/sources → 200

  UI
    ✓ /dashboard → HTML
    ✓ /token-swap/ → HTML

  Files
    ✓ config.json exists and valid
    ✓ apps/token-swap/app.json exists

  ✅ All checks passed (10/10)
```

JSON log (`verify-log.json`):
```json
{
  "timestamp": "2026-03-10T07:30:00Z",
  "passed": 10,
  "failed": 0,
  "skipped": 0,
  "checks": [
    {
      "layer": "framework",
      "name": "server_reachable",
      "status": "pass",
      "detail": "http://localhost:3700 → 200"
    }
  ]
}
```

---

## App-Specific Checks: `verify.json`

Each app can ship a `verify.json` in its root directory to define additional checks that `vel verify` runs:

```json
{
  "checks": [
    {
      "type": "http_get",
      "path": "/token-swap/api/status",
      "expect_status": 200,
      "expect_json_field": "tokens",
      "hint": "Token swap status endpoint not responding. Check the app is registered."
    },
    {
      "type": "file_exists",
      "path": "token-swap-config.json",
      "relative_to": "workspace",
      "hint": "No token swap config found. Add a token via the UI first."
    }
  ]
}
```

This makes verify extensible — new apps define their own checks without modifying framework code.

---

## Integration with `deploy.sh`

```bash
# In sdk/vel/deploy.sh, after restart:

echo "🔍 Verifying deployment..."
vel verify --json

if [ $? -ne 0 ]; then
    echo "❌ Verify failed! Check verify-log.json"
    # Optional: rollback
    exit 1
fi

echo "✅ Deploy complete — verified"
```

**The rule:** `deploy.sh` never exits 0 unless `vel verify` passes. An agent running deploy will see the failure, read verify-log.json, fix the issue, and re-deploy — all without human intervention.

---

## For AI Agents Building Vel Apps

If you're an AI agent building or deploying a Vel app:

1. **Write tests** (`*_test.go`) for your app's logic. Run `vel test` before committing.
2. **Write `verify.json`** in your app root. Define checks for your key endpoints and required files.
3. **After deploying, run `vel verify`.** Read the output carefully.
4. **If verify fails, fix it.** Read the hint, make the fix, restart, verify again.
5. **Only report "all good" after verify passes.** Never assume — verify.
6. **Common mistakes to watch for:**
   - Auth mode mismatch (config says one thing, environment says another)
   - Missing config files (exist in dev, not in production)
   - Wrong file paths (relative vs absolute, dev vs prod directories)
   - API endpoints returning errors because a dependency isn't set up
   - UI pages returning 403 because auth isn't configured for browser access

---

## Summary

```
vel test   → "Does my code work?"     → Run in development + CI
vel verify → "Does my deployment work?" → Run after every deploy

Tests catch bugs. Verify catches broken deployments.
Together, they guarantee: if it's deployed and verified, it works.
```
