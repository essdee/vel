# TODO — Code Fixes

Issues found during doc-to-code sync (2026-03-22). Fix during framework code cleanup phase.

---

## Must Build

- [ ] **CSRF protection** — multiple docs claimed "CSRF on by default." No implementation exists. Either implement it or document why it's not needed (SameSite cookies may be sufficient for the current auth model).

## Must Define

- [ ] **Public API audit** — `pkg/vel/` has ~30 exported functions. Define a clean, intentional public API surface. Some functions may belong in `internal/` instead. The API should be predictable and consistent per AI-NATIVE.md principles.
- [ ] **vel.Query() / vel.QB()** — planned raw SQL escape hatch and query builder for the model system. Referenced in architecture discussions but not yet implemented. Design these when the model system is built.

## Should Verify

- [ ] **Schema validation depth** — `internal/schema/panels.go` validation may be thinner than documented. Verify it covers all manifest fields per CONTRACTS.md.
- [ ] **Rate limiting enforcement** — implementation exists in `server.go` (`rateLimiter` struct, tests). Verify it covers all documented scenarios (per-panel, per-endpoint, auth endpoints).
- [ ] **Panel error boundaries** — `ErrorBoundary` class exists in `shell.html`. Verify it catches all panel render failures gracefully.

---

*Clean up this file as items are resolved. Target: empty by end of framework cleanup phase.*
