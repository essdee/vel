package debug

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// ServerInfo holds the information needed by debug endpoints.
type ServerInfo struct {
	Version    string
	StartTime  time.Time
	Config     map[string]interface{} // full config (will be redacted)
	Routes     []string               // list of registered routes
	Middleware []string               // ordered middleware list
	// Callbacks for dynamic data
	SessionCountFn func() (active int, oldest, newest time.Time)
	// For in-process verify: raw mux (no auth middleware) and full handler (with auth)
	Mux     http.Handler // raw mux without auth middleware
	Handler http.Handler // full handler with auth middleware
	// Server config for verify
	RootDir      string
	FrameworkDir string
	Port         int
	AuthMode     string // "none", "token", "telegram"
}

var serverInfo *ServerInfo

// SetServerInfo stores the server info for debug endpoints.
func SetServerInfo(info *ServerInfo) {
	serverInfo = info
}

// RegisterEndpoints registers all debug endpoints on the given mux.
func RegisterEndpoints(mux *http.ServeMux, cfg DebugConfig) {
	// Layer 2: Debug endpoints (always registered on debug server)
	mux.HandleFunc("/debug/health", handleHealth)
	mux.HandleFunc("/debug/routes", handleRoutes)
	mux.HandleFunc("/debug/config", handleConfig)
	mux.HandleFunc("/debug/middleware", handleMiddleware)
	mux.HandleFunc("/debug/sessions", handleSessions)
	mux.HandleFunc("/debug/verify", handleVerify)

	// Layer 3: AI Debug endpoints
	if cfg.AIDebug {
		mux.HandleFunc("/debug/request/", handleRequestByID)
		mux.HandleFunc("/debug/errors/recent", handleRecentErrors)
		mux.HandleFunc("/debug/diagnose", handleDiagnose)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	info := serverInfo
	if info == nil {
		writeJSON(w, map[string]interface{}{"status": "ok"})
		return
	}

	uptime := time.Since(info.StartTime)
	writeJSON(w, map[string]interface{}{
		"status":        "ok",
		"uptime":        formatDuration(uptime),
		"version":       info.Version,
		"go_version":    runtime.Version(),
		"goroutines":    runtime.NumGoroutine(),
		"debug_mode":    IsDebugMode(),
		"ai_debug_mode": IsAIDebugMode(),
	})
}

func handleRoutes(w http.ResponseWriter, r *http.Request) {
	info := serverInfo
	if info == nil {
		writeJSON(w, map[string]interface{}{"routes": []string{}})
		return
	}
	writeJSON(w, map[string]interface{}{"routes": info.Routes})
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	info := serverInfo
	if info == nil {
		writeJSON(w, map[string]interface{}{})
		return
	}
	redacted := redactConfig(info.Config)
	writeJSON(w, redacted)
}

func handleMiddleware(w http.ResponseWriter, r *http.Request) {
	info := serverInfo
	if info == nil {
		writeJSON(w, map[string]interface{}{"middleware": []string{}})
		return
	}
	writeJSON(w, map[string]interface{}{"middleware": info.Middleware})
}

func handleSessions(w http.ResponseWriter, r *http.Request) {
	info := serverInfo
	if info == nil || info.SessionCountFn == nil {
		writeJSON(w, map[string]interface{}{
			"active_count": 0,
			"oldest":       "n/a",
			"newest":       "n/a",
		})
		return
	}
	active, oldest, newest := info.SessionCountFn()
	oldestStr := "n/a"
	newestStr := "n/a"
	if active > 0 {
		oldestStr = formatDuration(time.Since(oldest)) + " ago"
		newestStr = formatDuration(time.Since(newest)) + " ago"
	}
	writeJSON(w, map[string]interface{}{
		"active_count": active,
		"oldest":       oldestStr,
		"newest":       newestStr,
	})
}

func handleRequestByID(w http.ResponseWriter, r *http.Request) {
	buf := GetBuffer()
	if buf == nil {
		writeJSON(w, map[string]interface{}{"error": "AI debug mode not enabled"})
		return
	}

	// Extract request ID from path: /debug/request/{id}
	id := strings.TrimPrefix(r.URL.Path, "/debug/request/")
	if id == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]interface{}{"error": "request ID required"})
		return
	}

	entry := buf.Get(id)
	if entry == nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]interface{}{"error": "request not found in buffer"})
		return
	}
	writeJSON(w, entry)
}

func handleRecentErrors(w http.ResponseWriter, r *http.Request) {
	buf := GetBuffer()
	if buf == nil {
		writeJSON(w, map[string]interface{}{"error": "AI debug mode not enabled"})
		return
	}

	n := 10
	if qn := r.URL.Query().Get("n"); qn != "" {
		fmt.Sscanf(qn, "%d", &n)
		if n <= 0 {
			n = 10
		}
		if n > 100 {
			n = 100
		}
	}

	errors := buf.RecentErrors(n)
	if errors == nil {
		errors = []RequestLog{}
	}
	writeJSON(w, map[string]interface{}{"errors": errors, "count": len(errors)})
}

// redactConfig deep-copies a config map and replaces values whose keys suggest secrets.
func redactConfig(cfg map[string]interface{}) map[string]interface{} {
	if cfg == nil {
		return map[string]interface{}{}
	}
	result := make(map[string]interface{})
	for k, v := range cfg {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "token") || strings.Contains(lk, "key") ||
			strings.Contains(lk, "secret") || strings.Contains(lk, "password") {
			result[k] = "[REDACTED]"
			continue
		}
		switch val := v.(type) {
		case map[string]interface{}:
			result[k] = redactConfig(val)
		default:
			result[k] = v
		}
	}
	return result
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hours, minutes)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
