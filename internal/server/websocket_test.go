package server

import (
	"net/http"
	"testing"
)

func TestCheckOrigin(t *testing.T) {
	cfg := &Config{
		Port: 3700,
		PublicConfig: map[string]interface{}{
			"authUrl": "https://dashboard.example.com/auth/telegram/callback",
		},
	}

	check := checkOrigin(cfg)

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{"matching origin", "https://dashboard.example.com", true},
		{"matching origin http", "http://dashboard.example.com", true},
		{"matching origin with path", "https://dashboard.example.com/some/path", true},
		{"mismatched origin", "https://evil.example.com", false},
		{"empty origin", "", true},
		{"different domain", "https://google.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := http.NewRequest("GET", "/ws/", nil)
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			got := check(r)
			if got != tt.want {
				t.Errorf("checkOrigin(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}

func TestCheckOriginFallbackLocalhost(t *testing.T) {
	cfg := &Config{Port: 3700}
	check := checkOrigin(cfg)

	r, _ := http.NewRequest("GET", "/ws/", nil)
	r.Header.Set("Origin", "http://localhost:3700")
	if !check(r) {
		t.Error("expected localhost:3700 to be allowed when no authUrl configured")
	}

	r2, _ := http.NewRequest("GET", "/ws/", nil)
	r2.Header.Set("Origin", "http://evil.com")
	if check(r2) {
		t.Error("expected evil.com to be rejected")
	}
}
