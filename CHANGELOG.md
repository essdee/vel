# Changelog

All notable changes to the Vel framework will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).
Version numbers follow the [Vel versioning scheme](./README.md): `0.X.Y` where X is a roadmap milestone and Y is a breaking change counter.

## [0.1.0] — 2026-03-13

### The Foundation

First public release of the Vel framework.

### Added

**Core Framework**
- App discovery system (apps/ directory + VEL_APPS environment variable)
- Panel system with WebSocket streaming and real-time updates
- Capability sandbox: compile-time import enforcement for app code
- `vel build` — strict build with AST rewriting and capability wrappers
- `vel verify` — post-deploy health checks
- Debug server on localhost:6060
- Error logging via `vel.LogError()` + HTTP middleware
- Framework functions: `vel.SetRoot()`, `vel.RefreshUsage()`, `vel.LogError()`, `vel.InitErrorLog()`

**Authentication**
- Telegram HMAC-SHA256 verification
- API keys (`vel_ak_live_` prefix, SHA256 hashed)
- Magic links (`vel_ml_` prefix, time-limited)
- Signed session cookies (`vel_session`, Secure/SameSite=Lax/HttpOnly)
- Admin API (`/api/auth/users`, `/api/auth/keys`)

**Build System**
- Strict mode: blocked imports enforced at build time
- Capability declarations in `app.json`
- App-to-app dependency validation (`dependencies`, `optionalDependencies`)
- Auto-generated `appimports.go` for app route registration
- Single binary output with all apps compiled in

**Dashboard**
- Responsive dark theme with CSS custom properties
- Preact + HTM frontend (5KB vendored, zero build step)
- Service worker for offline-capable static assets
- Core panels: auth-settings, error-log, verify-status

**Infrastructure**
- SDK scripts: `sdk/vel/deploy.sh`, `sdk/vel/verify-cron.sh`
- SDK scripts: `sdk/openclaw/restart.sh`, `sdk/openclaw/claude-usage-poll.sh`
- Health endpoint: `/api/health` (no auth, safe for monitoring)
- Config: JSON-based with sensible defaults

**Documentation**
- AGENT-SETUP.md — step-by-step install guide for AI agents
- AGENT-EXTEND.md — app development playbook
- AI-NATIVE.md — design principles
- CONTRACTS.md — panel contracts, manifest schema
- CONVENTIONS.md — decision framework and naming
- TESTING.md — testing strategy

### Apps Available at Launch

- **VelMetrics** — Server monitoring (CPU, memory, disk, processes, uptime, crons)
- **VelClawBoard** — OpenClaw command center (usage, sessions, models, status, updates)
- **VelBridge** — Remote browser control via Chrome DevTools Protocol
