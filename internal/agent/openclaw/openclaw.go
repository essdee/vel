// Package openclaw implements the agent.SDK interface using OpenClaw's
// /hooks/agent API with webhook callback for result delivery.
package openclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"crypto/rand"
	"encoding/hex"

	"vel/pkg/agent"
)

func init() {
	agent.RegisterSDK("openclaw", func(raw json.RawMessage, callbackBaseURL string) (agent.SDK, error) {
		var cfg Config
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, fmt.Errorf("openclaw agent: parse config: %w", err)
		}
		return New(cfg, callbackBaseURL), nil
	})
}

// Config holds settings for the OpenClaw agent SDK.
type Config struct {
	GatewayURL            string `json:"gatewayUrl"`
	HooksToken            string `json:"hooksToken"`
	DefaultModel          string `json:"defaultModel"`
	DefaultTimeoutSeconds int    `json:"defaultTimeoutSeconds"`
	MaxRetries            int    `json:"maxRetries"`
}

// SDK implements agent.SDK using OpenClaw's /hooks/agent API.
type SDK struct {
	cfg         Config
	callbackURL string // base URL for callbacks, e.g. "http://localhost:3700"
	client      *http.Client
}

// New creates an OpenClaw agent SDK.
func New(cfg Config, callbackBaseURL string) *SDK {
	return &SDK{
		cfg:         cfg,
		callbackURL: callbackBaseURL,
		client:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SDK) RunTask(ctx context.Context, task string, tc agent.TaskContext, opts agent.TaskOptions) (agent.TaskResult, error) {
	taskId := generateTaskId()
	timeout := opts.TimeoutSeconds
	if timeout == 0 {
		timeout = s.cfg.DefaultTimeoutSeconds
	}
	if timeout == 0 {
		timeout = 900 // default 15 minutes
	}
	model := opts.Model
	if model == "" {
		model = s.cfg.DefaultModel
	}

	// Build the full prompt with context
	prompt := buildPrompt(task, tc, taskId, s.callbackURL)

	// Register the task channel before sending the request
	ch := agent.RegisterTask(taskId)
	defer agent.UnregisterTask(taskId)

	// POST to OpenClaw hooks
	if err := s.postHook(ctx, prompt, model, timeout, opts.Name, taskId); err != nil {
		return agent.TaskResult{}, fmt.Errorf("openclaw agent: post hook: %w", err)
	}

	slog.Info("openclaw agent: task dispatched", "taskId", taskId, "model", model, "timeout", timeout)

	// Wait for callback or timeout
	deadline := time.Duration(timeout+30) * time.Second // extra 30s buffer beyond agent timeout
	timer := time.NewTimer(deadline)
	defer timer.Stop()

	select {
	case result := <-ch:
		return result, nil
	case <-timer.C:
		return agent.TaskResult{
			Status: "timeout",
			Error:  fmt.Sprintf("no callback received within %ds", timeout+30),
		}, nil
	case <-ctx.Done():
		return agent.TaskResult{
			Status: "cancelled",
			Error:  ctx.Err().Error(),
		}, ctx.Err()
	}
}

// postHook sends the task to OpenClaw's /hooks/agent endpoint.
func (s *SDK) postHook(ctx context.Context, message, model string, timeout int, name, taskId string) error {
	payload := map[string]interface{}{
		"message":        message,
		"name":           name,
		"wakeMode":       "now",
		"deliver":        false, // don't send to chat — result goes via callback
		"timeoutSeconds": timeout,
	}
	if model != "" {
		payload["model"] = model
	}
	// Use a unique session key per task
	payload["sessionKey"] = "agent-sdk:" + taskId

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := strings.TrimRight(s.cfg.GatewayURL, "/") + "/hooks/agent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.cfg.HooksToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("hook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("hook returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// buildPrompt constructs the full agent prompt from task + context.
func buildPrompt(task string, tc agent.TaskContext, taskId, callbackBaseURL string) string {
	var b strings.Builder

	b.WriteString(task)
	b.WriteString("\n\n")

	if tc.WorkDir != "" {
		fmt.Fprintf(&b, "Working directory: %s\n\n", tc.WorkDir)
	}

	if len(tc.Files) > 0 {
		b.WriteString("## Files\n")
		for _, f := range tc.Files {
			editable := ""
			if f.Editable {
				editable = " [EDITABLE]"
			}
			fmt.Fprintf(&b, "- `%s`%s — %s\n", f.Path, editable, f.Description)
		}
		b.WriteString("\n")
	}

	if len(tc.References) > 0 {
		b.WriteString("## References\n")
		for k, v := range tc.References {
			fmt.Fprintf(&b, "- **%s**: %s\n", k, v)
		}
		b.WriteString("\n")
	}

	if len(tc.PreviousAttempts) > 0 {
		b.WriteString("## Previous Attempts\n")
		for _, a := range tc.PreviousAttempts {
			fmt.Fprintf(&b, "- Attempt %d: %s → %s", a.Attempt, a.Action, a.Result)
			if a.DiffScore > 0 {
				fmt.Fprintf(&b, " (diff: %.3f)", a.DiffScore)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Callback instruction
	callbackURL := fmt.Sprintf("%s/api/agent/callback/%s", strings.TrimRight(callbackBaseURL, "/"), taskId)
	fmt.Fprintf(&b, "## When Done\n")
	fmt.Fprintf(&b, "When you have completed the task (or if you cannot complete it), POST your result as JSON to:\n")
	fmt.Fprintf(&b, "`POST %s`\n\n", callbackURL)
	fmt.Fprintf(&b, "JSON body:\n```json\n")
	fmt.Fprintf(&b, `{"status":"completed","summary":"what you did","modifiedFiles":["file1.html"],"artifacts":{}}`)
	fmt.Fprintf(&b, "\n```\n")
	fmt.Fprintf(&b, "Status must be one of: `completed`, `failed`.\n")
	fmt.Fprintf(&b, "Use curl or equivalent HTTP tool to POST the result.\n")

	return b.String()
}

func generateTaskId() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}
