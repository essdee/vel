# Vel — Feature Inventory

A full list of features the framework will support. Grouped by domain. Used when architecting any feature — look at the full picture, design so nothing gets blocked.

Features move from "Yet to Build" → "Built" as projects demand them.

---

## Built ✅

### Core
- App discovery system (apps/ directory, app.json manifest)
- Panel system (manifest.json + ui.js, auto-discovered, WebSocket streaming)
- Hook engine (filters + actions, Go-native)
- Config system (vel.json, project root auto-detection)
- Data sources (file-based with polling, WebSocket broadcast)
- CLI: start, build, caps, verify, test, version

### Build System
- `vel build` — compile framework + app Go code → single binary
- Capability sandbox (3-tier: always available / declarable / blacklisted)
- AST rewriting for capability wrappers
- App server code discovery (server/ subdirectory convention)
- Privileged apps (two-key system, decision approved) ⚠️ *implementation needs verification*

### Auth
- Session-first, provider-second architecture
- Telegram HMAC-SHA256 provider
- API key provider (vel_ak_ prefix, SHA-256 hashed, scoped)
- Magic link provider (vel_ml_ prefix, time-limited, single-use)
- Session store (bbolt)
- users.json user management
- Login page with available auth methods
- Rate limiting ⚠️ *needs verification of enforcement*

### Frontend
- Preact + HTM (5KB vendored, zero build step)
- Responsive layout, dark theme
- Service worker for offline static assets
- Login page

### Public API (pkg/vel/)
- RegisterApp, AppConfig, auth helpers
- Health checks, secrets, security utilities
- Error logging

### Agent SDK
- Generic interface (pkg/agent/)
- OpenClaw adapter
- Callback-based task completion

### Verification
- `vel verify` runtime health checks (CLI + /api/health)
- `vel test` runner
- Schema validation for panel manifests ⚠️ *only FormatError helper — may be thinner than documented*

### Apps on Vel
- VelMetrics (server monitoring)
- VelBridge (remote browser control)
- VelClawBoard (OpenClaw dashboard)
- Menayra (static site)
- Token Swap (private)
- Pipeline (task management, SQLite)

---

## Yet to Build

### Data / Models
- Model system (JSON schema → SQLite tables)
- SQLite via modernc.org/sqlite (pure Go, decided)
- Auto-migrations (dev mode)
- `vel migrate` (production)
- REST CRUD API auto-generated per model
- Document lifecycle hooks (validate, before_insert, after_insert, before_save, on_update, before_delete, on_delete)
- Field types: string, text, integer, decimal, boolean, datetime, date, enum, link, json
- Link fields (foreign key relationships)
- Naming rules (auto-increment, field-based, hash, custom)
- Standard fields (name, creation, modified, modified_by, owner)
- List API (pagination, filtering, field selection, ordering)
- Search fields (typeahead on link fields)
- Custom fields (per-site additions without modifying app schemas)
- Query builder (vel.QB() for complex queries)
- Raw SQL escape hatch (vel.Query())
- KV store for simple state (vel.KV())
- Framework-enforced app isolation (table prefixing)
- Model validation at build time
- Child tables (sub-models embedded in parent)

### Auth & Permissions
- User model (email/password alongside Telegram)
- Roles (multiple per user)
- Role-based permissions (per-model CRUD)
- Row-level permissions (ownership, department, custom rules)
- Field-level permissions (hide/read-only by role)
- User permissions (restrict link field values)
- Permission debugging CLI
- OAuth 2.0 provider + client (Google, GitHub)
- Two-factor auth (TOTP)

### UI / Frontend
- Auto-generated List View per model
- Auto-generated Form View per model
- Desk shell (sidebar, breadcrumbs, module grouping)
- Field widgets (text, date, select, link selector, JSON editor, etc.)
- Client-side validation
- Attachments (file upload/download)
- Comments (threaded)
- Activity log (auto-tracked changes)
- Deep linking (URL-driven routing)
- Panel composition / extension system

### Jobs & Automation
- Background job queue (SQLite-backed)
- Task scheduler (cron expressions in app.json)
- Workflow engine (states, transitions, role-based rules)
- Submit/Cancel/Amend document lifecycle
- Naming series (auto-incrementing prefixed names)
- Assignment rules
- Notification system (in-app)

### Communication
- Email accounts (IMAP/SMTP)
- Email queue + templates
- Email-to-document linking
- Print formats (Go HTML templates)
- PDF generation
- Webhooks (outgoing HTTP on events)
- Web forms (public, no login, create documents)

### Web / Portal
- Website routing (static + dynamic pages)
- Page templates with layout inheritance
- Blog system
- Portal views (users manage own documents)
- SEO (meta tags, OpenGraph, sitemap, robots.txt)
- Full-text search (SQLite FTS5)
- i18n / translations

### Enterprise / Scale
- Report builder (query-based)
- Script reports (Go-defined)
- Dashboard system (number cards, charts)
- Data import/export (CSV/JSON)
- S3-compatible file storage
- Virtual models (backed by external APIs)
- Audit trail (immutable)
- Rate limiting (per-user, per-endpoint)

### Developer Experience
- `vel new-app` scaffolding
- `vel doctor` (automated health/consistency checks)
- Dev mode (auto-rebuild on file changes)
- App install/uninstall lifecycle
- App dependencies (resolved at build time)
- Patch system (versioned data migrations)
- Fixtures (export/import standard records)
- Domain setup wizards

### Deployment
- Backup/restore (4 folders + patches)
- Manifest generation (vel snapshot)
- Graceful shutdown (SIGTERM handling)
- Health endpoints (readiness + liveness)
- Logs to stdout
- Config from environment variables
