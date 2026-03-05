package build

import (
	"encoding/json"
	"testing"
)

func TestTier1Allowed(t *testing.T) {
	for _, pkg := range []string{"fmt", "strings", "encoding/json", "crypto/sha256", "time"} {
		if !Tier1Packages[pkg] {
			t.Errorf("expected %q in Tier1Packages", pkg)
		}
	}
}

func TestBlacklistedPackages(t *testing.T) {
	for _, pkg := range []string{"syscall", "unsafe", "plugin", "reflect"} {
		if !BlacklistedPackages[pkg] {
			t.Errorf("expected %q in BlacklistedPackages", pkg)
		}
	}
	// os and os/exec are now capability-gated (tier 2), not blacklisted
	for _, pkg := range []string{"os", "os/exec"} {
		if BlacklistedPackages[pkg] {
			t.Errorf("%q should NOT be in BlacklistedPackages (now tier 2)", pkg)
		}
	}
}

func TestHasCapabilityForImport(t *testing.T) {
	manifest := &AppManifest{
		Capabilities: map[string]json.RawMessage{
			"net": json.RawMessage(`["api.example.com"]`),
		},
	}
	if !hasCapabilityForImport(manifest, "net/http") {
		t.Error("expected net/http allowed with net capability")
	}
	if hasCapabilityForImport(manifest, "database/sql") {
		t.Error("expected database/sql denied without db capability")
	}
}

func TestHasThirdPartyCapability(t *testing.T) {
	manifest := &AppManifest{
		Capabilities: map[string]json.RawMessage{
			"github.com/shirou/gopsutil/v3": json.RawMessage(`{}`),
		},
	}
	if !hasThirdPartyCapability(manifest, "github.com/shirou/gopsutil/v3/cpu") {
		t.Error("expected gopsutil subpackage allowed")
	}
	if hasThirdPartyCapability(manifest, "github.com/other/pkg") {
		t.Error("expected other package denied")
	}
}

func TestNilCapabilities(t *testing.T) {
	manifest := &AppManifest{}
	if hasCapabilityForImport(manifest, "net/http") {
		t.Error("nil capabilities should deny everything")
	}
	if hasThirdPartyCapability(manifest, "github.com/foo/bar") {
		t.Error("nil capabilities should deny everything")
	}
}
