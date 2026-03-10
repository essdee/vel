package server

import (
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"vel/internal/apps"
	"vel/internal/auth"
	"vel/internal/data"
	"vel/internal/datasource"
	"vel/internal/hooks"
	"vel/internal/panels"
	"vel/internal/verify"
	vel "vel/pkg/vel"
)

// Ensure auth import is used (legacy functions still referenced in some paths).
var _ = auth.IsTestMode

type Config struct {
	RootDir      string
	Workspace    string
	ConfigPath   string
	Port         int
	Registry     *panels.Registry
	Order        []string
	Disabled     []string
	Version      string
	PublicConfig map[string]interface{} // safe fields for landing page
	Apps         []*apps.App
	Hooks        *hooks.Engine
	DSManager    *datasource.Manager
	AuthManager  *auth.AuthManager // new auth system (nil = legacy mode)
}

type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	max      int
	window   time.Duration
	skipOK   bool
}

func newRateLimiter(max int, window time.Duration, skipOK bool) *rateLimiter {
	return &rateLimiter{requests: make(map[string][]time.Time), max: max, window: window, skipOK: skipOK}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-rl.window)

	reqs := rl.requests[ip]
	var valid []time.Time
	for _, t := range reqs {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) >= rl.max {
		rl.requests[ip] = valid
		return false
	}
	rl.requests[ip] = append(valid, now)
	return true
}

type gzipResponseWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	return r.RemoteAddr
}

// healthCacher is a per-server health result cache (60s TTL).
type healthCacher struct {
	mu     sync.Mutex
	result json.RawMessage
	at     time.Time
}

func newHealthCacher() *healthCacher { return &healthCacher{} }

func (hc *healthCacher) get(cfg *Config) json.RawMessage {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if time.Since(hc.at) < 60*time.Second && hc.result != nil {
		return hc.result
	}

	vcfg := verify.VerifyConfig{
		RootDir:  cfg.RootDir,
		Apps:     cfg.Apps,
		Registry: cfg.Registry,
	}
	result := verify.RunVerify(vcfg)

	type healthResponse struct {
		Status    string               `json:"status"`
		Version   string               `json:"version"`
		Timestamp string               `json:"timestamp"`
		Checks    []verify.CheckResult `json:"checks"`
		Passed    int                  `json:"passed"`
		Failed    int                  `json:"failed"`
	}
	resp := healthResponse{
		Status:    result.Status,
		Version:   cfg.Version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    result.Checks,
		Passed:    result.Passed,
		Failed:    result.Failed,
	}
	data, _ := json.Marshal(resp)
	hc.result = data
	hc.at = time.Now()
	return hc.result
}

