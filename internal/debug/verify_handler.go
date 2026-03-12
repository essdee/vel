package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"vel/internal/auth"
)

// RuntimeCheckResult matches verify.CheckResult format.
type RuntimeCheckResult struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "ok", "fail", "skipped"
	Detail string `json:"detail,omitempty"`
	Hint   string `json:"hint,omitempty"`
	Layer  int    `json:"layer"`
}

// RuntimeVerifyResult is the response from /debug/verify.
type RuntimeVerifyResult struct {
	Status  string               `json:"status"` // "ok" or "fail"
	Checks  []RuntimeCheckResult `json:"checks"`
	Passed  int                  `json:"passed"`
	Failed  int                  `json:"failed"`
	Skipped int                  `json:"skipped"`
}

// handleVerify runs all runtime checks in-process and returns JSON results.
func handleVerify(w http.ResponseWriter, r *http.Request) {
	info := serverInfo
	if info == nil || info.Mux == nil {
		w.WriteHeader(503)
		writeJSON(w, map[string]interface{}{"error": "server info not available"})
		return
	}

	var checks []RuntimeCheckResult

	// ── Runtime check 1: Endpoint data correctness (via raw mux, bypassing auth) ──
	checks = append(checks, checkEndpointViaHandler(info.Mux, "/api/health", "runtime:endpoint:/api/health", 200, "", 2)...)

	// Check /dashboard serves HTML
	checks = append(checks, checkEndpointViaHandler(info.Mux, "/dashboard", "runtime:endpoint:/dashboard", 200, "html", 2)...)

	// Check /api/sources returns data (bypassing auth via raw mux)
	checks = append(checks, checkEndpointViaHandler(info.Mux, "/api/sources", "runtime:endpoint:/api/sources", 200, "", 2)...)

	// ── Runtime check 2: Auth enforcement (via full handler with auth middleware) ──
	if info.Handler != nil && info.AuthMode != "none" {
		checks = append(checks, checkAuthEnforcement(info.Handler, info.AuthMode)...)
	}

	// ── Runtime check 3: App verify.json http_get checks (via raw mux, bypassing auth) ──
	checks = append(checks, checkAppVerifyHTTPGet(info)...)

	// ── Runtime check 4: deploy.sh health ──
	checks = append(checks, checkDeployScript(info)...)

	// Tally
	passed, failed, skipped := 0, 0, 0
	for _, c := range checks {
		switch c.Status {
		case "ok":
			passed++
		case "fail":
			failed++
		case "skipped":
			skipped++
		}
	}

	status := "ok"
	if failed > 0 {
		status = "fail"
	}

	writeJSON(w, RuntimeVerifyResult{
		Status:  status,
		Checks:  checks,
		Passed:  passed,
		Failed:  failed,
		Skipped: skipped,
	})
}

// identityContextKey must match the key used in server/authmiddleware.go.
const identityContextKey = "vel_identity"

// withSyntheticAuth adds a synthetic auth.Identity to the request context so inline
// auth checks (checkAuth) pass. This lets us verify data correctness in-process.
func withSyntheticAuth(req *http.Request) *http.Request {
	ctx := context.WithValue(req.Context(), identityContextKey, &auth.Identity{
		UserID:   "verify-internal",
		Provider: "debug",
		Name:     "Verify Check",
	})
	return req.WithContext(ctx)
}

// checkEndpointViaHandler makes an in-process request to the handler and validates the response.
func checkEndpointViaHandler(handler http.Handler, path, name string, expectStatus int, expectBody string, layer int) []RuntimeCheckResult {
	req := httptest.NewRequest("GET", path, nil)
	req.Host = "localhost"
	req = withSyntheticAuth(req)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != expectStatus {
		return []RuntimeCheckResult{{
			Name:   name,
			Status: "fail",
			Detail: fmt.Sprintf("expected status %d, got %d", expectStatus, resp.StatusCode),
			Layer:  layer,
		}}
	}

	if expectBody == "html" {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		bodyStr := strings.ToLower(string(body))
		if !strings.Contains(bodyStr, "<html") && !strings.Contains(bodyStr, "<!doctype") {
			return []RuntimeCheckResult{{
				Name:   name,
				Status: "fail",
				Detail: "response does not look like HTML",
				Layer:  layer,
			}}
		}
	}

	return []RuntimeCheckResult{{
		Name:   name,
		Status: "ok",
		Detail: fmt.Sprintf("HTTP %d", resp.StatusCode),
		Layer:  layer,
	}}
}

