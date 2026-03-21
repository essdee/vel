package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// sessionsCache provides a short TTL cache for GetSessionsData.
var sessionsCache struct {
	sync.Mutex
	result *SessionsData
	ts     time.Time
}

const sessionsCacheTTL = 2 * time.Second

// prevUpdatedAt tracks the previous updatedAt values for delta-based working detection.
var prevUpdatedAt sync.Map

// SessionEntry represents a single OpenClaw session.
type SessionEntry struct {
	Key          string  `json:"key"`
	AgentID      string  `json:"agentId"`
	Kind         string  `json:"kind"`
	Label        string  `json:"label"`
	Model        string  `json:"model"`
	TotalTokens  int     `json:"totalTokens"`
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	MaxContext   int     `json:"maxContextTokens"`
	ContextPct   float64 `json:"contextPct"`
	UpdatedAt    int64   `json:"updatedAt"`
	AgeMins      float64 `json:"ageMins"`
	Active       bool    `json:"active"`
	Working      bool    `json:"working"`
	UserLabel    string  `json:"userLabel"`
	Provider     string  `json:"provider"`
	ChatType     string  `json:"chatType"`
	TelegramID   string  `json:"telegramId"`
}

// SessionsData is the top-level structure returned to panels.
type SessionsData struct {
	Total    int            `json:"total"`
	Active   int            `json:"active"`
	Working  int            `json:"working"`
	ByKind   map[string]int `json:"byKind"`
	ByModel  map[string]int `json:"byModel"`
	ByAgent  map[string]int `json:"byAgent"`
	AgentIDs []string       `json:"agentIds"`
	Recent   []SessionEntry `json:"recent"`
	TS       int64          `json:"ts"`
}

// rawSession is used to unmarshal individual session JSON entries.
type rawSession struct {
	Model         string      `json:"model"`
	UpdatedAt     int64       `json:"updatedAt"`
	InputTokens   int         `json:"inputTokens"`
	OutputTokens  int         `json:"outputTokens"`
	TotalTokens   int         `json:"totalTokens"`
	ContextTokens int         `json:"contextTokens"`
	Label         string      `json:"label"`
	ChatType      string      `json:"chatType"`
	Origin        interface{} `json:"origin"`
}

// Known topic display names
var topicNames = map[string]string{
	"1": "General",
	"2": "Architect",
	"3": "Coder",
	"4": "Sentinel",
}

// Known agent display names
var agentNames = map[string]string{
	"main":      "Ram",
	"architect": "Architect Ram",
	"coder":     "Coder Ram",
	"sentinel":  "Sentinel Ram",
}

// GetSessionsData returns parsed session data with caching and delta-based working detection.
func GetSessionsData() *SessionsData {
	sessionsCache.Lock()
	if sessionsCache.result != nil && time.Since(sessionsCache.ts) < sessionsCacheTTL {
		r := sessionsCache.result
		sessionsCache.Unlock()
		return r
	}
	sessionsCache.Unlock()

	result := fetchSessionsData()

	sessionsCache.Lock()
	sessionsCache.result = result
	sessionsCache.ts = time.Now()
	sessionsCache.Unlock()

	return result
}

