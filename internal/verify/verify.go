package verify

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"vel/internal/apps"
	"vel/internal/panels"
	vel "vel/pkg/vel"
)

// CheckResult is the result of a single health check.
type CheckResult struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "ok", "fail", or "skipped"
	Detail string `json:"detail,omitempty"`
	Hint   string `json:"hint,omitempty"`
	Layer  int    `json:"layer"`
}

// VerifyResult is the overall result of a health check run.
type VerifyResult struct {
	Status  string        `json:"status"` // "ok" or "fail"
	Checks  []CheckResult `json:"checks"`
	Passed  int           `json:"passed"`
	Failed  int           `json:"failed"`
	Skipped int           `json:"skipped"`
}

// VerifyConfig holds runtime context for the verify run.
type VerifyConfig struct {
	RootDir   string
	Apps      []*apps.App
	Registry  *panels.Registry
	Workspace string
}

// serverConfig holds extracted server settings from config.json.
type serverConfig struct {
	port      int
	authToken string
	authMode  string
}

// readServerConfig extracts port, auth token, and auth mode from config.json.
func readServerConfig(rootDir string) serverConfig {
	cfg := serverConfig{port: 3700} // default

	// Check PORT env
	if p := os.Getenv("PORT"); p != "" {
		fmt.Sscanf(p, "%d", &cfg.port)
	}

	configPath := filepath.Join(rootDir, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return cfg
	}

	// Port: check top-level "port" field first, then "server.port"
	if v, ok := raw["port"]; ok {
		var port int
		if json.Unmarshal(v, &port) == nil && port > 0 {
			cfg.port = port
		}
	}
	if v, ok := raw["server"]; ok {
		var srv struct {
			Port int `json:"port"`
		}
		if json.Unmarshal(v, &srv) == nil && srv.Port > 0 {
			cfg.port = srv.Port
		}
	}

	// Auth settings
	if authRaw, ok := raw["auth"]; ok {
		var authObj struct {
			Mode  string `json:"mode"`
			Token string `json:"token"`
		}
		if json.Unmarshal(authRaw, &authObj) == nil {
			cfg.authToken = authObj.Token
			cfg.authMode = authObj.Mode
		}
	}

	// Detect if new auth system is active (users.json exists)
	usersPath := filepath.Join(rootDir, "users.json")
	newAuthActive := false
	if _, err := os.Stat(usersPath); err == nil {
		newAuthActive = true
	}

	// Auto-detect auth mode
	// When new auth system is active, override legacy config mode
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		if envData, err := os.ReadFile(filepath.Join(rootDir, ".env")); err == nil {
			for _, line := range strings.Split(string(envData), "\n") {
				if strings.HasPrefix(line, "BOT_TOKEN=") {
					botToken = strings.TrimSpace(strings.TrimPrefix(line, "BOT_TOKEN="))
				}
			}
		}
	}

	if newAuthActive {
		// New auth system: telegram provider if BOT_TOKEN exists
		if botToken != "" {
			cfg.authMode = "telegram"
		} else {
			cfg.authMode = "none"
		}
		cfg.authToken = "" // legacy token not used in new auth
	} else if cfg.authMode == "" {
		// Legacy auto-detect
		if botToken != "" {
			cfg.authMode = "telegram"
		} else if cfg.authToken != "" {
			cfg.authMode = "token"
		} else {
			cfg.authMode = "none"
		}
	}

	return cfg
}

