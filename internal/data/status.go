package data

import (
	"encoding/json"
	"os/exec"
	"regexp"
	"strconv"
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
	chanRe := regexp.MustCompile(`│\s*(telegram|discord|whatsapp|signal)\s*│\s*(ON|OFF)\s*│`)
	if m := chanRe.FindStringSubmatch(raw); m != nil {
		channel = &ChannelStatus{Name: m[1], Status: m[2]}
	}

	var security *SecurityStatus
	secRe := regexp.MustCompile(`Summary:\s*(\d+)\s*critical[,·]\s*(\d+)\s*warn[,·]\s*(\d+)\s*info`)
	if m := secRe.FindStringSubmatch(raw); m != nil {
		c, _ := strconv.Atoi(m[1])
		w, _ := strconv.Atoi(m[2])
		i, _ := strconv.Atoi(m[3])
		security = &SecurityStatus{Critical: c, Warn: w, Info: i}
	}

	s := &OpenclawStatus{
		Online:    true,
		Version:   version,
		Heartbeat: get("Heartbeat"),
		Sessions:  get("Sessions"),
		Memory:    get("Memory"),
		Security:  security,
	}
	if channel != nil {
		s.Channel = channel
	} else {
		s.Channel = &ChannelStatus{Name: "unknown", Status: "ON"}
	}
	if s.Version == "" {
		s.Version = "unknown"
	}
	return s
}
