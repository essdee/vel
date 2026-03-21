package server

import (
	"encoding/json"
	"net/http"
	"time"

	"vel/internal/data"
)

// PanelDataFunc returns the data for a panel, or nil if unavailable.
type PanelDataFunc func(cfg *Config) interface{}

// panelDataRegistry maps panel IDs to their data functions.
var panelDataRegistry = map[string]PanelDataFunc{}

// RegisterPanelData registers a data provider for a panel ID.
// Called during init or server setup — not concurrent-safe (setup-time only).
func RegisterPanelData(id string, fn PanelDataFunc) {
	panelDataRegistry[id] = fn
}

// GetPanelData returns the data for a panel ID using the registry.
func GetPanelData(id string, cfg *Config) interface{} {
	fn, ok := panelDataRegistry[id]
	if !ok {
		return nil
	}
	return fn(cfg)
}

// servePanelResult writes the result from GetPanelData to the HTTP response,
// handling json.RawMessage and []byte correctly.
func servePanelResult(w http.ResponseWriter, result interface{}) {
	if result == nil {
		writeJSON(w, nil)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch v := result.(type) {
	case json.RawMessage:
		w.Write(v)
	case []byte:
		w.Write(v)
	default:
		json.NewEncoder(w).Encode(v)
	}
}

func init() {
	RegisterPanelData("cpu", func(cfg *Config) interface{} {
		m, _ := data.GetSystemMetrics()
		if m != nil && m.CPU != nil {
			return m.CPU
		}
		return nil
	})

	RegisterPanelData("memory", func(cfg *Config) interface{} {
		m, _ := data.GetSystemMetrics()
		if m != nil && m.Memory != nil {
			return m.Memory
		}
		return nil
	})

	RegisterPanelData("disk", func(cfg *Config) interface{} {
		m, _ := data.GetSystemMetrics()
		if m != nil && m.Disk != nil {
			return m.Disk
		}
		return nil
	})

	RegisterPanelData("uptime", func(cfg *Config) interface{} {
		m, _ := data.GetSystemMetrics()
		if m != nil {
			return map[string]interface{}{"uptime": m.Uptime, "hostname": m.Hostname}
		}
		return nil
	})

	RegisterPanelData("processes", func(cfg *Config) interface{} {
		m, _ := data.GetSystemMetrics()
		if m != nil && m.Processes != nil {
			return map[string]interface{}{
				"total": m.Processes.Total, "running": m.Processes.Running,
				"sleeping": m.Processes.Sleeping, "os": m.OS,
			}
		}
		return nil
	})

	RegisterPanelData("claude-usage", func(cfg *Config) interface{} {
		return data.GetUsageData(cfg.Workspace)
	})

	RegisterPanelData("crons", func(cfg *Config) interface{} {
		return json.RawMessage(data.GetCronJobs(cfg.Workspace))
	})

	RegisterPanelData("models", func(cfg *Config) interface{} {
		return json.RawMessage(data.GetAgentInfo(cfg.Workspace))
	})

	RegisterPanelData("openclaw-status", func(cfg *Config) interface{} {
		if cached := data.GetSystemStatusCached(); cached != nil {
			return json.RawMessage(cached)
		}
		return nil
	})

	RegisterPanelData("updates", func(cfg *Config) interface{} {
		return data.GetUpdatesStatus(cfg.RootDir)
	})

	RegisterPanelData("_test", func(cfg *Config) interface{} {
		return map[string]interface{}{"message": "Hello from _test panel!", "ts": time.Now().UnixMilli()}
	})
}
