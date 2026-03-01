# Vel Framework Roadmap

Vel is a Go framework where AI agents build the apps and the framework guarantees correctness. Single binary. Manifest-driven. Compile-time enforcement.

Each version is **fully usable on its own**. You never ship half a framework. And v0.1 is the architecture freeze — every structural decision, every convention, every interface is locked before feature work begins. No refactors after v0.1.

---

## v0.1 — The Foundation ✅

**Status:** Complete (code) · Architecture freeze in progress (specs)

v0.1 is two things: a working framework, and the blueprint for everything after it. The code is done. The structural specs are being frozen so that v0.2–v0.9 are pure implementation against stable contracts.

### What's built

**Core (5,756 lines Go, 89 tests, 10 packages):**
- Single Go binary (net/http stdlib)
- App discovery system (`apps/` directory, `app.json` manifest)
- Panel system (`manifest.json` + `ui.js`, auto-discovered)
- WebSocket-first real-time data streaming
- Hook engine (filters + actions, Go-native)
- Schema validation for panel manifests with Elm-quality error messages
- Config system (`config.json`, site-level)

**Build system:**
- `vel build` — compiles framework + app Go code into single binary
- Capability system (Deno-inspired, category-based) — apps declare imports, build enforces
- AST rewriting for capability wrappers
- App server code discovery (`server/` subdirectory convention)

**Auth:**
- Telegram HMAC-SHA256 authentication
- Signed cookie sessions
- Rate limiting on auth endpoints
- Bot token validation
- Allowed users whitelist

**Data:**
- File-based data sources with configurable polling intervals
- System metrics handlers (CPU, memory, disk, uptime, processes)

**Frontend:**
- Preact + HTM (vendored, 5KB, zero build step)
- Responsive layout engine
- Panel error boundaries
- Service worker for offline static assets

**Public API (`pkg/vel/`):**
- `RegisterApp()` — apps register routes via `init()`
- `AppConfig` — runtime context for apps
- Auth helpers — `Check()`, `IsAllowed()`, `CheckBotToken()`, `GetBotToken()`

**CLI:**
- `vel start` — run the server
- `vel build` — compile with app code
- `vel caps` — list/export capabilities
- `vel version` — print version