func fetchSessionsData() *SessionsData {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	agentsDir := filepath.Join(home, ".openclaw", "agents")
	pattern := filepath.Join(agentsDir, "*/sessions/sessions.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}

	// Merge all agent session files
	allSessions := make(map[string]json.RawMessage)
	agentIDSet := make(map[string]struct{})

	for _, f := range files {
		// Extract agent ID from path: ~/.openclaw/agents/<agentId>/sessions/sessions.json
		rel, _ := filepath.Rel(agentsDir, f)
		parts := strings.SplitN(rel, string(filepath.Separator), 3)
		if len(parts) >= 1 {
			agentIDSet[parts[0]] = struct{}{}
		}

		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}

		var agentSessions map[string]json.RawMessage
		if err := json.Unmarshal(raw, &agentSessions); err != nil {
			continue
		}
		for k, v := range agentSessions {
			allSessions[k] = v
		}
	}

	agentIDs := make([]string, 0, len(agentIDSet))
	for id := range agentIDSet {
		agentIDs = append(agentIDs, id)
	}
	sort.Strings(agentIDs)

	nowMs := float64(time.Now().UnixMilli())

	byKind := map[string]int{"main": 0, "cron": 0, "subagent": 0, "dm": 0, "other": 0}
	byModel := make(map[string]int)
	byAgent := make(map[string]int)
	var entries []SessionEntry

	for k, rawVal := range allSessions {
		// Skip cron run sessions and slash sessions
		if strings.Contains(k, ":run:") || strings.Contains(k, ":slash:") {
			continue
		}

		// Determine kind
		kind := classifyKind(k)

		// Extract agent ID
		agentID := "main"
		if strings.HasPrefix(k, "agent:") {
			parts := strings.SplitN(k, ":", 3)
			if len(parts) >= 2 {
				agentID = parts[1]
			}
		}

		// Parse session fields
		var s rawSession
		if err := json.Unmarshal(rawVal, &s); err != nil {
			continue
		}

		// Extract origin fields
		userLabel, provider, chatType := extractOrigin(s.Origin, s.ChatType)

		// Extract telegram ID
		telegramID := ""
		if idx := strings.Index(k, ":telegram:direct:"); idx >= 0 {
			rest := k[idx+len(":telegram:direct:"):]
			parts := strings.SplitN(rest, ":", 2)
			telegramID = parts[0]
		}

		// Build label
		label := buildLabel(k, s.Label, kind, agentID, userLabel)

		// Tokens
		totalTokens := s.TotalTokens
		if totalTokens == 0 {
			totalTokens = s.InputTokens + s.OutputTokens
		}
		maxCtx := s.ContextTokens
		if maxCtx == 0 {
			maxCtx = 200000
		}
		ctxPct := 0.0
		if maxCtx > 0 && totalTokens > 0 {
			ctxPct = float64(totalTokens) / float64(maxCtx) * 100
			ctxPct = float64(int(ctxPct*10)) / 10
		}

		// Age
		ageMins := 999999.0
		if s.UpdatedAt > 0 {
			ageMins = (nowMs - float64(s.UpdatedAt)) / 60000
			ageMins = float64(int(ageMins*10)) / 10
		}
		active := ageMins < 60

		// Delta tracking for working state
		working := false
		prevVal, loaded := prevUpdatedAt.Load(k)
		if loaded {
			if prev, ok := prevVal.(int64); ok && s.UpdatedAt > prev {
				working = true
			}
		}
		prevUpdatedAt.Store(k, s.UpdatedAt)

		// Model (short form)
		model := s.Model
		if idx := strings.LastIndex(model, "/"); idx >= 0 {
			model = model[idx+1:]
		}

		byKind[kind]++
		byModel[model]++
		byAgent[agentID]++

		entries = append(entries, SessionEntry{
			Key:          truncate(k, 120),
			AgentID:      agentID,
			Kind:         kind,
			Label:        truncate(label, 60),
			Model:        model,
			TotalTokens:  totalTokens,
			InputTokens:  s.InputTokens,
			OutputTokens: s.OutputTokens,
			MaxContext:   maxCtx,
			ContextPct:   ctxPct,
			UpdatedAt:    s.UpdatedAt,
			AgeMins:      ageMins,
			Active:       active,
			Working:      working,
			UserLabel:    truncate(userLabel, 60),
			Provider:     provider,
			ChatType:     chatType,
			TelegramID:   telegramID,
		})
	}

	// Sort by updatedAt descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].UpdatedAt > entries[j].UpdatedAt
	})

	// Count active and working
	activeCount := 0
	workingCount := 0
	for _, e := range entries {
		if e.Active {
			activeCount++
		}
		if e.Working {
			workingCount++
		}
	}

	// Cap recent at 30
	recent := entries
	if len(recent) > 30 {
		recent = recent[:30]
	}

	return &SessionsData{
		Total:    len(entries),
		Active:   activeCount,
		Working:  workingCount,
		ByKind:   byKind,
		ByModel:  byModel,
		ByAgent:  byAgent,
		AgentIDs: agentIDs,
		Recent:   recent,
		TS:       int64(nowMs),
	}
}

func classifyKind(key string) string {
	if strings.HasSuffix(key, ":main") {
		return "main"
	}
	if strings.Contains(key, ":cron:") {
		return "cron"
	}
	if strings.Contains(key, ":subagent:") || strings.Contains(key, ":spawn:") {
		return "subagent"
	}
	if strings.Contains(key, ":dm:") {
		return "dm"
	}
	return "other"
}

func extractOrigin(origin interface{}, chatType string) (userLabel, provider, ct string) {
	ct = chatType
	if origin == nil {
		return
	}
	m, ok := origin.(map[string]interface{})
	if !ok {
		return
	}
	if v, ok := m["label"].(string); ok {
		userLabel = v
	}
	if v, ok := m["provider"].(string); ok {
		provider = v
	}
	if ct == "" {
		if v, ok := m["chatType"].(string); ok {
			ct = v
		}
	}
	return
}

func buildLabel(key, sessionLabel, kind, agentID, userLabel string) string {
	if sessionLabel != "" {
		return sessionLabel
	}
	if kind == "main" {
		return "Main Session"
	}
	if strings.Contains(key, ":topic:") {
		topicID := ""
		idx := strings.Index(key, ":topic:")
		if idx >= 0 {
			rest := key[idx+len(":topic:"):]
			parts := strings.SplitN(rest, ":", 2)
			topicID = parts[0]
		}
		topicName := topicNames[topicID]
		if topicName == "" {
			topicName = "Topic " + topicID
		}
		agentName := agentNames[agentID]
		if agentName == "" {
			agentName = capitalize(agentID)
		}
		return agentName + " — " + topicName
	}
	if strings.Contains(key, ":telegram:") && userLabel != "" {
		label := userLabel
		if idx := strings.Index(label, " ("); idx >= 0 {
			label = label[:idx]
		}
		return label
	}
	if kind == "subagent" {
		suffix := key
		if len(key) > 8 {
			suffix = key[len(key)-8:]
		}
		return "Sub-agent " + suffix
	}
	// Fallback: last segment
	parts := strings.Split(key, ":")
	last := parts[len(parts)-1]
	return truncate(last, 16)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
