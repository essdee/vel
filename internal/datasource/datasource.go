package datasource

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	vel "vel/pkg/vel"
)

// StaleInfo describes why a source is stale.
type StaleInfo struct {
	Since time.Time `json:"staleSince"`
	Error string    `json:"error"`
}

// SourceState is the full state of a data source, used for API/broadcast.
type SourceState struct {
	Type       string          `json:"type"`
	Path       string          `json:"path"`
	Interval   string          `json:"interval"`
	OK         bool            `json:"ok"`
	Data       json.RawMessage `json:"data,omitempty"`
	Error      string          `json:"error,omitempty"`
	Stale      bool            `json:"stale,omitempty"`
	StaleSince *time.Time      `json:"staleSince,omitempty"`
	LastUpdate *time.Time      `json:"lastUpdate,omitempty"`
}

// SourceStatus is a lightweight status for the _sourceStatus broadcast field.
type SourceStatus struct {
	OK         bool       `json:"ok"`
	Stale      bool       `json:"stale,omitempty"`
	StaleSince *time.Time `json:"staleSince,omitempty"`
}

// FileSource represents a single file-backed data source.
type FileSource struct {
	Name     string
	Path     string // absolute, ~ already expanded
	Interval time.Duration
	AppName  string
	AppDir   string // app directory — used for test fixture path rewriting

	mu         sync.RWMutex
	lastData   json.RawMessage
	lastOK     bool
	lastErr    string
	lastGood   time.Time
	stale      bool
	staleSince time.Time

	stopCh chan struct{}
}

// Manager manages all data sources.
type Manager struct {
	sources map[string]*FileSource // namespaced: "appname:sourcename"
	mu      sync.RWMutex
}

// NewManager creates a new datasource manager.
func NewManager() *Manager {
	return &Manager{
		sources: make(map[string]*FileSource),
	}
}

// expandTilde expands ~ to home directory.
func expandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot expand ~: %w", err)
	}
	return home + path[1:], nil
}

// AddFileSource registers a file data source.
// appDir is the app's root directory, used for test fixture path rewriting.
func (m *Manager) AddFileSource(appName, appDir, name, path string, interval time.Duration) error {
	if interval < time.Second {
		return fmt.Errorf("interval must be at least 1s, got %s", interval)
	}

	expanded, err := expandTilde(path)
	if err != nil {
		return err
	}

	// Permissions check: try reading the file
	if _, err := os.ReadFile(expanded); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet — warn but allow (it may appear later)
			fmt.Printf("  ⚠ Data source %q — file %q not found (will poll until it appears)\n", appName+":"+name, expanded)
		} else if os.IsPermission(err) {
			return fmt.Errorf("cannot read %q: permission denied\n  → Check file permissions: ls -la %s", expanded, expanded)
		} else {
			return fmt.Errorf("cannot read %q: %w", expanded, err)
		}
	}

	key := appName + ":" + name

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sources[key]; exists {
		return fmt.Errorf("data source %q already registered", key)
	}

	m.sources[key] = &FileSource{
		Name:     name,
		Path:     expanded,
		Interval: interval,
		AppName:  appName,
		AppDir:   appDir,
		stopCh:   make(chan struct{}),
	}
	return nil
}

// Start begins polling all registered sources.
func (m *Manager) Start() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, src := range m.sources {
		go src.poll()
	}
}

// Stop stops all polling.
func (m *Manager) Stop() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, src := range m.sources {
		close(src.stopCh)
	}
}

func (src *FileSource) poll() {
	// Initial read
	src.readFile()

	ticker := time.NewTicker(src.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			src.readFile()
		case <-src.stopCh:
			return
		}
	}
}

