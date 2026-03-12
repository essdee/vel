# AGENT-EXTEND.md — AI Agent Playbook

**For AI agents:** How to extend Vel apps on behalf of your human. Create custom panels, override core panels, register hooks, add routes, install apps, and create themes.

---

## ⚠️ Docs Ship With Code — Mandatory

Every code change **must** include corresponding documentation updates:

- New panel → update AGENT-EXTEND.md with usage example
- Contract change → update CONTRACTS.md
- Architecture change → update ARCHITECTURE.md
- Testing change → update TESTING.md
- Breaking change → update BREAKING_CHANGES.md

Enforced by CI — see `.github/workflows/docs-check.yml`.

---

## Architecture Overview

See [`ARCHITECTURE.md`](./ARCHITECTURE.md) for WHY decisions.

Extension points: `custom/panels/`, `custom/overrides/`, `custom/theme/`, `apps/`, config-driven routes, Go-native hooks.

**Override resolution (last wins):** core → custom → apps → overrides.

---

## 1. Create a Custom Panel

### Step 1: Copy an existing panel

```bash
cp -r core/panels/uptime custom/panels/my-panel
```

### Step 2: Edit manifest.json

```json
{
  "id": "my-panel",
  "name": "My Panel",
  "version": "1.0.0",
  "contractVersion": "1.0",
  "description": "What this panel shows",
  "author": "agent",
  "position": 100,
  "size": "half",
  "refreshMs": 5000,
  "requires": [],
  "capabilities": ["fetch"],
  "dataSchema": { "type": "object", "properties": {} },
  "config": {}
}
```

See [`CONTRACTS.md`](./CONTRACTS.md) for all required fields.

### Step 3: Data handler (Go)

Add a function in `internal/data/` and wire it into the panel data switch in `internal/server/server.go`:

```go
// internal/data/mypanel.go
package data

import "encoding/json"

func GetMyPanelData() json.RawMessage {
    result, _ := json.Marshal(map[string]interface{}{
        "value": 42,
        "label": "My Metric",
    })
    return result
}
```

### Step 4: Write ui.js

```javascript
import { html } from '/core/vendor/preact-htm.js';

export default function MyPanel({ data, error, connected, cls }) {
  if (error) return html`<div class=${cls('error')}>${error.error}</div>`;
  if (!data) return html`<div class=${cls('wrap')}><div class=${cls('label')}>Loading...</div></div>`;

  return html`
    <div class=${cls('wrap')}>
      ${!connected && html`<div class=${cls('stale')}>⚠ Stale</div>`}
      <div class=${cls('label')}>${data.label}</div>
      <div class=${cls('value')}>${data.value}</div>
    </div>
  `;
}
```

### Rules for ui.js

See [`CONTRACTS.md`](./CONTRACTS.md) for the full contract. Key: import from `/core/vendor/preact-htm.js`, use `cls()`, handle `data === null`.

### CSS Variables

See `core/public/core.css` for the full list: `--bg`, `--card`, `--accent`, `--text`, `--text-dim`, `--red`, `--green`, `--yellow`, `--cyan`, etc.

---

## 2. Override a Core Panel

```bash
mkdir -p custom/overrides/cpu
cp core/panels/cpu/ui.js custom/overrides/cpu/ui.js
# Edit the copy — your override replaces the core panel's UI
```

**To revert:** Delete the folder in custom/overrides/.

---

## 3. Hooks

Go-native (`internal/hooks/hooks.go`):

```go
hookEngine.AddFilter("panel.cpu.data", func(data interface{}) interface{} {
    return data // must return
})

hookEngine.On("core.server.ready", func() {
    fmt.Println("Server is ready!")
})
```

| Hook Name | Type | Description |
|-----------|------|-------------|
| `core.server.init` | action | Server initializing |
| `core.server.ready` | action | Server listening |
| `core.panels.discovered` | action | All panels found |
| `panel.{id}.data` | filter | Modify panel data response |
| `config.loaded` | filter | Modify config after loading |

---

## 4. Add Custom Routes

Config-driven static file serving:

```json
{ "routes": { "/screenshots/": "custom/screenshots", "/docs/": "custom/docs" } }
```

---

## 5. Install an App

```bash
# Apps can be in apps/ or the VEL_APPS directory
cd apps/  # or: cd $VEL_APPS
git clone https://github.com/someone/vel-app-docker docker
```

App panels discovered from both `apps/*/panels/*/manifest.json` and `$VEL_APPS/*/panels/*/manifest.json`.

**If the app has Go server code** (a `server/` directory), you must run `vel build` to compile it into the binary:

```bash
cd /path/to/vel
./vel build
```

Check `app.json` for a `"server"` field to know if this is needed.

---

## 5b. Create an App with Go Server Code

Apps can ship Go server code in a `server/` directory. This lets apps register HTTP routes, WebSocket handlers, etc.

### app.json

```json
{
  "name": "my-app",
  "version": "1.0.0",
  "panels": "panels",
  "server": {
    "package": "server"
  },
  "capabilities": {
    "net": {}
  }
}
```

### server/register.go

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
    mux.HandleFunc("/my-app/hello", func(w http.ResponseWriter, r *http.Request) {
        user := vel.Check(r) // auth check
        if user == nil {
            http.Error(w, "Unauthorized", 401)
            return
        }
        w.Write([]byte("Hello!"))
    })
}
```

### Build and run

```bash
./vel build   # compiles app server code into binary
./vel start   # or just ./vel
```

**Public API** (`vel/pkg/vel`):
- `vel.RegisterApp(reg)` — register routes from `init()`
- `vel.Check(r)` — get authenticated user from request
- `vel.IsAllowed(id)` — check if user ID is allowed
- `vel.CheckBotToken(token)` — validate bot token
- `vel.GetBotToken()` — get configured bot token

---

## 6. Create a Theme

```css
/* custom/theme/theme.css — loaded after core.css, your values win */
:root {
  --accent: #e94560;
  --bg: #0a0a12;
}
```

---

## Error Handling

- Panel data handler panics → error JSON (server stays up)
- Panel ui.js throws → ErrorBoundary shows error card
- Hook panics → logged, other hooks continue

**The app never goes down because of custom/app code.**

---

## Testing

```bash
go test ./...
go build -o /dev/null .
TEST_MODE=true BOT_TOKEN=dummy ./vel
curl http://localhost:3700/api/panels/my-panel
```

---

## Conventions

1. **NEVER edit anything in `core/`** — use custom/ or apps/
2. **Panel IDs:** lowercase, hyphens only
3. **Use `cls()`** for scoped class names
4. **Use CSS variables** for colors
5. **Return data from filters** — forgetting `return` silently drops data
6. **Test before deploying** — `go test ./...`