// RunVerify executes all built-in checks plus any app-registered checks.
func RunVerify(cfg VerifyConfig) VerifyResult {
	var checks []CheckResult

	// ── Layer -1: existing core checks (config, auth token, openclaw-cli, telegram domain)
	checks = append(checks, checkConfig(cfg.RootDir))
	checks = append(checks, checkAuth(cfg.RootDir))
	checks = append(checks, checkOpenclawCLI())
	checks = append(checks, checkTelegramDomain(cfg.RootDir))

	// ── Layer -1: existing panel + data checks
	checks = append(checks, checkPanels(cfg.Registry)...)
	checks = append(checks, checkPanelData(cfg.Apps)...)

	// ── Layer -1: app-registered checks
	for _, hc := range vel.GetHealthChecks() {
		pass, detail := hc.Check()
		status := "ok"
		if !pass {
			status = "fail"
		}
		checks = append(checks, CheckResult{
			Name:   hc.Name,
			Status: status,
			Detail: detail,
			Layer:  -1,
		})
	}

	// ── Layer 0: Framework — server reachability
	srvCfg := readServerConfig(cfg.RootDir)
	serverUp := false
	layer0Check := checkServerHealth(srvCfg.port)
	checks = append(checks, layer0Check)
	if layer0Check.Status == "ok" {
		serverUp = true
	}

	// ── Layer 1: Auth probe
	authProbeOk := false
	if serverUp {
		authProbe := checkAuthProbe(srvCfg)
		checks = append(checks, authProbe)
		if authProbe.Status == "ok" {
			authProbeOk = true
		}
	} else {
		checks = append(checks, CheckResult{
			Name:   "auth.probe",
			Status: "skipped",
			Detail: "skipped — server not reachable",
			Layer:  1,
		})
	}

	// ── Layer 2: Endpoint checks
	if serverUp && authProbeOk {
		checks = append(checks, checkEndpoints(srvCfg, cfg.Apps)...)
	} else if serverUp {
		// Auth failed — skip endpoints
		checks = append(checks, CheckResult{
			Name:   "endpoints",
			Status: "skipped",
			Detail: "skipped — auth probe failed (all endpoint checks skipped)",
			Layer:  2,
		})
	} else {
		checks = append(checks, CheckResult{
			Name:   "endpoints",
			Status: "skipped",
			Detail: "skipped — server not reachable",
			Layer:  2,
		})
	}


	// ── Layer 3: App verify.json (always runs)
	checks = append(checks, checkAppVerifyJSON(cfg.RootDir, srvCfg, cfg.Apps)...)

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

	return VerifyResult{
		Status:  status,
		Checks:  checks,
		Passed:  passed,
		Failed:  failed,
		Skipped: skipped,
	}
}

// ── Layer 0 ──────────────────────────────────────────────────────────────────

// checkServerHealth checks if the vel server is reachable on the configured port.
func checkServerHealth(port int) CheckResult {
	url := fmt.Sprintf("http://localhost:%d/api/health", port)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return CheckResult{
			Name:   "server.reachable",
			Status: "fail",
			Detail: fmt.Sprintf("server not reachable on port %d: %s", port, err),
			Hint:   "Start the vel server with: ./vel start",
			Layer:  0,
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return CheckResult{
			Name:   "server.reachable",
			Status: "fail",
			Detail: fmt.Sprintf("GET /api/health returned %d (expected 200)", resp.StatusCode),
			Layer:  0,
		}
	}
	return CheckResult{
		Name:   "server.reachable",
		Status: "ok",
		Detail: fmt.Sprintf("server up on port %d", port),
		Layer:  0,
	}
}

// ── Layer 1 ──────────────────────────────────────────────────────────────────

