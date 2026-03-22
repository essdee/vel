# Vel — Pillars & Core Design Principles

---

## The Three Pillars

Pillars define what makes Vel, Vel. Remove a pillar, it's a different framework.

### 1. Agent-First 🤖
Designed for AI developers from day one. JSON manifests (agents corrupt JSON less than code). Minimal API surface. Elm-quality error messages. Convention over configuration — fewer decisions, fewer mistakes, fewer tokens wasted. Built entirely through a Telegram chat — no human wrote a line of code.

### 2. *(Open — yet to find)*

### 3. *(Open — yet to find)*

---

## Core Design Principles

Principles guide how we build. They serve the pillars. Ordered by importance.

### Structurally Safe
The wrong code doesn't compile. Capability sandbox enforces imports at compile time. `os/exec` blacklisted. CORS/CSRF/security headers on by default. Parameterized queries when model system ships. The path of least resistance is the secure path.

### Composable
Apps are independent folders. Install one, remove one, nothing else breaks. No dependency hell. Single binary with all apps compiled in.

### Zero-Config Deploy
Single Go binary, no Node.js/Python/Docker. `vel build` → deploy anywhere.

### Deterministic-First
If a task is deterministic, don't pass it through a non-deterministic system. LLMs only at judgment points. Everything else: scripts, algorithms, rules, SQL.

### One Way to Do Everything
One project structure. One routing convention. One auth strategy. Fewer decisions = fewer mistakes = fewer tokens wasted.

### The Framework Reads Like a Spec
`app.json`, `manifest.json`, model schemas — all JSON, all machine-readable. Agents orient fast. JSON-first because agents corrupt JSON less than Markdown.

### Security by Construction
Not convention. Not guidelines. Capabilities enforced at build time. Parameterized queries. The agent focuses on business logic; the framework handles everything else.

### Errors That Teach
Every error: what went wrong, where, what was expected vs received, how to fix. Saves 5 retry loops per error. Both human-readable and JSON format.

### Predictable > Small
A 50-function API where every function follows the same pattern beats a 5-function API with special cases. Same URL pattern, same method signatures, same error shape everywhere.

### 400ms Rule
Every app built on Vel should naturally tend toward sub-400ms response times. The framework optimizes at every layer. This is a UX cornerstone, not a nice-to-have.

### Token Efficiency
Two files = a panel. One JSON file = a model. Flat directories. Co-located code. Minimal scaffolding.

### Guardrails, Not Guidelines
Guidelines are suggestions. Agents ignore suggestions. Vel enforces at compile time, rejects at startup, or prevents at runtime.
