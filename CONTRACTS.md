# Vel Contracts v1.0

> **LOCKED.** Breaking these = major version bump. Read before writing any panel, plugin, hook, or route.

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
| `position` | ✅ | Hint, not guarantee. Core: 10-90. Custom: 100+. Plugin: 200+. Tiebreak: alphabetical by id. User `panels.order` in config.json always wins. |
| `size` | ✅ | `"half"` or `"full"` |
| `refreshMs` | ✅ | Min 1000, max 300000 |
| `requires` | ✅ | Dependency IDs. Empty array = none |
| `capabilities` | ✅ | What the panel needs from `api` prop: `["fetch"]` for v0.1. `["fetch", "store"]` for v0.2+. Core validates against current version. |
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
| `api` | `object` | Injected helpers. v0.1: `{ fetch }`. v0.2+: adds `store`. v0.3+: adds `navigate`. |
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
- `plugin.{name}.*` — plugin hooks

**Filters:** modify and return data. If handler returns `nil`, previous value kept.

**Actions:** side effects only, return ignored.

---

## Route Contract

Routes are config-driven in `config.json`:

```json
{
  "routes": {
    "/screenshots/": "custom/screenshots",
    "/docs/": "custom/docs"
  }
}
```

**Rules:**
- Static file serving only — no programmatic handlers in config
- Reserved prefixes (don't use): `/api/panels/`, `/auth/`, `/ws/`, `/public/`, `/dashboard`
- Custom API routes require modifying Go code in `internal/server/`

---

## CSS Contract

```css
/* Use CSS variables — never hardcode colors */
:root { --bg, --bg2, --card, --card-border, --accent, --accent-dim, --accent-glow, --text, --text-dim, --text-mid, --green, --green-dim, --yellow, --yellow-dim, --red, --red-dim, --cyan, --cyan-dim }

/* Prefixes */
.p-{panel-id}-{name}    /* Panels (use cls() helper) */
.plg-{plugin-name}-{name}  /* Plugins */
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

**Custom panels do NOT receive WebSocket updates.** Custom panels must poll their own `/api/panels/{id}` endpoint using the `api.fetch()` prop. Per-panel WebSocket registration is planned for a future version.

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

1. **New `api` prop methods** — v0.2 adds `store`, v0.3 adds `navigate`. Panels that don't use new methods are unaffected.
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