// checkAuthProbe verifies auth behaviour by probing a protected API endpoint.
// Note: /dashboard and /api/health are public (HTML shell + health); we probe /api/sources
// which requires authentication for all modes except "none".
func checkAuthProbe(cfg serverConfig) CheckResult {
	base := fmt.Sprintf("http://localhost:%d", cfg.port)
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	// /api/sources is a protected endpoint that returns 403 without auth
	protectedPath := "/api/sources"

	switch cfg.authMode {
	case "none":
		// All requests should succeed
		resp, err := client.Get(base + protectedPath)
		if err != nil {
			return CheckResult{
				Name:   "auth.probe",
				Status: "fail",
				Detail: "could not reach server for auth probe: " + err.Error(),
				Layer:  1,
			}
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return CheckResult{
				Name:   "auth.probe",
				Status: "ok",
				Detail: "auth mode: none — unauthenticated access works",
				Layer:  1,
			}
		}
		return CheckResult{
			Name:   "auth.probe",
			Status: "fail",
			Detail: fmt.Sprintf("auth mode: none — expected 200 on %s, got %d", protectedPath, resp.StatusCode),
			Layer:  1,
		}

	case "token":
		// Without token: should get 401/403
		resp, err := client.Get(base + protectedPath)
		if err != nil {
			return CheckResult{
				Name:   "auth.probe",
				Status: "fail",
				Detail: "could not reach server for auth probe: " + err.Error(),
				Layer:  1,
			}
		}
		resp.Body.Close()
		unauthStatus := resp.StatusCode

		// With token: should get 200 (try Bearer header first, fall back to query param)
		var authStatus int
		if cfg.authToken != "" {
			// Try Bearer header first (new auth system)
			req, _ := http.NewRequest("GET", base+protectedPath, nil)
			req.Header.Set("Authorization", "Bearer "+cfg.authToken)
			resp2, err := client.Do(req)
			if err != nil {
				return CheckResult{
					Name:   "auth.probe",
					Status: "fail",
					Detail: "auth probe with token failed: " + err.Error(),
					Layer:  1,
				}
			}
			resp2.Body.Close()
			authStatus = resp2.StatusCode

			// Bearer header is the only supported method for new auth
		}

		unauthRejected := unauthStatus == 401 || unauthStatus == 403
		authAccepted := cfg.authToken == "" || authStatus == 200

		if unauthRejected && authAccepted {
			detail := fmt.Sprintf("auth mode: token — unauth=%d (rejected)", unauthStatus)
			if cfg.authToken != "" {
				detail += fmt.Sprintf(", with token=%d (accepted)", authStatus)
			}
			return CheckResult{
				Name:   "auth.probe",
				Status: "ok",
				Detail: detail,
				Layer:  1,
			}
		}
		return CheckResult{
			Name:   "auth.probe",
			Status: "fail",
			Detail: fmt.Sprintf("auth mode: token — unauth=%d (want 401/403), with token=%d (want 200)", unauthStatus, authStatus),
			Hint:   "Check auth.token in config.json matches what the server uses",
			Layer:  1,
		}

	case "telegram":
		// Without auth cookie: should get 403
		resp, err := client.Get(base + protectedPath)
		if err != nil {
			return CheckResult{
				Name:   "auth.probe",
				Status: "fail",
				Detail: "could not reach server for auth probe: " + err.Error(),
				Layer:  1,
			}
		}
		resp.Body.Close()
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return CheckResult{
				Name:   "auth.probe",
				Status: "ok",
				Detail: fmt.Sprintf("auth mode: telegram — unauthenticated requests correctly rejected (%d)", resp.StatusCode),
				Layer:  1,
			}
		}
		return CheckResult{
			Name:   "auth.probe",
			Status: "fail",
			Detail: fmt.Sprintf("auth mode: telegram — expected 401/403 on unauthenticated %s, got %d", protectedPath, resp.StatusCode),
			Layer:  1,
		}

	default:
		return CheckResult{
			Name:   "auth.probe",
			Status: "ok",
			Detail: fmt.Sprintf("auth mode: %s — probe skipped", cfg.authMode),
			Layer:  1,
		}
	}
}

// ── Layer 2 ──────────────────────────────────────────────────────────────────

