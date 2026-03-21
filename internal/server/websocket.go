package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"vel/internal/auth"
	"vel/internal/data"

	"github.com/gorilla/websocket"
)

type wsAuthMsg struct {
	Type       string     `json:"type"`
	InitData   string     `json:"initData,omitempty"`
	CookieAuth bool       `json:"cookieAuth,omitempty"`
	User       *auth.User `json:"user,omitempty"`
}

// checkOrigin validates that the WebSocket request originates from the configured domain.
// Allows requests with no Origin header (same-origin clients may omit it).
func checkOrigin(cfg *Config) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		// Extract host from origin (strip scheme)
		host := origin
		if idx := strings.Index(host, "://"); idx >= 0 {
			host = host[idx+3:]
		}
		// Strip any trailing path
		if idx := strings.Index(host, "/"); idx >= 0 {
			host = host[:idx]
		}
		return strings.EqualFold(host, getDomain(cfg))
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request, cfg *Config) {
	upgrader := websocket.Upgrader{
		CheckOrigin: checkOrigin(cfg),
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	authenticated := false
	done := make(chan struct{})

	// Check if already authenticated via session middleware (cookie sent with WS upgrade)
	if cfg.AuthManager != nil {
		identity := GetIdentity(r)
		if identity != nil {
			authenticated = true
		}
	}

	// Auth timeout
	go func() {
		time.Sleep(10 * time.Second)
		if !authenticated {
			conn.Close()
		}
	}()

	// If pre-authenticated via session cookie, skip auth handshake
	if authenticated {
		// Still need to read the auth message from the client (they send it)
		// but we can respond immediately and proceed
		go func() {
			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					close(done)
					return
				}
				var authMsg wsAuthMsg
				if json.Unmarshal(msg, &authMsg) == nil && authMsg.Type == "auth" {
					// Already authenticated, just confirm
					conn.WriteJSON(map[string]interface{}{"type": "auth", "ok": true})
					// Keep reading to detect close
					for {
						if _, _, err := conn.ReadMessage(); err != nil {
							close(done)
							return
						}
					}
				}
			}
		}()

		// Start broadcasting immediately
		broadcastMetrics(conn, cfg, done)
		return
	}

	// Read auth message (legacy path or new provider-based)
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var authMsg wsAuthMsg
		if json.Unmarshal(msg, &authMsg) != nil {
			continue
		}

		if authMsg.Type != "auth" {
			continue
		}

		// Try new auth system first
		if cfg.AuthManager != nil && authMsg.InitData != "" {
			tgProvider := cfg.AuthManager.GetProvider("telegram")
			if tgProvider != nil {
				creds := auth.TelegramCredentials{InitData: authMsg.InitData}
				if _, authErr := tgProvider.Authenticate(creds); authErr == nil {
					authenticated = true
				}
			}
		}

		// Legacy fallback
		if !authenticated {
			var user *auth.User
			if authMsg.InitData != "" {
				user = auth.ValidateInitData(authMsg.InitData)
			} else if authMsg.CookieAuth && authMsg.User != nil {
				user = authMsg.User
			}

			// TEST_MODE bypass
			if auth.IsTestMode() {
				user = &auth.User{ID: 0, FirstName: "Test"}
			}

			// "none" mode bypass
			if auth.GetAuthMode() == "none" && user == nil {
				user = &auth.User{ID: 1, FirstName: "Admin", Username: "admin"}
			}

			if user != nil && auth.IsAllowed(user.ID) {
				authenticated = true
			}
		}

		if !authenticated {
			conn.WriteJSON(map[string]interface{}{"type": "auth", "ok": false})
			conn.Close()
			return
		}

		conn.WriteJSON(map[string]interface{}{"type": "auth", "ok": true})

		// Start broadcasting
		go broadcastMetrics(conn, cfg, done)

		// Keep reading to detect close
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				close(done)
				return
			}
		}
	}
}

// broadcastMetrics sends metrics to a WebSocket connection every 2 seconds.
func broadcastMetrics(conn *websocket.Conn, cfg *Config, done chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	sendMetrics := func() {
		tTotal := time.Now()
		t0 := time.Now()
		metrics, err := data.GetSystemMetrics()
		if err != nil {
			return
		}
		log.Printf("[ws] GetSystemMetrics: %v", time.Since(t0))

		// Build raw data for all known panel IDs via registry
		rawData := make(map[string]interface{})
		for id, info := range cfg.Registry.Entries() {
			if info.Manifest == nil {
				continue
			}
			if d := GetPanelData(id, cfg); d != nil {
				rawData[id] = d
			}
		}

		// Add data source data (file-based sources)
		if cfg.DSManager != nil {
			for key, state := range cfg.DSManager.GetAllData() {
				if _, exists := rawData[key]; !exists {
					rawData[key] = state.Data
				}
				// Also add under short name (strip "appname:" prefix) for panel matching
				if idx := strings.Index(key, ":"); idx >= 0 {
					short := key[idx+1:]
					if _, exists := rawData[short]; !exists {
						rawData[short] = state.Data
					}
				}
			}
		}

		// Build per-panel data, respecting dataSource subscription and dataEnvelope
		panelData := make(map[string]interface{})
		for id, info := range cfg.Registry.Entries() {
			if info.Manifest == nil {
				continue
			}
			m := info.Manifest

			// Determine which data key this panel consumes
			dataKey := id
			if m.DataSource != "" {
				dataKey = m.DataSource
			}

			d, exists := rawData[dataKey]
			if !exists {
				continue
			}

			// If panel wants full envelope and data comes from a managed source
			if m.DataEnvelope && cfg.DSManager != nil {
				state := cfg.DSManager.GetSourceState(dataKey)
				if state != nil {
					panelData[id] = map[string]interface{}{
						"ok":         state.OK,
						"stale":      state.Stale,
						"staleSince": state.StaleSince,
						"data":       state.Data,
					}
					continue
				}
			}

			panelData[id] = d
		}

		msg := map[string]interface{}{
			"type":   "metrics",
			"data":   metrics,
			"usage":  data.GetUsageData(cfg.Workspace),
			"agent":  json.RawMessage(data.GetAgentInfo(cfg.Workspace)),
			"crons":  json.RawMessage(data.GetCronJobs(cfg.Workspace)),
			"panels": panelData,
		}

		// Add source status metadata
		if cfg.DSManager != nil {
			msg["_sourceStatus"] = cfg.DSManager.GetStatus()
		}

		log.Printf("[ws] sendMetrics total: %v", time.Since(tTotal))
		if err := conn.WriteJSON(msg); err != nil {
			return
		}
	}

	sendMetrics()
	for {
		select {
		case <-ticker.C:
			sendMetrics()
		case <-done:
			return
		}
	}
}

func init() {
	_ = log.Prefix // suppress unused import
}
