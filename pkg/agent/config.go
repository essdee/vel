package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SDKFactory creates an SDK implementation given a raw JSON config block
// and the callback base URL. Registered by each sdk/ package at init time.
type SDKFactory func(raw json.RawMessage, callbackBaseURL string) (SDK, error)

var (
	factories = make(map[string]SDKFactory)
)

// RegisterSDK registers a named SDK factory. Called by sdk/ packages in init().
func RegisterSDK(name string, factory SDKFactory) {
	factories[name] = factory
}

// Config is the top-level agent-sdk.json structure.
// The SDK-specific config is stored as raw JSON keyed by the sdk name.
type Config struct {
	SDKName string                     `json:"sdk"`
	Rest    map[string]json.RawMessage `json:"-"`
}

// UnmarshalJSON custom unmarshals to capture the sdk name and preserve
// the rest as raw JSON for the specific SDK factory.
func (c *Config) UnmarshalJSON(data []byte) error {
	// First pass: get the sdk name
	var peek struct {
		SDK string `json:"sdk"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return err
	}
	c.SDKName = peek.SDK

	// Second pass: get all top-level keys as raw JSON
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}
	delete(all, "sdk")
	c.Rest = all
	return nil
}

// CallbackConfig holds the Vel server's callback endpoint info.
type CallbackConfig struct {
	BaseURL string // e.g., "http://localhost:3700"
}

var globalCallback *CallbackConfig

// RegisterCallback sets the callback URL base. Called by the Vel server at startup.
func RegisterCallback(baseURL string) {
	globalCallback = &CallbackConfig{BaseURL: baseURL}
}

// FromAppConfig loads agent-sdk.json from the given app directory and returns
// the configured SDK implementation.
func FromAppConfig(appDir string) (SDK, error) {
	cfgPath := filepath.Join(appDir, "agent-sdk.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("agent: read config %s: %w", cfgPath, err)
	}

	// Resolve env vars in the raw JSON before parsing
	data = []byte(resolveEnvInString(string(data)))

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("agent: parse config %s: %w", cfgPath, err)
	}

	factory, ok := factories[cfg.SDKName]
	if !ok {
		known := make([]string, 0, len(factories))
		for k := range factories {
			known = append(known, k)
		}
		return nil, fmt.Errorf("agent: unknown sdk %q (registered: %v)", cfg.SDKName, known)
	}

	if globalCallback == nil {
		return nil, fmt.Errorf("agent: callback not registered — call agent.RegisterCallback at server startup")
	}

	// Pass the SDK-specific config block to the factory
	sdkConfig, ok := cfg.Rest[cfg.SDKName]
	if !ok {
		return nil, fmt.Errorf("agent: config missing %q block", cfg.SDKName)
	}

	return factory(sdkConfig, globalCallback.BaseURL)
}

// resolveEnvInString replaces all {{env:VAR}} occurrences in a string.
func resolveEnvInString(s string) string {
	var result strings.Builder
	for {
		start := strings.Index(s, "{{env:")
		if start == -1 {
			result.WriteString(s)
			return result.String()
		}
		end := strings.Index(s[start:], "}}")
		if end == -1 {
			result.WriteString(s)
			return result.String()
		}
		end += start + 2
		key := s[start+6 : end-2]
		result.WriteString(s[:start])
		if val := os.Getenv(key); val != "" {
			result.WriteString(val)
		} else {
			result.WriteString(s[start:end]) // leave unresolved
		}
		s = s[end:]
	}
}