// checkEndpoints verifies key HTTP endpoints return expected responses.
func checkEndpoints(cfg serverConfig, appList []*apps.App) []CheckResult {
	base := fmt.Sprintf("http://localhost:%d", cfg.port)
	client := &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // don't follow redirects
		},
	}

	// Build authenticated request helper
	authedReq := func(path string) *http.Request {
		url := base + path
		if cfg.authToken != "" && cfg.authMode == "token" {
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("Authorization", "Bearer "+cfg.authToken)
			return req
		}
		req, _ := http.NewRequest("GET", url, nil)
		return req
	}

	var results []CheckResult

	// /api/health — public health endpoint (always accessible)
	results = append(results, checkHTTPEndpoint(client, base+"/api/health", "endpoint:/api/health", 200, "", 2))

	// For telegram auth mode, we can't authenticate in verify.
	// Instead, verify that protected endpoints correctly reject unauthenticated requests.
	if cfg.authMode == "telegram" {
		// /dashboard — should redirect to login (302)
		results = append(results, checkHTTPEndpoint(client, base+"/dashboard", "endpoint:/dashboard", 302, "", 2))
		// /api/sources — should return 401
		results = append(results, checkHTTPEndpoint(client, base+"/api/sources", "endpoint:/api/sources", 401, "", 2))
		// /login — should be accessible (200)
		results = append(results, checkHTTPEndpoint(client, base+"/login", "endpoint:/login", 200, "html", 2))
		return results
	}

	// For token auth mode, test with authentication
	if cfg.authMode == "token" && cfg.authToken != "" {
		// /dashboard — authenticated
		req := authedReq("/dashboard")
		resp, err := client.Do(req)
		if err != nil {
			results = append(results, CheckResult{Name: "endpoint:/dashboard", Status: "fail", Detail: "request failed: " + err.Error(), Layer: 2})
		} else {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				results = append(results, CheckResult{Name: "endpoint:/dashboard", Status: "ok", Detail: "HTTP 200", Layer: 2})
			} else {
				results = append(results, CheckResult{Name: "endpoint:/dashboard", Status: "fail", Detail: fmt.Sprintf("expected 200, got %d", resp.StatusCode), Layer: 2})
			}
		}
		// /api/sources — authenticated
		req2 := authedReq("/api/sources")
		resp2, err := client.Do(req2)
		if err != nil {
			results = append(results, CheckResult{Name: "endpoint:/api/sources", Status: "fail", Detail: "request failed: " + err.Error(), Layer: 2})
		} else {
			resp2.Body.Close()
			if resp2.StatusCode == 200 {
				results = append(results, CheckResult{Name: "endpoint:/api/sources", Status: "ok", Detail: "HTTP 200", Layer: 2})
			} else {
				results = append(results, CheckResult{Name: "endpoint:/api/sources", Status: "fail", Detail: fmt.Sprintf("expected 200, got %d", resp2.StatusCode), Layer: 2})
			}
		}
	} else {
		// auth mode "none" — no auth needed
		results = append(results, checkHTTPEndpoint(client, base+"/dashboard", "endpoint:/dashboard", 200, "html", 2))
		results = append(results, checkHTTPEndpoint(client, base+"/api/sources", "endpoint:/api/sources", 200, "", 2))
	}

	// Each registered app's actual routes (from app.json)
	for _, app := range appList {
		if len(app.Routes) == 0 {
			continue
		}
		for routePath := range app.Routes {
			if cfg.authMode == "telegram" {
				// Can't auth — skip app route checks
				results = append(results, CheckResult{Name: "endpoint:" + routePath, Status: "skipped", Detail: "skipped — telegram auth (no token for verify)", Layer: 2})
			} else if cfg.authMode == "token" && cfg.authToken != "" {
				req := authedReq(routePath)
				resp, err := client.Do(req)
				if err != nil {
					results = append(results, CheckResult{Name: "endpoint:" + routePath, Status: "fail", Detail: "request failed: " + err.Error(), Layer: 2})
				} else {
					resp.Body.Close()
					if resp.StatusCode == 200 {
						results = append(results, CheckResult{Name: "endpoint:" + routePath, Status: "ok", Detail: "HTTP 200", Layer: 2})
					} else {
						results = append(results, CheckResult{Name: "endpoint:" + routePath, Status: "fail", Detail: fmt.Sprintf("expected 200, got %d", resp.StatusCode), Layer: 2})
					}
				}
			} else {
				results = append(results, checkHTTPEndpoint(client, base+routePath, "endpoint:"+routePath, 200, "", 2))
			}
			break
		}
	}

	return results
}

