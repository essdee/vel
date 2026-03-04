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
	Update  string `json:"update,omitempty"`
	Gateway string `json:"gateway,omitempty"`
}

type ChannelStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type SecurityWarning struct {
	Level   string `json:"level"`
	Title   string `json:"title"`
	Detail  string `json:"detail,omitempty"`
	Fix     string `json:"fix,omitempty"`
}

type SecurityStatus struct {
	Critical int               `json:"critical"`
	Warn     int               `json:"warn"`
	Info     int               `json:"info"`
	Items    []SecurityWarning `json:"items,omitempty"`
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

		// Parse individual security items
		// Format: "  WARN|CRITICAL|INFO\n  key Title text\n    detail\n    Fix: fix text"
		itemRe := regexp.MustCompile(`(?m)^(WARN|CRITICAL|INFO)\n`)
		titleRe := regexp.MustCompile(`(?m)^\S+\s+(.+)$`)
		fixRe := regexp.MustCompile(`(?m)^\s+Fix:\s+(.+)$`)

		// Split by WARN/CRITICAL/INFO section headers
		sections := regexp.MustCompile(`(?m)^(WARN|CRITICAL|INFO)\s*$`).Split(raw, -1)
		levels := itemRe.FindAllStringSubmatch(raw, -1)

		for i, section := range sections[1:] { // skip before first header
			if i >= len(levels) {
				break
			}
			level := strings.ToLower(levels[i][1])

			// Each section can have multiple items separated by entries starting with non-whitespace
			entryRe := regexp.MustCompile(`(?m)^(\S+\.\S+)\s+(.+)`)
			entries := entryRe.FindAllStringSubmatchIndex(section, -1)

			for j, loc := range entries {
				end := len(section)
				if j+1 < len(entries) {
					end = entries[j+1][0]
				}
				block := section[loc[0]:end]
				title := section[loc[4]:loc[5]]

				detail := ""
				fix := ""
				lines := strings.Split(block, "\n")
				for _, l := range lines[1:] {
					trimmed := strings.TrimSpace(l)
					if strings.HasPrefix(trimmed, "Fix:") {
						fix = strings.TrimSpace(strings.TrimPrefix(trimmed, "Fix:"))
					} else if trimmed != "" && detail == "" {
						detail = trimmed
					}
				}

				_ = titleRe
				_ = fixRe
				security.Items = append(security.Items, SecurityWarning{
					Level:  level,
					Title:  title,
					Detail: detail,
					Fix:    fix,
				})
			}
		}
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

	s := &OpenclawStatus{
		Online:    true,
		Version:   version,
		Heartbeat: get("Heartbeat"),
		Sessions:  get("Sessions"),
		Memory:    get("Memory"),
		Security:  security,
		Update:    update,
		Gateway:   gateway,
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
