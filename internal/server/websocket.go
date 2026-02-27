package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"vel/internal/auth"
	"vel/internal/data"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsAuthMsg struct {
	Type       string     `json:"type"`
	InitData   string     `json:"initData,omitempty"`
	CookieAuth bool       `json:"cookieAuth,omitempty"`
	User       *auth.User `json:"user,omitempty"`
}

func handleWebSocket(w http.ResponseWriter, r *http.Request, cfg *Config) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	authenticated := false
	done := make(chan struct{})

	// Auth timeout
	go func() {
		time.Sleep(10 * time.Second)
		if !authenticated {
			conn.Close()
		}
	}()

	// Read auth message
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

		if user == nil || !auth.IsAllowed(user.ID) {
			conn.WriteJSON(map[string]interface{}{"type": "auth", "ok": false})
			conn.Close()
			return
		}

		authenticated = true
		conn.WriteJSON(map[string]interface{}{"type": "auth", "ok": true})

		// Start broadcasting
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			sendMetrics := func() {
				metrics, err := data.GetSystemMetrics()
				if err != nil {
					return
				}

				panelData := make(map[string]interface{})
				for id, info := range cfg.Registry.Entries() {
					if info.Manifest == nil {
						continue
					}
					switch id {
					case "cpu":
						if metrics.CPU != nil {
							panelData["cpu"] = metrics.CPU
						}
					case "memory":
						if metrics.Memory != nil {
							panelData["memory"] = metrics.Memory
						}
					case "disk":
						if metrics.Disk != nil {
							panelData["disk"] = metrics.Disk
						}
					case "uptime":
						panelData["uptime"] = map[string]interface{}{"uptime": metrics.Uptime, "hostname": metrics.Hostname}
					case "processes":
						if metrics.Processes != nil {
							panelData["processes"] = map[string]interface{}{
								"total": metrics.Processes.Total, "running": metrics.Processes.Running,
								"sleeping": metrics.Processes.Sleeping, "os": metrics.OS,
							}
						}
					case "claude-usage":
						panelData["claude-usage"] = data.GetUsageData(cfg.Workspace)
					case "crons":
						panelData["crons"] = json.RawMessage(data.GetCronJobs(cfg.Workspace))
					case "models":
						panelData["models"] = json.RawMessage(data.GetAgentInfo(cfg.Workspace))
					case "openclaw-status":
						panelData["openclaw-status"] = json.RawMessage(data.GetSystemStatus())
					case "_test":
						panelData["_test"] = map[string]interface{}{"message": "Hello from _test panel!", "ts": time.Now().UnixMilli()}
					}
				}

				msg := map[string]interface{}{
					"type":   "metrics",
					"data":   metrics,
					"usage":  data.GetUsageData(cfg.Workspace),
					"agent":  json.RawMessage(data.GetAgentInfo(cfg.Workspace)),
					"crons":  json.RawMessage(data.GetCronJobs(cfg.Workspace)),
					"panels": panelData,
				}

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
		}()

		// Keep reading to detect close
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				close(done)
				return
			}
		}
	}
}

func init() {
	_ = log.Prefix // suppress unused import
}
