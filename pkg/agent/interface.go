// Package agent provides a generic interface for Vel apps to spawn
// intelligent agent sessions. Agents can read files, edit code, run
// commands, and iterate autonomously.
//
// Apps use FromAppConfig to load their agent-sdk.json, then call
// RunTask to delegate work to an agent backend (OpenClaw, etc.).
package agent

import "context"

// SDK is the interface Vel apps use to call an intelligent agent.
// Implementations can use OpenClaw, direct API calls, or any other backend.
type SDK interface {
	// RunTask spawns an agent session with a task description and context.
	// The agent works autonomously and returns a result when done.
	// The context.Context controls cancellation and deadline.
	RunTask(ctx context.Context, task string, tc TaskContext, opts TaskOptions) (TaskResult, error)
}

// TaskContext provides the agent with everything it needs.
type TaskContext struct {
	// WorkDir is the directory the agent should work in.
	WorkDir string `json:"workDir"`

	// Files the agent should be aware of (paths relative to WorkDir).
	Files []ContextFile `json:"files,omitempty"`

	// References — URLs, images, or data the agent needs.
	References map[string]string `json:"references,omitempty"`

	// PreviousAttempts — if this is a retry, what was tried before.
	PreviousAttempts []AttemptSummary `json:"previousAttempts,omitempty"`
}

// ContextFile describes a file the agent should know about.
type ContextFile struct {
	Path        string `json:"path"`
	Description string `json:"description"`
	// Editable indicates the agent is expected to modify this file.
	Editable bool `json:"editable"`
}

// AttemptSummary records a previous attempt for retry context.
type AttemptSummary struct {
	Attempt   int     `json:"attempt"`
	Action    string  `json:"action"`
	Result    string  `json:"result"`
	DiffScore float64 `json:"diffScore,omitempty"`
}

// TaskOptions configures the agent session.
type TaskOptions struct {
	// Model to use (e.g., "anthropic/claude-opus-4-6").
	// Empty uses the SDK default.
	Model string `json:"model,omitempty"`

	// TimeoutSeconds — max time for the agent to work.
	// 0 uses the SDK default.
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`

	// MaxRetries — if the agent fails, how many times to retry.
	MaxRetries int `json:"maxRetries,omitempty"`

	// Name is a human-readable label for the task (used in logs/sessions).
	Name string `json:"name,omitempty"`
}

// TaskResult is what comes back when the agent is done.
type TaskResult struct {
	// Status: "completed", "failed", "timeout", "cancelled"
	Status string `json:"status"`

	// Summary — agent's description of what it did.
	Summary string `json:"summary"`

	// ModifiedFiles — list of files the agent changed.
	ModifiedFiles []string `json:"modifiedFiles,omitempty"`

	// Artifacts — any output data (diff scores, screenshots, etc.).
	Artifacts map[string]interface{} `json:"artifacts,omitempty"`

	// Error message if status is "failed" or "timeout".
	Error string `json:"error,omitempty"`

	// RuntimeSeconds is how long the agent ran.
	RuntimeSeconds float64 `json:"runtimeSeconds"`

	// TokensUsed is the number of tokens consumed (if available).
	TokensUsed int `json:"tokensUsed,omitempty"`
}
