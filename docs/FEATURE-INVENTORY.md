# Vel — Feature Inventory

Full list of framework features, grouped by domain. Used when architecting — look at the full picture, design so nothing gets blocked.

Features move from "Yet to Build" → "Built" as projects demand them.

**Rule:** Only meta features belong in the framework. If it's a full app (blog, e-commerce, BI tool), it's a Vel app, not a framework feature. The framework provides hooks and APIs that apps plug into.

---

## 1. Data / Models

**Built:**
- *(none yet — v0.2 work)*

**Yet to Build:**
- Schema definition (JSON/declarative → database tables)
- Field types: string, text, integer, decimal, boolean, date, datetime, time, enum/select, link (FK), table (child), JSON, currency, percent, duration, color, image, file, password, geolocation, rating, signature
- Auto-migrations (dev mode)
- Explicit migrations (production — `vel migrate`)
- Standard fields (name/ID, created, modified, created_by, modified_by)
- Naming rules (auto-increment, field-based, format string, hash, custom function)
- Link fields (foreign keys between models)
- Child tables (sub-models embedded in parent)
- Custom fields (per-site additions without modifying source)
- Virtual fields (computed, not stored)
- Document lifecycle hooks (validate, before_insert, after_insert, before_save, after_save, before_delete, after_delete, on_change)
- Model validation at build time
- Field-level validation rules (required, unique, min/max, regex, custom)
- Default values (static, dynamic/function)
- Submittable documents (docstatus: 0=draft, 1=submitted, 2=cancelled)

---

## 2. Database

**Built:**
- SQLite via modernc.org/sqlite (pure Go, embedded)

**Yet to Build:**

*Engines:*
- PostgreSQL support (production)
- MariaDB support (production)
- Database adapter interface (swap engines without app code changes)
- Per-model database selection (e.g., model X on PostgreSQL, model Y on SQLite)
- Separate database for time-series / immutable data (logs, audit trails)
- Virtual models (backed by external API instead of database table)

*Query:*
- CRUD API (Create, Read, Update, Delete)
- List API (pagination, filtering, ordering, field selection, search)
- Query builder (joins, aggregations, subqueries — complex queries without raw SQL)
- Raw SQL escape hatch (read-only and read-write)
- Full-text search
- Bulk operations (insert many, update many)
- Transactions (begin, commit, rollback)

*Infrastructure:*
- KV store (simple key-value for app settings/state)
- App isolation (table prefixing)
- Connection pooling
- Database backup/restore

---

## 3. API

**Built:**
- Health endpoint (`/api/health`)
- Auth endpoints
- Pipeline API (task lifecycle)

**Yet to Build:**
- REST CRUD auto-generated per model (GET/POST/PUT/DELETE)
- List endpoint with query params (filters, pagination, ordering)
- Bulk operations endpoint
- Method API (custom endpoints registered by apps)
- File upload/download endpoints
- Public/whitelisted API routes (no auth required, declarative)
- API overrides (apps can override existing API endpoints)
- Rate limiting (per-user, per-endpoint, per-API-key)
- Request validation (type checking, required fields)
- Consistent error response format
- CORS handling
- API documentation (auto-generated from models)

---

## 4. Auth & Users

**Built:**
- Telegram HMAC-SHA256 provider
- API key provider (scoped, hashed)
- Magic link provider
- Session store (bbolt)
- users.json management
- Login page
- Rate limiting (partial — needs verification)

**Yet to Build:**
- User model (email, password, roles, status)
- Email/password auth provider
- Password hashing (bcrypt)
- Password reset flow
- Two-factor auth (TOTP)
- Roles (define, assign multiple per user)
- Role-based permissions (per-model CRUD by role)
- Row-level permissions (ownership, department, custom rules)
- Field-level permissions (hide/read-only by role)
- User permissions (restrict link field values per user)
- Permission debugging/introspection
- OAuth 2.0 provider (Vel as auth server)
- OAuth 2.0 client (social login — Google, GitHub)
- Impersonation (admin acts as another user, with audit trail)

---

## 5. UI / Frontend

**Built:**
- Panel system (WebSocket real-time dashboards)
- Preact + HTM (5KB vendored, zero build step)
- Responsive layout, dark mode
- Login page
- Service worker

**Yet to Build:**

*Admin views (developer/admin use only):*
- Auto-generated List View per model (sortable, filterable, paginated)
- Auto-generated Form View per model (field widgets, validation)
- Admin shell (sidebar, navigation, module grouping)

*Crafted UI enablement (the real UI — built by agents per role/user):*
- Framework makes wiring custom frontend to backend trivial
- API layer is predictable, consistent, easy for agents to build against
- Per-role UI scoping
- Per-user UI customization
- See: rants-and-ideas/2026-03-22-ui-vision-crafted-ui.md

