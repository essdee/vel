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
	Status string `json:"status"` // "ok" or "fail"
	Detail string `json:"detail,omitempty"`
}

// VerifyResult is the overall result of a health check run.
type VerifyResult struct {
	Status  string        `json:"status"` // "ok" or "fail"
	Checks  []CheckResult `json:"checks"`
	Passed  int           `json:"passed"`
	Failed  int           `json:"failed"`
}

// VerifyConfig holds runtime context for the verify run.
type VerifyConfig struct {
	RootDir   string
	Apps      []*apps.App
	Registry  *panels.Registry
	Workspace string
}

// RunVerify executes all built-in checks plus any app-registered checks.
func RunVerify(cfg VerifyConfig) VerifyResult {
	var checks []CheckResult

	// 1. Config check
	checks = append(checks, checkConfig(cfg.RootDir))

	// 2. Auth check (bot token)
	checks = append(checks, checkAuth(cfg.RootDir))

	// 3. Panels check
	checks = append(checks, checkPanels(cfg.Registry)...)

	// 4. Panel-data check (data sources exist)
	checks = append(checks, checkPanelData(cfg.Apps)...)

	// 5. openclaw-cli check
	checks = append(checks, checkOpenclawCLI())

	// 6. Telegram Login Widget domain check
	checks = append(checks, checkTelegramDomain(cfg.RootDir))

	// 7. App-registered checks
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
		})
	}

	passed, failed := 0, 0
	for _, c := range checks {
		if c.Status == "ok" {
			passed++
		} else {
			failed++
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
	}
}

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
