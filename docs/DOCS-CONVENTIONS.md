# Documentation Conventions

Where things live, when they get updated, and how to keep them honest.

---

## Three Folders, Clean Split

### 1. Vel Repo (`/opt/vel-staging/vel/`)

**What belongs:** Anything that describes the framework — how it works, how to use it, how to extend it, what's planned, what's locked.

**What does NOT belong:** Strategic/org decisions (→ vetrivel), decision history with alternatives explored (→ vel-project-notes), operational pipeline docs (→ pipeline app), personal agent working notes (→ agent workspace).

**When it gets updated:** Every code change that affects behavior must update the relevant doc in the same commit. No code-only commits that change documented behavior.

**Staleness check:** Sentinel verifies doc-code consistency on every review. Periodic: compare ROADMAP.md claims against actual code.

### 2. Vel-Project-Notes

**What belongs:** Decision records (numbered, append-only). Research reference docs. Historical context.

**What does NOT belong:** Active framework docs (→ vel repo), strategy (→ vetrivel), task specs (→ pipeline).

**When it gets updated:** When a new decision is made. Existing records are never modified — they're history.

**Staleness check:** Rarely stales — it's an archive. Review quarterly for unimplemented decisions.

### 3. Vetrivel

**What belongs:** Arogara mission and vision. Strategic decisions. Rants and ideas. Operational methodology (playbook).

**What does NOT belong:** Framework docs (→ vel repo), decision history (→ vel-project-notes), code.

**When it gets updated:** When strategy or mission changes. When new rants/ideas come up.

**Staleness check:** Monthly review.

---

## Vel Repo Structure

```
vel/
├── README.md          ← Public face, what Vel is
├── AGENTS.md          ← Agent instructions (standard root convention)
├── CHANGELOG.md       ← Version history (updated from v1.0 onwards)
├── SECURITY.md        ← Security policy
│
├── dev-docs/          ← For app developers (agents building ON Vel)
│   ├── INDEX.md       ← Reading order, start here
│   ├── SETUP.md       ← Set up a Vel project
│   ├── BUILDING-APPS.md ← How to build apps
│   ├── CONVENTIONS.md ← Naming, patterns, file structure
│   ├── CONTRACTS.md   ← API contracts (locked)
│   ├── AUTH.md        ← Auth for your app
│   ├── TESTING.md     ← Testing your app
│   ├── AGENT-SDK.md   ← Calling AI agents
│   ├── EMAIL-SETUP.md ← Email configuration
│   └── AI-DEBUGGING.md ← Debugging guide
│
├── docs/              ← Framework internals (for contributors)
│   ├── DOCS-CONVENTIONS.md ← This file
│   ├── PILLARS.md     ← Pillars + core design principles
│   ├── ARCHITECTURE.md ← Why Vel is built this way
│   ├── AI-NATIVE.md   ← Agent-first design deep dive
│   └── ROADMAP.md     ← What's planned
```

---

## The Simple Test

**"Am I building an app or building the framework?"**
- Building an app → dev-docs/
- Building the framework → docs/

**"Did we choose between meaningful alternatives?"**
- Yes → write a decision record in vel-project-notes
- No → just write the spec and build it

**"Is this about mission, strategy, or org direction?"**
- Yes → vetrivel
- No → vel repo

---

## Rules

1. **One canonical location per fact.** If it's in dev-docs/CONTRACTS.md, don't repeat it in docs/ARCHITECTURE.md. Link instead.

2. **Code changes = doc changes.** Same commit. No exceptions for documented behavior changes.

3. **Don't claim what isn't verified.** No ✅ on features without checking the code. Use ⚠️ for "exists but unverified" and ❓ for "may not exist."

4. **Docs describe reality, not aspirations.** What's planned goes in ROADMAP.md. What exists goes everywhere else. Don't mix.