func NewServer(cfg *Config) http.Handler {
	// Initialize error logger before serving any requests.
	vel.InitErrorLog(filepath.Join(cfg.RootDir, "logs"))

	mux := http.NewServeMux()
	apiLimiter := newRateLimiter(1000, 15*time.Minute, false)
	authLimiter := newRateLimiter(10, 15*time.Minute, true)
	hcacher := newHealthCacher()

	// Register app server routes (from init() registrations)
	for _, reg := range vel.GetRegistrations() {
		appDir := ""
		for _, a := range cfg.Apps {
			if a.Name == reg.Name {
				appDir = a.Dir
				break
			}
		}
		reg.Register(mux, vel.AppConfig{
			Name:      reg.Name,
			Dir:       appDir,
			Workspace: cfg.Workspace,
		})
		fmt.Printf("[Server] Registered app routes: %s\n", reg.Name)
	}

	// Pages — check if any app provides a landing page
	landingFile := filepath.Join(cfg.RootDir, "core", "public", "landing.html")
	for _, app := range cfg.Apps {
		if app.LandingPage != "" {
			candidate := filepath.Join(app.Dir, app.LandingPage, "index.html")
			if _, err := os.Stat(candidate); err == nil {
				landingFile = candidate
				break
			}
		}
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			serve404(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		http.ServeFile(w, r, landingFile)
	})
	mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		// Check if any app has panels
		hasPanels := false
		for _, app := range cfg.Apps {
			if app.Panels != "" {
				hasPanels = true
				break
			}
		}
		if hasPanels {
			http.ServeFile(w, r, filepath.Join(cfg.RootDir, "core", "public", "shell.html"))
		} else {
			http.ServeFile(w, r, filepath.Join(cfg.RootDir, "core", "public", "welcome.html"))
		}
	})

	// Static files
	publicDir := filepath.Join(cfg.RootDir, "core", "public")
	mux.Handle("/public/", http.StripPrefix("/public/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Service worker must never be cached — browser needs to always check for updates
		if strings.HasSuffix(r.URL.Path, "sw.js") {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		http.FileServer(http.Dir(publicDir)).ServeHTTP(w, r)
	})))

	vendorDir := filepath.Join(cfg.RootDir, "core", "vendor")
	mux.Handle("/core/vendor/", http.StripPrefix("/core/vendor/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		http.FileServer(http.Dir(vendorDir)).ServeHTTP(w, r)
	})))

	// App-driven routes (static dirs and single pages)
	for _, app := range cfg.Apps {
		for urlPrefix, route := range app.Routes {
			absDir := filepath.Join(app.Dir, route.Dir)
			switch route.Type {
			case "static":
				if _, err := os.Stat(absDir); err == nil {
					staticDir := absDir    // capture for closure
					urlPfx := urlPrefix    // capture for closure
					cacheMode := route.Cache // capture for closure
					mux.Handle(urlPfx, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						switch cacheMode {
						case "none":
							w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
							w.Header().Set("Pragma", "no-cache")
						case "aggressive":
							w.Header().Set("Cache-Control", "public, max-age=86400")
						default:
							w.Header().Set("Cache-Control", "public, max-age=3600")
						}
						fs := http.StripPrefix(urlPfx, http.FileServer(http.Dir(staticDir)))
						fs.ServeHTTP(w, r)
					}))
				}
			case "proxy":
				if route.Target != "" {
					target := route.Target
					prefix := urlPrefix
					mux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
						proxyURL := target + strings.TrimPrefix(r.URL.Path, strings.TrimSuffix(prefix, "/"))
						if r.URL.RawQuery != "" {
							proxyURL += "?" + r.URL.RawQuery
						}
						proxyReq, err := http.NewRequest(r.Method, proxyURL, r.Body)
						if err != nil {
							http.Error(w, "Bad gateway", 502)
							return
						}
						for k, vv := range r.Header {
							for _, v := range vv {
								proxyReq.Header.Add(k, v)
							}
						}
						resp, err := http.DefaultClient.Do(proxyReq)
						if err != nil {
							http.Error(w, "Bad gateway", 502)
							return
						}
						defer resp.Body.Close()
						for k, vv := range resp.Header {
							for _, v := range vv {
								w.Header().Add(k, v)
							}
						}
						w.WriteHeader(resp.StatusCode)
						buf := make([]byte, 32*1024)
						for {
							n, readErr := resp.Body.Read(buf)
							if n > 0 {
								w.Write(buf[:n])
							}
							if readErr != nil {
								break
							}
						}
					})
					fmt.Printf("[Server] Proxy route: %s → %s\n", prefix, target)
				}
			case "page":
				indexFile := filepath.Join(absDir, "index.html")
				if _, err := os.Stat(indexFile); err == nil {
					file := indexFile   // capture for closure
					cacheMode := route.Cache // capture for closure
					mux.HandleFunc(urlPrefix, func(w http.ResponseWriter, r *http.Request) {
						switch cacheMode {
						case "aggressive":
							w.Header().Set("Cache-Control", "public, max-age=86400")
						case "none":
							w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
							w.Header().Set("Pragma", "no-cache")
						default:
							w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
							w.Header().Set("Pragma", "no-cache")
						}
						http.ServeFile(w, r, file)
					})
				}
			}
		}
	}

	// Theme support
	themeFile := filepath.Join(cfg.RootDir, "custom", "theme", "theme.css")
	if _, err := os.Stat(themeFile); err == nil {
		mux.HandleFunc("/custom/theme/theme.css", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, themeFile)
		})
	}

	// Panel routes - handle both /api/panels and /api/panels/
	panelsHandler := func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/panels")
		path = strings.TrimPrefix(path, "/")

		// GET /api/panels - list
		if path == "" {
			panelList := panels.BuildPanelList(cfg.Registry, cfg.Order, cfg.Disabled, auth.IsTestMode())
			writeJSON(w, panelList)
			return
		}

		// GET /api/panels/{id}/ui.js
		if strings.HasSuffix(path, "/ui.js") {
			panelID := strings.TrimSuffix(path, "/ui.js")
			info := cfg.Registry.Get(panelID)
			if info == nil {
				http.Error(w, "Panel not found", 404)
				return
			}
			uiPath := filepath.Join(info.Path, "ui.js")
			if _, err := os.Stat(uiPath); os.IsNotExist(err) {
				http.Error(w, "No UI for panel", 404)
				return
			}
			w.Header().Set("Content-Type", "application/javascript")
			w.Header().Set("Cache-Control", "no-cache, must-revalidate")
			http.ServeFile(w, r, uiPath)
			return
		}

		// GET /api/panels/{id} - panel data
		panelID := strings.TrimSuffix(path, "/")
		servesPanelData(w, r, panelID, cfg)
	}
	mux.HandleFunc("/api/panels", panelsHandler)
	mux.HandleFunc("/api/panels/", panelsHandler)

	// API routes
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if !apiLimiter.allow(getClientIP(r)) {
			http.Error(w, "Too many requests", 429)
			return
		}
		// Return public config (no auth secrets)
		conf := make(map[string]interface{})
		for k, v := range cfg.PublicConfig {
			conf[k] = v
		}
		conf["panels"] = map[string]interface{}{
			"order":    cfg.Order,
			"disabled": cfg.Disabled,
		}
		writeJSON(w, conf)
	})

	mux.HandleFunc("/api/auth", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			// Telegram initData authentication
			var body struct {
				InitData string `json:"initData"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			if body.InitData == "" {
				w.WriteHeader(401)
				writeJSON(w, map[string]interface{}{"ok": false, "error": "initData required"})
				return
			}

			// Use the Telegram provider to authenticate
			if cfg.AuthManager != nil {
				tgProvider := cfg.AuthManager.GetProvider("telegram")
				if tgProvider == nil {
					w.WriteHeader(500)
					writeJSON(w, map[string]interface{}{"ok": false, "error": "telegram provider not configured"})
					return
				}
				creds := auth.TelegramCredentials{InitData: body.InitData}
				identity, err := tgProvider.Authenticate(creds)
				if err != nil {
					w.WriteHeader(401)
					writeJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
					return
				}
				// Create session
				sess, err := cfg.AuthManager.CreateSession(identity)
				if err != nil {
					w.WriteHeader(500)
					writeJSON(w, map[string]interface{}{"ok": false, "error": "session creation failed"})
					return
				}
				setSessionCookie(w, cfg.AuthManager, sess.ID)
				writeJSON(w, map[string]interface{}{
					"ok": true,
					"user": map[string]interface{}{
						"id":   identity.UserID,
						"name": identity.Name,
						"role": identity.Role,
					},
				})
				return
			}

			// Legacy fallback
			user := auth.ValidateInitData(body.InitData)
			if user == nil || !auth.IsAllowed(user.ID) {
				w.WriteHeader(401)
				writeJSON(w, map[string]interface{}{"ok": false})
				return
			}
			userInfo, _ := json.Marshal(map[string]interface{}{
				"id":         user.ID,
				"first_name": user.FirstName,
				"username":   user.Username,
			})
			signed := auth.SignCookie(string(userInfo))
			http.SetCookie(w, &http.Cookie{
				Name:     "tg_user",
				Value:    signed,
				HttpOnly: true,
				Secure:   true,
				SameSite: http.SameSiteNoneMode,
				MaxAge:   7 * 24 * 60 * 60,
				Path:     "/",
			})
			writeJSON(w, map[string]interface{}{"ok": true, "user": map[string]interface{}{"id": user.ID, "first_name": user.FirstName}})
			return
		}

		// GET — check auth status
		resp := map[string]interface{}{}
		if cfg.AuthManager != nil {
			identity := GetIdentity(r)
			if identity == nil {
				w.WriteHeader(401)
				resp["ok"] = false
				writeJSON(w, resp)
				return
			}
			resp["ok"] = true
			resp["user"] = map[string]interface{}{
				"id":   identity.UserID,
				"name": identity.Name,
				"role": identity.Role,
			}
		} else {
			resp["authMode"] = auth.GetAuthMode()
			user := auth.Check(r)
			if user == nil {
				w.WriteHeader(401)
				resp["ok"] = false
				writeJSON(w, resp)
				return
			}
			resp["ok"] = true
			resp["user"] = user
		}
		writeJSON(w, resp)
	})

	// Scoped tokens API
	mux.HandleFunc("/api/tokens", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuthNotScoped(r, cfg) {
			w.WriteHeader(403)
			writeJSON(w, map[string]interface{}{"error": "Unauthorized"})
			return
		}
		switch r.Method {
		case "GET":
			tokens := auth.GetScopedTokens()
			writeJSON(w, map[string]interface{}{"tokens": tokens})
		case "POST":
			var body struct {
				Name   string   `json:"name"`
				Scopes []string `json:"scopes"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(400)
				writeJSON(w, map[string]interface{}{"error": "Bad request"})
				return
			}
			if body.Name == "" || len(body.Scopes) == 0 {
				w.WriteHeader(400)
				writeJSON(w, map[string]interface{}{"error": "name and scopes required"})
				return
			}
			token, err := auth.AddScopedToken(body.Name, body.Scopes)
			if err != nil {
				w.WriteHeader(409)
				writeJSON(w, map[string]interface{}{"error": err.Error()})
				return
			}
			// Write back to config file
			if writeErr := writeScopedTokensToConfig(cfg.ConfigPath); writeErr != nil {
				fmt.Printf("[Auth] Warning: failed to persist scoped tokens: %v\n", writeErr)
			}
			writeJSON(w, map[string]interface{}{"ok": true, "name": body.Name, "token": token})
		case "DELETE":
			name := r.URL.Query().Get("name")
			if name == "" {
				var body struct {
					Name string `json:"name"`
				}
				json.NewDecoder(r.Body).Decode(&body)
				name = body.Name
			}
			if name == "" {
				w.WriteHeader(400)
				writeJSON(w, map[string]interface{}{"error": "name required"})
				return
			}
			if !auth.RemoveScopedToken(name) {
				w.WriteHeader(404)
				writeJSON(w, map[string]interface{}{"error": "token not found"})
				return
			}
			if writeErr := writeScopedTokensToConfig(cfg.ConfigPath); writeErr != nil {
				fmt.Printf("[Auth] Warning: failed to persist scoped tokens: %v\n", writeErr)
			}
			writeJSON(w, map[string]interface{}{"ok": true})
		default:
			http.Error(w, "Method not allowed", 405)
		}
	})

	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"version": cfg.Version,
			"source":  "github:karthikeyan5/vel",
			"repo":    "https://github.com/karthikeyan5/vel",
		})
	})

	mux.HandleFunc("/api/mode", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"testMode": auth.IsTestMode()})
	})

	// /api/health — no auth required; results cached 60s
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		result := hcacher.get(cfg)
		w.Header().Set("Content-Type", "application/json")
		w.Write(result)
	})

	mux.HandleFunc("/api/usage/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if !checkAuth(r, cfg) {
			w.WriteHeader(403)
			writeJSON(w, map[string]interface{}{"error": "Unauthorized"})
			return
		}
		scriptPath := filepath.Join(cfg.RootDir, "sdk", "openclaw", "claude-usage-poll.sh")
		// Fallback to legacy location
		if _, err := os.Stat(scriptPath); err != nil {
			scriptPath = filepath.Join(cfg.Workspace, "skills", "claude-usage-monitor", "scripts", "claude-usage-poll.sh")
		}
		cmd := exec.Command("bash", scriptPath)
		cmd.Env = append(os.Environ(), "HOME="+os.Getenv("HOME"))
		if err := cmd.Run(); err != nil {
			w.WriteHeader(500)
			writeJSON(w, map[string]interface{}{"error": "Refresh failed"})
			return
		}
		usage := data.GetUsageData(cfg.Workspace)
		if usage == nil {
			writeJSON(w, map[string]interface{}{"error": "No data after refresh"})
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(usage)
		}
	})

	mux.HandleFunc("/api/crons/action", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if !checkAuth(r, cfg) {
			w.WriteHeader(403)
			writeJSON(w, map[string]interface{}{"error": "Unauthorized"})
			return
		}
		var body struct {
			JobID  string `json:"jobId"`
			Action string `json:"action"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.JobID == "" || body.Action == "" {
			w.WriteHeader(400)
			writeJSON(w, map[string]interface{}{"error": "Missing jobId or action"})
			return
		}
		if body.Action != "run" && body.Action != "enable" && body.Action != "disable" {
			w.WriteHeader(400)
			writeJSON(w, map[string]interface{}{"error": "Invalid action"})
			return
		}

		var cmd *exec.Cmd
		switch body.Action {
		case "run":
			cmd = exec.Command("openclaw", "cron", "run", body.JobID)
		case "enable":
			cmd = exec.Command("openclaw", "cron", "update", body.JobID, "--enabled", "true")
		case "disable":
			cmd = exec.Command("openclaw", "cron", "update", body.JobID, "--enabled", "false")
		}
		if err := cmd.Run(); err != nil {
			w.WriteHeader(500)
			writeJSON(w, map[string]interface{}{"error": "Action failed"})
			return
		}
		writeJSON(w, map[string]interface{}{"ok": true, "action": body.Action, "jobId": body.JobID})
	})

	// Auth routes
	mux.HandleFunc("/auth/telegram/callback", func(w http.ResponseWriter, r *http.Request) {
		if !authLimiter.allow(getClientIP(r)) {
			http.Error(w, "Too many requests", 429)
			return
		}
		params := make(map[string]string)
		for k, v := range r.URL.Query() {
			if len(v) > 0 {
				params[k] = v[0]
			}
		}
		if params["hash"] == "" || params["id"] == "" {
			http.Error(w, "Invalid login data", 400)
			return
		}
		if !auth.ValidateTelegramLogin(params) {
			http.Error(w, "Authentication failed", 401)
			return
		}
		authDate, _ := strconv.ParseInt(params["auth_date"], 10, 64)
		if time.Now().Unix()-authDate > 86400 {
			http.Error(w, "Login expired", 401)
			return
		}
		userID, _ := strconv.ParseInt(params["id"], 10, 64)

		redirect := "/dashboard"

		if cfg.AuthManager != nil {
			// New auth: look up user and create session
			providerID := strconv.FormatInt(userID, 10)
			record := cfg.AuthManager.UserStore().FindUserByIdentity("telegram", providerID)
			if record == nil {
				http.Error(w, "Access denied", 403)
				return
			}
			identity := &auth.Identity{
				UserID:   record.ID,
				Name:     record.Name,
				Provider: "telegram",
				Role:     record.Role,
				Meta: map[string]string{
					"telegram_id":       providerID,
					"telegram_name":     params["first_name"],
					"telegram_username": params["username"],
				},
			}
			sess, err := cfg.AuthManager.CreateSession(identity)
			if err != nil {
				http.Error(w, "Session creation failed", 500)
				return
			}
			setSessionCookie(w, cfg.AuthManager, sess.ID)
		} else {
			// Legacy
			if !auth.IsAllowed(userID) {
				http.Error(w, "Access denied", 403)
				return
			}
			userInfo, _ := json.Marshal(map[string]interface{}{
				"id":         userID,
				"first_name": params["first_name"],
				"username":   params["username"],
			})
			signed := auth.SignCookie(string(userInfo))
			http.SetCookie(w, &http.Cookie{
				Name:     "tg_user",
				Value:    signed,
				HttpOnly: true,
				Secure:   true,
				SameSite: http.SameSiteNoneMode,
				MaxAge:   7 * 24 * 60 * 60,
				Path:     "/",
			})
		}
		http.Redirect(w, r, redirect, http.StatusFound)
	})

	// Token auth login
	mux.HandleFunc("/auth/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if !authLimiter.allow(getClientIP(r)) {
			http.Error(w, "Too many requests", 429)
			return
		}
		var body struct {
			Token string `json:"token"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if !auth.ValidateToken(body.Token) {
			w.WriteHeader(401)
			writeJSON(w, map[string]interface{}{"ok": false, "error": "Invalid token"})
			return
		}
		userInfo, _ := json.Marshal(map[string]interface{}{
			"id":         1,
			"first_name": "Admin",
			"username":   "admin",
		})
		signed := auth.SignCookie(string(userInfo))
		http.SetCookie(w, &http.Cookie{
			Name:     "tg_user",
			Value:    signed,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteNoneMode,
			MaxAge:   7 * 24 * 60 * 60,
			Path:     "/",
		})
		writeJSON(w, map[string]interface{}{"ok": true})
	})

	// Dev auto-login (TEST_MODE only)
	mux.HandleFunc("/auth/dev", func(w http.ResponseWriter, r *http.Request) {
		if !auth.IsTestMode() {
			http.Error(w, "Not available", 404)
			return
		}
		userInfo, _ := json.Marshal(map[string]interface{}{
			"id":         0,
			"first_name": "Developer",
			"username":   "dev",
		})
		signed := auth.SignCookie(string(userInfo))
		http.SetCookie(w, &http.Cookie{
			Name:     "tg_user",
			Value:    signed,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteNoneMode,
			MaxAge:   7 * 24 * 60 * 60,
			Path:     "/",
		})
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})

	// Magic link validation endpoint — PUBLIC
	mux.HandleFunc("/auth/magic", func(w http.ResponseWriter, r *http.Request) {
		// Prevent proxy/browser caching of auth responses
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")

		// Block bot/crawler prefetch (Telegram, WhatsApp, Slack, etc.)
		// These fetch URLs for link previews and would consume single-use tokens.
		ua := strings.ToLower(r.UserAgent())
		if strings.Contains(ua, "telegrambot") || strings.Contains(ua, "whatsapp") ||
			strings.Contains(ua, "slackbot") || strings.Contains(ua, "discordbot") ||
			strings.Contains(ua, "facebookexternalhit") || strings.Contains(ua, "twitterbot") ||
			strings.Contains(ua, "linkedinbot") || strings.Contains(ua, "bot") && strings.Contains(ua, "http") {
			w.WriteHeader(204)
			return
		}

		if !authLimiter.allow(getClientIP(r)) {
			http.Error(w, "Too many requests", 429)
			return
		}

		token := r.URL.Query().Get("ml_token")
		if token == "" || cfg.AuthManager == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(400)
			fmt.Fprint(w, magicLinkErrorHTML("Invalid or expired link", "The login link is missing or malformed."))
			return
		}

		mlProvider := cfg.AuthManager.GetProvider("magic_link")
		if mlProvider == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(500)
			fmt.Fprint(w, magicLinkErrorHTML("Login unavailable", "Magic link login is not configured."))
			return
		}

		creds := auth.MagicLinkCredentials{Token: token}
		identity, err := mlProvider.Authenticate(creds)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(401)
			fmt.Fprint(w, magicLinkErrorHTML("Invalid or expired link", "This login link has already been used or has expired. Please request a new one."))
			return
		}

		// Create session
		sess, err := cfg.AuthManager.CreateSession(identity)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(500)
			fmt.Fprint(w, magicLinkErrorHTML("Login failed", "Session creation failed. Please try again."))
			return
		}
		setSessionCookie(w, cfg.AuthManager, sess.ID)

		// Redirect
		redirect := r.URL.Query().Get("redirect")
		if redirect == "" {
			redirect = "/dashboard"
		}
		http.Redirect(w, r, redirect, http.StatusFound)
	})

	// Admin endpoint to generate magic links — RequireAdmin
	mux.HandleFunc("/api/auth/magic-link", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}

		// Check path exactly (don't match /api/auth/magic-link/request)
		if r.URL.Path != "/api/auth/magic-link" {
			http.NotFound(w, r)
			return
		}

		// Require admin
		id := GetIdentity(r)
		if id == nil || id.Role != "admin" {
			w.WriteHeader(403)
			writeJSON(w, map[string]interface{}{"error": "Forbidden"})
			return
		}

		if cfg.AuthManager == nil {
			w.WriteHeader(500)
			writeJSON(w, map[string]interface{}{"ok": false, "error": "auth not configured"})
			return
		}

		var body struct {
			UserID        string `json:"user_id"`
			ExpiresMinutes int   `json:"expires_minutes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(400)
			writeJSON(w, map[string]interface{}{"ok": false, "error": "invalid request body"})
			return
		}
		if body.UserID == "" {
			w.WriteHeader(400)
			writeJSON(w, map[string]interface{}{"ok": false, "error": "user_id required"})
			return
		}
		if body.ExpiresMinutes <= 0 {
			body.ExpiresMinutes = 15
		}

		// Verify user exists
		user := cfg.AuthManager.UserStore().FindUserByID(body.UserID)
		if user == nil {
			w.WriteHeader(404)
			writeJSON(w, map[string]interface{}{"ok": false, "error": "user not found"})
			return
		}

		mlStore := cfg.AuthManager.MagicLinkStore()
		if mlStore == nil {
			w.WriteHeader(500)
			writeJSON(w, map[string]interface{}{"ok": false, "error": "magic link store not configured"})
			return
		}

		token, err := mlStore.Create(body.UserID, body.ExpiresMinutes)
		if err != nil {
			w.WriteHeader(429)
			writeJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}

		// Build URL using authUrl domain or fallback
		domain := getDomain(cfg)
		url := fmt.Sprintf("https://%s/auth/magic?ml_token=%s", domain, token)

		writeJSON(w, map[string]interface{}{"ok": true, "url": url})
	})

	// Public endpoint to request magic link via email — no auth
	mux.HandleFunc("/api/auth/magic-link/request", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}

		// Always return same response to prevent email enumeration
		safeResponse := map[string]interface{}{
			"ok":      true,
			"message": "If that email is registered, a login link was sent.",
		}

		if cfg.AuthManager == nil {
			writeJSON(w, map[string]interface{}{"ok": false, "error": "Email login not available"})
			return
		}

		mlStore := cfg.AuthManager.MagicLinkStore()
		if mlStore == nil {
			writeJSON(w, map[string]interface{}{"ok": false, "error": "Email login not available"})
			return
		}

		// Check if email sending is configured
		mlCfg := cfg.AuthManager.MagicLinkConfig()
		if mlCfg == nil || !mlCfg.EmailEnabled {
			writeJSON(w, map[string]interface{}{"ok": false, "error": "Email login not available"})
			return
		}

		if !auth.IsHimalayaAvailable() {
			writeJSON(w, map[string]interface{}{"ok": false, "error": "Email login not available"})
			return
		}

		var body struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
			writeJSON(w, safeResponse)
			return
		}

		// Rate limit by IP
		if !authLimiter.allow(getClientIP(r)) {
			writeJSON(w, safeResponse)
			return
		}

		// Look up user by email
		user := cfg.AuthManager.UserStore().FindUserByEmail(body.Email)
		if user == nil {
			// Don't reveal that the email doesn't exist
			writeJSON(w, safeResponse)
			return
		}

		// Generate magic link
		token, err := mlStore.Create(user.ID, mlCfg.ExpiryMinutes)
		if err != nil {
			// Rate limit or other error — still return safe response
			writeJSON(w, safeResponse)
			return
		}

		domain := getDomain(cfg)
		magicURL := fmt.Sprintf("https://%s/auth/magic?ml_token=%s", domain, token)

		// Send email
		if err := auth.SendMagicLinkEmail(body.Email, mlCfg.EmailFrom, magicURL, domain); err != nil {
			fmt.Printf("[Auth] Failed to send magic link email to %s: %v\n", body.Email, err)
		}

		writeJSON(w, safeResponse)
	})

	// ── Admin Auth API: Users CRUD ──
	mux.HandleFunc("/api/auth/users", func(w http.ResponseWriter, r *http.Request) {
		if cfg.AuthManager == nil {
			w.WriteHeader(500)
			writeJSON(w, map[string]interface{}{"error": "auth not configured"})
			return
		}

		// Require admin
		id := GetIdentity(r)
		if id == nil || id.Role != "admin" {
			w.WriteHeader(403)
			writeJSON(w, map[string]interface{}{"error": "Forbidden"})
			return
		}

		switch r.Method {
		case "GET":
			users := cfg.AuthManager.UserStore().GetAllUsers()
			if users == nil {
				users = []auth.UserRecord{}
			}
			writeJSON(w, map[string]interface{}{"users": users})

		case "POST":
			var body struct {
				ID         string              `json:"id"`
				Name       string              `json:"name"`
				Email      string              `json:"email"`
				Role       string              `json:"role"`
				Identities []auth.UserIdentity `json:"identities"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(400)
				writeJSON(w, map[string]interface{}{"ok": false, "error": "invalid request body"})
				return
			}
			if body.ID == "" || body.Name == "" {
				w.WriteHeader(400)
				writeJSON(w, map[string]interface{}{"ok": false, "error": "id and name are required"})
				return
			}
			if body.Role == "" {
				body.Role = "user"
			}
			user := auth.UserRecord{
				ID:         body.ID,
				Name:       body.Name,
				Email:      body.Email,
				Role:       body.Role,
				Identities: body.Identities,
			}
			if err := cfg.AuthManager.UserStore().AddUser(user); err != nil {
				w.WriteHeader(409)
				writeJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
				return
			}
			writeJSON(w, map[string]interface{}{"ok": true, "user": user})

		case "DELETE":
			userID := r.URL.Query().Get("id")
			if userID == "" {
				w.WriteHeader(400)
				writeJSON(w, map[string]interface{}{"ok": false, "error": "id query parameter required"})
				return
			}
			found, err := cfg.AuthManager.UserStore().RemoveUser(userID)
			if err != nil {
				w.WriteHeader(500)
				writeJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
				return
			}
			if !found {
				w.WriteHeader(404)
				writeJSON(w, map[string]interface{}{"ok": false, "error": "user not found"})
				return
			}
			writeJSON(w, map[string]interface{}{"ok": true})

		default:
			http.Error(w, "Method not allowed", 405)
		}
	})

	// ── Admin Auth API: API Keys CRUD ──
	mux.HandleFunc("/api/auth/keys", func(w http.ResponseWriter, r *http.Request) {
		if cfg.AuthManager == nil {
			w.WriteHeader(500)
			writeJSON(w, map[string]interface{}{"error": "auth not configured"})
			return
		}

		// Require admin
		id := GetIdentity(r)
		if id == nil || id.Role != "admin" {
			w.WriteHeader(403)
			writeJSON(w, map[string]interface{}{"error": "Forbidden"})
			return
		}

		switch r.Method {
		case "GET":
			keys := cfg.AuthManager.UserStore().GetAllAPIKeys()
			// Strip key_hash from response — never expose hashes
			type safeKey struct {
				ID        string   `json:"id"`
				Name      string   `json:"name"`
				Role      string   `json:"role"`
				Scopes    []string `json:"scopes,omitempty"`
				CreatedBy string   `json:"created_by,omitempty"`
				CreatedAt string   `json:"created_at,omitempty"`
			}
			safeKeys := make([]safeKey, 0, len(keys))
			for _, k := range keys {
				scopes := k.Scopes
				if scopes == nil {
					scopes = []string{}
				}
				safeKeys = append(safeKeys, safeKey{
					ID:        k.ID,
					Name:      k.Name,
					Role:      k.Role,
					Scopes:    scopes,
					CreatedBy: k.CreatedBy,
					CreatedAt: k.CreatedAt,
				})
			}
			writeJSON(w, map[string]interface{}{"keys": safeKeys})

		case "POST":
			var body struct {
				Name   string   `json:"name"`
				Role   string   `json:"role"`
				Scopes []string `json:"scopes"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(400)
				writeJSON(w, map[string]interface{}{"ok": false, "error": "invalid request body"})
				return
			}
			if body.Name == "" {
				w.WriteHeader(400)
				writeJSON(w, map[string]interface{}{"ok": false, "error": "name is required"})
				return
			}
			if body.Role == "" {
				body.Role = "viewer"
			}

			// Generate API key
			keyBytes := make([]byte, 32)
			if _, err := rand.Read(keyBytes); err != nil {
				w.WriteHeader(500)
				writeJSON(w, map[string]interface{}{"ok": false, "error": "key generation failed"})
				return
			}
			plainKey := "vel_ak_live_" + hex.EncodeToString(keyBytes)

			// Hash
			keyHash := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(plainKey)))

			apiKey := auth.APIKey{
				ID:        body.Name,
				Name:      body.Name,
				KeyHash:   keyHash,
				Role:      body.Role,
				Scopes:    body.Scopes,
				CreatedBy: id.UserID,
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
			}

			if err := cfg.AuthManager.UserStore().AddAPIKey(apiKey); err != nil {
				w.WriteHeader(409)
				writeJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
				return
			}

			// Return plaintext key ONCE
			writeJSON(w, map[string]interface{}{"ok": true, "key": plainKey, "id": body.Name})

		case "DELETE":
			keyID := r.URL.Query().Get("id")
			if keyID == "" {
				w.WriteHeader(400)
				writeJSON(w, map[string]interface{}{"ok": false, "error": "id query parameter required"})
				return
			}
			found, err := cfg.AuthManager.UserStore().RemoveAPIKey(keyID)
			if err != nil {
				w.WriteHeader(500)
				writeJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
				return
			}
			if !found {
				w.WriteHeader(404)
				writeJSON(w, map[string]interface{}{"ok": false, "error": "API key not found"})
				return
			}
			writeJSON(w, map[string]interface{}{"ok": true})

		default:
			http.Error(w, "Method not allowed", 405)
		}
	})

	mux.HandleFunc("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if cfg.AuthManager != nil {
			// Destroy session
			sess, err := cfg.AuthManager.GetSession(r)
			if err == nil && sess != nil {
				cfg.AuthManager.DestroySession(sess.ID)
			}
			clearSessionCookie(w, cfg.AuthManager)
		}
		// Also clear legacy cookie for backward compat
		http.SetCookie(w, &http.Cookie{
			Name:   "tg_user",
			Value:  "",
			MaxAge: -1,
			Path:   "/",
		})
		http.Redirect(w, r, "/", http.StatusFound)
	})

	// Data sources API
	mux.HandleFunc("/api/sources", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(r, cfg) {
			w.WriteHeader(403)
			writeJSON(w, map[string]interface{}{"error": "Unauthorized"})
			return
		}
		if cfg.DSManager == nil {
			writeJSON(w, map[string]interface{}{})
			return
		}
		writeJSON(w, cfg.DSManager.GetAllData())
	})

	mux.HandleFunc("/api/source/", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(r, cfg) {
			w.WriteHeader(403)
			writeJSON(w, map[string]interface{}{"error": "Unauthorized"})
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/source/")
		if name == "" || cfg.DSManager == nil {
			http.Error(w, "Source not found", 404)
			return
		}
		state := cfg.DSManager.GetSourceState(name)
		if state == nil {
			http.Error(w, "Source not found", 404)
			return
		}
		writeJSON(w, state)
	})

	// Updates API
	mux.HandleFunc("/api/updates/check", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if !checkAuth(r, cfg) {
			w.WriteHeader(403)
			writeJSON(w, map[string]interface{}{"error": "Unauthorized"})
			return
		}
		// Force a fresh check
		data.InvalidateUpdatesCache()
		result := data.GetUpdatesStatus(cfg.RootDir)
		w.Header().Set("Content-Type", "application/json")
		w.Write(result)
	})

	mux.HandleFunc("/api/updates/apply", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if !checkAuth(r, cfg) {
			w.WriteHeader(403)
			writeJSON(w, map[string]interface{}{"error": "Unauthorized"})
			return
		}
		deployScript := filepath.Join(cfg.RootDir, "sdk", "vel", "deploy.sh")
		// Fallback to legacy root location
		if _, err := os.Stat(deployScript); err != nil {
			deployScript = filepath.Join(cfg.RootDir, "deploy.sh")
		}
		if _, err := os.Stat(deployScript); err != nil {
			w.WriteHeader(500)
			writeJSON(w, map[string]interface{}{"error": "deploy.sh not found"})
			return
		}
		// Invalidate cache so next check reflects post-deploy state
		data.InvalidateUpdatesCache()
		// Run deploy in background — script restarts the service so this process will die
		cmd := exec.Command("bash", deployScript)
		cmd.Env = append(os.Environ(), "HOME="+os.Getenv("HOME"))
		if err := cmd.Start(); err != nil {
			w.WriteHeader(500)
			writeJSON(w, map[string]interface{}{"error": "Failed to start deploy: " + err.Error()})
			return
		}
		// Detach — deploy.sh will restart the service
		go cmd.Wait()
		writeJSON(w, map[string]interface{}{"ok": true, "message": "Deploy started. Dashboard will restart shortly."})
	})

	// Gateway restart (SDK)
	var lastGatewayRestart time.Time
	var gatewayRestartMu sync.Mutex
	mux.HandleFunc("/api/gateway/restart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if !checkAuthNotScoped(r, cfg) {
			w.WriteHeader(403)
			writeJSON(w, map[string]interface{}{"error": "Unauthorized"})
			return
		}

		// Rate limit: 1 restart per 60 seconds
		gatewayRestartMu.Lock()
		if time.Since(lastGatewayRestart) < 60*time.Second {
			gatewayRestartMu.Unlock()
			w.WriteHeader(429)
			writeJSON(w, map[string]interface{}{"error": "Please wait 60 seconds between restarts"})
			return
		}
		lastGatewayRestart = time.Now()
		gatewayRestartMu.Unlock()

		scriptPath := filepath.Join(cfg.RootDir, "sdk", "openclaw", "restart.sh")
		if _, err := os.Stat(scriptPath); err != nil {
			w.WriteHeader(500)
			writeJSON(w, map[string]interface{}{"error": "restart.sh not found in sdk/openclaw/"})
			return
		}

		cmd := exec.Command("bash", scriptPath)
		cmd.Env = append(os.Environ(), "HOME="+os.Getenv("HOME"))
		output, err := cmd.CombinedOutput()

		result := map[string]interface{}{
			"output": string(output),
		}
		if err != nil {
			result["ok"] = false
			result["error"] = err.Error()
		} else {
			result["ok"] = true
		}
		writeJSON(w, result)
	})

	// Verify status API — reads logs/verify.jsonl and returns latest + history (auth-protected).
	mux.HandleFunc("/api/verify-status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if !checkAuthNotScoped(r, cfg) {
			w.WriteHeader(403)
			writeJSON(w, map[string]interface{}{"error": "Unauthorized"})
			return
		}

		logPath := filepath.Join(cfg.RootDir, "logs", "verify.jsonl")
		fileData, err := os.ReadFile(logPath)
		if err != nil {
			// File doesn't exist yet — return empty history
			writeJSON(w, map[string]interface{}{
				"latest":  nil,
				"history": []interface{}{},
				"healthy": true,
			})
			return
		}

		// Parse JSONL lines
		rawLines := strings.Split(strings.TrimSpace(string(fileData)), "\n")
		var history []json.RawMessage
		for _, line := range rawLines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			history = append(history, json.RawMessage(line))
		}

		// Keep last 20
		if len(history) > 20 {
			history = history[len(history)-20:]
		}

		var latest json.RawMessage
		healthy := true
		if len(history) > 0 {
			latest = history[len(history)-1]
			var latestObj map[string]interface{}
			if json.Unmarshal(latest, &latestObj) == nil {
				if status, ok := latestObj["status"].(string); ok && status != "ok" {
					healthy = false
				}
			}
		}

		writeJSON(w, map[string]interface{}{
			"latest":  latest,
			"history": history,
			"healthy": healthy,
		})
	})

	// Error log API — returns recent error log entries (auth-protected).
	mux.HandleFunc("/api/errors", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if !checkAuthNotScoped(r, cfg) {
			w.WriteHeader(403)
			writeJSON(w, map[string]interface{}{"error": "Unauthorized"})
			return
		}
		limit := 100
		if lStr := r.URL.Query().Get("limit"); lStr != "" {
			if n, err := strconv.Atoi(lStr); err == nil && n > 0 {
				if n > 1000 {
					n = 1000
				}
				limit = n
			}
		}
		entries := vel.GetRecentErrors(limit)
		if entries == nil {
			entries = []vel.ErrorEntry{}
		}
		writeJSON(w, map[string]interface{}{
			"errors": entries,
			"total":  len(entries),
		})
	})

	// Login page
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		http.ServeFile(w, r, filepath.Join(cfg.RootDir, "core", "public", "login.html"))
	})
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		http.ServeFile(w, r, filepath.Join(cfg.RootDir, "core", "public", "login.html"))
	})

	// WebSocket
	mux.HandleFunc("/ws/metrics", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(w, r, cfg)
	})

	// Pre-warm slow caches in background so first client doesn't wait
	go func() {
		data.GetSystemStatus()
		fmt.Println("[Server] Pre-warmed openclaw-status cache")
	}()

	// Build handler chain
	var handler http.Handler = mux

	// Apply auth middleware if AuthManager is configured
	// Execution order: SessionMiddleware (check cookie) → AuthMiddleware (try providers) → handler
	// SessionMiddleware is outermost so it runs first.
	if cfg.AuthManager != nil {
		handler = RequireAuthPaths(handler)
		handler = AuthMiddleware(cfg.AuthManager)(handler)
		handler = SessionMiddleware(cfg.AuthManager)(handler)
	}

	// Wrap with middleware: recovery (outermost) → security/gzip → auth → mux.
	return recoveryMiddleware(applyMiddleware(handler), cfg)
}

// checkAuth checks authentication using the new AuthManager if available,
// falling back to legacy auth.Check. Returns true if authenticated.
// For the new system, it checks the request context (set by middleware).
func checkAuth(r *http.Request, cfg *Config) bool {
	if cfg.AuthManager != nil {
		return GetIdentity(r) != nil
	}
	return auth.Check(r) != nil
}

// checkAuthNotScoped checks that the user is authenticated and NOT a scoped/api-key user.
func checkAuthNotScoped(r *http.Request, cfg *Config) bool {
	if cfg.AuthManager != nil {
		id := GetIdentity(r)
		return id != nil && id.Provider != "api_key"
	}
	user := auth.Check(r)
	return user != nil && !auth.IsScopedUser(user)
}

func servesPanelData(w http.ResponseWriter, r *http.Request, panelID string, cfg *Config) {
	info := cfg.Registry.Get(panelID)
	if info == nil {
		http.Error(w, "Panel not found", 404)
		return
	}

	var result json.RawMessage
	switch panelID {
	case "cpu":
		m, _ := data.GetSystemMetrics()
		if m != nil && m.CPU != nil {
			result, _ = json.Marshal(m.CPU)
		}
	case "memory":
		m, _ := data.GetSystemMetrics()
		if m != nil && m.Memory != nil {
			result, _ = json.Marshal(m.Memory)
		}
	case "disk":
		m, _ := data.GetSystemMetrics()
		if m != nil && m.Disk != nil {
			result, _ = json.Marshal(m.Disk)
		}
	case "uptime":
		m, _ := data.GetSystemMetrics()
		if m != nil {
			result, _ = json.Marshal(map[string]interface{}{"uptime": m.Uptime, "hostname": m.Hostname})
		}
	case "processes":
		m, _ := data.GetSystemMetrics()
		if m != nil && m.Processes != nil {
			result, _ = json.Marshal(map[string]interface{}{
				"total": m.Processes.Total, "running": m.Processes.Running,
				"sleeping": m.Processes.Sleeping, "os": m.OS,
			})
		}
	case "claude-usage":
		result = data.GetUsageData(cfg.Workspace)
	case "crons":
		result = data.GetCronJobs(cfg.Workspace)
	case "models":
		result = data.GetAgentInfo(cfg.Workspace)
	case "openclaw-status":
		result = data.GetSystemStatusCached()
	case "updates":
		result = data.GetUpdatesStatus(cfg.RootDir)
	case "_test":
		result, _ = json.Marshal(map[string]interface{}{"message": "Hello from _test panel!", "ts": time.Now().UnixMilli()})
	default:
		http.Error(w, "No data handler for panel", 404)
		return
	}

	if result == nil {
		writeJSON(w, nil)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// writeScopedTokensToConfig reads the config file, updates auth.tokens, and writes back.
func writeScopedTokensToConfig(configPath string) error {
	if configPath == "" {
		return fmt.Errorf("config path not set")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// Parse auth section
	var authSection map[string]json.RawMessage
	if raw, ok := cfg["auth"]; ok {
		if err := json.Unmarshal(raw, &authSection); err != nil {
			authSection = map[string]json.RawMessage{}
		}
	} else {
		authSection = map[string]json.RawMessage{}
	}

	// Update tokens
	tokens := auth.GetScopedTokensFull()
	tokensJSON, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("marshal tokens: %w", err)
	}
	if len(tokens) == 0 {
		delete(authSection, "tokens")
	} else {
		authSection["tokens"] = tokensJSON
	}

	// Write auth section back
	authJSON, err := json.Marshal(authSection)
	if err != nil {
		return fmt.Errorf("marshal auth: %w", err)
	}
	cfg["auth"] = authJSON

	// Write config file with indentation
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(configPath, out, 0600)
}

func cacheHandler(h http.Handler, maxAge string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%s", maxAge))
		h.ServeHTTP(w, r)
	})
}

// getDomain extracts the domain from the AuthURL config or falls back to localhost.
func getDomain(cfg *Config) string {
	if cfg.PublicConfig != nil {
		if authURL, ok := cfg.PublicConfig["authUrl"].(string); ok && authURL != "" {
			// Extract domain from URL like https://w-ram.ai.essd.ee/auth/telegram/callback
			authURL = strings.TrimPrefix(authURL, "https://")
			authURL = strings.TrimPrefix(authURL, "http://")
			if idx := strings.Index(authURL, "/"); idx > 0 {
				return authURL[:idx]
			}
			return authURL
		}
	}
	return fmt.Sprintf("localhost:%d", cfg.Port)
}

// magicLinkErrorHTML returns a styled error page for magic link failures.
func magicLinkErrorHTML(title, message string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s — Vel</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; background: #0a0a0f; color: #e2e2e8; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; }
    .card { background: #12121a; border: 1px solid #1e1e2e; border-radius: 16px; padding: 40px 32px; text-align: center; max-width: 400px; }
    h1 { font-size: 24px; margin-bottom: 12px; color: #f87171; }
    p { color: #6e6e82; font-size: 14px; line-height: 1.6; margin-bottom: 24px; }
    a { color: #c9a84c; text-decoration: none; }
    a:hover { text-decoration: underline; }
  </style>
</head>
<body>
  <div class="card">
    <h1>%s</h1>
    <p>%s</p>
    <a href="/login">← Back to login</a>
  </div>
</body>
</html>`, title, title, message)
}

func applyMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Gzip
		isWebSocket := strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") && !strings.HasPrefix(r.URL.Path, "/ws/") && !isWebSocket {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			h.ServeHTTP(gzipResponseWriter{ResponseWriter: w, Writer: gz}, r)
			return
		}

		h.ServeHTTP(w, r)
	})
}
