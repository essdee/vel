package schema

import (
	"strings"
	"testing"
)

func TestFormatError(t *testing.T) {
	t.Run("contains panel id", func(t *testing.T) {
		result := FormatError("cpu", "name", "missing field", "add name")
		if !strings.Contains(result, `"cpu"`) {
			t.Fatalf("expected panel id in error, got: %s", result)
		}
	})

	t.Run("contains fix", func(t *testing.T) {
		result := FormatError("memory", "size", "invalid value", "use half or full")
		if !strings.Contains(result, "Fix: use half or full") {
			t.Fatalf("expected fix in error, got: %s", result)
		}
	})

	t.Run("contains reference", func(t *testing.T) {
		result := FormatError("disk", "version", "missing", "add version")
		if !strings.Contains(result, "core/panels/cpu/manifest.json") {
			t.Fatalf("expected reference path, got: %s", result)
		}
	})

	t.Run("different panels", func(t *testing.T) {
		r1 := FormatError("a", "f", "i", "x")
		r2 := FormatError("b", "f", "i", "x")
		if r1 == r2 {
			t.Fatal("different panel ids should produce different errors")
		}
	})
}
