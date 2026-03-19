# Agent SDK

Vel's Agent SDK lets apps delegate tasks to AI agents (OpenClaw, Anthropic direct, custom backends). An app describes *what* it needs done; the SDK handles *how* — dispatching to the configured backend, collecting results via callback, and handling timeouts/retries.

## Architecture

```
pkg/agent/                          ← Public interface (import this)
├── interface.go                    ← SDK interface, types (TaskContext, TaskResult, etc.)
├── config.go                       ← Load agent-sdk.json, factory registry, env resolution
├── callback.go                     ← Shared callback HTTP handler (/api/agent/callback/)
└── agent_test.go                   ← Tests for config loading, env resolution

internal/agent/openclaw/            ← OpenClaw adapter (built-in)
├── openclaw.go                     ← Implements SDK via OpenClaw /hooks/agent
└── openclaw_test.go

sdk/openclaw/                       ← Operational scripts (NOT Go code)
├── restart.sh
└── claude-usage-poll.sh
```

**Key principle:** Go implementations live in `internal/agent/{platform}/`. Shell scripts and operational tooling live in `sdk/{platform}/`. The public interface in `pkg/agent/` is what apps import.

## Quick Start

### 1. Create `agent-sdk.json` in your app directory

```json
{
  "sdk": "openclaw",
  "openclaw": {
    "gatewayUrl": "{{env:OPENCLAW_GATEWAY_URL}}",
    "hooksToken": "{{env:OPENCLAW_HOOKS_TOKEN}}",
    "defaultModel": "anthropic/claude-sonnet-4-6",
    "defaultTimeoutSeconds": 900,
    "maxRetries": 1
  }
}
```

Environment variables are resolved at load time using `{{env:VAR}}` syntax.

### 2. Use the SDK in your Go handler

```go
import "vel/pkg/agent"

func handleSomeTask(w http.ResponseWriter, r *http.Request) {
    sdk, err := agent.FromAppConfig("/path/to/your/app")
    if err != nil {
        // handle error
    }

    result, err := sdk.RunTask(r.Context(), "Redesign the login page", agent.TaskContext{
        WorkDir: "/opt/vel/apps/myapp",
        Files: []agent.ContextFile{
            {Path: "panels/login/login.html", Description: "Current login page", Editable: true},
        },
    }, agent.TaskOptions{
        Name:    "login-redesign",
        Model:   "anthropic/claude-opus-4-6",  // optional override
        TimeoutSeconds: 600,
    })

    // result.Status is "completed", "failed", "timeout", or "cancelled"
    // result.Summary describes what the agent did
    // result.ModifiedFiles lists changed files
}
```

## Interface

### `agent.SDK`

```go
type SDK interface {
    RunTask(ctx context.Context, task string, tc TaskContext, opts TaskOptions) (TaskResult, error)
}
```

### `TaskContext`

| Field              | Type                    | Description                              |
| ------------------ | ----------------------- | ---------------------------------------- |
| `WorkDir`          | `string`                | Directory the agent works in             |
| `Files`            | `[]ContextFile`         | Files the agent should know about        |
| `References`       | `map[string]string`     | URLs, images, or data for context        |
| `PreviousAttempts` | `[]AttemptSummary`      | Retry context from earlier attempts      |

### `TaskOptions`

| Field            | Type     | Description                                  |
| ---------------- | -------- | -------------------------------------------- |
| `Model`          | `string` | Model override (empty = SDK default)         |
| `TimeoutSeconds` | `int`    | Max agent runtime (0 = SDK default, usually 900s) |
| `MaxRetries`     | `int`    | Retry count on failure                       |
| `Name`           | `string` | Human-readable label for logs/sessions       |

### `TaskResult`

| Field            | Type                     | Description                          |
| ---------------- | ------------------------ | ------------------------------------ |
| `Status`         | `string`                 | `completed`, `failed`, `timeout`, `cancelled` |
| `Summary`        | `string`                 | What the agent did                   |
| `ModifiedFiles`  | `[]string`               | Files changed by the agent           |
| `Artifacts`      | `map[string]interface{}` | Extra output (scores, screenshots)   |
| `Error`          | `string`                 | Error message if not completed       |
| `RuntimeSeconds` | `float64`                | Wall-clock time used                 |
| `TokensUsed`     | `int`                    | Token count if available             |