// checkHTTPEndpoint performs a single HTTP GET and validates the response.
func checkHTTPEndpoint(client *http.Client, url, name string, expectStatus int, expectBody string, layer int) CheckResult {
	resp, err := client.Get(url)
	if err != nil {
		return CheckResult{
			Name:   name,
			Status: "fail",
			Detail: "request failed: " + err.Error(),
			Layer:  layer,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectStatus {
		return CheckResult{
			Name:   name,
			Status: "fail",
			Detail: fmt.Sprintf("expected status %d, got %d", expectStatus, resp.StatusCode),
			Layer:  layer,
		}
	}

	if expectBody == "html" {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if err != nil {
			return CheckResult{
				Name:   name,
				Status: "fail",
				Detail: "could not read response body",
				Layer:  layer,
			}
		}
		bodyStr := strings.ToLower(string(body))
		if !strings.Contains(bodyStr, "<html") && !strings.Contains(bodyStr, "<!doctype") {
			return CheckResult{
				Name:   name,
				Status: "fail",
				Detail: "response does not look like HTML",
				Layer:  layer,
			}
		}
	}

	return CheckResult{
		Name:   name,
		Status: "ok",
		Detail: fmt.Sprintf("HTTP %d", resp.StatusCode),
		Layer:  layer,
	}
}

// ── Layer 3 ──────────────────────────────────────────────────────────────────

// AppVerifyCheck is a single check from an app's verify.json.
type AppVerifyCheck struct {
	Type            string `json:"type"`             // "http_get" | "file_exists"
	Path            string `json:"path"`             // URL path or file path
	RelativeTo      string `json:"relative_to"`      // "app" | "root" | "workspace" | "absolute"
	ExpectStatus    int    `json:"expect_status"`    // for http_get
	ExpectJSONField string `json:"expect_json_field"` // for http_get
	Hint            string `json:"hint"`
}

// AppVerifyFile is the schema for verify.json in each app directory.
type AppVerifyFile struct {
	Checks []AppVerifyCheck `json:"checks"`
}

// checkAppVerifyJSON reads and runs verify.json checks for each app.
func checkAppVerifyJSON(rootDir string, cfg serverConfig, appList []*apps.App) []CheckResult {
	var results []CheckResult
	base := fmt.Sprintf("http://localhost:%d", cfg.port)
	home, _ := os.UserHomeDir()

	client := &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // don't follow redirects
		},
	}

	// authedRequest creates an authenticated request for the given path
	authedRequest := func(path string) *http.Request {
		url := base + path
		req, _ := http.NewRequest("GET", url, nil)
		if cfg.authToken != "" && cfg.authMode == "token" {
			req.Header.Set("Authorization", "Bearer "+cfg.authToken)
		}
		return req
	}
	// For telegram mode, skip http_get checks on protected endpoints
	_ = authedRequest

	// Map app name → app dir
	appDirs := make(map[string]string)
	for _, app := range appList {
		appDirs[app.Name] = app.Dir
	}

	// Scan all app directories under rootDir/apps/
	appsDir := filepath.Join(rootDir, "apps")
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
			continue // no verify.json — that's fine
		}

		var vf AppVerifyFile
		if err := json.Unmarshal(data, &vf); err != nil {
			results = append(results, CheckResult{
				Name:   fmt.Sprintf("verify.json:%s", appName),
				Status: "fail",
				Detail: "invalid verify.json: " + err.Error(),
				Layer:  3,
			})
			continue
		}

		for i, check := range vf.Checks {
			checkName := fmt.Sprintf("app:%s:check%d", appName, i+1)
			if check.Path != "" {
				// Use path as suffix for readability
				short := check.Path
				if len(short) > 30 {
					short = "..." + short[len(short)-27:]
				}
				checkName = fmt.Sprintf("app:%s:%s", appName, short)
			}

			switch check.Type {
			case "http_get":
				expectStatus := check.ExpectStatus
				if expectStatus == 0 {
					expectStatus = 200
				}

				req := authedRequest(check.Path)
				resp, err := client.Do(req)
				if err != nil {
					results = append(results, CheckResult{
						Name:   checkName,
						Status: "fail",
						Detail: "request failed: " + err.Error(),
						Hint:   check.Hint,
						Layer:  3,
					})
					continue
				}

				body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
				resp.Body.Close()

				if resp.StatusCode != expectStatus {
					results = append(results, CheckResult{
						Name:   checkName,
						Status: "fail",
						Detail: fmt.Sprintf("expected status %d, got %d", expectStatus, resp.StatusCode),
						Hint:   check.Hint,
						Layer:  3,
					})
					continue
				}

				if check.ExpectJSONField != "" {
					var obj map[string]json.RawMessage
					if err := json.Unmarshal(body, &obj); err != nil {
						results = append(results, CheckResult{
							Name:   checkName,
							Status: "fail",
							Detail: fmt.Sprintf("expected JSON with field %q but response is not valid JSON", check.ExpectJSONField),
							Hint:   check.Hint,
							Layer:  3,
						})
						continue
					}
					if _, ok := obj[check.ExpectJSONField]; !ok {
						results = append(results, CheckResult{
							Name:   checkName,
							Status: "fail",
							Detail: fmt.Sprintf("JSON field %q not found in response", check.ExpectJSONField),
							Hint:   check.Hint,
							Layer:  3,
						})
						continue
					}
				}

				results = append(results, CheckResult{
					Name:   checkName,
					Status: "ok",
					Detail: fmt.Sprintf("HTTP %d", resp.StatusCode),
					Layer:  3,
				})

			case "file_exists":
				var fullPath string
				switch check.RelativeTo {
				case "app":
					fullPath = filepath.Join(appDir, check.Path)
				case "root":
					fullPath = filepath.Join(rootDir, check.Path)
				case "workspace":
					fullPath = filepath.Join(home, ".openclaw", "workspace", check.Path)
				case "absolute", "":
					if filepath.IsAbs(check.Path) {
						fullPath = check.Path
					} else {
						fullPath = filepath.Join(appDir, check.Path)
					}
				default:
					fullPath = filepath.Join(appDir, check.Path)
				}

				if _, err := os.Stat(fullPath); err != nil {
					results = append(results, CheckResult{
						Name:   checkName,
						Status: "fail",
						Detail: fmt.Sprintf("file not found: %s", fullPath),
						Hint:   check.Hint,
						Layer:  3,
					})
				} else {
					results = append(results, CheckResult{
						Name:   checkName,
						Status: "ok",
						Detail: fmt.Sprintf("file exists: %s", fullPath),
						Layer:  3,
					})
				}

			default:
				results = append(results, CheckResult{
					Name:   checkName,
					Status: "fail",
					Detail: fmt.Sprintf("unknown check type: %s", check.Type),
					Layer:  3,
				})
			}
		}
	}

	return results
}

