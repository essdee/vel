# AI-Native Design Principles

Vel is built for a world where AI agents write most of the code. This isn't a marketing angle — it's a structural decision that shapes every part of the framework.

AI-generated code produces [1.7× more issues per pull request](https://www.coderabbit.ai/blog/the-state-of-ai-in-code-reviews) than human-written code, with specific weaknesses in security, concurrency, and logic correctness. Agents hallucinate APIs, attempt too much at once, and default to insecure patterns when given a choice.

The solution isn't better prompts. It's a framework where the wrong thing is impossible and the right thing is automatic.

---

## The core idea

**Agents shouldn't have to make decisions the framework can make for them.**

Every decision point is a potential failure point. Every configuration option is a hallucination vector. Every "flexible" API is an opportunity to pick the insecure path.

Vel removes these decision points. There's one way to define a model, one way to register routes, one way to handle auth. The framework enforces correctness at compile time. The agent writes the business logic — everything else is guaranteed.

---

## Principles

### 1. One way to do everything

Agents perform best on well-scoped tasks. When there are five ways to define routes, agents pick the wrong one. When there's one way, they get it right.

- One project structure. One routing convention. One auth strategy.
- The default path is the production path. No dev-only shortcuts.
- `app.json` is the single source of truth for what an app does.

### 2. The framework reads like a spec

Agents arriving in a fresh context need to orient fast. Vel's architecture is machine-readable by design.

- `app.json` describes every app's capabilities, routes, data sources, and dependencies.
- Panel manifests describe UI components as JSON — not code.
- Model schemas describe data entities as JSON — not migrations.
- `vel status` (planned) will output the entire project state in structured JSON.

Anthropic's research found that agents are "less likely to inappropriately change or overwrite JSON files" compared to Markdown. Vel is JSON-first for exactly this reason.

### 3. Small, verifiable steps

Agents try to build everything at once, run out of context mid-implementation, and leave half-finished features. The antidote is atomic units with immediate verification.

- A panel is two files. Drop it in, restart, it works or it doesn't.
- A model is one JSON file. The framework validates it at build time.
- Tests are co-generated with code. Every scaffold includes its test.
- `vel build` catches capability violations before the binary exists.

The agent should never be more than one step away from knowing if things work.

### 4. Security by construction, not convention

AI code has 1.5–2× more security issues than human code. Agents default to the simplest auth method, not the most secure one. They don't read security best practices before writing code.

Vel doesn't rely on agents knowing security best practices:

- **Capability system** — apps declare what they can import. `os/exec`, `syscall`, `unsafe` are blacklisted at compile time. An agent literally cannot import them.
- **Auth built in** — Telegram HMAC-SHA256, signed cookies, rate limiting. All on by default.
- **Parameterized queries only** — the model system will make SQL injection impossible by construction.
- **CORS, security headers** — on by default, not opt-in. CSRF protection is planned.

The path of least resistance is the secure path. Always.

### 5. Errors that teach

Agents waste enormous token budgets on cryptic errors. A stack trace saying "cannot read property 'id' of undefined" gives the agent nothing to work with. One clear error message saves five retry loops.

Vel errors include:
- What went wrong
- Where it happened
- What was expected vs. what was received
- How to fix it

Panel manifest validation already does this (Elm-quality messages). The model system, API layer, and build system will follow the same pattern. All errors will be available in both human-readable and JSON format — agents parse JSON better.

### 6. Predictable API, not just small API

A small API is easy to learn. A predictable API is easy to use correctly — even when it's large.

Today, Vel's public API (`pkg/vel`) has ~30 exported functions across auth, health, security, error handling, and app registration. It will grow. The model system, permissions, workflows, and email will all add surface area. Pretending it won't is dishonest.

What matters isn't the count — it's the consistency. Every model should have the same methods. Every resource should follow the same URL pattern. Every error should have the same shape. An agent that understands how `Item` works should be able to work with `Invoice` without reading a single new line of documentation.

The rules:
- **Same patterns everywhere.** If `GET /api/resource/Item` lists items, then `GET /api/resource/Invoice` lists invoices. No exceptions.
- **Same method signatures.** If `validate` takes `(doc, old)` on one model, it takes `(doc, old)` on every model.
- **Same error shape.** Every error — validation, permission, not found, build failure — follows one envelope format.
- **Discoverable.** `vel status` outputs every available API endpoint. An agent never guesses what exists.
- **No aliases.** One name per concept. If it's called `on_update` in models, it's not called `after_save` somewhere else.

A larger API where every function follows the same pattern is better than a small API with special cases.

### 7. Token efficiency

Opus 4.6 consumes tokens significantly faster than its predecessor. Framework verbosity translates directly to cost.

- Two files = a panel. One JSON file = a model. `app.json` = an app.
- Flat directory structure. No `src/modules/users/controllers/v1/UserController.go`.
- Co-located code. Route + handler + validation in one place.
- Minimal scaffolding output. A new app is ~5 files, not 50.

### 8. Guardrails, not guidelines

This is the principle that ties everything together. Guidelines are suggestions. Agents ignore suggestions.

Vel's capability system doesn't *suggest* which packages are safe — it *enforces* it at compile time. Panel manifests don't *recommend* a schema — they *reject* invalid ones with specific error messages. Auth isn't *available* — it's *on*.

The framework doesn't trust the agent to make good decisions. It makes the decisions, and lets the agent focus on what it's good at: writing business logic.

---

## What this means for contributors

If you're building a Vel app — whether you're human or AI:

1. **Follow the conventions.** There's one way to do things. That's intentional.
2. **Write JSON, not code** for anything structural. Models, manifests, config.
3. **Write Go for business logic only.** The framework handles plumbing.
4. **Trust the framework.** If something feels like boilerplate, you're probably doing it wrong — the framework should be handling it.
5. **Keep it small.** One panel per folder. One model per file. One concern per function.

If you're contributing to Vel itself:

1. **Every new API must justify its existence.** Can the framework handle this without exposing it?
2. **Every error must be actionable.** If an agent can't fix the problem from the error message alone, the error message is wrong.
3. **Every default must be the safe default.** If someone has to opt into security, we failed.
4. **JSON over code for declarations.** Code is for logic. JSON is for structure.

---

## Research

These principles are grounded in published research on AI coding agent behaviour:

- **Anthropic** — [Effective Harnesses for Long-Running Agents](https://www.anthropic.com/engineering/swe-bench-sonnet) (structured manifests, incremental progress, git integration)
- **CodeRabbit** — [State of AI vs. Human Code Generation](https://www.coderabbit.ai/blog/the-state-of-ai-in-code-reviews) (1.7× more issues, security anti-patterns, excessive I/O)
- **OpenAI** — Codex Agent Loop documentation (AGENTS.md convention, session management, context window handling)
- **VentureBeat** — [Why AI Coding Agents Aren't Production-Ready](https://venturebeat.com/) (hallucinated APIs, environment blindness, repetitive error loops)
- **Stack Overflow** — 2025 Developer Survey (65% weekly AI agent usage)

Full research document: [vel-project-notes/research/ai-agent-framework-design-principles.md](https://github.com/essdee/vel-project-notes/blob/main/research/ai-agent-framework-design-principles.md)

---

---

## Open tensions

These are known holes in the principles above. They don't have clean answers yet. They'll be resolved as the framework grows — but acknowledging them now is better than discovering them later.

**When convention doesn't fit.** "One way to do everything" works for CRUD. Manufacturing BOMs, double-entry accounting, country-specific payroll — these are unconventional by nature. The framework needs a designed escape hatch: one explicit, auditable way to break convention when business logic demands it. Not hidden workarounds.

**Humans debug what agents write.** Every principle optimises for AI writing code. None address a human debugging production at 2 AM. JSON manifests are great for agents, terrible for tracing why a request returned 403. The framework needs end-to-end request traceability — not just good errors, but observable execution.

**JSON has limits.** "JSON for structure, code for logic" is the right direction. But where's the line? A 40-field invoice model with conditional required fields and computed totals pushes JSON to its limits. The boundary between declaration and code needs to be explicitly defined before the model system is built.

**Runtime security isn't compile-time security.** The capability system prevents agents from importing dangerous packages. That's compile-time. But most real breaches are runtime: data leaking between tenants, users approving their own expenses, row-level access violations. The permission model must address this — compile-time guardrails alone aren't enough.

**Agent capabilities change fast.** These principles are derived from 2025-2026 agent behaviour. Context windows went from 8K to 1M in two years. Token costs drop yearly. Principles based on current limitations could become unnecessarily restrictive. Every guardrail should be removable without refactoring — so the framework evolves with agents, not against them.

**Why, not just what.** `vel status` will tell an agent what exists. It won't tell the agent *why* it exists — what constraints were discussed, what alternatives were rejected. Agents lose context between sessions. The framework should support decision provenance: a machine-readable history of why things are the way they are.

**Reports need dynamic queries.** "Parameterized queries only, no raw SQL" is correct for user-facing data. But a report builder needs grouping, aggregation, joins, date ranges. The principle should distinguish between write paths (strict, safe) and read-only analytics (where controlled dynamic queries are necessary).

**Reviewability.** An AI agent builds an app. A human needs to review it. How long does that take? If the framework's conventions are strong enough, a human should understand any Vel app's structure in minutes — not because they wrote it, but because every app looks the same. This is the flip side of "one way to do everything" and it's worth measuring.

---

*These principles evolve as we learn. But the direction doesn't change: make the right thing automatic, make the wrong thing impossible.*