## Callback Flow

The Agent SDK uses a callback pattern — not polling:

1. App calls `sdk.RunTask()` → blocks on a channel
2. SDK sends the task to the agent backend (e.g., OpenClaw `/hooks/agent`)
3. The prompt includes a callback URL: `POST /api/agent/callback/{taskId}`
4. Agent works autonomously, then POSTs its result to the callback URL
5. Vel's callback handler receives the result, delivers it to the waiting channel
6. `RunTask()` unblocks and returns the `TaskResult`

The callback handler is registered on the Vel server mux automatically. Each task gets a unique 24-character hex ID.

## Supported Backends

### OpenClaw (`"sdk": "openclaw"`)

Ships built-in. Uses OpenClaw's `/hooks/agent` API to spawn isolated agent sessions.

**Config (`agent-sdk.json`):**

```json
{
  "sdk": "openclaw",
  "openclaw": {
    "gatewayUrl": "http://localhost:3000",
    "hooksToken": "your-hooks-token",
    "defaultModel": "anthropic/claude-sonnet-4-6",
    "defaultTimeoutSeconds": 900,
    "maxRetries": 1
  }
}
```

| Field                   | Required | Description                            |
| ----------------------- | -------- | -------------------------------------- |
| `gatewayUrl`            | Yes      | OpenClaw gateway URL                   |
| `hooksToken`            | Yes      | Bearer token for /hooks/agent          |
| `defaultModel`          | No       | Default model for tasks                |
| `defaultTimeoutSeconds` | No       | Default timeout (default: 900)         |
| `maxRetries`            | No       | Default retry count                    |

## Adding a New Backend

To add support for a new agent platform (e.g., direct Anthropic API, OpenAI Assistants, custom):

### 1. Create the adapter package

```
internal/agent/anthropic/
├── anthropic.go      ← implements agent.SDK
└── anthropic_test.go
```

### 2. Implement the interface

```go
package anthropic

import (
    "vel/pkg/agent"
    "encoding/json"
)

func init() {
    agent.RegisterSDK("anthropic", func(raw json.RawMessage, callbackBaseURL string) (agent.SDK, error) {
        var cfg Config
        if err := json.Unmarshal(raw, &cfg); err != nil {
            return nil, err
        }
        return New(cfg, callbackBaseURL), nil
    })
}

type Config struct {
    APIKey string `json:"apiKey"`
    Model  string `json:"model"`
}

type SDK struct {
    cfg         Config
    callbackURL string
}

func New(cfg Config, callbackBaseURL string) *SDK {
    return &SDK{cfg: cfg, callbackURL: callbackBaseURL}
}

func (s *SDK) RunTask(ctx context.Context, task string, tc agent.TaskContext, opts agent.TaskOptions) (agent.TaskResult, error) {
    taskId := generateTaskId()
    ch := agent.RegisterTask(taskId)
    defer agent.UnregisterTask(taskId)

    // ... send task to your backend, including callback URL ...
    // ... wait on ch for result ...
}
```

### 3. Import it in server.go

```go
import _ "vel/internal/agent/anthropic"
```

The `init()` function registers the factory. Apps can then use `"sdk": "anthropic"` in their `agent-sdk.json`.

### 4. Write the config block

```json
{
  "sdk": "anthropic",
  "anthropic": {
    "apiKey": "{{env:ANTHROPIC_API_KEY}}",
    "model": "claude-sonnet-4-6"
  }
}
```

The config key must match the SDK name passed to `RegisterSDK`.

## Environment Variable Resolution

All `{{env:VAR_NAME}}` patterns in `agent-sdk.json` are resolved at load time. If a variable is not set, the placeholder is left as-is (useful for detecting misconfiguration).

## Testing

```bash
# Run all agent SDK tests
go test ./pkg/agent/ ./internal/agent/openclaw/ -v

# Run just the interface/config tests
go test ./pkg/agent/ -v
```

Tests cover: config unmarshaling, env resolution, callback delivery, prompt building, and integration with the factory registry.
