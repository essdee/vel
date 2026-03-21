package server

import "testing"

func TestIsValidJobID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		// Valid
		{"my-cron-job", true},
		{"app.daily_backup", true},
		{"task123", true},
		{"A", true},
		{"a1.b2-c3_d4", true},

		// Invalid
		{"", false},
		{"--config /etc/passwd", false},
		{"; rm -rf /", false},
		{"../../etc", false},
		{"-flag", false},
		{"_underscore", false},
		{".dotstart", false},
		{"has space", false},
		{"has\ttab", false},
		{"has\nnewline", false},
		{string(make([]byte, 65)), false}, // too long (65 null bytes)
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := isValidJobID(tt.id)
			if got != tt.want {
				t.Errorf("isValidJobID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}
