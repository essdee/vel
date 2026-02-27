package data

import (
	"testing"
)

func TestGetSystemMetrics(t *testing.T) {
	metrics, err := GetSystemMetrics()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metrics.CPU == nil {
		t.Error("expected CPU metrics")
	}
	if metrics.Memory == nil {
		t.Error("expected memory metrics")
	}
	if metrics.Disk == nil {
		t.Error("expected disk metrics")
	}
	if metrics.Hostname == "" {
		t.Error("expected hostname")
	}
	if metrics.CPU.Cores < 1 {
		t.Errorf("expected at least 1 core, got %d", metrics.CPU.Cores)
	}
}
