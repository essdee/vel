package data

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AgentInfo struct {
	Primary           string   `json:"primary"`
	Fallbacks         []string `json:"fallbacks"`
	Subagent          *string  `json:"subagent"`
	Heartbeat         *string  `json:"heartbeat"`
	HeartbeatInterval *string  `json:"heartbeatInterval"`
	Context           string   `json:"context"`
	Channel           string   `json:"channel"`
	StreamMode        string   `json:"streamMode"`
	Name              string   `json:"name"`
}

func GetAgentInfo(workspace string) json.RawMessage {
	cfgPath := filepath.Join(workspace, "..", "openclaw.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil
	}

	var cfg map[string]interface{}
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}

	info := AgentInfo{
		Primary:   "unknown",
		Fallbacks: []string{},
		Context:   "200k",
		Channel:   "unknown",
		StreamMode: "off",
		Name:      "Agent",
	}

	if agents, ok := cfg["agents"].(map[string]interface{}); ok {
		if defaults, ok := agents["defaults"].(map[string]interface{}); ok {
			if model, ok := defaults["model"].(map[string]interface{}); ok {
				if p, ok := model["primary"].(string); ok {
					info.Primary = p
				}
				if fb, ok := model["fallbacks"].([]interface{}); ok {
					for _, f := range fb {
						if s, ok := f.(string); ok {
							info.Fallbacks = append(info.Fallbacks, s)
						}
					}
				}
			}
			if sa, ok := defaults["subagents"].(map[string]interface{}); ok {
				if m, ok := sa["model"].(string); ok {
					info.Subagent = &m
				}
			}
			if hb, ok := defaults["heartbeat"].(map[string]interface{}); ok {
				if m, ok := hb["model"].(string); ok {
					info.Heartbeat = &m
				}
				if e, ok := hb["every"].(string); ok {
					info.HeartbeatInterval = &e
				}
			}
		}
	}

	if channels, ok := cfg["channels"].(map[string]interface{}); ok {
		if tg, ok := channels["telegram"].(map[string]interface{}); ok {
			if enabled, ok := tg["enabled"].(bool); ok && enabled {
				info.Channel = "Telegram"
			}
			if sm, ok := tg["streamMode"].(string); ok {
				info.StreamMode = sm
			}
		}
	}

	if ui, ok := cfg["ui"].(map[string]interface{}); ok {
		if asst, ok := ui["assistant"].(map[string]interface{}); ok {
			if n, ok := asst["name"].(string); ok {
				info.Name = n
			}
		}
	}

	result, _ := json.Marshal(info)
	return result
}
