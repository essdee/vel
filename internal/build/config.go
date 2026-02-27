package build

import (
	"encoding/json"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SiteConfig represents vel.yaml capabilities section.
type SiteConfig struct {
	Capabilities struct {
		Block []string                       `yaml:"block"`
		Apps  map[string]SiteAppCapabilities `yaml:"apps"`
	} `yaml:"capabilities"`
}

// SiteAppCapabilities represents per-app capabilities in vel.yaml.
type SiteAppCapabilities struct {
	Allow []string `yaml:"allow"`
}

// LoadSiteConfig reads vel.yaml from rootDir if it exists.
func LoadSiteConfig(rootDir string) (*SiteConfig, error) {
	path := filepath.Join(rootDir, "vel.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SiteConfig{}, nil
		}
		return nil, err
	}
	var cfg SiteConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// DepCapabilities represents the allow_blocked field for a third-party dependency.
type DepCapabilities struct {
	AllowBlocked []string `json:"allow_blocked"`
}

// ParseDepCapabilities extracts allow_blocked for a specific dependency from app.json capabilities.
func ParseDepCapabilities(manifest *AppManifest, depPath string) *DepCapabilities {
	if manifest.Capabilities == nil {
		return nil
	}
	raw, ok := manifest.Capabilities[depPath]
	if !ok {
		return nil
	}
	var dc DepCapabilities
	if err := json.Unmarshal(raw, &dc); err != nil {
		return nil
	}
	return &dc
}
