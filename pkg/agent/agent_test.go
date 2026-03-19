package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCallbackHandler(t *testing.T) {
	taskId := "test-task-123"
	ch := RegisterTask(taskId)
	defer UnregisterTask(taskId)

	handler := CallbackHandler()

	result := TaskResult{
		Status:        "completed",
		Summary:       "Fixed 3 pages",
		ModifiedFiles: []string{"about.html", "gallery.html"},
	}
	body, _ := json.Marshal(result)

	req := httptest.NewRequest(http.MethodPost, "/api/agent/callback/"+taskId, strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	select {
	case got := <-ch:
		if got.Status != "completed" {
			t.Fatalf("expected completed, got %s", got.Status)
		}
		if got.Summary != "Fixed 3 pages" {
			t.Fatalf("expected 'Fixed 3 pages', got %s", got.Summary)
		}
		if len(got.ModifiedFiles) != 2 {
			t.Fatalf("expected 2 modified files, got %d", len(got.ModifiedFiles))
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for callback result")
	}
}

func TestCallbackHandler_UnknownTaskId(t *testing.T) {
	handler := CallbackHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/agent/callback/unknown-id", strings.NewReader(`{"status":"completed"}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown taskId, got %d", w.Code)
	}
}

func TestCallbackHandler_PlainText(t *testing.T) {
	taskId := "test-plain"
	ch := RegisterTask(taskId)
	defer UnregisterTask(taskId)

	handler := CallbackHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/agent/callback/"+taskId, strings.NewReader("I fixed everything"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	select {
	case got := <-ch:
		if got.Status != "completed" {
			t.Fatalf("expected completed, got %s", got.Status)
		}
		if got.Summary != "I fixed everything" {
			t.Fatalf("expected plain text summary, got %s", got.Summary)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestResolveEnvInString(t *testing.T) {
	t.Setenv("TEST_TOKEN_123", "my-secret")

	tests := []struct {
		input    string
		expected string
	}{
		{"{{env:TEST_TOKEN_123}}", "my-secret"},
		{"plain text", "plain text"},
		{"prefix {{env:TEST_TOKEN_123}} suffix", "prefix my-secret suffix"},
		{"{{env:NONEXISTENT_VAR_XYZ}}", "{{env:NONEXISTENT_VAR_XYZ}}"}, // unresolved
	}

	for _, tt := range tests {
		got := resolveEnvInString(tt.input)
		if got != tt.expected {
			t.Errorf("resolveEnvInString(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestConfigUnmarshalJSON(t *testing.T) {
	raw := `{"sdk":"openclaw","openclaw":{"gatewayUrl":"http://localhost:18789","hooksToken":"secret"}}`
	var cfg Config
	if err := cfg.UnmarshalJSON([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	if cfg.SDKName != "openclaw" {
		t.Fatalf("expected openclaw, got %s", cfg.SDKName)
	}
	if _, ok := cfg.Rest["openclaw"]; !ok {
		t.Fatal("expected openclaw block in Rest")
	}
}
