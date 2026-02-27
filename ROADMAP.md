# Vel — Roadmap

Each version adds one layer. Panels and apps written for v0.1.0 will work in v5.

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

### v1.0 — Stable Release
**Adds:** Stable API, production-ready, battle-tested contracts

**You can build:** Everything from v0.1.0 with confidence it won't break

- API surface locked — contractVersion 1.0 is final
- Performance hardened
- Documentation complete
- CI/CD pipeline solid

---

### v2 — Store + Forms
**Adds:** SQLite store, form rendering, CRUD operations, `vel doctor` CLI

**You can build:** Todo apps, trackers, note-taking, simple data management

- SQLite default store (pure Go, no CGO)
- Swappable adapters (PostgreSQL, MySQL)
- Panels declare forms in manifest.json
- Auto-migration on schema change
- `vel doctor` CLI for validation

---

### v3 — Pages + Routing
**Adds:** Multi-page apps, sidebar navigation, URL routing

**You can build:** Multi-page apps, wikis, project management tools

- Config-driven pages with panel assignments
- Auto-generated sidebar/tab navigation
- URL routing (/dashboard, /todos, /settings)
- Panel lazy-loading per page
- `/api/schema` introspection endpoint

---

### v4 — Roles + Permissions
**Adds:** Role-based access control

**You can build:** Team workspaces, shared apps, basic ERP

- Config-driven roles with user assignments
- Panel-level permission requirements
- Page filtering by role
- Row-level security for store queries

---

### v5 — Events + Files
**Adds:** Inter-panel communication, file handling, search, notifications

**You can build:** Full ERP, CRM, document management, business apps

- Event bus: `api.emit()` / `api.on()`
- Shared state across panels
- SQLite FTS5 search
- File upload + storage
- Server push notifications

---

## Principles (All Versions)

1. **Config over code** — Users customize via config.json, never edit core/
2. **Convention over configuration** — Predictable file locations, consistent contracts
3. **AI-agent-first** — Manifest-driven, validation built in, zero build step
4. **Forward compatible** — v0.1.0 apps work in v5 without changes
