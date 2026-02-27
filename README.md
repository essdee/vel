<p align="center">
  <strong>வேல்</strong>
</p>

<h1 align="center">⚡ Vel</h1>

<p align="center">
  <strong>Your app runs before they finish reading this.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-0.1.0-c9a84c?style=flat-square" alt="Version">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square" alt="Go">
  <img src="https://img.shields.io/badge/TTFB-4ms-brightgreen?style=flat-square" alt="TTFB">
  <img src="https://img.shields.io/badge/RAM-2.6MB-brightgreen?style=flat-square" alt="RAM">
  <img src="https://img.shields.io/badge/binary-10MB-00ADD8?style=flat-square" alt="Binary">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License">
  <img src="https://img.shields.io/badge/tests-passing-brightgreen?style=flat-square" alt="Tests">
  <img src="https://img.shields.io/badge/PRs-welcome-brightgreen?style=flat-square" alt="PRs Welcome">
</p>

<p align="center">
  <sub>AI-native framework for real-time web apps. Single Go binary. Manifest-driven panels.<br>WebSocket-first. Hook engine. Plugin system. Telegram auth. Zero build step.</sub>
</p>

---

## The pitch

You want a real-time dashboard. Or a monitoring panel. Or an internal tool. Or an agent UI.

Here's what usually happens: install Node, install React, install a bundler, configure TypeScript, set up a WebSocket library, write auth, write middleware, wire it all together, deploy somehow. Three days later you have a loading spinner.

**Vel gives you a running app in 30 seconds:**

```bash
git clone https://github.com/essdee/vel.git
cd vel
go build -o vel .
cp config.example.json config.json
BOT_TOKEN=your-telegram-token ./vel
```

Open `localhost:3700`. Done.

### How fast?

| Metric | Vel | Typical dashboard |
|--------|-----|-------------------|
| TTFB | **4ms** | 200-500ms |
| Full page load | **76ms** | 3,000-8,000ms |
| RAM usage | **2.6MB** | 100-300MB |
| Binary size | **10MB** | 200MB+ (node_modules) |

<details>
<summary>How we measured</summary>
Chrome DevTools Protocol, Navigation Timing API, two tabs against the same server. These numbers are from a $7/month VPS.
</details>

---

## What is Vel?

Vel is an **AI-native Go framework** for building real-time panel-based web applications. Instead of pages and routes, you think in **panels** — self-contained UI components with manifest-driven configuration, live data via WebSocket, and zero build tooling.

**Core features:**
- 🔲 **Panel system** — `manifest.json` + `ui.js` = a panel. Auto-discovered. Override-capable.
- 🔌 **WebSocket-first** — All panels receive live data. No polling. No refresh.
- 🪝 **Hook engine** — WordPress-style filters + actions, Go-native.
- 🧩 **Plugin system** — `git clone` into `plugins/`, restart. Done.
- 🔐 **Telegram auth** — HMAC-SHA256, signed cookies, rate limiting. Built in.
- 🎨 **Theming** — CSS variables, custom themes, config-driven personality.
- 📦 **Single binary** — No runtime dependencies. No node_modules. Ever.

---

## Architecture

```
vel/
├── main.go              # Entrypoint — wire and go
├── internal/            # Go backend
│   ├── auth/            # Telegram HMAC + cookie signing
│   ├── data/            # Data handlers (metrics, APIs, etc.)
│   ├── hooks/           # Filter + action engine
│   ├── panels/          # Panel discovery + registry
│   ├── schema/          # Validation rules (single source of truth)
│   └── server/          # HTTP + WebSocket + middleware
├── core/
│   ├── panels/          # Built-in panels (manifest + ui.js)
│   ├── vendor/          # Preact+HTM bundle (5KB, vendored)
│   └── public/          # Shell, landing, CSS, service worker
├── custom/              # Your panels, overrides, themes (git-ignored)
├── plugins/             # Third-party plugins (git-ignored)
└── config.json          # Your config (git-ignored)
```

Decisions documented in [`ARCHITECTURE.md`](./ARCHITECTURE.md). Conventions in [`CONVENTIONS.md`](./CONVENTIONS.md).

---

## The Panel System

A panel is a folder with two files:

```
core/panels/my-panel/
├── manifest.json    # What it is — name, size, refresh interval, capabilities
└── ui.js            # What it looks like — Preact+HTM component
```

### manifest.json

```json
{
  "id": "my-panel",
  "contractVersion": "1.0",
  "name": "My Panel",
  "description": "Shows something useful",
  "version": "1.0.0",
  "author": "you",
  "position": 100,
  "size": "half",
  "refreshMs": 2000,
  "requires": [],
  "capabilities": ["fetch"],
  "dataSchema": { "type": "object", "properties": {} },
  "config": {}
}
```

### ui.js

```javascript
import { html } from '/core/vendor/preact-htm.js';

export default function MyPanel({ data, error, connected, cls }) {
  if (error) return html`<div class=${cls('error')}>${error.error}</div>`;
  if (!data) return html`<div class=${cls('loading')}>Loading...</div>`;
  return html`<div class=${cls('value')}>${data.value}</div>`;
}
```

**That's the entire API.** Drop a folder, restart, your panel appears. No registration. No config changes. Auto-discovered.

See [`CONTRACTS.md`](./CONTRACTS.md) for the full panel contract.

---

## Extension Points

### Custom panels
```bash
cp -r core/panels/uptime custom/panels/my-panel  # copy, rename, modify
```

### Override core panels
```bash
mkdir -p custom/overrides/cpu
cp core/panels/cpu/ui.js custom/overrides/cpu/ui.js  # your version wins
```

