# Vel — Roadmap

Each version adds one layer. Panels and apps written for v0.1.0 will work in v5.0.

---

### v0.1.0 — Panel Framework ✅ CURRENT
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

### v1.0 — Data & CLI
**Adds:** Stable app.json contract, data sources, task scheduler, `vel` CLI

**You can build:** Dashboard apps with live data from files, commands, and HTTP APIs

- Stable app.json — the contract for Vel apps is locked
- Data sources: file, exec, HTTP — panels declare what they need
- Task scheduler — periodic background jobs
- `vel` CLI — create, validate, run apps locally
- Ready for third-party dashboard apps

---

### v2.0 — Models & Storage
**Adds:** SQLite models, auto-CRUD API, migrations

**You can build:** Todo apps, trackers, note-taking, simple data management

- SQLite default store (pure Go, no CGO)
- Models defined in app.json — auto-generates CRUD endpoints
- `vel migrate` — schema migrations handled automatically
- Swappable adapters (PostgreSQL, MySQL) for later

---

### v3.0 — Pages & Scripting
**Adds:** Multi-page apps, forms, list views, Go scripting

**You can build:** Multi-page apps, wikis, project management tools

- Config-driven pages with panel assignments
- Form rendering from model definitions
- List views with filtering and sorting
- Go scripting via `vel build` — compile custom logic into the app binary
- URL routing (/dashboard, /todos, /settings)

---

### v4.0 — Roles & Permissions
**Adds:** Role-based access control, row-level security

**You can build:** Team workspaces, shared apps, basic ERP

- Config-driven roles with user assignments
- Panel-level permission requirements
- Page filtering by role
- Row-level security for model queries

---

### v5.0 — Events & Platform
**Adds:** Inter-panel events, search, file uploads, notifications, print

**You can build:** Full ERP, CRM, document management, business apps

- Event bus: `api.emit()` / `api.on()`
- Search across all models (SQLite FTS5)
- File upload + storage
- Server push notifications
- Print support for reports and documents

---

## After v5.0

Port ERPNext modules as Vel apps — HR, inventory, accounting, CRM — each as an independent app.json package.

---

## Principles (All Versions)

1. **Config over code** — Users customize via config.json, never edit core/
2. **Convention over configuration** — Predictable file locations, consistent contracts
3. **AI-agent-first** — Manifest-driven, validation built in, zero build step
4. **Forward compatible** — v0.1.0 apps work in v5.0 without changes
