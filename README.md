<p align="center">
  <strong>வேல்</strong>
</p>

<h1 align="center">⚡ Vel</h1>

<p align="center">
  <strong>Apps your AI agent builds. Guaranteed by the framework.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-0.1.0-c9a84c?style=flat-square" alt="Version">
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square" alt="Go">
  <img src="https://img.shields.io/badge/tests-89-brightgreen?style=flat-square" alt="Tests">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License">
</p>

<p align="center">
  <em>The first framework where the primary developer is not human.</em>
</p>

---

## The Problem

Your AI agent can write code. But where does that code run? How do you know it won't break your server? How do you install someone else's agent-built tool without worrying?

Everyone's making agents smarter at writing code. Nobody's making a framework where that code is **guaranteed safe**.

## The Solution

Tell your AI agent what you need. It writes two files. Vel compiles, sandboxes, validates, and serves.

**If it builds, it works. If it doesn't build, the error tells you exactly what's wrong.**

```
Agent writes:  app.json + ui.js
Vel does:      discover → validate → compile → sandbox → serve
You get:       a live dashboard panel with real-time data
```

---

## Why Vel?

### 🤖 Agent-First
Built for AI developers from day one. JSON manifests (agents corrupt JSON less than code). 5-function public API (less to hallucinate). Elm-quality error messages. This entire framework was built through a Telegram chat — no human wrote a line of code.

### 🛡️ Structurally Safe
The wrong code won't compile. Apps declare what they import, `vel build` enforces it at compile time. `os/exec` is blocked by default. In a world where [373 OpenClaw skills were flagged as malicious](https://github.com/VoltAgent/awesome-openclaw-skills), Vel makes the unsafe path impossible.

### 🧩 Composable
Install anything. Remove anything. Nothing else breaks. Every app is independent. No dependency hell. No version conflicts. Add one, remove one — everything else keeps working.

### ⚡ Zero-Config Deploy
One Go binary. No Node.js. No Python. No Docker. No runtime dependencies. `vel build` → deploy anywhere.

---

## Apps built on Vel

| App | What it does | Install |
|-----|-------------|---------|
| [VelMetrics](https://github.com/karthikeyan5/velmetrics) | Server monitoring — CPU, memory, disk, processes | `git clone` into apps/ |
| [VelClawBoard](https://github.com/karthikeyan5/velclawboard) | OpenClaw command center — usage, sessions, models | `git clone` into apps/ |
| [VelBridge](https://github.com/karthikeyan5/velbridge) | Your agent controls your browser. Pair with a code, watch it work. | `git clone` into apps/ |

These apps don't fork Vel. They don't conflict with each other. They compose. That's the point.

---

## Quick start

```bash
# Set up project directory
mkdir my-vel-project && cd my-vel-project

# Clone the framework
git clone https://github.com/essdee/vel.git

# Configure
mkdir -p config
cp vel/config.example.json config/vel.json
# Edit config/vel.json with your bot token and user IDs

# Build and run
cd vel && go run . build && cd ..
BOT_TOKEN=your-token ./bin/vel
```

Open `localhost:3700`. That's it.

### Add an app

```bash
cd apps/
git clone https://github.com/karthikeyan5/velmetrics.git
cd ../vel && go run . build && cd ..
./bin/vel
```

Reload the page. Six new monitoring panels appear. No config. No restart. Just rebuild and serve.

---

## How apps work

An app is a folder with an `app.json`:

```
apps/my-app/
├── app.json           # Manifest — name, capabilities, routes
├── panels/            # UI panels (manifest.json + ui.js each)
└── server/            # Optional Go code (compiled into binary)
```

Apps can be **panel-only** (zero Go code, just UI) or ship **full server-side logic**. Either way: discovery, routing, auth, streaming, and error boundaries are handled.

### The capability system

Apps declare what standard library packages they need:

```json
{
  "capabilities": {
    "read": {},
    "write": {},
    "net": {}
  }
}
```

`vel build` scans the app's Go code and blocks any import not covered by declared capabilities. Your agent can't accidentally (or intentionally) import `os/exec`, `syscall`, or `unsafe`.

---

## The roadmap

v0.1 is the foundation. Everything structural is frozen. Feature work builds on stable contracts.

| Version | Theme | What it unlocks |
|---------|-------|-----------------|
| **v0.1** ✅ | The Foundation | Apps, panels, auth, WebSocket, `vel build`, capability sandbox |
| **v0.2** | The Model | JSON schema → SQLite tables, auto CRUD APIs |
| **v0.3** | The Ecosystem | App lifecycle, dependencies, patches, `vel new-app` |
| **v0.4** | The Guardian | Users, roles, row-level permissions |
| **v0.5** | The Desk | Auto-generated List/Form views from models |
| **v0.6** → **v1.0** | Scale | Jobs, email, portals, reports, enterprise features |

By v0.9, you can build anything — accounting, inventory, CRM, HR — as composable Vel apps.

📋 **[Full roadmap →](./ROADMAP.md)**

---

## v0.1 — what's here today

**5,756 lines of Go. 89 tests. 10 packages.**

- **App discovery** — drop a folder in apps/, rebuild, done
- **Panel system** — real-time WebSocket streaming, auto-layout, error boundaries
- **Capability sandbox** — compile-time import enforcement (Deno-inspired)
- **Auth** — Telegram HMAC-SHA256, API keys, magic links, signed cookies
- **Build system** — AST rewriting, capability wrappers, single binary output
- **Debug server** — localhost:6060 for diagnostics
- **Health checks** — `vel verify` + `/api/health`
- **Frontend** — Preact + HTM (5KB vendored), service worker, responsive layout

---

## For AI agents

Vel ships with comprehensive agent instructions:

- **[AGENT-SETUP.md](./AGENT-SETUP.md)** — Step-by-step install guide your agent can follow
- **[AGENT-EXTEND.md](./AGENT-EXTEND.md)** — How to build apps on Vel
- **[AI-NATIVE.md](./AI-NATIVE.md)** — Why Vel is designed this way

Every error message tells your agent **how to fix it**, not just what broke. Convention over configuration means fewer decisions, fewer mistakes, fewer tokens wasted.

---

## Docs

| Doc | What it covers |
|-----|---------------|
| [AI-NATIVE.md](./AI-NATIVE.md) | Design principles for AI-native development |
| [CONTRACTS.md](./CONTRACTS.md) | Panel contracts, manifest schema, hooks, CSS |
| [CONVENTIONS.md](./CONVENTIONS.md) | Decision framework and naming |
| [TESTING.md](./TESTING.md) | Testing strategy and conventions |
| [AGENT-EXTEND.md](./AGENT-EXTEND.md) | Build your first Vel app |
| [AUTH.md](./docs/AUTH.md) | Comprehensive authentication & authorization reference |

---

## License

[MIT](./LICENSE)

---

<p align="center">
  <sub><strong>Vel</strong> (வேல்) — the divine spear of Murugan. Sharp. Fast. Unerring.</sub>
</p>