// ── Existing checks (unchanged) ──────────────────────────────────────────────

// checkConfig verifies config.json exists and has required fields.
func checkConfig(rootDir string) CheckResult {
	configPath := filepath.Join(rootDir, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return CheckResult{Name: "config", Status: "fail", Detail: "config.json not found"}
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return CheckResult{Name: "config", Status: "fail", Detail: "config.json is not valid JSON: " + err.Error()}
	}

	// Check required fields (flexible: authUrl or siteUrl, allowedTelegramUsers or allowedUsers)
	var missing []string

	// botToken is in .env, not config.json — skip here, handled by auth check
	// siteUrl / authUrl
	if _, ok := raw["authUrl"]; !ok {
		if _, ok2 := raw["siteUrl"]; !ok2 {
			missing = append(missing, "authUrl")
		}
	}

	// allowedTelegramUsers / allowedUsers
	hasUsers := false
	if authRaw, ok := raw["auth"]; ok {
		var authObj map[string]json.RawMessage
		if json.Unmarshal(authRaw, &authObj) == nil {
			if _, ok2 := authObj["allowedTelegramUsers"]; ok2 {
				hasUsers = true
			}
		}
	}
	if !hasUsers {
		if _, ok := raw["allowedUsers"]; ok {
			hasUsers = true
		}
	}
	if !hasUsers {
		missing = append(missing, "allowedTelegramUsers")
	}

	if len(missing) > 0 {
		return CheckResult{
			Name:   "config",
			Status: "fail",
			Detail: "missing fields: " + strings.Join(missing, ", "),
		}
	}
	return CheckResult{Name: "config", Status: "ok", Detail: "config.json valid"}
}

