package debug

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// RecoveryMiddleware catches panics from downstream handlers, logs the error
// with a stack trace, and returns a structured JSON 500 response.
func RecoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					stack := string(debug.Stack())
					requestID := RequestID(r.Context())

					logger.Error("panic recovered",
						slog.String("request_id", requestID),
						slog.String("panic", fmt.Sprintf("%v", err)),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.String("stack", stack),
					)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"error": map[string]interface{}{
							"code":       ErrInternal,
							"message":    "An internal error occurred",
							"request_id": requestID,
						},
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