**Apps built on Vel:**
- [Velboard](https://github.com/karthikeyan5/velboard) — dashboard with 9 monitoring panels
- [VelReach](https://github.com/karthikeyan5/velreach) — remote browser control via CDP
- Menayra — static site app

**CI:** GitHub Actions, all tests passing

### What's being frozen (architecture specs)

Before v0.2 begins, every structural decision must be locked:

- Multi-tenancy model (per-site DB vs tenant column)
- Database choice and abstraction layer
- Custom fields storage design
- Model JSON schema (field types, meta fields, naming)
- Permission model (role-based, row-level, field-level)
- API URL patterns and error response format
- App lifecycle (install, uninstall, dependencies, patches, fixtures)
- File storage abstraction (local + S3 behind interface)
- Frontend component contracts (forms, lists — beyond panels)
- Config merge strategy (site config + app defaults)
- Hook execution order contract
- CLI command tree (all planned commands)
- DevOps conventions (deployment, backup, monitoring, logging, upgrades)
- DevX conventions (scaffolding, testing, dev mode, error quality)

📋 **[Full freeze checklist → specs/v01-architecture-freeze.md](https://github.com/essdee/vel-project-notes/blob/main/specs/v01-architecture-freeze.md)**

---

## v0.2 — The Model

**Theme:** Data gets a home. JSON schemas become database tables with auto-generated APIs.

**You can now build:** Todo apps, contact managers, inventory lists, a simple CRM with companies, contacts, and deals — anything that stores and retrieves structured data.

- **Model system** — JSON schema files define data entities
- **SQLite database** — embedded, zero-config, single-file storage
- **Auto-migrations** — schema changes detected and applied on startup
- **Field types** — string, text, integer, decimal, boolean, datetime, date, enum, link, json
- **REST CRUD API** — auto-generated GET/POST/PUT/DELETE per model
- **Document lifecycle hooks** — validate, before_insert, after_insert, before_save, on_update, before_delete, on_delete (Go functions)
- **Link fields** — foreign key relationships between models
- **Naming rules** — auto-increment, field-based, hash, or custom
- **List API** — pagination, filtering, field selection, ordering
- **Search fields** — configurable fields for typeahead on link fields
- **Standard fields** — name, creation, modified, modified_by, owner auto-added
- **Custom fields** — per-site field additions without modifying app schemas
- **Multi-tenancy** — per-site SQLite databases, designed into the data layer from day one
- **`vel migrate`** — explicit migration command for production
- **`vel test`** — testing framework and conventions, so all code from here on ships tested
- **Model validation** — schema correctness checked at build time

---

## v0.3 — The Ecosystem

**Theme:** App packaging done right. Install, uninstall, upgrade — without breaking anything.

**You can now build:** Distributable apps. An inventory app someone else installs into their Vel instance. Apps that ship default data and upgrade cleanly.

- **App install/uninstall lifecycle** — install hooks, uninstall cleanup
- **App dependencies** — `app.json` declares dependencies, resolved at install/build time
- **Patch system** — versioned data migration scripts (Go functions), run once per site on upgrade
- **Fixtures** — export/import standard records (roles, workflows, defaults) as JSON with apps
- **App load order** — deterministic ordering based on dependency graph
- **`vel new-app`** — scaffold a new app with correct directory structure
- **`vel doctor`** — validate config, models, app dependencies, capability declarations
- **AGENTS.md template** — standardized AI developer instructions for apps
- **Dev mode** — auto-rebuild and restart on file changes

---

## v0.4 — The Guardian

**Theme:** Multi-user access. Roles, permissions, and proper auth — before any UI is built on top.

**You can now build:** Team apps. A project tracker where managers see everything, members see their tasks. Multi-department expense systems. Any app where different people see different things.

- **User model** — built-in user management (email/password auth alongside Telegram)
- **Roles** — define roles, assign multiple per user
- **Role-based permissions** — per-model CRUD permissions by role
- **Row-level permissions** — access to individual documents based on ownership, department, custom rules
- **Field-level permissions** — hide or make fields read-only by role
- **User permissions** — restrict link field values per user
- **Session management** — multiple auth providers, session listing, force logout
- **Password hashing** — bcrypt
- **API key auth** — token-based access for integrations
- **Permission debugging** — `vel permissions` CLI to inspect effective permissions

---

## v0.5 — The Desk

**Theme:** Every model gets a UI. Zero frontend code needed for CRUD apps.

**You can now build:** Internal tools, admin panels, data management apps — without writing frontend. Asset trackers, expense loggers, bug trackers, all auto-generated from model schemas.

- **Auto-generated List View** — sortable, filterable, paginated table per model
- **Auto-generated Form View** — create/edit forms with proper field widgets
- **Desk shell** — sidebar navigation, breadcrumbs, module grouping
- **Field widgets** — text, textarea, number, date picker, select, checkbox, link selector, JSON editor
- **Client-side validation** — required fields, type checks, custom validators
- **Child tables** — sub-models embedded in parent documents
- **Attachments** — file upload/download linked to documents
- **Comments** — threaded comments on any document
- **Activity log** — auto-tracked creation, modification, field changes
- **Indicator/status colours** — configurable on list and form views
- **Deep linking** — URL-driven routing, browser back/forward

---

## v0.6 — The Automator

**Theme:** Background jobs, scheduled tasks, and workflow. Apps come alive.

**You can now build:** Approval workflows (leave requests, purchase orders). Scheduled reports. Data sync jobs. HR leave management with multi-level approval chains.

- **Background job queue** — enqueue Go functions for async execution (SQLite-backed)
- **Task scheduler** — cron expressions in `app.json`
- **Workflow engine** — states, transitions, role-based transition rules, auto-actions
- **Workflow UI** — state indicator on forms, action buttons for allowed transitions
- **Submit/Cancel/Amend** — submittable documents with docstatus lifecycle
- **Naming series** — auto-incrementing prefixed names (e.g., `INV-2026-00001`)
- **Assignment rules** — auto-assign documents based on conditions
- **Notification system** — in-app notifications on document events
- **Scheduled job logging** — execution history, error tracking, retry
- **`vel jobs`** — list running/pending jobs, retry failed

---

## v0.7 — The Communicator

**Theme:** Email, printing, webhooks. Documents leave the system and reach the outside world.

**You can now build:** Invoice systems that email PDFs. Notification-heavy apps. Accounting with printable invoices. Anything that needs to talk to external systems.

- **Email accounts** — IMAP/SMTP configuration, multiple accounts
- **Email queue** — outgoing email via background jobs
- **Email templates** — Go HTML templates with document context
- **Email-to-document** — incoming emails linked to documents
- **Notifications** — email + in-app, triggered by events/conditions/schedules
- **Print formats** — Go HTML templates, multiple per model
- **PDF generation** — HTML-to-PDF (chromedp or wkhtmltopdf)
- **Print settings** — letterhead, header/footer, custom CSS
- **File manager** — centralized storage, type validation, size limits
- **Webhooks** — outgoing HTTP on document events
- **Web forms** — public-facing forms (no login) that create documents

---

## v0.8 — The Portal

**Theme:** Public-facing web. Vel becomes a full web platform, not just an internal tool.

**You can now build:** Customer portals, blogs, landing pages, knowledge bases. E-commerce storefronts with product catalog, customer accounts, order tracking.

- **Website routing** — static pages, dynamic pages from models, clean URLs
- **Page templates** — Go HTML templates with layout inheritance
- **Blog system** — blog posts model, categories, tags, RSS
- **Portal views** — logged-in users manage their own documents
- **SEO** — meta tags, OpenGraph, sitemap.xml, robots.txt
- **Full-text search** — SQLite FTS5, global search bar
- **i18n / translations** — JSON files per app, runtime language switching
- **Static asset serving** — apps bundle CSS/JS/images at `/assets/appname/`
- **Custom routes** — apps register additional website routes

---

## v0.9 — The Enterprise

**Theme:** Scale, reporting, and integrations. Everything needed for production business apps.

**You can now build:** Anything. Accounting, HR, inventory, CRM, manufacturing, project management, helpdesk, e-commerce — as installable Vel apps that compose without conflict.

- **Report builder** — query-based reports with grouping, aggregation, charts
- **Script reports** — Go-defined reports with custom data sources
- **Dashboard system** — number cards, charts, shortcuts
- **Chart rendering** — bar, line, pie, donut (lightweight, vendored)
- **Data import/export** — CSV/JSON import with validation, export
- **OAuth 2.0 provider** — Vel as OAuth server
- **OAuth 2.0 client** — social login (Google, GitHub, etc.)
- **S3-compatible storage** — file attachments in S3/MinIO
- **Database query builder** — programmatic joins, subqueries, aggregations
- **Virtual models** — models backed by external APIs instead of database
- **Property setters** — override field properties per site without changing source
- **`vel site`** — create, migrate, backup, restore sites
- **Rate limiting** — per-user, per-endpoint API limits
- **Audit trail** — immutable log of all document changes
- **Two-factor auth** — TOTP for email/password users
- **Domain setup wizards** — apps define first-time configuration flows
- **Prepared reports** — background-generated, cached for access
- **Workspace customization** — users customize desk layout

---

## v1.0 — The Rock

**Theme:** Hardening. No new features.

**You can now build:** Production-grade business applications with confidence.

- Security audit — auth, permissions, injection, XSS, CSRF, sessions
- Penetration testing — all API endpoints
- Load testing — concurrent users, large datasets, job throughput
- Complete documentation — framework guide, API reference, tutorials, cookbook
- Migration guide — v0.x to v1.0
- Error handling audit — consistent responses, proper status codes, no leaked internals
- SQLite optimization — WAL mode, connection pooling, query tuning
- Backup/restore verification
- CI/CD templates — GitHub Actions for building, testing, deploying Vel apps
- **Stable public API** — `pkg/vel/` frozen, backward compatibility guaranteed
- Release tooling — versioning, changelog, upgrade validation

---

*Last updated: 2026-03-01*