// checkAuth verifies a bot token is configured.
func checkAuth(rootDir string) CheckResult {
	// Check environment
	if token := os.Getenv("BOT_TOKEN"); token != "" {
		return CheckResult{Name: "auth", Status: "ok", Detail: "bot token configured (env)"}
	}

	// Check .env file
	envData, err := os.ReadFile(filepath.Join(rootDir, ".env"))
	if err == nil {
		for _, line := range strings.Split(string(envData), "\n") {
			if strings.HasPrefix(line, "BOT_TOKEN=") {
				token := strings.TrimSpace(strings.TrimPrefix(line, "BOT_TOKEN="))
				if token != "" {
					return CheckResult{Name: "auth", Status: "ok", Detail: "bot token configured (.env)"}
				}
			}
		}
	}

	// Check config.json for token auth mode
	configPath := filepath.Join(rootDir, "config.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var raw map[string]json.RawMessage
		if json.Unmarshal(data, &raw) == nil {
			if authRaw, ok := raw["auth"]; ok {
				var authObj map[string]interface{}
				if json.Unmarshal(authRaw, &authObj) == nil {
					if token, ok := authObj["token"].(string); ok && token != "" {
						return CheckResult{Name: "auth", Status: "ok", Detail: "token auth configured"}
					}
					if mode, ok := authObj["mode"].(string); ok && mode == "none" {
						return CheckResult{Name: "auth", Status: "ok", Detail: "auth mode: none"}
					}
				}
			}
		}
	}

	return CheckResult{Name: "auth", Status: "fail", Detail: "no BOT_TOKEN found (check .env or environment)"}
}

// checkPanels verifies all loaded panels have valid manifests.
func checkPanels(registry *panels.Registry) []CheckResult {
	if registry == nil {
		return []CheckResult{{Name: "panels", Status: "fail", Detail: "no panel registry"}}
	}

	entries := registry.Entries()
	if len(entries) == 0 {
		return []CheckResult{{Name: "panels", Status: "ok", Detail: "no panels registered"}}
	}

	var results []CheckResult
	for id, info := range entries {
		if strings.HasPrefix(id, "_") {
			continue // skip test panels
		}
		name := fmt.Sprintf("panel:%s", id)
		if info.Manifest == nil {
			results = append(results, CheckResult{Name: name, Status: "fail", Detail: "no manifest"})
			continue
		}
		results = append(results, CheckResult{Name: name, Status: "ok"})
	}

	if len(results) == 0 {
		return []CheckResult{{Name: "panels", Status: "ok", Detail: "no panels to check"}}
	}
	return results
}

// checkPanelData verifies data source files/generators exist for panels that need them.
func checkPanelData(appList []*apps.App) []CheckResult {
	var results []CheckResult
	home, _ := os.UserHomeDir()

	for _, app := range appList {
		for _, ds := range app.ParsedSources {
			path := ds.Path
			// Expand ~ to home dir
			if strings.HasPrefix(path, "~/") {
				path = filepath.Join(home, path[2:])
			}

			name := fmt.Sprintf("data:%s:%s", app.Name, ds.Name)
			if _, err := os.Stat(path); err != nil {
				results = append(results, CheckResult{
					Name:   name,
					Status: "fail",
					Detail: fmt.Sprintf("data file not found: %s", ds.Path),
				})
			} else {
				results = append(results, CheckResult{
					Name:   name,
					Status: "ok",
					Detail: fmt.Sprintf("file exists: %s", ds.Path),
				})
			}
		}
	}

	return results
}

