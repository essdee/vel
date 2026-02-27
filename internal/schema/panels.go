package schema

import "fmt"

// ValidateManifestField provides Elm-quality error messages for panel manifests
func FormatError(panelID, field, issue, fix string) string {
	return fmt.Sprintf("Panel %q — %s\n  → Fix: %s\n  → See: core/panels/cpu/manifest.json for reference", panelID, issue, fix)
}
