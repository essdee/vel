# BUILDING-APPS.md — AI Agent Playbook

**For AI agents:** How to extend Vel apps on behalf of your human. Create custom panels, override core panels, register hooks, add routes, install apps, and create themes.

---

## ⚠️ Docs Ship With Code — Mandatory

Every code change **must** include corresponding documentation updates:

- New panel → update BUILDING-APPS.md with usage example
- Contract change → update CONTRACTS.md
- Architecture change → update ARCHITECTURE.md
- Testing change → update TESTING.md
- Breaking change → note in CHANGELOG.md

---

## Architecture Overview

See [`ARCHITECTURE.md`](./ARCHITECTURE.md) for WHY decisions.

Extension points: `custom/panels/`, `custom/overrides/`, `custom/theme/`, `apps/`, config-driven routes, Go-native hooks.

**Override resolution (last wins):** core → custom → apps → overrides.

---

## Build Your First App

A step-by-step tutorial to build an app called **hello** from scratch. You'll create a panel, add a file-based data source, add Go server code, and build everything into the Vel binary.

### Prerequisites

- Vel project cloned at `/opt/vel-staging/`
- Go installed (`go version` works)
- You can run `cd /opt/vel-staging/vel && go run . build`

### Step 1: Create the app directory

```bash
mkdir -p /opt/vel-staging/apps/hello
cd /opt/vel-staging/apps/hello
git init
```

Expected output:
```
Initialized empty Git repository in /opt/vel-staging/apps/hello/.git/
```

### Step 2: Create app.json

Write `/opt/vel-staging/apps/hello/app.json`:

```json
{
  "name": "hello",
  "version": "1.0.0",
  "title": "Hello",
  "description": "A minimal Vel app — panel, data source, and server route",
  "panels": "panels"
}
```

Fields:
- `name` — app ID, must match directory name
- `panels` — subdirectory containing panel folders
- No `server` field yet — we'll add that in Step 6

### Step 3: Create a panel

Create the panel directory:

```bash
mkdir -p /opt/vel-staging/apps/hello/panels/hello-message
```

Write `/opt/vel-staging/apps/hello/panels/hello-message/manifest.json`:

```json
{
  "id": "hello-message",
  "contractVersion": "1.0",
  "name": "Hello Message",
  "description": "Displays a greeting message",
  "version": "1.0.0",
  "author": "agent",
  "position": 200,
  "size": "half",
  "refreshMs": 5000,
  "requires": [],
  "capabilities": ["fetch"],
  "dataSchema": {
    "type": "object",
    "properties": {
      "message": { "type": "string" },
      "timestamp": { "type": "string" }
    },
    "required": ["message"]
  },
  "rateLimit": {
    "windowMs": 60000,
    "max": 30
  },
  "config": {}
}
```

Key choices:
- `position: 200` — app panels should use 200+ (core: 10-90, custom: 100+)
- `size: "half"` — takes half the dashboard width
- `id` must match the folder name (`hello-message`)

Write `/opt/vel-staging/apps/hello/panels/hello-message/ui.js`:

```javascript
import { html } from '/core/vendor/preact-htm.js';

export default function HelloMessagePanel({ data, error, connected, cls }) {
  if (error) return html`<div class=${cls('error')}>${error.error}</div>`;
  if (!data) return html`<div class=${cls('wrap')}><div class=${cls('label')}>Loading...</div></div>`;

  return html`
    <div class=${cls('wrap')} style="text-align: center; padding: 1.5rem;">
      ${!connected && html`<div class=${cls('stale')}>⚠ Stale</div>`}
      <div class=${cls('icon')} style="font-size: 2rem; margin-bottom: 0.5rem;">👋</div>
      <div class=${cls('label')} style="color: var(--text-dim); font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em;">Hello App</div>
      <div class=${cls('value')} style="color: var(--accent); font-size: 1.25rem; margin-top: 0.5rem;">${data.message}</div>
      ${data.timestamp && html`
        <div class=${cls('sub')} style="color: var(--text-dim); font-size: 0.7rem; margin-top: 0.75rem;">${data.timestamp}</div>
      `}
    </div>
  `;
}
```

