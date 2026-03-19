package agent

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
)

// pendingTasks maps taskId → channel that RunTask is blocking on.
var (
	pendingMu    sync.RWMutex
	pendingTasks = make(map[string]chan TaskResult)
)

// RegisterTask creates a channel for the given taskId and stores it.
// Called by SDK implementations before dispatching a task.
// Returns the channel to block on.
func RegisterTask(taskId string) chan TaskResult {
	ch := make(chan TaskResult, 1)
	pendingMu.Lock()
	pendingTasks[taskId] = ch
	pendingMu.Unlock()
	return ch
}

// UnregisterTask removes a task from the pending map.
// Called by SDK implementations when RunTask completes (via defer).
func UnregisterTask(taskId string) {
	pendingMu.Lock()
	delete(pendingTasks, taskId)
	pendingMu.Unlock()
}

// CallbackHandler returns an http.HandlerFunc that receives agent results.
// Register this on the Vel server mux at /api/agent/callback/
func CallbackHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract taskId from path: /api/agent/callback/{taskId}
		path := strings.TrimPrefix(r.URL.Path, "/api/agent/callback/")
		taskId := strings.TrimSuffix(path, "/")
		if taskId == "" {
			http.Error(w, "missing taskId", http.StatusBadRequest)
			return
		}

		// Read body
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB max
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}

		// Parse result
		var result TaskResult
		if err := json.Unmarshal(body, &result); err != nil {
			// If the agent sent plain text instead of JSON, wrap it
			result = TaskResult{
				Status:  "completed",
				Summary: string(body),
			}
		}

		// Deliver to waiting RunTask
		pendingMu.RLock()
		ch, ok := pendingTasks[taskId]
		pendingMu.RUnlock()

		if !ok {
			slog.Warn("agent callback: unknown taskId", "taskId", taskId)
			http.Error(w, "unknown taskId", http.StatusNotFound)
			return
		}

		// Non-blocking send (channel is buffered with size 1)
		select {
		case ch <- result:
			slog.Info("agent callback: delivered result", "taskId", taskId, "status", result.Status)
		default:
			slog.Warn("agent callback: duplicate result ignored", "taskId", taskId)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}
}