*Shared:*
- Field widgets (text, date, select, link, file, etc.)
- Client-side validation
- Attachments (file upload/download linked to documents)
- Comments (threaded, on any document)
- Activity log / timeline
- Deep linking (URL-driven routing)
- Panel composition (apps enhancing other apps' panels)

---

## 6. File Management

**Built:**
- *(basic file serving only)*

**Yet to Build:**

*Storage:*
- File upload to local storage
- File upload to S3-compatible storage
- Public files: option to serve direct S3 URL (bypass server)
- Public files: option to relay through server
- Private files: pre-signed URLs with expiry (time-limited, count-limited)

*Processing:*
- Image optimization API (built-in, any frontend can call)
- Image cropping API
- Per-field compression settings (e.g., receipt photo compressed harder than product photo)
- Configurable defaults per document type
- Image thumbnails generation

*Limits & Validation:*
- File type validation
- File size limits: per-field, per-document, per-site (global)
- Attachment linking (files linked to specific documents)

---

## 7. Jobs & Scheduling

**Built:**
- *(none yet)*

**Yet to Build:**
- Background job queue
- Job scheduling (cron expressions)
- Job logging (execution history, errors)
- Job retry (configurable count, backoff)
- Job monitoring (`vel jobs` CLI)
- Deferred actions (run after commit)
- Webhooks (outgoing HTTP on document events)
- Webhook retry with backoff

---

## 8. Workflow & Automation

**Built:**
- *(none yet)*

**Yet to Build:**
- Workflow engine (states, transitions, role-based rules)
- Workflow templates (submit/cancel/amend as a default template)
- Workflow UI (state indicator, action buttons)
- Auto-assignment rules
- Notification triggers (on events/conditions/schedules)
- Approval chains (multi-level)

---

## 9. Communication

**Built:**
- *(none yet)*

**Yet to Build:**

*Core (framework provides):*
- Communication channel interface (pluggable)
- Channel capability declaration (supports: subject, recipients, phone, email, custom fields, etc.)
- Channel types: email, instant chat, push, in-app
- Routing: any document can send via any channel
- Notifications as one-way communications (same system)
- Delivery status tracking
- See: rants-and-ideas/2026-03-22-communication-channels-vision.md

*Built-in channels (or first-party plugins):*
- Email (IMAP/SMTP)
- In-app notifications
- *(WhatsApp, SMS, push — as plugins)*

---

## 10. Web / CMS

**Built:**
- Static file serving
- Static site app (Menayra)

**Yet to Build:**

*Framework meta features:*
- Static page serving
- Dynamic model-backed pages
- Static site generation on data change (render + cache until underlying data changes)
- Page templates (HTML with layout inheritance)
- Web forms (public-facing, no login, create documents)
- Portal views (logged-in users manage own documents)
- SEO defaults (meta tags, OpenGraph, sitemap.xml, robots.txt — good defaults out of box)
- i18n / translations
- Custom routes (apps register website routes)
- Redirects (301/302 management)
- Page analytics (views, load time, performance)

*Apps (NOT framework):*
- Blog system
- Landing page builder
- URL shortener

---

## 11. Reporting & Data

**Built:**
- *(none yet)*

**Yet to Build:**

*Framework meta features:*
- Report builder (query-based, grouping, aggregation)
- Script reports (Go-defined, custom data sources)
- Data export (CSV, JSON)
- Data import (CSV/JSON with validation)
- Prepared reports (background-generated, cached)
- Query report (parameterized SQL)

*Apps (NOT framework):*
- BI tool / dashboard builder
- Charts / visualization

---

## 12. App Ecosystem

**Built:**
- App discovery (apps/ directory, app.json manifest)
- `vel build` (compile apps into binary)
- Capability sandbox

**Yet to Build:**
- App lifecycle (install, uninstall, upgrade)
- App dependencies (declared in app.json, resolved at build)
- Patch system (versioned data migrations)
- Fixtures (export/import default records)
- App load order (deterministic, dependency-based)
- `vel new-app` scaffolding *(debatable — good docs might be enough)*
- `vel doctor` (validate config, models, dependencies, capabilities)
- App marketplace / registry (future)

---

## 13. AI Developer Experience

**Built:**
- `vel test` runner
- `vel verify` (post-deploy health checks)
- Debug server
- Elm-quality error messages (partial — needs hardening)
- Schema introspection (partial)

**Yet to Build:**
- Dev mode (auto-rebuild on file changes)
- Linting (out of box, AI-accessible)
- Fast feedback loops (incremental build, quick test runs)
- Database query logging (N+1 detection)
- Request tracing (end-to-end request ID)
- `vel doctor` (automated consistency checks)
- Fixtures for test data
- Error messages that teach (every error: what, where, expected vs got, how to fix)
- Schema introspection API (list all models, fields, types — for agent orientation)

---

## 14. Security

**Built:**
- Capability sandbox (compile-time import enforcement)
- Session security (HttpOnly, Secure, SameSite cookies)
- API key hashing (SHA-256)
- Security headers

**Yet to Build:**
- CSRF protection
- Parameterized queries (when model system ships)
- Input sanitization (XSS prevention)
- Audit trail (immutable log of all document changes)
- Brute-force protection (login rate limiting, account lockout)
- Secrets management (vel.Secrets())
- Dependency vulnerability scanning

---

## 15. Deployment & Operations

**Built:**
- Single binary deployment
- Config from file (vel.json)
- Health endpoints
- Logging

**Yet to Build:**
- Config from environment variables
- Graceful shutdown (SIGTERM handling)
- Zero-downtime upgrades (old process serves until new is ready, seamless switchover)
- Backup/restore (database + files)
- Structured logging (JSON option)
- Monitoring endpoints (metrics)
- Upgrade path (version migration tooling)
- CLI: migrate, doctor, new-app, jobs, permissions, snapshot

---

## 16. Integration

**Built:**
- REST API (partial)
- Agent SDK (OpenClaw adapter)

**Yet to Build:**
- Incoming webhooks (receive external events)
- External API calling (framework HTTP client with retry)
- Data sync patterns (periodic pull from external systems)

---

*No multi-tenancy in framework. Multi-tenancy = separate instances on separate ports.*
*No e-commerce in framework. E-commerce = Vel app, not framework feature.*

---

*Last updated: 2026-03-22*
