<p align="center">
  <strong>வேல்</strong>
</p>

<h1 align="center">⚡ Vel</h1>

<p align="center">
  <strong>The framework where AI builds and the framework guarantees.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-0.1.0-c9a84c?style=flat-square" alt="Version">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square" alt="Go">
  <img src="https://img.shields.io/badge/tests-89-brightgreen?style=flat-square" alt="Tests">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License">
</p>

---

## Stop waiting for PRs

Every framework works the same way. You need a feature. You open an issue. Maybe someone builds it. Maybe they don't. You fork and maintain your own copy forever.

Vel is different. Tell your AI agent what you need. It builds it. The framework makes sure it can't break anything else.

**Your agent writes the app. Vel guarantees it works.**

That's not a tagline — it's the architecture. Manifest-driven apps, sandboxed capabilities, compile-time enforcement. The framework doesn't suggest how things should work. It enforces correctness structurally. AI writes two files, and it works. Every time.

---

## What Vel gives you

- **Single Go binary** — framework + all app code compiled into one binary. No runtime dependencies. No Node.js. No Python.
- **App system** — apps are folders with a manifest. They ship panels, routes, data sources, Go server code. Drop an app in, rebuild, done.
- **Panel system** — real-time UI components. `manifest.json` + `ui.js` = a live panel with WebSocket streaming, layout, and error handling built in.
- **Capability system** — apps declare what they're allowed to import. `vel build` enforces it at compile time. Your agent can't accidentally import `os/exec`.
- **Hook engine** — filters and actions, Go-native. Apps modify behaviour without touching framework code.
- **Telegram auth** — HMAC-SHA256, signed cookies, rate limiting. One config, done.

---

## Why AI-native?

AI agents write code that's 1.7× buggier than human code — especially around security, concurrency, and logic. The fix isn't better prompts. It's a framework where the insecure path doesn't exist, the wrong structure is rejected at build time, and the agent only writes the parts that matter.

Vel enforces this through compile-time capability checks, manifest-driven validation, a five-function public API (less to hallucinate), and JSON-first declarations (agents corrupt JSON less than code). Every default is the safe default. Every error tells you how to fix it.

📖 **[AI-Native Design Principles →](./AI-NATIVE.md)**

---

## Apps built on Vel

| App | What it does |
|-----|-------------|
| [Velboard](https://github.com/karthikeyan5/velboard) | The dashboard that builds itself. 9 live monitoring panels — and your agent builds the next ones. |
| [VelBridge](https://github.com/karthikeyan5/velbridge) | Your agent can use your browser. Pair with a code, watch it work. No passwords shared. |

These apps don't fork Vel. They don't conflict with each other. They compose. That's the point.

---

## How apps work

An app is a folder in `apps/` with an `app.json`:

```
apps/my-app/
├── app.json           # Manifest — name, capabilities, routes
├── panels/            # Panel folders (manifest.json + ui.js each)
└── server/            # Optional Go code (compiled into binary)
```

Apps can be panel-only (zero Go code) or ship full server-side logic. Either way, the framework handles discovery, routing, auth, streaming, and error boundaries.

Apps compose. They don't conflict. Add one, remove one — nothing else changes.

---

## The roadmap

Vel v0.1 is the foundation — apps, panels, auth, real-time streaming, capability-enforced builds. Everything structural is being frozen in v0.1 before feature work begins.

| Version | Theme | What it unlocks |
|---------|-------|-----------------|
| **v0.1** ✅ | The Foundation | Apps, panels, WebSocket, auth, `vel build`, capability system |
| **v0.2** | The Model | JSON schema → SQLite tables, auto CRUD APIs, migrations |
| **v0.3** | The Desk | Auto-generated List/Form views from models, zero frontend code |
| **v0.4** | The Guardian | Users, roles, row-level permissions |
| **v0.5** | The Automator | Background jobs, scheduler, workflow engine |
| **v0.6** | The Communicator | Email, PDF generation, webhooks |
| **v0.7** | The Portal | Website routing, blog, full-text search, i18n |
| **v0.8** | The Enterprise | Multi-tenancy, reports, dashboards, OAuth, S3 |
| **v0.9** | The Ecosystem | App lifecycle, patches, fixtures, testing framework |
| **v1.0** | The Rock | Security audit, load testing, documentation. No new features. |

By v0.9, you can build anything — accounting, inventory, CRM, HR, manufacturing, e-commerce. As apps that compose on Vel.

📋 **[Full roadmap →](https://github.com/essdee/vel-project-notes/blob/main/ROADMAP.md)**

---

## Quick start

```bash
git clone https://github.com/essdee/vel.git
cd vel
go build -o vel .
cp config.example.json config.json
BOT_TOKEN=your-telegram-token ./vel
```

Open `localhost:3700`.

### Add an app

```bash
cd apps/
git clone https://github.com/karthikeyan5/velboard.git
cd .. && ./vel build && ./vel
```

---

## v0.1 — what's here today

**5,756 lines of Go. 89 tests. 10 packages.**

### Core
| Package | What it does |
|---------|-------------|
| `internal/apps` | App discovery — scans `apps/`, parses `app.json`, loads panels/routes/data sources |
| `internal/auth` | Telegram HMAC-SHA256 + signed cookies + rate limiting + user whitelist |
| `internal/build` | `vel build` — capability scanning, AST rewriting, app compilation into single binary |
| `internal/cap` | Capability definitions — tier 1 (safe), tier 2 (declared), blacklisted |
| `internal/data` | System data handlers — CPU, memory, disk, uptime, processes, crons, models, agent status |
| `internal/datasource` | File-based data sources with configurable polling intervals |
| `internal/hooks` | Filter + action engine (Go-native) |
| `internal/panels` | Panel discovery + registry across core/custom/app directories |
| `internal/schema` | Manifest validation with Elm-quality error messages |
| `internal/server` | HTTP server + WebSocket streaming + middleware + static files |

### Public API (`pkg/vel/`)
| Function | Purpose |
|----------|---------|
| `RegisterApp(reg)` | Apps register routes from `init()` |
| `Check(r)` | Returns authenticated user from request |
| `IsAllowed(id)` | Check user against whitelist |
| `CheckBotToken(token)` | Validate bot token |
| `GetBotToken()` | Get configured bot token |

### CLI
| Command | What it does |
|---------|-------------|
| `vel start` | Start the server (default command) |
| `vel build` | Compile framework + app code into single binary |
| `vel caps list [app]` | List capabilities for all or one app |
| `vel caps export [app]` | Export capability report |
| `vel version` | Print version |

### Frontend
- **Preact + HTM** — vendored (5KB), zero build step, native ES modules
- **Responsive layout** — panels auto-arrange by size (half/full)
- **Error boundaries** — panels fail independently
- **Service worker** — offline-capable static assets

---

## Docs

| Doc | What it covers |
|-----|---------------|
| [AI-NATIVE.md](./AI-NATIVE.md) | Why Vel is designed for AI agents, and what that means |
| [CONTRACTS.md](./CONTRACTS.md) | Panel contract, manifest schema, hooks, CSS |
| [CONVENTIONS.md](./CONVENTIONS.md) | Decision framework |
| [TESTING.md](./TESTING.md) | Testing strategy |
| [AGENT-EXTEND.md](./AGENT-EXTEND.md) | AI agent playbook for building Vel apps |

---

## License

[MIT](./LICENSE)

---

<p align="center">
  <sub><strong>Vel</strong> (வேல்) — the divine spear of Murugan. Sharp. Fast. Unerring.</sub>
</p>
