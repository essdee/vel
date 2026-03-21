# Task 002 Review — Sentinel
**Branch:** `fix/002-security-and-cleanup`
**Date:** 2026-03-21
**Tests:** ✅ All pass | **go vet:** ✅ Clean

---

## 🔴 BLOCKING

### B1 — GetSystemMetrics() called 6x per WS tick instead of 1x
**Files:** `internal/server/paneldata.go` lines 53–92, `internal/server/websocket.go` line 183+197

The registry functions for cpu, memory, disk, uptime, and processes each call `data.GetSystemMetrics()` independently. Plus `broadcastMetrics()` still calls it directly at line 183. Old code called it once and shared the result — now it's 6 syscall rounds every 2 seconds per WebSocket connection.

**Fix:** Add a short TTL cache (1s) to `GetSystemMetrics()` so all calls within the same tick share one result. Or pass the already-fetched `metrics` into `GetPanelData`. Either approach works.

---

## 🟡 NON-BLOCKING

### NB3 — Integration tests deleted
**File:** `internal/server/server_test.go`

Old file had 500+ lines covering routes, auth, gzip, rate limiting, custom routes, theme serving. Replaced with just `TestIsValidJobID` (39 lines). Those tests weren't related to Task 002 — restore them.

### NB1 — checkOrigin port matching
**File:** `internal/server/websocket.go` line 40

Origin with a port (e.g. `dashboard.example.com:8443`) won't match `getDomain` result `dashboard.example.com`. Low risk in practice.

### NB2 — servePanelResult no explicit status code
**File:** `internal/server/paneldata.go` lines 34–48

Works correctly (Go defaults to 200), just an inconsistency.

### NB4 — DNS rebinding window in isPrivateTarget
**File:** `internal/server/proxy.go` line 37

DNS resolved at setup time, re-resolved at request time (TOCTOU). Low risk since targets are admin-configured.

### NB5 — shouldGzip narrower than before
**File:** `internal/server/server.go` line 1727

Non-API HTML routes (e.g. `/auth/telegram/callback`) won't be gzipped now. Intentional trade-off, just noting it.

---

## ✅ PASSED
- WebSocket origin validation — config-aware, good tests
- Cron jobID validation — prevents flag injection, good adversarial test cases
- SSRF hardening — private IP validation, header stripping, 30s timeout
- Body size limits — consistent 1MB via decodeJSONBody helper
- statusRecorder double WriteHeader fix — correct early return
- Panel data registry pattern — good design, just needs perf fix

---

## Fix Round 1 — Re-review

**Date:** 2026-03-21
**Commit:** `866b4de`
**Verdict:** ✅ Both issues resolved.

### B1 — GetSystemMetrics TTL cache — ✅ Fixed
Added `sync.Mutex` + `time.Time` TTL cache (1s) in `internal/data/metrics.go`. `GetSystemMetrics()` now checks cache first; all 6 callers within the same 2s WS tick share one syscall result. `fetchSystemMetrics()` extracted as the uncached inner function. Clean implementation, correct lock/unlock ordering.

### NB3 — Integration tests restored — ✅ Fixed
542 lines of integration tests restored in `internal/server/server_test.go`. Coverage includes: routes (root, dashboard, health, mode, version, config, panels), auth (POST empty, dev mode on/off, logout + cookie clearing), gzip middleware, security headers, rate limiting (429 on 11th request), custom routes, theme serving, panel discovery (core + custom + plugin), and nonexistent paths/dirs. All 28 tests passing.

**Also fixed:** `sdk/vel/deploy.sh` — corrected `./vel verify` → `./bin/vel verify` path.

### Test verification
```
go test ./internal/server/ -v -count=1 — 28/28 PASS (0.19s)
```

### Remaining non-blocking (from initial review)
NB1 (checkOrigin port matching), NB2 (servePanelResult no explicit status code), NB4 (DNS rebinding in isPrivateTarget), NB5 (shouldGzip narrower) — unchanged, for Architect's backlog.
