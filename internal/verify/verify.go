package verify

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	DebugPort int // port for debug server; 0 = default (6060)
}

// RunStaticChecks executes only static checks that don't require a running server.
func RunStaticChecks(cfg VerifyConfig) []CheckResult {
	var checks []CheckResult

	// ── Layer -1: core checks (config, auth token, openclaw-cli, telegram domain)
	checks = append(checks, checkConfig(cfg.RootDir))
	checks = append(checks, checkAuth(cfg.RootDir))
	checks = append(checks, checkOpenclawCLI())
	checks = append(checks, checkTelegramDomain(cfg.RootDir))

	// ── Layer -1: panel + data checks
	checks = append(checks, checkPanels(cfg.Registry)...)
	checks = append(checks, checkPanelData(cfg.Apps)...)

	// ── Layer -1: app-registered health checks
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

	// ── Layer 3 (static): file_exists checks from verify.json
	checks = append(checks, checkAppVerifyFileExists(cfg.RootDir, cfg.Apps)...)

	return checks
}

// FetchRuntimeChecks calls the debug server's /debug/verify endpoint for in-process runtime checks.
// Returns the checks and true if successful, or nil and false if the debug server is unreachable.
func FetchRuntimeChecks(debugPort int) ([]CheckResult, bool) {
	if debugPort <= 0 {
		debugPort = 6060
	}

	url := fmt.Sprintf("http://localhost:%d/debug/verify", debugPort)
	log.Printf("[verify] Attempting runtime checks at %s", url)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("[verify] Debug server unreachable at %s: %v", url, err)
		return nil, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, false
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, false
	}

	var result struct {
		Status  string        `json:"status"`
		Checks  []CheckResult `json:"checks"`
		Passed  int           `json:"passed"`
		Failed  int           `json:"failed"`
		Skipped int           `json:"skipped"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, false
	}

	return result.Checks, true
}

// RunVerify executes all checks: static locally, runtime via debug server.
func RunVerify(cfg VerifyConfig) VerifyResult {
	var checks []CheckResult

	// Run static checks locally
	checks = append(checks, RunStaticChecks(cfg)...)

	// Determine debug port
	debugPort := cfg.DebugPort
	if debugPort <= 0 {
		debugPort = readDebugPort(cfg.RootDir)
	}

	// Try runtime checks via debug server
	runtimeChecks, ok := FetchRuntimeChecks(debugPort)
	if ok {
		checks = append(checks, runtimeChecks...)
	} else {
		// Debug server not reachable — add skipped markers for runtime checks
		checks = append(checks, CheckResult{
			Name:   "runtime",
			Status: "skipped",
			Detail: fmt.Sprintf("debug server not reachable on port %d — runtime checks skipped (server not running?)", debugPort),
			Hint:   "Start the vel server to run runtime checks (endpoint correctness, auth enforcement, app http_get checks)",
			Layer:  0,
		})
	}

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

// readDebugPort reads the debug port from config.json, defaulting to 6060.
func readDebugPort(rootDir string) int {
	configPath := filepath.Join(rootDir, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 6060
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return 6060
	}

	if debugRaw, ok := raw["debug"]; ok {
		var debugObj struct {
			DebugPort int `json:"debug_port"`
		}
		if json.Unmarshal(debugRaw, &debugObj) == nil && debugObj.DebugPort > 0 {
			return debugObj.DebugPort
		}
	}

	return 6060
}

// (Layers 0-2 runtime checks are now handled by the debug server's /debug/verify endpoint)

// ── Layer 3 (static) ─────────────────────────────────────────────────────────

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

// checkAppVerifyFileExists runs only the file_exists checks from verify.json (static, no server needed).
// http_get checks are now handled by the debug server's /debug/verify endpoint.
func checkAppVerifyFileExists(rootDir string, appList []*apps.App) []CheckResult {
	var results []CheckResult
	home, _ := os.UserHomeDir()

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
			if check.Type != "file_exists" {
				continue // http_get checks are runtime — handled by debug server
			}

			checkName := fmt.Sprintf("app:%s:check%d", appName, i+1)
			if check.Path != "" {
				short := check.Path
				if len(short) > 30 {
					short = "..." + short[len(short)-27:]
				}
				checkName = fmt.Sprintf("app:%s:%s", appName, short)
			}

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

	// User authorization: users.json (new auth) or allowedTelegramUsers/allowedUsers (legacy)
	usersPath := filepath.Join(rootDir, "users.json")
	hasUsers := false
	if _, err := os.Stat(usersPath); err == nil {
		hasUsers = true // new auth system with users.json
	}
	if !hasUsers {
		if authRaw, ok := raw["auth"]; ok {
			var authObj map[string]json.RawMessage
			if json.Unmarshal(authRaw, &authObj) == nil {
				if _, ok2 := authObj["allowedTelegramUsers"]; ok2 {
					hasUsers = true
				}
			}
		}
	}
	if !hasUsers {
		if _, ok := raw["allowedUsers"]; ok {
			hasUsers = true
		}
	}
	if !hasUsers {
		missing = append(missing, "users.json or allowedTelegramUsers")
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
// Also checks producer infrastructure (scripts, crontab entries) if declared.
func checkPanelData(appList []*apps.App) []CheckResult {
	var results []CheckResult
	home, _ := os.UserHomeDir()

	// Read crontab once for all checks
	crontabOut, _ := exec.Command("crontab", "-l").Output()
	crontab := string(crontabOut)

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

			// Check producer infrastructure if declared
			if ds.Producer != nil && ds.Producer.Script != "" {
				producerName := fmt.Sprintf("data:%s:%s:producer", app.Name, ds.Name)
				scriptPath := ds.Producer.Script
				// Resolve relative paths against app directory
				if !filepath.IsAbs(scriptPath) {
					scriptPath = filepath.Join(app.Dir, scriptPath)
				}
				// Expand ~ to home dir
				if strings.HasPrefix(scriptPath, "~/") {
					scriptPath = filepath.Join(home, scriptPath[2:])
				}

				if _, err := os.Stat(scriptPath); err != nil {
					results = append(results, CheckResult{
						Name:   producerName,
						Status: "fail",
						Detail: fmt.Sprintf("producer script not found: %s", scriptPath),
						Hint:   "This script generates the data file. Ensure it exists and is executable.",
					})
				} else {
					// Check if it's in crontab
					if crontab != "" && !strings.Contains(crontab, filepath.Base(scriptPath)) {
						results = append(results, CheckResult{
							Name:   producerName,
							Status: "warn",
							Detail: fmt.Sprintf("script exists but not found in crontab: %s", filepath.Base(scriptPath)),
							Hint:   "Data source may not be updating. Check crontab -l.",
						})
					} else {
						results = append(results, CheckResult{
							Name:   producerName,
							Status: "ok",
							Detail: fmt.Sprintf("script exists + crontab entry found"),
						})
					}
				}
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