// checkAuthEnforcement verifies that the full handler (with auth) correctly blocks unauthenticated requests.
func checkAuthEnforcement(handler http.Handler, authMode string) []RuntimeCheckResult {
	var results []RuntimeCheckResult

	// Protected endpoints that should reject unauthenticated requests
	protectedPaths := []struct {
		path       string
		expectCode int // expected status for unauthenticated request
	}{
		{"/api/sources", 401},
		{"/dashboard", 302}, // redirects to login
	}

	for _, pp := range protectedPaths {
		req := httptest.NewRequest("GET", pp.path, nil)
		req.Host = "localhost"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		resp := rec.Result()
		resp.Body.Close()

		name := fmt.Sprintf("runtime:auth_enforced:%s", pp.path)

		// For telegram/token auth, unauthenticated should get rejected
		rejected := resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 302
		if rejected {
			results = append(results, RuntimeCheckResult{
				Name:   name,
				Status: "ok",
				Detail: fmt.Sprintf("unauthenticated request correctly rejected (HTTP %d)", resp.StatusCode),
				Layer:  1,
			})
		} else {
			results = append(results, RuntimeCheckResult{
				Name:   name,
				Status: "fail",
				Detail: fmt.Sprintf("expected auth rejection, got HTTP %d", resp.StatusCode),
				Layer:  1,
			})
		}
	}

	return results
}

// AppVerifyCheck mirrors the verify.json check schema.
type AppVerifyCheck struct {
	Type            string `json:"type"`
	Path            string `json:"path"`
	RelativeTo      string `json:"relative_to"`
	ExpectStatus    int    `json:"expect_status"`
	ExpectJSONField string `json:"expect_json_field"`
	Hint            string `json:"hint"`
}

// AppVerifyFile is the schema for verify.json.
type AppVerifyFile struct {
	Checks []AppVerifyCheck `json:"checks"`
}

// checkAppVerifyHTTPGet runs http_get checks from app verify.json files in-process.
func checkAppVerifyHTTPGet(info *ServerInfo) []RuntimeCheckResult {
	var results []RuntimeCheckResult

	if info.RootDir == "" {
		return results
	}

	appsDir := filepath.Join(info.RootDir, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return results
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		appName := entry.Name()
		appDir := filepath.Join(appsDir, appName)

		verifyPath := filepath.Join(appDir, "verify.json")
		data, err := os.ReadFile(verifyPath)
		if err != nil {
			continue
		}

		var vf AppVerifyFile
		if err := json.Unmarshal(data, &vf); err != nil {
			results = append(results, RuntimeCheckResult{
				Name:   fmt.Sprintf("app:%s:verify.json", appName),
				Status: "fail",
				Detail: "invalid verify.json: " + err.Error(),
				Layer:  3,
			})
			continue
		}

		for i, check := range vf.Checks {
			if check.Type != "http_get" {
				continue // file_exists checks are static, handled by CLI
			}

			checkName := fmt.Sprintf("app:%s:check%d", appName, i+1)
			if check.Path != "" {
				short := check.Path
				if len(short) > 30 {
					short = "..." + short[len(short)-27:]
				}
				checkName = fmt.Sprintf("app:%s:%s", appName, short)
			}

			expectStatus := check.ExpectStatus
			if expectStatus == 0 {
				expectStatus = 200
			}

			// Use raw mux with synthetic auth to check data correctness
			req := httptest.NewRequest("GET", check.Path, nil)
			req.Host = "localhost"
			req = withSyntheticAuth(req)
			rec := httptest.NewRecorder()
			info.Mux.ServeHTTP(rec, req)
			resp := rec.Result()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
			resp.Body.Close()

			// For checks that expect auth rejection (302, 401), we need the full handler
			if expectStatus == 302 || expectStatus == 401 {
				// This check expects auth-related behavior — use the full handler
				if info.Handler != nil && info.AuthMode != "none" {
					authReq := httptest.NewRequest("GET", check.Path, nil)
					authReq.Host = "localhost"
					authRec := httptest.NewRecorder()
					info.Handler.ServeHTTP(authRec, authReq)
					authResp := authRec.Result()
					authResp.Body.Close()

					if authResp.StatusCode == expectStatus {
						results = append(results, RuntimeCheckResult{
							Name:   checkName,
							Status: "ok",
							Detail: fmt.Sprintf("HTTP %d (auth enforced correctly)", authResp.StatusCode),
							Layer:  3,
						})
					} else {
						results = append(results, RuntimeCheckResult{
							Name:   checkName,
							Status: "fail",
							Detail: fmt.Sprintf("expected status %d, got %d", expectStatus, authResp.StatusCode),
							Hint:   check.Hint,
							Layer:  3,
						})
					}
				} else {
					// No auth configured — check raw mux response instead
					// Without auth, the endpoint should return 200 (data accessible)
					if resp.StatusCode == 200 {
						results = append(results, RuntimeCheckResult{
							Name:   checkName,
							Status: "ok",
							Detail: fmt.Sprintf("HTTP %d (no auth, endpoint accessible)", resp.StatusCode),
							Layer:  3,
						})
					} else {
						results = append(results, RuntimeCheckResult{
							Name:   checkName,
							Status: "fail",
							Detail: fmt.Sprintf("expected 200 (no auth mode), got %d", resp.StatusCode),
							Hint:   check.Hint,
							Layer:  3,
						})
					}
				}
				continue
			}

			// Normal status check against raw mux
			if resp.StatusCode != expectStatus {
				results = append(results, RuntimeCheckResult{
					Name:   checkName,
					Status: "fail",
					Detail: fmt.Sprintf("expected status %d, got %d", expectStatus, resp.StatusCode),
					Hint:   check.Hint,
					Layer:  3,
				})
				continue
			}

			// Check JSON field if required
			if check.ExpectJSONField != "" {
				var obj map[string]json.RawMessage
				if err := json.Unmarshal(body, &obj); err != nil {
					results = append(results, RuntimeCheckResult{
						Name:   checkName,
						Status: "fail",
						Detail: fmt.Sprintf("expected JSON with field %q but response is not valid JSON", check.ExpectJSONField),
						Hint:   check.Hint,
						Layer:  3,
					})
					continue
				}
				if _, ok := obj[check.ExpectJSONField]; !ok {
					results = append(results, RuntimeCheckResult{
						Name:   checkName,
						Status: "fail",
						Detail: fmt.Sprintf("JSON field %q not found in response", check.ExpectJSONField),
						Hint:   check.Hint,
						Layer:  3,
					})
					continue
				}
			}

			results = append(results, RuntimeCheckResult{
				Name:   checkName,
				Status: "ok",
				Detail: fmt.Sprintf("HTTP %d", resp.StatusCode),
				Layer:  3,
			})
		}
	}

	return results
}

