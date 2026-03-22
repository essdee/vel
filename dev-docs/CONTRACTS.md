# Vel Contracts v1.0

> **LOCKED.** Breaking these = major version bump. Read before writing any panel, app, hook, or route.

---

## Panel Contract

### File Structure
```
{panel-id}/
├── manifest.json    ← REQUIRED
└── ui.js            ← REQUIRED (ESM, Preact+HTM)
```

`{panel-id}` = folder name = manifest id = API route segment.

Panel data is served by Go handlers in `internal/data/`. There are no per-panel server-side JS files.

### manifest.json
```json
{
  "id": "cpu",
  "contractVersion": "1.0",
  "name": "CPU Load",
  "description": "Real-time CPU usage and core count",
  "version": "1.0.0",
  "author": "core",
  "position": 10,
  "size": "half",
  "refreshMs": 2000,
  "requires": [],
  "capabilities": ["fetch"],
  "dataSchema": {
    "type": "object",
    "properties": {
      "load": { "type": "number" },
      "cores": { "type": "integer" }
    },
    "required": ["load", "cores"]
  },
  "rateLimit": {
    "windowMs": 60000,
    "max": 30
  },
  "config": {}
}
```

| Field | Required | Rule |
|-------|----------|------|
| `id` | ✅ | Must match folder name |
| `contractVersion` | ✅ | `"1.0"` — core rejects unknown |
| `name` | ✅ | Max 30 chars |
| `description` | ✅ | Max 100 chars |
| `version` | ✅ | Semver |
| `author` | ✅ | `"core"` or author name |
| `position` | ✅ | Hint, not guarantee. Core: 10-90. Custom: 100+. App: 200+. Tiebreak: alphabetical by id. User `panels.order` in `config/vel.json` always wins. |
| `size` | ✅ | `"half"` or `"full"` |
| `refreshMs` | ✅ | Min 1000, max 300000 |
| `requires` | ✅ | Dependency IDs. Empty array = none |
| `capabilities` | ✅ | What the panel needs from `api` prop: `["fetch"]` for v0.1.0. `["fetch", "store"]` for v2+. Core validates against current version. |
| `dataSchema` | ✅ | JSON Schema for panel data response. Validated at startup + TEST_MODE. |
| `rateLimit` | ❌ | Per-panel rate limit. Default: 30/min |
| `config` | ❌ | Custom config, accessible via `config.panels.{id}` |

### ui.js (ESM, Preact+HTM)
```js
import { html, useState, useEffect } from '/core/vendor/preact-htm.js';

export default function CpuPanel({ data, error, connected, lastUpdate, api, config, cls }) {
  if (error) return html`<div class=${cls('error')}>${error.error}</div>`;
  if (!data) return html`<div class=${cls('loading')}>Loading...</div>`;

  return html`
    ${!connected && html`<div class=${cls('stale')}>⚠ Stale</div>`}
    <div class=${cls('metric')}>${data.cpu}%</div>
    <div class=${cls('cores')}>${data.cores} cores</div>
  `;
}
```

**Props contract:**
| Prop | Type | Description |
|------|------|-------------|
| `data` | `object\|null` | Latest panel data. `null` before first load. Matches `dataSchema`. |
| `error` | `{error: string, code?: string, retry?: boolean}\|null` | Error from data handler or core. |
| `connected` | `boolean` | WebSocket alive? |
| `lastUpdate` | `number\|null` | Timestamp (ms) of last data push. |
| `api` | `object` | Injected helpers. v0.1.0: `{ fetch }`. v2+: adds `store`. v3+: adds `navigate`. |
| `config` | `object` | Panel config from `config.panels.{id}`. |
| `cls` | `(name) => string` | Scoped class helper. `cls('metric')` → `'p-cpu-metric'`. |

**Rules:**
- `export default` = Preact function component
- Component name = PascalCase of panel-id: `cpu` → `CpuPanel`, `claude-usage` → `ClaudeUsagePanel`
- Import ONLY from `/core/vendor/preact-htm.js`
- Use `cls()` for all CSS classes — never hardcode `.p-` prefix
- Handle `data === null` (loading) and `error` (failure) gracefully
- No direct DOM manipulation
- Shared sub-components accept `className` prop, not `cls()`

---

## Hook Contract

Hooks are Go-native (`internal/hooks/hooks.go`). Filters and actions are registered programmatically in Go.

```go
// Filters modify and return data
hookEngine.AddFilter("panel.cpu.data", func(data interface{}) interface{} {
    return data // must return
})

// Actions are fire-and-forget
hookEngine.On("core.server.ready", func() {
    // side effects only
})
```

**Naming:** `{scope}.{target}.{action}` — always 3 segments.
- `core.*` — reserved for core
- `panel.*` — panel hooks
- `custom.*` — custom hooks
- `app.{name}.*` — app hooks

**Filters:** modify and return data. If handler returns `nil`, previous value kept.

**Actions:** side effects only, return ignored.

---

## Route Contract

Routes are declared per-app in `app.json` under `"routes"`:

