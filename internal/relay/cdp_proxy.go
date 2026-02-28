package relay

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// extractToken gets relay token from query param, header, or cookie.
func extractToken(r *http.Request) string {
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	if t := r.Header.Get("x-openclaw-relay-token"); t != "" {
		return t
	}
	if t := r.Header.Get("Authorization"); strings.HasPrefix(t, "Bearer ") {
		return strings.TrimPrefix(t, "Bearer ")
	}
	return ""
}

// deriveWSScheme returns "ws" or "wss" based on the request.
func deriveWSScheme(r *http.Request) string {
	if r.TLS != nil {
		return "wss"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd == "https" {
		return "wss"
	}
	return "ws"
}

// HandleCDPJsonVersion returns a CDP-compatible /json/version response.
// This allows OpenClaw (or any CDP client) to discover the relay's WS endpoint.
func (rl *Relay) HandleCDPJsonVersion(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		http.Error(w, `{"error":"missing token"}`, 401)
		return
	}
	sess := rl.sessions.GetByToken(token)
	if sess == nil {
		http.Error(w, `{"error":"invalid token"}`, 401)
		return
	}

	wsScheme := deriveWSScheme(r)
	wsURL := fmt.Sprintf("%s://%s/relay/cdp/ws?token=%s", wsScheme, r.Host, token)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"Browser":              "Chrome/Relay",
		"Protocol-Version":     "1.3",
		"User-Agent":           "Vel-Relay/1.0",
		"webSocketDebuggerUrl": wsURL,
	})
}

// HandleCDPJsonList returns a CDP-compatible target list with per-target WS URLs.
func (rl *Relay) HandleCDPJsonList(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		http.Error(w, `{"error":"missing token"}`, 401)
		return
	}
	sess := rl.sessions.GetByToken(token)
	if sess == nil {
		http.Error(w, `{"error":"invalid token"}`, 401)
		return
	}

	targets := sess.GetTargets()
	if targets == nil {
		targets = []CDPTarget{}
	}

	wsScheme := deriveWSScheme(r)

	// Add webSocketDebuggerUrl to each target
	type cdpTarget struct {
		CDPTarget
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	out := make([]cdpTarget, len(targets))
	for i, t := range targets {
		out[i] = cdpTarget{
			CDPTarget:            t,
			WebSocketDebuggerURL: fmt.Sprintf("%s://%s/relay/cdp/page/%s?token=%s", wsScheme, r.Host, t.ID, token),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// HandleCDPProxyWS is a raw CDP WebSocket proxy.
// It speaks standard CDP JSON-RPC (no envelope) on the agent side,
// and wraps/unwraps messages using our relay envelope format internally.
func (rl *Relay) HandleCDPProxyWS(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		http.Error(w, "Unauthorized", 401)
		return
	}
	sess := rl.sessions.GetByToken(token)
	if sess == nil {
		http.Error(w, "Invalid token", 401)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[relay-cdp] WS upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Replace agent WS with this CDP proxy connection
	sess.SetAgentWS(conn)
	log.Printf("[relay-cdp] CDP proxy connected for user %d", sess.UserID)

	// Send initial status
	sess.mu.Lock()
	browserConnected := sess.BrowserWS != nil
	sess.mu.Unlock()
	if browserConnected {
		// Send a synthetic CDP event so the client knows the browser is ready
		conn.WriteJSON(map[string]interface{}{
			"method": "Relay.browserConnected",
			"params": map[string]bool{"connected": true},
		})
	}

	// We need a goroutine to forward browser→agent messages
	done := make(chan struct{})

	// Create a message channel for browser→agent forwarding
	// We intercept messages from the browser WS in a custom way:
	// Register this connection as agent WS, and override the forwarding behavior.
	// The existing relay.go HandleBrowserWS forwards envelope messages to AgentWS.
	// Those messages arrive as envelope format. We need to unwrap them.

	// The trick: we set AgentWS to a wrapper that unwraps envelopes.
	// But since we can't easily wrap websocket.Conn, instead we'll
	// use the existing agent WS slot and add a flag for raw CDP mode.
	sess.mu.Lock()
	sess.CDPRawMode = true
	sess.mu.Unlock()

	defer func() {
		sess.ClearAgentWS()
		sess.mu.Lock()
		sess.CDPRawMode = false
		sess.mu.Unlock()
		log.Printf("[relay-cdp] CDP proxy disconnected for user %d", sess.UserID)
		close(done)
	}()

	// Read loop: agent sends raw CDP, we wrap in envelope and forward to browser
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		sess.IncrementMsgCount()

		// Wrap raw CDP message in relay envelope
		envelope := Envelope{
			Type: "cdp",
			Data: json.RawMessage(msg),
		}
		envBytes, _ := json.Marshal(envelope)

		sess.mu.Lock()
		browserWS := sess.BrowserWS
		sess.mu.Unlock()
		if browserWS != nil {
			browserWS.WriteMessage(websocket.TextMessage, envBytes)
		}
	}
}

// HandleCDPStatusJSON returns enhanced status for CDP integration.
func (rl *Relay) HandleCDPStatusJSON(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		// Fall back to cookie auth
		rl.HandleStatus(w, r)
		return
	}
	sess := rl.sessions.GetByToken(token)
	if sess == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"state":   "disconnected",
			"targets": []CDPTarget{},
		})
		return
	}

	sess.mu.Lock()
	browserConnected := sess.BrowserWS != nil
	agentConnected := sess.AgentWS != nil
	msgCount := sess.MsgCount
	targets := sess.Targets
	connAt := sess.ConnectedAt
	lastAct := sess.LastActivity
	sess.mu.Unlock()

	state := "disconnected"
	if browserConnected && agentConnected {
		state = "agent_active"
	} else if browserConnected {
		state = "connected"
	}

	if targets == nil {
		targets = []CDPTarget{}
	}

	resp := map[string]interface{}{
		"state":    state,
		"msgCount": msgCount,
		"targets":  targets,
	}
	if browserConnected {
		resp["connectedSince"] = connAt
		resp["lastActivity"] = lastAct
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