### Plugins
```bash
cd plugins/
git clone https://github.com/someone/vel-plugin-docker docker
# restart — panels auto-discovered from plugins/*/panels/
```

### Hooks (Go-native)
```go
hookEngine.AddFilter("panel.cpu.data", func(data interface{}) interface{} {
    return data // filters modify and return
})
hookEngine.On("core.server.ready", func() {
    // actions are fire-and-forget
})
```

### Config-driven routes
```json
{ "routes": { "/docs/": "custom/docs" } }
```

### Themes
```css
/* custom/theme/theme.css — loaded after core.css, your values win */
:root { --accent: #e94560; --bg: #0a0a12; }
```

---

## Config

```json
{
  "name": "My Agent",
  "emoji": "🤖",
  "accent": "#c9a84c",
  "subtitle": "Always watching",
  "role": "AI Assistant",
  "quote": "I don't wait for permission.",
  "botUsername": "mybot",
  "allowedUsers": [123456789],
  "port": 3700,
  "panels": { "order": ["cpu", "memory", "disk"] },
  "routes": {},
  "plugins": []
}
```

Your app gets a personality. Your panels get a soul.

---

## Security

Not an afterthought:

- **HMAC-SHA256** Telegram auth (timing-safe)
- **Signed httpOnly cookies** for sessions
- **Rate limiting** — auth: 10 req/15min, API: 1000 req/15min
- **Security headers** — X-Content-Type-Options, X-Frame-Options, X-XSS-Protection, Referrer-Policy
- **WebSocket authenticates on connect**
- **`allowedUsers` whitelist** — no user enumeration
- **Gzip compression** on all responses

---

## For AI Agents

Vel is designed to be extended by AI agents. The manifest-driven architecture means:

- **No ambiguity** — `manifest.json` is the contract, not comments in code
- **Copy-and-modify** — every panel is a self-contained example
- **Validation at startup** — bad manifests fail fast with Elm-quality error messages
- **Zero build step** — no webpack/vite/bundler for agents to wrestle with

> **Send your agent the repo link. It reads [`AGENT-SETUP.md`](./AGENT-SETUP.md) and handles everything.**

See [`AGENT-EXTEND.md`](./AGENT-EXTEND.md) for the full AI agent playbook.

---

## Why Vel over...

| | Vel | Grafana | Uptime Kuma | Dashy | From scratch |
|---|-----|---------|-------------|-------|-------------|
| Panel-based architecture | ✅ | ✅ | ❌ | ✅ | You build it |
| AI-agent extensible | ✅ | ❌ | ❌ | ❌ | Maybe |
| Single binary | ✅ | ❌ | ❌ | ❌ | If you try |
| WebSocket-first | ✅ | ✅ | ✅ | ❌ | You build it |
| Plugin system | ✅ | ✅ | ❌ | ❌ | You build it |
| Hook engine | ✅ | ❌ | ❌ | ❌ | You build it |
| Telegram auth built-in | ✅ | Plugin | ❌ | ❌ | You build it |
| RAM usage | 2.6MB | 200MB+ | 80MB+ | 50MB+ | Varies |
| Setup time | 30 seconds | Hours | Minutes | Minutes | Days-weeks |
| Zero build step | ✅ | ❌ | ❌ | ❌ | Unlikely |

---

## Screenshots

<table>
<tr>
<td><img src="./screenshots/landing.png" alt="Landing" width="280"></td>
<td><img src="./screenshots/dashboard.png" alt="Dashboard" width="280"></td>
</tr>
<tr>
<td align="center"><sub>Landing page</sub></td>
<td align="center"><sub>Live dashboard</sub></td>
</tr>
</table>

---

## Development

```bash
# Run tests
go test ./... -race

# Build
go build -o vel .

# Dev mode (no Telegram needed)
TEST_MODE=true BOT_TOKEN=dummy ./vel
```

CI enforces tests + docs on every PR. See [`TESTING.md`](./TESTING.md).

---

## Built with Vel

| App | Description |
|-----|-------------|
| [🦞 Clawboard](https://github.com/karthikeyan5/clawboard) | Real-time dashboard for OpenClaw AI agents — 9 panels, Claude usage monitoring, cron management |

*Building something with Vel? Open a PR to add it here.*

---

## Roadmap

See [`ROADMAP.md`](./ROADMAP.md) for the full version plan.

- **v0.1** ✅ — Panels, hooks, plugins, auth, config
- **v0.2** — SQLite store, forms, CRUD
- **v0.3** — Pages, routing, navigation
- **v0.4** — Roles, permissions
- **v0.5** — Events, files, search

---

## Docs

| Doc | What it covers |
|-----|---------------|
| [`ARCHITECTURE.md`](./ARCHITECTURE.md) | WHY decisions with "would change if" conditions |
| [`CONTRACTS.md`](./CONTRACTS.md) | Panel contract, manifest schema, hooks, CSS, errors |
| [`CONVENTIONS.md`](./CONVENTIONS.md) | Decision framework when contracts don't cover it |
| [`TESTING.md`](./TESTING.md) | Three-layer testing strategy |
| [`AGENT-EXTEND.md`](./AGENT-EXTEND.md) | AI agent playbook for extending Vel |
| [`AGENT-SETUP.md`](./AGENT-SETUP.md) | AI agent setup instructions |
| [`ROADMAP.md`](./ROADMAP.md) | Version-by-version feature plan |

---

## Contributing

PRs welcome. One panel per folder. Keep it fast. Docs ship with code.

```bash
go test ./... -race
```

---

## License

[MIT](./LICENSE)

---

<p align="center">
  <sub><strong>Vel</strong> (வேல்) — the divine spear of Murugan. Sharp. Fast. Unerring.</sub>
</p>