// checkDeployScript validates that deploy.sh exists, has valid bash syntax,
// and can detect the correct systemd service.
func checkDeployScript(info *ServerInfo) []RuntimeCheckResult {
	var results []RuntimeCheckResult
	rootDir := info.RootDir

	// Find deploy.sh
	deployScript := filepath.Join(rootDir, "sdk", "vel", "deploy.sh")
	if _, err := os.Stat(deployScript); err != nil {
		deployScript = filepath.Join(rootDir, "deploy.sh")
	}

	// Check 1: deploy.sh exists
	if _, err := os.Stat(deployScript); err != nil {
		results = append(results, RuntimeCheckResult{
			Name:   "runtime:deploy:script-exists",
			Status: "fail",
			Detail: "deploy.sh not found at sdk/vel/deploy.sh or root",
			Hint:   "Copy sdk/vel/deploy.sh.example to sdk/vel/deploy.sh",
			Layer:  3,
		})
		return results
	}
	results = append(results, RuntimeCheckResult{
		Name:   "runtime:deploy:script-exists",
		Status: "ok",
		Detail: deployScript,
		Layer:  3,
	})

	// Check 2: bash syntax valid
	cmd := exec.Command("bash", "-n", deployScript)
	if out, err := cmd.CombinedOutput(); err != nil {
		results = append(results, RuntimeCheckResult{
			Name:   "runtime:deploy:bash-syntax",
			Status: "fail",
			Detail: strings.TrimSpace(string(out)),
			Hint:   "Fix syntax errors in deploy.sh",
			Layer:  3,
		})
		return results
	}
	results = append(results, RuntimeCheckResult{
		Name:   "runtime:deploy:bash-syntax",
		Status: "ok",
		Detail: "syntax valid",
		Layer:  3,
	})

	// Check 3: VEL_DIR resolves correctly
	// Run the script's directory resolution logic to verify it points to rootDir
	resolveCmd := exec.Command("bash", "-c", fmt.Sprintf(
		`cd "$(dirname "%s")/../.." && pwd`, deployScript))
	if out, err := resolveCmd.Output(); err == nil {
		resolved := strings.TrimSpace(string(out))
		if resolved == rootDir {
			results = append(results, RuntimeCheckResult{
				Name:   "runtime:deploy:vel-dir",
				Status: "ok",
				Detail: fmt.Sprintf("resolves to %s", resolved),
				Layer:  3,
			})
		} else {
			results = append(results, RuntimeCheckResult{
				Name:   "runtime:deploy:vel-dir",
				Status: "fail",
				Detail: fmt.Sprintf("resolves to %s, expected %s", resolved, rootDir),
				Hint:   "deploy.sh VEL_DIR must resolve to the Vel root directory",
				Layer:  3,
			})
		}
	}

	return results
}
