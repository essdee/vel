# Architecture Decisions

This document explains **WHY** Vel is built the way it is. For **WHAT** the rules are, see [CONTRACTS.md](./CONTRACTS.md).

Each decision includes what would make us change our mind. Architecture should evolve, not fossilize.

---

### Why the name Vel

**Decision:** Name the framework "Vel" (வேல்).

**Why:** Vel is the divine spear of Murugan in Tamil mythology — sharp, fast, unerring. It captures the framework's philosophy: one binary, one purpose, no waste. The spear doesn't carry baggage. Neither does Vel.

**Would change if:** Never. Names are permanent.

---

### Why Go instead of Node.js

**Decision:** Go as the server language. Single binary deployment.

**Why:** No `node_modules`, no runtime dependency. 76ms cold start vs 520ms. 2.6MB RSS vs 186MB. Go's type system makes the schema architecture partially free — the compiler catches what Node.js tests had to catch. 

**Considered:** Node.js (familiar, but memory and startup pain), Rust (faster but harder for contributors), Deno/Bun (still JS ecosystem problems)

**Would change if:** Never. The benefits are permanent.

---

### Why Preact+HTM over Lit/Vanilla/React

**Decision:** Preact 10 + HTM 3 (~5KB vendored)

**Why:** Shadow DOM breaks Android WebView + Telegram Mini App. React is 40KB, Preact is 4KB with the same API. HTM = tagged templates, no JSX build step. AI agents write best React/Preact (most training data of any framework).

**Considered:** Vanilla JS (too limited for forms/CRUD), Lit (Shadow DOM breaks WebView), React (too heavy), Svelte (less AI training data, needs compiler)

**Would change if:** Preact stops being maintained, or a lighter framework with equal AI training coverage emerges.

---

### Why Go backend / ESM browser

**Decision:** Server-side = Go. ui.js = ESM (browser-native).

**Why:** Go handles all server logic as compiled code. Browsers are ESM-native for ui.js components. Clean separation — no module system confusion.

**Would change if:** Never — Go + browser ESM is the cleanest split possible.

---

### Why panels are vertical slices

**Decision:** Each panel = folder with manifest.json + ui.js. Data handlers live in `internal/data/` (Go).

**Why:** AI agents can understand one folder completely without loading the whole codebase. Adding a panel's UI = adding a folder, no touching core. Each file has a single clear purpose. Data handlers in Go get type safety and compile-time checks.

**Would change if:** Panel count exceeds ~50 and discovery becomes slow (unlikely — lazy loading planned).

---

### Why no build step

**Decision:** No webpack, no Vite, no bundler. ESM imports in browser.

**Why:** AI agents can't reliably run build tools. Debugging bundled code is harder for both humans and AI. ESM imports work natively in all modern browsers. "Clone and run" — zero tooling required.

**Would change if:** Browser ESM performance becomes a bottleneck with 50+ panels (would add optional bundling, never required).

---

### Why SQLite for store (planned v0.2)

**Decision:** SQLite as default store, adapter pattern for alternatives.

**Why:** Zero setup — single file, excellent Go support via `modernc.org/sqlite` (pure Go, no CGO). Matches "single binary" philosophy. Adapter pattern means swap to PostgreSQL/MySQL without panel code changes.

**Would change if:** Multi-user concurrent writes become a requirement.

---

### Why hooks are all async

**Decision:** Hook engine is Go-native with goroutine-safe execution.

**Why:** Go's concurrency model (goroutines + channels) handles hook execution naturally. No event loop blocking concerns. Filter chains run sequentially for predictable ordering; actions can fire concurrently.

**Would change if:** Never. Go-native hooks are simpler and faster than any embedded scripting approach.

---

### Why cls() instead of CSS-in-JS or CSS modules

**Decision:** `cls('metric')` → `'p-cpu-metric'` — simple string prefix function.

**Why:** Zero runtime cost (just string concatenation). No build step needed. Predictable output — AI agents and humans can read the generated class names in DevTools. Namespace isolation without Shadow DOM.

**Would change if:** A zero-build CSS scoping solution with better DX emerges.

---

### Why validation rules as single source of truth

**Decision:** Panel validation rules in `internal/schema/` are Vel's type system. Each validation check is a discrete function.

**Why:** Documentation and code are separate artifacts describing the same truth. They drift. The solution: make validation rules in code the single source of truth. Multiple consumers (startup validation, doctor CLI, introspection API) read the same rules. Drift becomes architecturally impossible.

**Growth path:** v0.1 = Elm-quality errors. v0.2 = extract to schema packages + doctor CLI. v0.3 = `/api/schema` introspection. Each version adds a package. No version changes the architecture.

**Would change if:** The validation contract (`{ level, message, fix, ref }`) proves insufficient for a new concern.

---

### Why capabilities field exists

**Decision:** Panels declare `capabilities: ["fetch"]` in manifest. Core validates against current version.

**Why:** Prevents "works on my machine" — a panel needing `store` won't silently fail on a version without store support. Forward-compatible: new capabilities added per version without breaking existing panels.

**Would change if:** Capability list exceeds ~10 items (would group into capability sets).