// getEffectivePath returns the path to read from, redirecting to a fixture
// file when test mode is active and a fixture override exists.
func (src *FileSource) getEffectivePath() string {
	if !vel.IsTestMode() {
		return src.Path
	}
	// Check if a fixture override exists in the app's testdata directory
	fixturePath := filepath.Join(src.AppDir, "testdata", vel.FixtureName(), filepath.Base(src.Path))
	if _, err := os.Stat(fixturePath); err == nil {
		return fixturePath
	}
	// Fall back to original path
	return src.Path
}

func (src *FileSource) readFile() {
	effectivePath := src.getEffectivePath()

	var data []byte
	var err error

	// Try up to 3 times with 50ms delay on JSON parse failure
	for attempt := 0; attempt < 3; attempt++ {
		data, err = os.ReadFile(effectivePath)
		if err != nil {
			// File read error (not found, permission, etc.)
			break
		}

		// Validate JSON
		if json.Valid(data) {
			src.mu.Lock()
			src.lastData = json.RawMessage(data)
			src.lastOK = true
			src.lastErr = ""
			src.lastGood = time.Now()
			src.stale = false
			src.staleSince = time.Time{}
			src.mu.Unlock()
			return
		}

		// Invalid JSON — retry
		if attempt < 2 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	// All attempts failed
	src.mu.Lock()
	defer src.mu.Unlock()

	errMsg := "invalid JSON"
	if err != nil {
		if os.IsNotExist(err) {
			errMsg = "file not found"
		} else {
			errMsg = err.Error()
		}
	}

	if !src.stale {
		src.stale = true
		src.staleSince = time.Now()
	}
	src.lastOK = false
	src.lastErr = errMsg
}

// GetData returns the current data for a source.
func (m *Manager) GetData(key string) (json.RawMessage, bool, *StaleInfo) {
	m.mu.RLock()
	src, exists := m.sources[key]
	m.mu.RUnlock()

	if !exists {
		return nil, false, nil
	}

	src.mu.RLock()
	defer src.mu.RUnlock()

	var staleInfo *StaleInfo
	if src.stale {
		staleInfo = &StaleInfo{Since: src.staleSince, Error: src.lastErr}
	}

	return src.lastData, src.lastOK, staleInfo
}

// GetAllData returns current state of all sources.
func (m *Manager) GetAllData() map[string]*SourceState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*SourceState, len(m.sources))
	for key, src := range m.sources {
		src.mu.RLock()
		state := &SourceState{
			Type:     "file",
			Path:     src.Path,
			Interval: src.Interval.String(),
			OK:       src.lastOK,
			Data:     src.lastData,
			Error:    src.lastErr,
			Stale:    src.stale,
		}
		if src.stale {
			t := src.staleSince
			state.StaleSince = &t
		}
		if !src.lastGood.IsZero() {
			t := src.lastGood
			state.LastUpdate = &t
		}
		src.mu.RUnlock()
		result[key] = state
	}
	return result
}

// GetStatus returns lightweight status for all sources (for WS broadcast).
func (m *Manager) GetStatus() map[string]*SourceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*SourceStatus, len(m.sources))
	for key, src := range m.sources {
		src.mu.RLock()
		s := &SourceStatus{OK: src.lastOK, Stale: src.stale}
		if src.stale {
			t := src.staleSince
			s.StaleSince = &t
		}
		src.mu.RUnlock()
		result[key] = s
	}
	return result
}

// GetSourceState returns the full state for a single source (for API).
func (m *Manager) GetSourceState(key string) *SourceState {
	m.mu.RLock()
	src, exists := m.sources[key]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	src.mu.RLock()
	defer src.mu.RUnlock()

	state := &SourceState{
		Type:     "file",
		Path:     src.Path,
		Interval: src.Interval.String(),
		OK:       src.lastOK,
		Data:     src.lastData,
		Error:    src.lastErr,
		Stale:    src.stale,
	}
	if src.stale {
		t := src.staleSince
		state.StaleSince = &t
	}
	if !src.lastGood.IsZero() {
		t := src.lastGood
		state.LastUpdate = &t
	}
	return state
}
