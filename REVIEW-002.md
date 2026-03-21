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