Rules followed:
- Import only from `/core/vendor/preact-htm.js`
- Component name = PascalCase of panel ID: `hello-message` → `HelloMessagePanel`
- Use `cls()` for all class names
- Use CSS variables (`var(--accent)`, `var(--text-dim)`) for colors
- Handle `error`, `data === null` (loading), and `!connected` (stale)

### Step 4: Build and verify

```bash
cd /opt/vel-staging/vel
go run . build
```

Expected output (includes your app):
```
... building ...
```

Test the panel is discovered:

```bash
cd /opt/vel-staging
BOT_TOKEN=dummy TEST_MODE=true ./bin/vel &
sleep 2
curl -s http://localhost:3700/api/panels | grep hello-message
kill %1
```

You should see `hello-message` in the panel list. The panel data endpoint (`/api/panels/hello-message`) will return an error since there's no data handler yet — that's expected.

### Step 5: Add a file-based data source

Data sources let panels consume data from files on disk. The Vel datasource manager polls the file and pushes updates via WebSocket.

Create the data file:

```bash
mkdir -p /opt/vel-staging/apps/hello/data
```

Write `/opt/vel-staging/apps/hello/data/hello.json`:

```json
{
  "message": "Hello from Vel!",
  "timestamp": "2026-01-01T00:00:00Z"
}
```

Update `/opt/vel-staging/apps/hello/app.json` to declare the data source:

```json
{
  "name": "hello",
  "version": "1.0.0",
  "title": "Hello",
  "description": "A minimal Vel app — panel, data source, and server route",
  "panels": "panels",
  "data_sources": {
    "hello-data": {
      "type": "file",
      "path": "data/hello.json",
      "interval": "5s"
    }
  }
}
```

- `path` is relative to the app directory
- `interval` — how often to re-read the file (minimum 1s)
- The source is registered as `hello:hello-data` internally (namespaced by app name)

Now update the panel manifest to subscribe to this data source. Edit `/opt/vel-staging/apps/hello/panels/hello-message/manifest.json` — add the `dataSource` field:

```json
{
  "id": "hello-message",
  "contractVersion": "1.0",
  "name": "Hello Message",
  "description": "Displays a greeting message",
  "version": "1.0.0",
  "author": "agent",
  "position": 200,
  "size": "half",
  "refreshMs": 5000,
  "dataSource": "hello-data",
  "requires": [],
  "capabilities": ["fetch"],
  "dataSchema": {
    "type": "object",
    "properties": {
      "message": { "type": "string" },
      "timestamp": { "type": "string" }
    },
    "required": ["message"]
  },
  "rateLimit": {
    "windowMs": 60000,
    "max": 30
  },
  "config": {}
}
```

The `dataSource` value matches the key in `data_sources` from `app.json`. The panel now receives live data from the file via WebSocket — no polling needed in `ui.js`.

Rebuild and test:

```bash
cd /opt/vel-staging/vel
go run . build
cd /opt/vel-staging
BOT_TOKEN=dummy TEST_MODE=true ./bin/vel &
sleep 2
curl -s http://localhost:3700/api/panels/hello-message
kill %1
```

Expected response:
```json
{"message":"Hello from Vel!","timestamp":"2026-01-01T00:00:00Z"}
```

Update the data file and the panel updates automatically on the next poll interval:

```bash
echo '{"message":"Updated live!","timestamp":"'$(date -Iseconds)'"}' > /opt/vel-staging/apps/hello/data/hello.json
```

### Step 6: Add Go server code

Apps can register HTTP routes by shipping Go code in a `server/` directory.

Create the server directory:

```bash
mkdir -p /opt/vel-staging/apps/hello/server
```

