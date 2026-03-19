package openclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vel/pkg/agent"
)

func TestBuildPrompt(t *testing.T) {
	tc := agent.TaskContext{
		WorkDir: "/tmp/test",
		Files: []agent.ContextFile{
			{Path: "index.html", Description: "Main page", Editable: true},
		},
		References: map[string]string{
			"diff": "/tmp/test/diff.png",
		},
	}

	prompt := buildPrompt("Fix the page", tc, "task-abc", "http://localhost:3700")

	if !strings.Contains(prompt, "Fix the page") {
		t.Fatal("missing task")
	}
	if !strings.Contains(prompt, "/tmp/test") {
		t.Fatal("missing workdir")
	}
	if !strings.Contains(prompt, "index.html") {
		t.Fatal("missing file")
	}
	if !strings.Contains(prompt, "[EDITABLE]") {
		t.Fatal("missing editable marker")
	}
	if !strings.Contains(prompt, "http://localhost:3700/api/agent/callback/task-abc") {
		t.Fatal("missing callback URL")
	}
}

func TestPostHookIntegration(t *testing.T) {
	// Mock OpenClaw hooks endpoint
	received := make(chan map[string]interface{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hooks/agent" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		received <- payload
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sdk := New(Config{
		GatewayURL:            server.URL,
		HooksToken:            "test-token",
		DefaultModel:          "test-model",
		DefaultTimeoutSeconds: 5,
	}, "http://localhost:3700")

	// Run task in background — it will block waiting for callback
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go sdk.RunTask(ctx, "test task", agent.TaskContext{WorkDir: "/tmp"}, agent.TaskOptions{Name: "test"})

	// Verify the hook was called
	select {
	case payload := <-received:
		msg, ok := payload["message"].(string)
		if !ok || !strings.Contains(msg, "test task") {
			t.Fatalf("expected task in message, got %v", payload["message"])
		}
		if payload["deliver"] != false {
			t.Fatal("expected deliver=false")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for hook call")
	}
}