// checkTelegramDomain detects Login Widget domain mismatch via oauth.telegram.org.
func checkTelegramDomain(rootDir string) CheckResult {
	const name = "auth.telegram_domain"

	// Get bot token (env → .env file)
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		if envData, err := os.ReadFile(filepath.Join(rootDir, ".env")); err == nil {
			for _, line := range strings.Split(string(envData), "\n") {
				if strings.HasPrefix(line, "BOT_TOKEN=") {
					botToken = strings.TrimSpace(strings.TrimPrefix(line, "BOT_TOKEN="))
				}
			}
		}
	}
	if botToken == "" {
		return CheckResult{Name: name, Status: "ok", Detail: "skipped (no bot token)"}
	}

	// Extract bot ID from token (format: BOTID:SECRET)
	parts := strings.SplitN(botToken, ":", 2)
	if len(parts) != 2 || parts[0] == "" {
		return CheckResult{Name: name, Status: "ok", Detail: "skipped (unrecognised token format)"}
	}
	botID := parts[0]

	// Get domain from config.json (authUrl or siteUrl)
	domain := ""
	configPath := filepath.Join(rootDir, "config.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var raw map[string]json.RawMessage
		if json.Unmarshal(data, &raw) == nil {
			for _, key := range []string{"authUrl", "siteUrl"} {
				if v, ok := raw[key]; ok {
					var s string
					if json.Unmarshal(v, &s) == nil && s != "" {
						// Strip path, keep scheme+host
						s = strings.TrimSpace(s)
						if idx := strings.Index(s, "://"); idx >= 0 {
							rest := s[idx+3:] // strip scheme
							if slash := strings.Index(rest, "/"); slash >= 0 {
								rest = rest[:slash]
							}
							domain = rest
						}
						break
					}
				}
			}
		}
	}
	if domain == "" {
		return CheckResult{Name: name, Status: "ok", Detail: "skipped (no siteUrl/authUrl in config)"}
	}

	// Call oauth.telegram.org
	url := fmt.Sprintf("https://oauth.telegram.org/auth?bot_id=%s&origin=https://%s", botID, domain)
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return CheckResult{Name: name, Status: "ok", Detail: "could not verify (network)"}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return CheckResult{Name: name, Status: "ok", Detail: "could not verify (read error)"}
	}

	if strings.Contains(string(body), "Bot domain invalid") {
		return CheckResult{
			Name:   name,
			Status: "fail",
			Detail: fmt.Sprintf(
				"BotFather Login Widget domain doesn't match config siteUrl. Fix: @BotFather → /mybots → Bot Settings → Domain → set to %s",
				domain,
			),
		}
	}

	return CheckResult{Name: name, Status: "ok", Detail: fmt.Sprintf("domain %s accepted by Telegram", domain)}
}

// openclawFallbackPaths returns paths to try when 'openclaw' isn't in PATH.
func openclawFallbackPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".npm-global", "bin", "openclaw"),
		"/usr/local/bin/openclaw",
		"/usr/bin/openclaw",
	}
}

// FindOpenclawBinary finds the openclaw binary, trying fallback paths if needed.
// Returns the path and true if found, or "" and false if not found.
func FindOpenclawBinary() (string, bool) {
	// First try PATH
	if path, err := exec.LookPath("openclaw"); err == nil {
		return path, true
	}

	// Try which
	if out, err := exec.Command("which", "openclaw").Output(); err == nil {
		path := strings.TrimSpace(string(out))
		if path != "" {
			if _, err := os.Stat(path); err == nil {
				return path, true
			}
		}
	}

	// Try fallback paths
	for _, path := range openclawFallbackPaths() {
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}

	return "", false
}

// checkOpenclawCLI checks whether the openclaw binary is accessible.
func checkOpenclawCLI() CheckResult {
	path, found := FindOpenclawBinary()
	if !found {
		return CheckResult{
			Name:   "openclaw-cli",
			Status: "fail",
			Detail: fmt.Sprintf("openclaw not found in PATH or fallback paths (%s)", strings.Join(openclawFallbackPaths(), ", ")),
		}
	}
	return CheckResult{
		Name:   "openclaw-cli",
		Status: "ok",
		Detail: fmt.Sprintf("found at %s", path),
	}
}