Write `/opt/vel-staging/apps/hello/server/register.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"time"

	vel "vel/pkg/vel"
)

func init() {
	vel.RegisterApp(vel.AppRegistration{
		Name:     "hello",
		Register: Register,
	})
}

func Register(mux *http.ServeMux, cfg vel.AppConfig) {
	mux.HandleFunc("/hello/api/greet", func(w http.ResponseWriter, r *http.Request) {
		user := vel.Check(r)
		if user == nil {
			http.Error(w, "Unauthorized", 401)
			return
		}

		name := r.URL.Query().Get("name")
		if name == "" {
			name = "World"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"greeting":  "Hello, " + name + "!",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})
}
```

Key patterns:
- `init()` calls `vel.RegisterApp()` — this is how the build system discovers your code
- `Name` must match `app.json` `name` field
- `vel.Check(r)` enforces authentication — always use it
- Route prefix: `/hello/api/` — use your app name to avoid collisions

Update `/opt/vel-staging/apps/hello/app.json` to declare the server:

```json
{
  "name": "hello",
  "version": "1.0.0",
  "title": "Hello",
  "description": "A minimal Vel app — panel, data source, and server route",
  "panels": "panels",
  "data_sources": {
    "hello-data": {
      "type": "file",
      "path": "data/hello.json",
      "interval": "5s"
    }
  },
  "server": {
    "package": "server"
  }
}
```

The `server.package` field tells `vel build` to compile the `server/` directory into the binary.

### Step 7: Build and test everything

```bash
cd /opt/vel-staging/vel
go run . build
```

The build compiles your Go server code into the binary. If there are syntax errors, you'll see them here.

Run and test:

```bash
cd /opt/vel-staging
BOT_TOKEN=dummy TEST_MODE=true ./bin/vel &
sleep 2

# Test the panel data
curl -s http://localhost:3700/api/panels/hello-message
# Expected: {"message":"Hello from Vel!","timestamp":"2026-01-01T00:00:00Z"}

# Test the server route (needs auth — use bot token in TEST_MODE)
curl -s -H "Authorization: Bearer dummy" http://localhost:3700/hello/api/greet?name=Vel
# Expected: {"greeting":"Hello, Vel!","timestamp":"2026-..."}

kill %1
```

### Final file structure

```
apps/hello/
├── app.json
├── data/
│   └── hello.json
├── panels/
│   └── hello-message/
│       ├── manifest.json
│       └── ui.js
└── server/
    └── register.go
```

### Git commit

```bash
cd /opt/vel-staging/apps/hello
git add -A
git commit -m "feat: initial hello app — panel, data source, and server route"
```

### What to do next

- **Add more panels:** Create new folders under `panels/` with `manifest.json` + `ui.js`
- **Add more routes:** Register additional handlers in `server/register.go`
- **Override core panels:** Copy to `custom/overrides/{panel-id}/`
- **Add a theme:** Create `custom/theme/theme.css`
- See the detailed sections below for each extension point.

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

Config-driven static file serving (in `config/vel.json`):

```json
{ "routes": { "/screenshots/": "custom/screenshots", "/docs/": "custom/docs" } }
```

---

## 5. Install an App

```bash
# Apps live in apps/ at the project root
cd apps/
git clone https://github.com/someone/vel-app-docker docker
```

The `VEL_APPS` environment variable is also supported for custom locations, but `apps/` in the project root is auto-discovered by default.

App panels discovered from `apps/*/panels/*/manifest.json` (and `$VEL_APPS/*/panels/*/manifest.json` if set).

**If the app has Go server code** (a `server/` directory), you must run `vel build` to compile it into the binary:

```bash
cd vel/
go run . build   # or: ../bin/vel build (if already built)
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
cd vel/
go run . build   # compiles app server code into bin/vel
cd ..
./bin/vel         # start the server
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
cd vel/
go test ./...
go build -o /dev/null .
cd ..
TEST_MODE=true BOT_TOKEN=dummy ./bin/vel
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
