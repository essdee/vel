package debug

import (
	"os"
	"strconv"
	"strings"
)

// DebugConfig holds all debug-related configuration.
type DebugConfig struct {
	Enabled    bool   // VEL_DEBUG=1
	AIDebug    bool   // VEL_AI_DEBUG=1 (implies Enabled)
	LogFormat  string // "json" (default) or "text"
	LogLevel   string // "debug", "info" (default), "warn", "error"
	DebugPort  int    // default 6060
	BufferSize int    // default 1000
}

// DefaultConfig returns a DebugConfig with sensible defaults.
func DefaultConfig() DebugConfig {
	return DebugConfig{
		Enabled:    false,
		AIDebug:    false,
		LogFormat:  "json",
		LogLevel:   "info",
		DebugPort:  6060,
		BufferSize: 1000,
	}
}

// LoadConfig builds a DebugConfig from a config map (from config.json "debug" section)
// and applies environment variable overrides.
func LoadConfig(cfgMap map[string]interface{}) DebugConfig {
	dc := DefaultConfig()

	// Read from config map
	if cfgMap != nil {
		if v, ok := cfgMap["enabled"].(bool); ok {
			dc.Enabled = v
		}
		if v, ok := cfgMap["ai_debug"].(bool); ok {
			dc.AIDebug = v
		}
		if v, ok := cfgMap["log_format"].(string); ok {
			dc.LogFormat = v
		}
		if v, ok := cfgMap["log_level"].(string); ok {
			dc.LogLevel = v
		}
		if v, ok := cfgMap["debug_port"].(float64); ok && v > 0 {
			dc.DebugPort = int(v)
		}
		if v, ok := cfgMap["buffer_size"].(float64); ok && v > 0 {
			dc.BufferSize = int(v)
		}
	}

	// Environment variable overrides (higher priority)
	if os.Getenv("VEL_DEBUG") == "1" {
		dc.Enabled = true
	}
	if os.Getenv("VEL_AI_DEBUG") == "1" {
		dc.AIDebug = true
	}
	if v := os.Getenv("VEL_LOG_FORMAT"); v != "" {
		dc.LogFormat = v
	}
	if v := os.Getenv("VEL_LOG_LEVEL"); v != "" {
		dc.LogLevel = strings.ToLower(v)
	}
	if v := os.Getenv("VEL_DEBUG_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			dc.DebugPort = p
		}
	}

	// AI debug implies debug
	if dc.AIDebug {
		dc.Enabled = true
	}

	return dc
}
