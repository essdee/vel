# Testing Strategy

This document is the **canonical location** for all testing decisions. For architecture WHY decisions, see [ARCHITECTURE.md](./ARCHITECTURE.md).

---

## Philosophy

Tests are a safety net, not a tax. Every test should justify its existence by catching a real class of bug.

**Go's type system is the first line of defense.** Struct fields, interfaces, and compile errors catch 60% of what dynamic languages test for. Our Go tests focus on the remaining 40%: runtime behavior, integration correctness, and edge cases.

---

## Three Layers

| Layer | What | How | When |
|-------|------|-----|------|
| **0 — Compile + Vet** | Type errors, unused imports, suspicious constructs | `go build` + `go vet` | Every commit (CI) |
| **1 — Unit Tests** | Package-level behavior, edge cases, contracts | `go test ./...` with `-race` | Every commit (CI) |
| **2 — Smoke Test** | Full server: endpoints respond, panels load, auth works | Start binary → curl endpoints → verify JSON | Before release |

### Layer 0: Compile + Vet (automatic)
The Go compiler rejects bad code. `go vet` catches subtle issues. Both run in CI on every push. Zero configuration.

### Layer 1: Unit Tests
Standard `testing` package. No test frameworks. No assertion libraries.

**Conventions:**
- Test files live next to the code they test: `auth.go` → `auth_test.go`
- Use `t.Run()` subtests for related cases
- Use `t.Parallel()` where safe (no shared state)
- Use `t.Setenv()` for environment variable tests
- Use `os.MkdirTemp()` for filesystem tests
- Table-driven tests for input/output variations
- `-race` flag in CI catches data races

**What to test:**
- Auth: HMAC validation, cookie signing/verification, TEST_MODE bypass
- Data: metrics collection returns valid structures
- Hooks: filter chaining, action firing, thread safety, nil handling
- Panels: discovery from core/custom/apps, override logic, invalid manifests
- Schema: error formatting
- Server: HTTP endpoints, middleware, auth flows

**What NOT to test:**
- Go stdlib behavior
- Simple getters/setters with no logic
- Code that would need mocking 5 dependencies to test one line

### Layer 2: Smoke Test
Start the binary with `TEST_MODE=true`, curl every endpoint, verify HTTP status codes and JSON shape.

---

## CI Pipeline

`.github/workflows/test.yml` runs on every push and PR to `main`:

```yaml
- go build .        # Layer 0: compiles
- go test ./... -race -count=1  # Layer 1: unit tests with race detection
- go vet ./...      # Layer 0: static analysis
```

Matrix: Go 1.22, 1.23, 1.24 on ubuntu-latest.

---

## Adding Tests for New Features

1. **New endpoint** → add test in `server_test.go`
2. **New data source** → add test in the relevant `data/*_test.go`
3. **New hook behavior** → add test in `hooks_test.go`
4. **New panel discovery logic** → add test in `registry_test.go`
5. **New auth flow** → add test in `auth_test.go`

---

---

## Test Mode & Fixtures (Phase 3)

`vel test` and `--test-mode` provide a fixture-based integration testing layer.

### Overview

| Feature | How |
|---------|-----|
| `vel start --test-mode` | Starts server with fixture data instead of real files |
| `vel start --test-mode --fixture=empty` | Uses a specific fixture set |
| `vel start --demo` | Shortcut: `--test-mode --fixture=demo` |
| `vel test` | Discovers apps, runs fixture-based checks, reports results |

### Startup banner

When test mode is active, the server prints:
```
⚡ Vel — TEST MODE
  Fixture: default
  Data sources redirected to testdata/default/

  ⚠️  Not for production use
```

### How data source path rewriting works

When `vel.IsTestMode()` is true, each `FileSource` checks for a fixture override before reading its real path:

```
original:  ~/.openclaw/workspace/claude-usage.json
fixture:   {appDir}/testdata/{fixture}/claude-usage.json
```

If the fixture file exists, it's used. Otherwise, the original path is used as a fallback.
This is transparent — apps and panels see the same data shape regardless.

### Creating testdata/ for an app

Each app that has file-based data sources should have:

```
apps/my-app/
  testdata/
    default/          ← realistic sample data
      my-data.json
    empty/            ← empty/zero-state data
      my-data.json
    stress/           ← large or edge-case data (optional)
      my-data.json
    demo/             ← curated demo data (optional)
      my-data.json
```

**Fixture set conventions:**
- `default` — realistic sample data, closest to production shape
- `empty` — zero state: empty arrays, null values, no records
- `stress` — large datasets, edge cases, unusual values
- `demo` — polished, curated data for demonstrations

The fixture filename must match the **basename** of the real source file.

Example: if `app.json` defines `"path": "~/.openclaw/workspace/token-swap-config.json"`,
the fixture file is `testdata/default/token-swap-config.json`.

### The `vel test` command

`vel test` starts an in-process test server (no subprocess exec) for each app+fixture combination:

1. Discovers all apps
2. For each app with a `testdata/` directory:
   - Finds available fixture sets (default, empty, stress, demo)
   - For each fixture: starts a server on a random port, waits for `/api/health`, runs checks, stops server
3. Checks per fixture:
   - `GET /api/health` → 200
   - `GET /dashboard` → 200 with HTML
   - `GET {first app route}` → not 5xx
4. Reports per-app, per-fixture results
5. Exits 0 if all pass, 1 if any fail

Apps without `testdata/` get a warning but don't fail the run.

### Running vel test

```bash
# From the vel-staging root
./vel test
```

### Demo mode shortcut

```bash
vel start --demo
# equivalent to:
vel start --test-mode --fixture=demo
```

---

## Decisions Log

| Decision | Rationale |
|----------|-----------|
| Standard `testing` only | Zero deps principle. `t.Run` + `if got != want` is sufficient. |
| `-race` flag in CI | Hooks engine and WebSocket use goroutines. Race detector catches data races. |
| No mock frameworks | Interfaces + manual test doubles. Keeps tests readable. |
| Co-located test files | Go convention. No separate `test/` directory. |
| Table-driven tests | Go idiom. Reduces boilerplate. |
| Fixture path rewriting in datasource | Transparent to apps — no API change needed. Basename match keeps it simple. |
| In-process test server | Shares code with prod binary, no exec overhead, port 0 for random assignment. |
