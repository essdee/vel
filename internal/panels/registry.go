package panels

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Registry struct {
	panels map[string]*PanelInfo
}

type Report struct {
	Loaded  []LoadedPanel
	Failed  []FailedPanel
	Skipped []SkippedPanel
}

type LoadedPanel struct {
	ID      string
	Source  string
	Version string
}

type FailedPanel struct {
	ID     string
	Source string
	Errors []string
}

type SkippedPanel struct {
	ID     string
	Source string
	Reason string
}

func NewRegistry() *Registry {
	return &Registry{panels: make(map[string]*PanelInfo)}
}

func (r *Registry) Get(id string) *PanelInfo {
	return r.panels[id]
}

func (r *Registry) Set(id string, info *PanelInfo) {
	r.panels[id] = info
}

func (r *Registry) Entries() map[string]*PanelInfo {
	return r.panels
}

func DiscoverPanels(rootDir string) (*Registry, *Report) {
	registry := NewRegistry()
	report := &Report{}

	sources := []struct {
		Dir    string
		Source string
	}{
		{filepath.Join(rootDir, "core", "panels"), "core"},
		{filepath.Join(rootDir, "custom", "panels"), "custom"},
	}

	// Discover plugin panels: plugins/*/panels/*/manifest.json
	pluginsDir := filepath.Join(rootDir, "plugins")
	if pluginEntries, err := os.ReadDir(pluginsDir); err == nil {
		for _, pe := range pluginEntries {
			if !pe.IsDir() {
				continue
			}
			pluginPanelsDir := filepath.Join(pluginsDir, pe.Name(), "panels")
			if _, err := os.Stat(pluginPanelsDir); err == nil {
				sources = append(sources, struct {
					Dir    string
					Source string
				}{pluginPanelsDir, "plugin:" + pe.Name()})
			}
		}
	}

	for _, src := range sources {
		entries, err := os.ReadDir(src.Dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			panelID := entry.Name()
			panelPath := filepath.Join(src.Dir, panelID)
			manifest, errors := loadManifest(panelPath, panelID)

			if manifest == nil {
				report.Failed = append(report.Failed, FailedPanel{ID: panelID, Source: src.Source, Errors: errors})
				continue
			}

			if len(errors) > 0 && manifest.ContractVersion != "" {
				report.Failed = append(report.Failed, FailedPanel{ID: panelID, Source: src.Source, Errors: errors})
				continue
			}

			if len(errors) > 0 {
				report.Skipped = append(report.Skipped, SkippedPanel{ID: panelID, Source: src.Source, Reason: "legacy (no contractVersion)"})
			}

			registry.Set(panelID, &PanelInfo{Manifest: manifest, Path: panelPath, Source: src.Source})
			report.Loaded = append(report.Loaded, LoadedPanel{ID: panelID, Source: src.Source, Version: manifest.Version})
		}
	}

	return registry, report
}

func loadManifest(panelPath, panelID string) (*PanelManifest, []string) {
	manifestPath := filepath.Join(panelPath, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, []string{fmt.Sprintf("Panel %q — manifest.json not found\n  → Fix: Create manifest.json\n  → See: core/panels/cpu/manifest.json", panelID)}
	}

	var manifest PanelManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, []string{fmt.Sprintf("Panel %q — manifest.json is not valid JSON: %s", panelID, err.Error())}
	}

	var errors []string
	errors = append(errors, validateManifestSchema(&manifest, panelID)...)

	if manifest.ID != "" && manifest.ID != panelID {
		errors = append(errors, fmt.Sprintf("Panel %q — manifest id %q doesn't match folder name %q", panelID, manifest.ID, panelID))
	}

	if _, err := os.Stat(filepath.Join(panelPath, "ui.js")); os.IsNotExist(err) {
		errors = append(errors, fmt.Sprintf("Panel %q — ui.js not found", panelID))
	}

	return &manifest, errors
}

func validateManifestSchema(m *PanelManifest, panelID string) []string {
	var errors []string
	requiredFields := map[string]string{
		"id": m.ID, "contractVersion": m.ContractVersion, "name": m.Name,
		"description": m.Description, "version": m.Version, "author": m.Author,
	}
	for field, val := range requiredFields {
		if val == "" {
			errors = append(errors, fmt.Sprintf("Panel %q — manifest.json missing required field %q\n  → Fix: Add %q to your manifest.json\n  → See: core/panels/cpu/manifest.json", panelID, field, field))
		}
	}

	if m.ContractVersion != "" && m.ContractVersion != "1.0" {
		errors = append(errors, fmt.Sprintf("Panel %q — unsupported contractVersion %q\n  → Fix: Set \"contractVersion\": \"1.0\"", panelID, m.ContractVersion))
	}

	if m.Size != "" && m.Size != "half" && m.Size != "full" {
		errors = append(errors, fmt.Sprintf("Panel %q — field \"size\" has invalid value %q\n  → Fix: Must be one of: half, full", panelID, m.Size))
	}

	if m.RefreshMs != 0 && (m.RefreshMs < 1000 || m.RefreshMs > 300000) {
		errors = append(errors, fmt.Sprintf("Panel %q — refreshMs %d out of range [1000, 300000]", panelID, m.RefreshMs))
	}

	return errors
}

// BuildPanelList returns ordered panel list for frontend
func BuildPanelList(registry *Registry, order []string, disabled []string, testMode bool) []map[string]interface{} {
	disabledSet := make(map[string]bool)
	for _, d := range disabled {
		disabledSet[d] = true
	}

	type panelEntry struct {
		manifest *PanelManifest
		source   string
	}

	var entries []panelEntry
	for _, info := range registry.Entries() {
		if disabledSet[info.Manifest.ID] {
			continue
		}
		if !testMode && strings.HasPrefix(info.Manifest.ID, "_") {
			continue
		}
		entries = append(entries, panelEntry{manifest: info.Manifest, source: info.Source})
	}

	orderMap := make(map[string]int)
	for i, id := range order {
		orderMap[id] = i
	}

	sort.Slice(entries, func(i, j int) bool {
		oi, oki := orderMap[entries[i].manifest.ID]
		oj, okj := orderMap[entries[j].manifest.ID]
		if !oki {
			oi = 99999
		}
		if !okj {
			oj = 99999
		}
		if oi != oj {
			return oi < oj
		}
		if entries[i].manifest.Position != entries[j].manifest.Position {
			return entries[i].manifest.Position < entries[j].manifest.Position
		}
		return entries[i].manifest.ID < entries[j].manifest.ID
	})

	var result []map[string]interface{}
	for _, e := range entries {
		m := e.manifest
		item := map[string]interface{}{
			"id":              m.ID,
			"contractVersion": m.ContractVersion,
			"name":            m.Name,
			"description":     m.Description,
			"version":         m.Version,
			"author":          m.Author,
			"position":        m.Position,
			"size":            m.Size,
			"refreshMs":       m.RefreshMs,
			"requires":        m.Requires,
			"capabilities":    m.Capabilities,
			"dataSchema":      json.RawMessage(m.DataSchema),
			"_source":         e.source,
		}
		if m.RateLimit != nil {
			item["rateLimit"] = m.RateLimit
		}
		if m.Config != nil {
			item["config"] = json.RawMessage(m.Config)
		}
		result = append(result, item)
	}
	return result
}
