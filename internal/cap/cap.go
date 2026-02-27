// Package cap provides runtime capability checking for vel apps.
package cap

import (
	"fmt"
	"strings"
	"sync"
)

var (
	// ErrBlacklisted is returned when a blacklisted function is called.
	ErrBlacklisted = fmt.Errorf("capability denied: blacklisted")
	// ErrNotDeclared is returned in strict mode when a capability is not declared.
	ErrNotDeclared = fmt.Errorf("capability denied: not declared in app.json")
)

// Config holds the capability configuration set at build/init time.
type Config struct {
	Mode       string            // "strict" or "bypass"
	Blacklist  map[string]bool   // "pkg.Func" → true
	AppCaps    map[string]bool   // capabilities declared by the app: "net/http" → true
	SiteBlock  map[string]bool   // additional site-level blocks
	SiteAllow  map[string]bool   // additional site-level allows
}

var (
	mu     sync.RWMutex
	config *Config
	logMu  sync.Mutex
	usageLog []LogEntry
)

// LogEntry records a capability check.
type LogEntry struct {
	Pkg    string
	Fn     string
	Action string // "allowed", "denied", "bypass"
}

// Init sets the capability configuration. Must be called before Check.
func Init(cfg *Config) {
	mu.Lock()
	defer mu.Unlock()
	config = cfg
}

// GetConfig returns the current config (for testing).
func GetConfig() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return config
}

// Check verifies that calling fn in pkg is allowed.
func Check(pkg, fn string) error {
	mu.RLock()
	cfg := config
	mu.RUnlock()

	if cfg == nil {
		// No config = allow (for non-vel binaries or testing)
		return nil
	}

	key := pkg + "." + fn

	// 1. Blacklisted? → always deny
	if cfg.Blacklist[key] || cfg.SiteBlock[key] {
		logUsage(pkg, fn, "denied")
		return fmt.Errorf("%w: %s", ErrBlacklisted, key)
	}

	// 2. App declared capability? → allow
	if cfg.AppCaps[pkg] || cfg.SiteAllow[pkg] {
		logUsage(pkg, fn, "allowed")
		return nil
	}

	// Also check parent package (e.g., "net" allows "net/http")
	for cap := range cfg.AppCaps {
		if strings.HasPrefix(pkg, cap+"/") || strings.HasPrefix(pkg, cap) {
			logUsage(pkg, fn, "allowed")
			return nil
		}
	}
	for cap := range cfg.SiteAllow {
		if strings.HasPrefix(pkg, cap+"/") || strings.HasPrefix(pkg, cap) {
			logUsage(pkg, fn, "allowed")
			return nil
		}
	}

	// 3. Mode-dependent
	switch cfg.Mode {
	case "bypass":
		logUsage(pkg, fn, "bypass")
		return nil
	default: // strict
		logUsage(pkg, fn, "denied")
		return fmt.Errorf("%w: %s.%s", ErrNotDeclared, pkg, fn)
	}
}

func logUsage(pkg, fn, action string) {
	logMu.Lock()
	defer logMu.Unlock()
	usageLog = append(usageLog, LogEntry{Pkg: pkg, Fn: fn, Action: action})
}

// GetLog returns all logged capability checks.
func GetLog() []LogEntry {
	logMu.Lock()
	defer logMu.Unlock()
	result := make([]LogEntry, len(usageLog))
	copy(result, usageLog)
	return result
}

// ResetLog clears the usage log.
func ResetLog() {
	logMu.Lock()
	defer logMu.Unlock()
	usageLog = nil
}
