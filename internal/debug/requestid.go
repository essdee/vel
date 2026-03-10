package debug

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// requestIDKey is a private type for the context key to avoid collisions.
type requestIDKeyType struct{}

var requestIDKey = requestIDKeyType{}

// RequestID extracts the request ID from the context.
// Returns empty string if no request ID is set.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// SetRequestID returns a new context with the given request ID.
func SetRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// generateRequestID creates a random hex string (16 bytes = 32 chars).
func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback: should never happen
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b)
}

// RequestIDMiddleware assigns a unique request ID to every request.
// Reads X-Request-ID from incoming headers; generates one if missing.
// Stores in context and sets X-Request-ID response header.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateRequestID()
		}
		ctx := SetRequestID(r.Context(), id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
