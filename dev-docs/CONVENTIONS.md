# CONVENTIONS.md — Decision Framework

When CONTRACTS.md doesn't cover your situation, these principles guide the decision. They're ordered by priority — when two conflict, the lower number wins.

## 1. Structure IS the convention

Don't document what can be discovered by reading the file system. `ls core/panels/` teaches panel naming. `cat manifest.json` teaches the schema. If an agent needs to read CONVENTIONS.md to understand naming, the project structure has failed.

**Exception:** empty extension points (no examples to learn from) need bootstrap templates. Every extension point (`custom/panels/`, `custom/routes/`, `apps/`) should have a `_template/` or `_example` that teaches the pattern.

## 2. Derive from core, don't memorize

Every convention follows from one rule: match the nearest existing **core** pattern.

- Adding a panel? Copy `core/panels/cpu/`, rename, modify.
- Adding a hook? Find an existing `hooks.filter()` call, follow its naming.
- Adding a route? Look at `custom/routes/` examples.

**Important:** always derive from `core/` implementations, never from `custom/` or `apps/`. Core is the reference implementation.

## 3. Names describe WHAT, not HOW

- `data.js` not `fetchAndParseSystemMetrics.js`
- `validator.js` not `jsonSchemaValidatorForManifests.js`
- `hooks.js` not `asyncHookRegistryWithFilterChain.js`

**Tiebreaker:** prefer the more general name. Names should accommodate growth.

## 4. When in doubt, choose the boring option

- Prefer established patterns over clever ones
- Prefer explicit over implicit
- Prefer one obvious location over flexible placement

If you're debating between two valid approaches, pick the one a junior developer would understand on first read.

## 5. Conventions are functional, not cosmetic

`p-cpu-value` isn't a style choice — `cls()` generates it and CSS matches on it. Break the naming, break the feature.

Distinction:
- **Rules** (functional, enforced): CSS prefixes, hook naming segments, API route prefixes, manifest field names.
- **Suggestions** (cosmetic, encouraged): Commit message format, component naming, code style.

Don't enforce suggestions as rules. Mark the difference.

## 6. The file system is the API

```
core/panels/       → ls = know all panels
custom/            → ls = know all user extensions
apps/           → ls = know all apps
```

If an agent needs to grep for something, the directory structure needs work.

## 7. One canonical location per concern

Every piece of information lives in exactly ONE place. Duplication creates contradictions.

**Canonical locations:**

| Concern | File |
|---------|------|
| What the rules are | `CONTRACTS.md` |
| Why it's built this way | `ARCHITECTURE.md` |
| How to extend it | `BUILDING-APPS.md` |
| How to set it up | `SETUP.md` |
| Testing strategy | `TESTING.md` |
| Decision framework | `CONVENTIONS.md` |
| What's planned | `FEATURE-INVENTORY.md` |
| What it is + install | `README.md` |

**Rule:** When updating information, find the canonical location first. If you update README.md with contract details that belong in CONTRACTS.md, you've created a future contradiction.

**Planned:** CI workflow (`.github/workflows/docs-check.yml`) to block PRs that change core Go files without updating documentation.
