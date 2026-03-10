package debug

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// debugEnabled and aiDebugEnabled are atomically-read flags for fast checks.
var (
	debugEnabled   atomic.Bool
	aiDebugEnabled atomic.Bool
	globalLogger   *slog.Logger
)

// Init sets the global debug state. Call once from main.
func Init(cfg DebugConfig, logger *slog.Logger) {
	debugEnabled.Store(cfg.Enabled)
	aiDebugEnabled.Store(cfg.AIDebug)
	globalLogger = logger

	if cfg.AIDebug && cfg.BufferSize > 0 {
		globalBuffer = NewRingBuffer(cfg.BufferSize)
	}
}

// IsDebugMode returns true if debug mode is enabled.
func IsDebugMode() bool {
	return debugEnabled.Load()
}

// IsAIDebugMode returns true if AI debug mode is enabled.
func IsAIDebugMode() bool {
	return aiDebugEnabled.Load()
}

// Logger returns the global debug logger.
func Logger() *slog.Logger {
	if globalLogger != nil {
		return globalLogger
	}
	return slog.Default()
}

// middlewareLogKeyType is the context key for middleware log entries.
type middlewareLogKeyType struct{}

var middlewareLogKey = middlewareLogKeyType{}

// handlerLogKeyType is the context key for handler log entry.
type handlerLogKeyType struct{}

var handlerLogKey = handlerLogKeyType{}

// MiddlewareEntry records what a middleware did for a request.
type MiddlewareEntry struct {
	Name   string         `json:"name"`
	Action string         `json:"action"`
	Fields map[string]any `json:"fields,omitempty"`
}

// HandlerEntry records what a handler did for a request.
type HandlerEntry struct {
	Name  string `json:"name"`
	Error string `json:"error,omitempty"`
	Note  string `json:"note,omitempty"`
}

// DebugLog logs a debug-level middleware action. No-op when debug mode is off.
// Also appends to the middleware log in context (for AI debug mode ring buffer).
func DebugLog(ctx context.Context, middleware string, action string, fields ...any) {
	if !debugEnabled.Load() {
		return
	}

	requestID := RequestID(ctx)

	// Build slog args
	args := []any{
		slog.String("request_id", requestID),
		slog.String("middleware", middleware),
		slog.String("action", action),
	}
	args = append(args, fields...)

	if globalLogger != nil {
		globalLogger.Debug("middleware", args...)
	}
}

// AppendMiddlewareLog adds a middleware entry to the context's middleware log.
// Returns the updated context. Used when AI debug is on.
func AppendMiddlewareLog(ctx context.Context, name, action string, fields map[string]any) context.Context {
	if !aiDebugEnabled.Load() {
		return ctx
	}
	entries := GetMiddlewareLog(ctx)
	entries = append(entries, MiddlewareEntry{
		Name:   name,
		Action: action,
		Fields: fields,
	})
	return context.WithValue(ctx, middlewareLogKey, entries)
}

// GetMiddlewareLog retrieves the middleware log from context.
func GetMiddlewareLog(ctx context.Context) []MiddlewareEntry {
	if v, ok := ctx.Value(middlewareLogKey).([]MiddlewareEntry); ok {
		return v
	}
	return nil
}

// SetHandlerLog stores a handler log entry in context.
func SetHandlerLog(ctx context.Context, entry *HandlerEntry) context.Context {
	return context.WithValue(ctx, handlerLogKey, entry)
}

// GetHandlerLog retrieves the handler log from context.
func GetHandlerLog(ctx context.Context) *HandlerEntry {
	if v, ok := ctx.Value(handlerLogKey).(*HandlerEntry); ok {
		return v
	}
	return nil
}
