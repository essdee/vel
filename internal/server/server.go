package server

import (
	"compress/gzip"
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
	"vel/internal/relay"
)

type Config struct {
	RootDir      string
	Workspace    string
	Port         int
	Registry     *panels.Registry
	Order        []string
	Disabled     []string
	Version      string
	PublicConfig map[string]interface{} // safe fields for landing page
	Apps         []*apps.App
	Hooks        *hooks.Engine
	DSManager    *datasource.Manager
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

func NewServer(cfg *Config) http.Handler {
	mux := http.NewServeMux()
	apiLimiter := newRateLimiter(1000, 15*time.Minute, false)
	authLimiter := newRateLimiter(10, 15*time.Minute, true)

	// Pages
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(cfg.RootDir, "core", "public", "landing.html"))
	})
	mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(cfg.RootDir, "core", "public", "shell.html"))
	})

	// Static files
	publicDir := filepath.Join(cfg.RootDir, "core", "public")
	mux.Handle("/public/", http.StripPrefix("/public/", cacheHandler(http.FileServer(http.Dir(publicDir)), "3600")))

	vendorDir := filepath.Join(cfg.RootDir, "core", "vendor")
	mux.Handle("/core/vendor/", http.StripPrefix("/core/vendor/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		http.FileServer(http.Dir(vendorDir)).ServeHTTP(w, r)
	})))

	// App-driven static routes
	for _, app := range cfg.Apps {
		for urlPrefix, route := range app.Routes {
			switch route.Type {
			case "static":
				absDir := filepath.Join(app.Dir, route.Dir)
				if _, err := os.Stat(absDir); err == nil {
					mux.Handle(urlPrefix, http.StripPrefix(urlPrefix, http.FileServer(http.Dir(absDir))))
				}
			case "page":
				filePath := filepath.Join(app.Dir, route.Dir, "index.html")
				if _, err := os.Stat(filePath); err == nil {
					fp := filePath // capture
					mux.HandleFunc(urlPrefix, func(w http.ResponseWriter, r *http.Request) {
						http.ServeFile(w, r, fp)
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
			var body struct {
				InitData string `json:"initData"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			if body.InitData == "" {
				writeJSON(w, map[string]interface{}{"ok": false})
				w.WriteHeader(401)
				return
			}
			user := auth.ValidateInitData(body.InitData)
			if user == nil || !auth.IsAllowed(user.ID) {
				w.WriteHeader(401)
				writeJSON(w, map[string]interface{}{"ok": false})
				return
			}
			writeJSON(w, map[string]interface{}{"ok": true, "user": map[string]interface{}{"id": user.ID, "first_name": user.FirstName}})
			return
		}

		// GET
		user := auth.GetUserFromCookie(r)
		if user == nil || !auth.IsAllowed(user.ID) {
			w.WriteHeader(401)
			writeJSON(w, map[string]interface{}{"ok": false})
			return
		}
		writeJSON(w, map[string]interface{}{"ok": true, "user": user})
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

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   cfg.Version,
		})
	})

	mux.HandleFunc("/api/usage/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		user := auth.Check(r)
		if user == nil {
			w.WriteHeader(403)
			writeJSON(w, map[string]interface{}{"error": "Unauthorized"})
			return
		}
		scriptPath := filepath.Join(cfg.Workspace, "skills", "claude-usage-monitor", "scripts", "claude-usage-poll.sh")
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
		user := auth.Check(r)
		if user == nil {
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
			SameSite: http.SameSiteLaxMode,
			MaxAge:   7 * 24 * 60 * 60,
			Path:     "/",
		})
		http.Redirect(w, r, "/dashboard", http.StatusFound)
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
			SameSite: http.SameSiteLaxMode,
			MaxAge:   7 * 24 * 60 * 60,
			Path:     "/",
		})
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})

	mux.HandleFunc("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
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
		user := auth.Check(r)
		if user == nil {
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
		user := auth.Check(r)
		if user == nil {
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

	// Browser Relay
	rl := relay.New()
	mux.HandleFunc("/relay/token", rl.HandleToken)
	mux.HandleFunc("/relay/json", rl.HandleTargets)
	mux.HandleFunc("/relay/ws", rl.HandleBrowserWS)
	mux.HandleFunc("/relay/cdp", rl.HandleAgentWS)
	mux.HandleFunc("/relay/download", rl.HandleDownload)
	mux.HandleFunc("/relay/bridge", rl.HandleBridge)
	mux.HandleFunc("/relay/status", rl.HandleStatus)
	mux.HandleFunc("/relay/pair/new", rl.HandlePairNew)
	mux.HandleFunc("/relay/pair/status", rl.HandlePairStatus)
	mux.HandleFunc("/relay/pair/activate", rl.HandlePairActivate)

	// WebSocket
	mux.HandleFunc("/ws/metrics", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(w, r, cfg)
	})

	// Wrap with middleware
	return applyMiddleware(mux)
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
		result = data.GetSystemStatus()
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

func cacheHandler(h http.Handler, maxAge string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%s", maxAge))
		h.ServeHTTP(w, r)
	})
}

func applyMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Gzip
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") && !strings.HasPrefix(r.URL.Path, "/ws/") && !strings.HasPrefix(r.URL.Path, "/relay/ws") && !strings.HasPrefix(r.URL.Path, "/relay/cdp") {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			h.ServeHTTP(gzipResponseWriter{ResponseWriter: w, Writer: gz}, r)
			return
		}

		h.ServeHTTP(w, r)
	})
}
