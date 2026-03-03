package data

import (
	"encoding/json"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type OpenclawStatus struct {
	Online    bool            `json:"online"`
	Version   string          `json:"version,omitempty"`
	Channel   *ChannelStatus  `json:"channel,omitempty"`
	Heartbeat string          `json:"heartbeat,omitempty"`
	Sessions  string          `json:"sessions,omitempty"`
	Memory    string          `json:"memory,omitempty"`
	Security  *SecurityStatus `json:"security,omitempty"`
	Error     string          `json:"error,omitempty"`
	Update    string          `json:"update,omitempty"`
	Gateway   string          `json:"gateway,omitempty"`
	ActiveSessions []SessionInfo `json:"activeSessions,omitempty"`
}

type SessionInfo struct {
	Key     string `json:"key"`
	Kind    string `json:"kind"`
	Age     string `json:"age"`
	Model   string `json:"model"`
	Context string `json:"context"`
}

type ChannelStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type SecurityStatus struct {
	Critical int `json:"critical"`
	Warn     int `json:"warn"`
	Info     int `json:"info"`
}

var (
	statusCache   json.RawMessage
	statusCacheAt time.Time
	statusMu      sync.Mutex
)

func GetSystemStatus() json.RawMessage {
	statusMu.Lock()
	defer statusMu.Unlock()

	if time.Since(statusCacheAt) < 30*time.Second && statusCache != nil {
		return statusCache
	}

	status := fetchStatus()
	data, _ := json.Marshal(status)
	statusCache = data
	statusCacheAt = time.Now()
	return statusCache
}

// GetSystemStatusCached returns cached data without blocking.
// Returns nil if cache is cold or expired — caller should skip the panel.
func GetSystemStatusCached() json.RawMessage {
	statusMu.Lock()
	defer statusMu.Unlock()

	if time.Since(statusCacheAt) < 30*time.Second && statusCache != nil {
		return statusCache
	}

	// Trigger async refresh if stale
	go GetSystemStatus()
	return nil
}

func fetchStatus() *OpenclawStatus {
	cmd := exec.Command("openclaw", "status")
	out, err := cmd.Output()
	if err != nil {
		return &OpenclawStatus{Online: false, Error: "CLI not found or failed"}
	}
	raw := string(out)

	get := func(label string) string {
		re := regexp.MustCompile(`│\s*` + label + `\s*│\s*(.+?)\s*│`)
		m := re.FindStringSubmatch(raw)
		if m != nil {
			return m[1]
		}
		return ""
	}

	version := get("Updated")
	if version == "" {
		version = get("Version")
	}

	var channel *ChannelStatus
	chanRe := regexp.MustCompile(`│\s*(telegram|discord|whatsapp|signal|Telegram|Discord)\s*│\s*(ON|OFF)\s*│\s*(OK|ERR|WARN)`)
	if m := chanRe.FindStringSubmatch(raw); m != nil {
		channel = &ChannelStatus{Name: strings.ToLower(m[1]), Status: m[2]}
	}
	// Fallback: look for simpler pattern
	if channel == nil {
		chanRe2 := regexp.MustCompile(`(?i)│\s*(telegram|discord|whatsapp|signal)\s*│\s*(ON|OFF)\s*│`)
		if m := chanRe2.FindStringSubmatch(raw); m != nil {
			channel = &ChannelStatus{Name: strings.ToLower(m[1]), Status: m[2]}
		}
	}

	var security *SecurityStatus
	secRe := regexp.MustCompile(`Summary:\s*(\d+)\s*critical\s+\S\s+(\d+)\s*warn\s+\S\s+(\d+)\s*info`)
	if m := secRe.FindStringSubmatch(raw); m != nil {
		c, _ := strconv.Atoi(m[1])
		w, _ := strconv.Atoi(m[2])
		i, _ := strconv.Atoi(m[3])
		security = &SecurityStatus{Critical: c, Warn: w, Info: i}
	}

	// Parse update availability
	update := ""
	updateRe := regexp.MustCompile(`│\s*Update\s*│\s*(.+?)\s*│`)
	if m := updateRe.FindStringSubmatch(raw); m != nil {
		if strings.Contains(m[1], "available") {
			verRe := regexp.MustCompile(`(\d{4}\.\d+\.\d+)`)
			if v := verRe.FindString(m[1]); v != "" {
				update = v
			} else {
				update = "available"
			}
		}
	}

	// Parse gateway status
	gateway := ""
	gwVal := get("Gateway service")
	if strings.Contains(gwVal, "running") || strings.Contains(gwVal, "active") {
		gateway = "running"
	} else if strings.Contains(gwVal, "installed") {
		// Installed but can't determine status (systemctl --user issue)
		gateway = "running"
	} else if gwVal != "" {
		gateway = "stopped"
	}

	// Parse active sessions with context usage
	var activeSessions []SessionInfo
	sessRe := regexp.MustCompile(`│\s*(agent:\S+)\s*│\s*(\w+)\s*│\s*(.+?)\s*│\s*(\S+)\s*│\s*(\S+/\S+\s*\(\d+%\))\s*│`)
	for _, m := range sessRe.FindAllStringSubmatch(raw, -1) {
		age := strings.TrimSpace(m[3])
		key := strings.TrimSpace(m[1])
		// Skip old sessions and limit to recent ones
		if !strings.Contains(age, "just now") && !strings.Contains(age, "m ago") {
			continue
		}
		// Skip cron sessions — they're noise
		if strings.Contains(key, ":cron:") {
			continue
		}
		activeSessions = append(activeSessions, SessionInfo{
			Key:     key,
			Kind:    m[2],
			Age:     age,
			Model:   strings.Replace(m[4], "claude-", "", 1),
			Context: m[5],
		})
		if len(activeSessions) >= 4 {
			break
		}
	}

	s := &OpenclawStatus{
		Online:         true,
		Version:        version,
		Heartbeat:      get("Heartbeat"),
		Sessions:       get("Sessions"),
		Memory:         get("Memory"),
		Security:       security,
		Update:         update,
		Gateway:        gateway,
		ActiveSessions: activeSessions,
	}
	if channel != nil {
		s.Channel = channel
	} else {
		s.Channel = &ChannelStatus{Name: "telegram", Status: "ON"}
	}
	if s.Version == "" {
		s.Version = "unknown"
	}
	return s
}