```json
{
  "routes": {
    "/myapp/": {
      "type": "static",
      "dir": "ui"
    },
    "/myapp/app": {
      "type": "page",
      "dir": "pages/app"
    }
  }
}
```

The route key is the URL path prefix. The route value is a Route object:

| Field | Required | Rule |
|-------|----------|------|
| `type` | ❌ | `"static"` (file server) or `"page"` (single HTML file). Omit when the route is handled by the app's server package. |
| `dir` | ❌ | Directory or file path, relative to the app dir. Required when `type` is set. |
| `public` | ❌ | `true` = route bypasses authentication. Default: `false`. |
| `cache` | ❌ | `"none"` or `"aggressive"`. Default: no-cache for `page`, 1-hour for `static`. |
| `target` | ❌ | Proxy target URL (e.g. `"http://localhost:3800"`). Used when `type` is omitted and the app proxies traffic. |

**App-level public shorthand:** Setting `"public": true` at the top level of `app.json` marks all of the app's routes as publicly accessible — equivalent to `"public": true` on every individual route.

**Public route rules:**
- `public: true` bypasses authentication for that URL prefix
- Framework validates at startup and logs each public route: `[Server] Public route: /myapp/ (from app myapp)`
- The following prefixes can **never** be made public — they are framework-owned and the server ignores the flag with a warning:
  - `/api/`
  - `/auth/`
  - `/dashboard`
  - `/ws/`
  - `/login`

**Custom API routes** require Go code in the app's server package — they cannot be declared in `app.json`.

**Examples from real apps:**

```json
// menayra/app.json — static site
"routes": {
  "/menayra/": { "type": "static", "dir": "site" }
}

// token-swap/app.json — static, no caching
"routes": {
  "/token-swap/": { "type": "static", "dir": "ui", "cache": "none" }
}

// velbridge/app.json — mix of page route and public server-handled routes
"routes": {
  "/bridge/debug/connect": { "type": "page", "dir": "pages/relay-connect" },
  "/bridge/debug/": { "public": true },
  "/bridge/proxy/": { "public": true }
}
```

---

## CSS Contract

```css
/* Use CSS variables — never hardcode colors */
:root { --bg, --bg2, --card, --card-border, --accent, --accent-dim, --accent-glow, --text, --text-dim, --text-mid, --green, --green-dim, --yellow, --yellow-dim, --red, --red-dim, --cyan, --cyan-dim }

/* Prefixes */
.p-{panel-id}-{name}    /* Panels (use cls() helper) */
.app-{app-name}-{name}  /* Apps */
.c-{name}               /* Core (reserved) */
```

**Rules:**
- Use `cls()` helper in panels — generates correct prefix
- Always use CSS variables for colors
- No `!important` (except theme overrides)
- No global selectors in panels (`body`, `*`, `div`)

---

## Error Contract

When a panel data handler fails, core wraps the error:
```json
{ "error": "Human-readable message", "code": "OPTIONAL_CODE", "retry": true }
```
Passed to ui.js as `error` prop. Panels should render error state, not crash.

---

## WebSocket Data Flow

Built-in panels receive live data via WebSocket (2-second interval). The server pushes data for all core panels in a single message.

**Custom panels without a data source do NOT receive WebSocket updates.** They must poll their own `/api/panels/{id}` endpoint using the `api.fetch()` prop. However, app panels that declare a `dataSource` in their manifest (backed by `data_sources` in `app.json`) DO receive live WebSocket updates from the datasource manager. Per-panel WebSocket registration for custom data handlers is planned for a future version.

## `refreshMs` — Currently Informational

The `refreshMs` field in manifest.json is part of the contract but currently has no effect on the server. All core panels receive data at a fixed 2-second WebSocket interval. Custom panels control their own refresh via `api.fetch()`.

## Core Guarantees

1. **Panels are unmounted, never hidden.** `useEffect` cleanup always runs.
2. **Schema validation at startup + TEST_MODE.** Skipped in production runtime.
3. **Filter chain is defensive.** `nil` returns are skipped, not propagated.
4. **Position is a hint.** Tiebreak: alphabetical. User `panels.order` overrides all.
5. **`/core/vendor/preact-htm.js` always available.** Vendored, no CDN dependency.

---

## Forward Compatibility Notes

These are **non-breaking** and may happen in minor versions:

1. **New `api` prop methods** — v2 adds `store`, v3 adds `navigate`. Panels that don't use new methods are unaffected.
2. **New reserved route prefixes** — Core may reserve new prefixes.
3. **New CSS variables** — Existing variables won't change names.
4. **Vendor bundle growth** — Existing imports remain stable.
5. **New manifest.json fields** — Optional fields may be added. Existing manifests continue to work.
6. **New capabilities** — The `capabilities` list grows per version.

---

## Breaking Changes (what bumps contractVersion to 2.0)

- Adding required fields to manifest.json
- Changing ui.js props shape (adding is OK, removing/renaming is breaking)
- Changing hook naming convention
- Changing `cls()` behavior
- Changing reserved route prefixes
- Changing CSS variable names

---

*Locked: 2026-02-27. Version 1.0.*
