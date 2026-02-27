package panels

import "encoding/json"

type PanelManifest struct {
	ID              string           `json:"id"`
	ContractVersion string           `json:"contractVersion"`
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	Version         string           `json:"version"`
	Author          string           `json:"author"`
	Position        int              `json:"position"`
	Size            string           `json:"size"`
	RefreshMs       int              `json:"refreshMs"`
	Requires        []string         `json:"requires"`
	Capabilities    []string         `json:"capabilities"`
	DataSchema      json.RawMessage  `json:"dataSchema"`
	RateLimit       *RateLimitConfig `json:"rateLimit,omitempty"`
	Config          json.RawMessage  `json:"config,omitempty"`
}

type RateLimitConfig struct {
	WindowMs int `json:"windowMs"`
	Max      int `json:"max"`
}

type PanelInfo struct {
	Manifest *PanelManifest
	Path     string
	Source   string
}
