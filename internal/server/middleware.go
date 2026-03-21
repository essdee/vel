package server

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	vel "vel/pkg/vel"
)

// recoveryMiddleware catches panics, logs them to the error log, and serves a
// styled error page (HTML for browser requests, JSON for API requests).
// It also logs 5xx responses that occur without a panic.
func recoveryMiddleware(next http.Handler, cfg *Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapped := &statusRecorder{ResponseWriter: w, status: 200}

		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				vel.LogError(vel.ErrorEntry{
					Timestamp: time.Now(),
					Path:      r.URL.Path,
					Method:    r.Method,
					Status:    500,
					Code:      "PANIC",
					Message:   fmt.Sprintf("panic: %v", err),
					Stack:     string(stack),
				})

				// If headers were already flushed we can't do much; still try to write body.
				acceptsJSON := strings.Contains(r.Header.Get("Accept"), "application/json") ||
					strings.HasPrefix(r.URL.Path, "/api/")

				if acceptsJSON {
					if !wrapped.wroteHeader {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusInternalServerError)
					}
					fmt.Fprintf(w, `{"error":"Internal Server Error","code":"PANIC"}`)
				} else {
					if !wrapped.wroteHeader {
						serve500(w, r)
					}
				}
			}
		}()

		next.ServeHTTP(wrapped, r)

		// Log 5xx responses that weren't panics (404 omitted — too noisy).
		if wrapped.status >= 500 {
			vel.LogError(vel.ErrorEntry{
				Timestamp: time.Now(),
				Path:      r.URL.Path,
				Method:    r.Method,
				Status:    wrapped.status,
				Code:      "HTTP_ERROR",
				Message:   fmt.Sprintf("HTTP %d", wrapped.status),
			})
		}
	})
}

// statusRecorder wraps http.ResponseWriter to capture the status code written
// by downstream handlers. Thread-safety is not required here because a single
// request is always handled by one goroutine.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.status = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker so that WebSocket upgrades work through the
// statusRecorder wrapper.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := r.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacking not supported by underlying ResponseWriter")
}

// Flush implements http.Flusher for streaming responses.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
