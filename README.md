<p align="center">
  <strong>வேல்</strong>
</p>

<h1 align="center">⚡ Vel</h1>

> ⚠️ **Pre-release software.** Vel is under active development and has not reached v1.0. APIs, config formats, and conventions may change between versions without notice. Use in production at your own risk.

<p align="center">
  <strong>AI-native framework for real-time web apps. Single Go binary.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-0.1.0--alpha-c9a84c?style=flat-square" alt="Version">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square" alt="Go">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License">
</p>

---

## What is Vel?

Vel is a **Go framework** for building real-time panel-based web applications. You think in **panels** — self-contained UI components with manifest-driven configuration and live data via WebSocket.

**Core features:**
- 🔲 **Panel system** — `manifest.json` + `ui.js` = a panel. Auto-discovered.
- 🔌 **WebSocket-first** — All panels receive live data.
- 🪝 **Hook engine** — Filters + actions, Go-native.
- 🧩 **App system** — Apps can ship panels, routes, data sources, themes, and **Go server code**.
- 🔐 **Telegram auth** — HMAC-SHA256, signed cookies, rate limiting.
- 📦 **Single binary** — `vel build` compiles everything (framework + app server code) into one binary.
- 🛡️ **Capability system** — Apps declare what stdlib/third-party packages they need; `vel build` enforces it.

---

## Quick Start

```bash
git clone https://github.com/essdee/vel.git
cd vel
go build -o vel .
cp config.example.json config.json
BOT_TOKEN=your-telegram-token ./vel
```

Open `localhost:3700`.

---

## Directory Structure

```
vel/
├── main.go              # Entrypoint — `vel start`, `vel build`, `vel caps`
├── internal/            # Framework internals
│   ├── apps/            # App discovery (app.json parsing)
│   ├── auth/            # Telegram HMAC + cookie signing
│   ├── build/           # `vel build` — capability scanning, AST rewriting, compilation
│   ├── cap/             # Capability definitions
│   ├── data/            # Data handlers (system metrics)
│   ├── datasource/      # File-based data sources with polling
│   ├── hooks/           # Filter + action engine
│   ├── panels/          # Panel discovery + registry
│   ├── schema/          # Manifest validation
│   └── server/          # HTTP + WebSocket + middleware
├── pkg/vel/             # Public API for apps with Go server code
│   ├── app.go           # RegisterApp, AppConfig, AppRegistration
│   └── auth.go          # Check, IsAllowed, CheckBotToken, GetBotToken
├── core/
│   ├── panels/          # Built-in panels (manifest + ui.js)
│   ├── vendor/          # Preact+HTM bundle (5KB, vendored)
│   └── public/          # Shell, landing, CSS, service worker
├── apps/                # Third-party apps (git-ignored)
└── config.json          # Site config (git-ignored)
```

---

## CLI Commands

### `vel start` (default)

Starts the server. Flags: `--port`.

```bash
./vel start --port 3700
# or just:
./vel
```

### `vel build`

Scans `apps/` for Go server code, checks import capabilities, and compiles a single binary.

```bash
./vel build                    # strict mode (default)
./vel build --mode bypass      # log violations but don't fail
./vel build --output myapp     # custom output name
./vel build --keep             # keep _build/ for debugging
```

**What `vel build` does:**
1. Discovers apps with `server/` directories containing Go code
2. Scans all imports against the capability system (tier 1 always allowed, blacklisted always blocked, tier 2 requires declaration in `app.json`)
3. Creates `_build/` directory with: real `main.go`, generated `appimports.go` (blank imports for app server packages), symlinked `internal/` and `pkg/`
4. Generates capability wrappers for tier 2 imports
5. Runs `go build`, outputs binary to project root
6. Cleans up `_build/`

### `vel caps`

List or export app capabilities.

```bash
./vel caps list              # all apps
./vel caps list myapp        # specific app
./vel caps export myapp      # export capability report
```

---

## The Panel System

A panel is a folder with two files:

```
panels/my-panel/
├── manifest.json
└── ui.js
```

Panels are auto-discovered from `core/panels/`, `custom/panels/`, and `apps/*/panels/`.

See [`CONTRACTS.md`](./CONTRACTS.md) for the full panel contract.

---

## App System

Apps live in `apps/` and are defined by `app.json`:

### Basic app (panels only)

```json
{
  "name": "my-app",
  "version": "1.0.0",
  "title": "My App",
  "panels": "panels"
}
```

### App with Go server code

```json
{
  "name": "my-app",
  "version": "1.0.0",
  "title": "My App",
  "panels": "panels",
  "routes": {
    "/my-route": {"type": "page", "dir": "pages/my-route"}
  },
  "server": {
    "package": "server"
  },
  "capabilities": {
    "net": {},
    "github.com/gorilla/websocket": {}
  }
}
```

When `"server"` is present, the app must have a `server/` directory with Go code. The server package must call `vel.RegisterApp()` from `init()`:

```go
package server

import (
    "net/http"
    vel "vel/pkg/vel"
)

func init() {
    vel.RegisterApp(vel.AppRegistration{
        Name:     "my-app",
        Register: Register,
    })
}

func Register(mux *http.ServeMux, cfg vel.AppConfig) {
    mux.HandleFunc("/my-app/api", handleAPI)
}
```

**Requires `vel build`** to compile app server code into the binary.

### app.json fields

| Field | Description |
|-------|-------------|
| `name` | Lowercase, hyphens. Must match folder name. |
| `version` | Semver |
| `title` | Display name |
| `description` | Short description |
| `panels` | Directory containing panel folders (usually `"panels"`) |
| `routes` | Map of URL path → `{type, dir}` for pages/static files |
| `data_sources` | Named data sources (`{type: "file", path, interval}`) |
| `server` | `{package: "server"}` — enables Go server code |
| `capabilities` | Map of capability/package names → config. See capability system. |
| `theme` | Path to theme CSS |

---

## Public API (`pkg/vel/`)

Apps with Go server code import `vel/pkg/vel`:

### `app.go`

```go
// RegisterApp registers app routes. Call from init().
vel.RegisterApp(vel.AppRegistration{
    Name:     "my-app",
    Register: func(mux *http.ServeMux, cfg vel.AppConfig) { ... },
})
```

`AppConfig` provides: `Name`, `Dir` (app directory), `Workspace`.

### `auth.go`

```go
vel.Check(r *http.Request) *vel.User  // Returns authenticated user or nil
vel.IsAllowed(id int64) bool          // Check if user ID is in allowedUsers
vel.CheckBotToken(token string) bool  // Validate against configured bot token
vel.GetBotToken() string              // Get the configured bot token
```

---

## Capability System

`vel build` enforces what packages app Go code can import:

| Tier | Rule | Examples |
|------|------|---------|
| **Tier 1** (always allowed) | Safe stdlib + `vel/pkg/vel` | `fmt`, `strings`, `encoding/json`, `crypto/sha256` |
| **Blacklisted** (always blocked) | Dangerous packages | `os/exec`, `syscall`, `unsafe`, `plugin`, `reflect` |
| **Tier 2** (declared) | Requires capability in `app.json` | `net/http` needs `"net"`, `os` needs `"read"` or `"write"` |
| **Third-party** | Must be listed in capabilities | `"github.com/gorilla/websocket": {}` |

Site-level overrides via `vel.yaml`:

```yaml
capabilities:
  block: ["os/exec"]
  apps:
    my-app:
      allow: ["os"]
```

---

## Production Deployment

Use `vel-prod/deploy.sh` pattern:

```bash
# git pull → vel build → deploy → restart
cd /path/to/vel
git pull
cd apps/my-app && git pull && cd ../..
./vel build
sudo systemctl restart vel
```

See [`AGENT-SETUP.md`](./AGENT-SETUP.md) for full setup with nginx, systemd, and Telegram bot configuration.

---

## Config

```json
{
  "name": "My Agent",
  "emoji": "🤖",
  "accent": "#c9a84c",
  "subtitle": "Always watching",
  "botUsername": "mybot",
  "allowedUsers": [123456789],
  "port": 3700,
  "panels": { "order": [], "disabled": [] },
  "routes": {},
  "apps": []
}
```

---

## Security

- **HMAC-SHA256** Telegram auth (timing-safe)
- **Signed httpOnly cookies** for sessions
- **Rate limiting** — auth: 10 req/15min, API: 1000 req/15min
- **Security headers** — X-Content-Type-Options, X-Frame-Options, X-XSS-Protection
- **WebSocket authenticates on connect**
- **`allowedUsers` whitelist**

---

## Development

```bash
go test ./... -race     # Run tests
go build -o vel .       # Build (no app server code)
./vel build             # Build with app server code
TEST_MODE=true BOT_TOKEN=dummy ./vel  # Dev mode
```

---

## Built with Vel

| App | Description |
|-----|-------------|
| [🦞 Clawboard](https://github.com/karthikeyan5/clawboard) | Dashboard + browser relay for OpenClaw agents — panels, Go server code for CDP relay |

---

## Docs

| Doc | What it covers |
|-----|---------------|
| [`ARCHITECTURE.md`](./ARCHITECTURE.md) | WHY decisions |
| [`CONTRACTS.md`](./CONTRACTS.md) | Panel contract, manifest schema, hooks, CSS |
| [`CONVENTIONS.md`](./CONVENTIONS.md) | Decision framework |
| [`TESTING.md`](./TESTING.md) | Testing strategy |
| [`AGENT-EXTEND.md`](./AGENT-EXTEND.md) | AI agent playbook for extending Vel |
| [`AGENT-SETUP.md`](./AGENT-SETUP.md) | AI agent setup instructions |
| [`ROADMAP.md`](./ROADMAP.md) | Version plan |

---

## License

[MIT](./LICENSE)

---

<p align="center">
  <sub><strong>Vel</strong> (வேல்) — the divine spear of Murugan. Sharp. Fast. Unerring.</sub>
</p>
