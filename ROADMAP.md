# Vel — Roadmap

Pre-release until all planned features are complete. v1.0 ships when everything through v0.6 is done.

---

### v0.1 — Panel Framework ✅ CURRENT
**Adds:** Panel architecture, hook engine, app system, Telegram auth, config-driven personality

**You can build:** Real-time dashboards, monitoring panels, status pages, agent UIs

- Manifest-driven panels (manifest.json + ui.js)
- Auto-discovery from core/, custom/, apps/
- Override system (custom/overrides/)
- WordPress-style hooks (filters + actions, Go-native)
- WebSocket live data (2-second push)
- Telegram HMAC auth + signed cookies
- Config-driven routes, theming, personality
- Elm-quality validation errors

---

### v0.2 — Data & CLI (in progress)
**Adds:** Stable app.json contract, data sources, task scheduler, `vel` CLI, app Go server code

**You can build:** Dashboard apps with live data from files, Go code, and HTTP APIs

- ✅ Data sources: file type with polling, stale tracking, JSON retry
- ✅ Panel stale badge — framework auto-shows ⚠️ on stale panels
- ✅ `dataEnvelope` manifest field — panels opt into full envelope or get clean data
- ✅ `dataSource` manifest field — panels subscribe to named sources
- ✅ `vel build` CLI — scans apps, checks capabilities, compiles binary
- ✅ `vel start` subcommand — server startup with --port flag
- ✅ Capability system — tier 1/2/3, blacklist, third-party scanning
- ✅ Apps can ship Go server code — `server/` directory, compiled via `vel build`
- ✅ Public API (`pkg/vel/`) — `RegisterApp`, `AppConfig`, auth helpers
- ✅ `vel caps` CLI — list/export app capabilities
- Task scheduler — periodic background jobs (pending)
- Ready for third-party dashboard apps

---

### v0.3 — Models & Storage
**Adds:** SQLite models, auto-CRUD API, migrations

**You can build:** Todo apps, trackers, note-taking, simple data management

- SQLite default store (pure Go, no CGO)
- Models defined in app.json — auto-generates CRUD endpoints
- `vel migrate` — schema migrations handled automatically
- Swappable adapters (PostgreSQL, MySQL) for later

---

### v0.4 — Pages & Scripting
**Adds:** Multi-page apps, forms, list views, Go scripting

**You can build:** Multi-page apps, wikis, project management tools

- Config-driven pages with panel assignments
- Form rendering from model definitions
- List views with filtering and sorting
- Go scripting via `vel build` — compile custom logic into the app binary
- URL routing (/dashboard, /todos, /settings)

---

### v0.5 — Roles & Permissions
**Adds:** Role-based access control, row-level security

**You can build:** Team workspaces, shared apps, basic ERP

- Config-driven roles with user assignments
- Panel-level permission requirements
- Page filtering by role
- Row-level security for model queries

---

### v0.6 — Events & Platform
**Adds:** Inter-panel events, search, file uploads, notifications, print

**You can build:** Full ERP, CRM, document management, business apps

- Event bus: `api.emit()` / `api.on()`
- Search across all models (SQLite FTS5)
- File upload + storage
- Server push notifications
- Print support for reports and documents

---

### v1.0 — Stable Release

All features from v0.1–v0.6 battle-tested and stable. Public release.

---

## After v1.0

Port ERPNext modules as Vel apps — HR, inventory, accounting, CRM — each as an independent app.json package.

---

## Principles (All Versions)

1. **Config over code** — Users customize via config.json, never edit core/
2. **Convention over configuration** — Predictable file locations, consistent contracts
3. **AI-agent-first** — Manifest-driven, validation built in, zero build step
4. **Forward compatible** — v0.1 apps work in v1.0 without changes
