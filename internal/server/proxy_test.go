package server

import "testing"

func TestIsPrivateTarget(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"http://localhost:3800", true},
		{"http://127.0.0.1:8080", true},
		{"http://10.0.0.5:3000", true},
		{"https://localhost:443", true},
		{"http://google.com", false},
		{"ftp://localhost", false},
		{"not-a-url", false},
		{"http://", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := isPrivateTarget(tt.url)
			if got != tt.want {
				t.Errorf("isPrivateTarget(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
