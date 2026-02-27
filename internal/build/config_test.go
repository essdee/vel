package build

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSiteConfigMissing(t *testing.T) {
	cfg, err := LoadSiteConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestLoadSiteConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := `capabilities:
  block:
    - "log.Fatal"
    - "net/http.ListenAndServe"
  apps:
    inventory:
      allow:
        - "database/sql"
`
	if err := os.WriteFile(filepath.Join(dir, "vel.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadSiteConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Capabilities.Block) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(cfg.Capabilities.Block))
	}
	if cfg.Capabilities.Block[0] != "log.Fatal" {
		t.Errorf("expected log.Fatal, got %s", cfg.Capabilities.Block[0])
	}

	appCfg, ok := cfg.Capabilities.Apps["inventory"]
	if !ok {
		t.Fatal("expected inventory app config")
	}
	if len(appCfg.Allow) != 1 || appCfg.Allow[0] != "database/sql" {
		t.Errorf("unexpected allow: %v", appCfg.Allow)
	}
}

func TestParseDepCapabilities(t *testing.T) {
	manifest := &AppManifest{
		Capabilities: map[string]json.RawMessage{
			"github.com/shirou/gopsutil/v3": json.RawMessage(`{"allow_blocked": ["os/exec.Command"]}`),
		},
	}

	dc := ParseDepCapabilities(manifest, "github.com/shirou/gopsutil/v3")
	if dc == nil {
		t.Fatal("expected non-nil dep capabilities")
	}
	if len(dc.AllowBlocked) != 1 || dc.AllowBlocked[0] != "os/exec.Command" {
		t.Errorf("unexpected allow_blocked: %v", dc.AllowBlocked)
	}

	// Non-existent dep
	dc2 := ParseDepCapabilities(manifest, "github.com/other/pkg")
	if dc2 != nil {
		t.Error("expected nil for non-existent dep")
	}
}
