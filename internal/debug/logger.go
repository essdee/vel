package debug

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// responseRecorder wraps http.ResponseWriter to capture status code and bytes written.
type responseRecorder struct {
	http.ResponseWriter
	status       int
	bytesWritten int64
	wroteHeader  bool
}

func (rr *responseRecorder) WriteHeader(code int) {
	if !rr.wroteHeader {
		rr.status = code
		rr.wroteHeader = true
	}
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	n, err := rr.ResponseWriter.Write(b)
	rr.bytesWritten += int64(n)
	return n, err
}

// Hijack implements http.Hijacker for WebSocket support.
func (rr *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rr.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacking not supported")
}

// Flush implements http.Flusher for streaming responses.
func (rr *responseRecorder) Flush() {
	if f, ok := rr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	return r.RemoteAddr
}

// RequestLoggerMiddleware logs every HTTP request after completion.
// Smart log levels: 2xx/3xx=info, 4xx=warn, 5xx=error.
func RequestLoggerMiddleware(logger *slog.Logger, cfg DebugConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rec := &responseRecorder{
				ResponseWriter: w,
				status:         200,
			}

			next.ServeHTTP(rec, r)

			latency := time.Since(start)
			requestID := RequestID(r.Context())
			identity := identityFromContext(r.Context())

			attrs := []slog.Attr{
				slog.String("request_id", requestID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int64("latency_ms", latency.Milliseconds()),
				slog.String("client_ip", getClientIP(r)),
				slog.String("user_agent", r.UserAgent()),
				slog.Int64("bytes_written", rec.bytesWritten),
				slog.String("identity", identity),
			}

			// Determine log level based on status code
			level := slog.LevelInfo
			msg := "request"
			if rec.status >= 500 {
				level = slog.LevelError
				msg = "request error"
			} else if rec.status >= 400 {
				level = slog.LevelWarn
				msg = "request warn"
			}

			// Convert attrs to args
			args := make([]any, len(attrs))
			for i, a := range attrs {
				args[i] = a
			}
			logger.LogAttrs(r.Context(), level, msg, attrs...)

			// If AI debug mode is on, store in ring buffer
			if cfg.AIDebug && globalBuffer != nil {
				entry := RequestLog{
					RequestID:    requestID,
					Timestamp:    start,
					Method:       r.Method,
					Path:         r.URL.Path,
					Query:        r.URL.RawQuery,
					ClientIP:     getClientIP(r),
					UserAgent:    r.UserAgent(),
					Identity:     identity,
					Status:       rec.status,
					LatencyMs:    latency.Milliseconds(),
					BytesWritten: rec.bytesWritten,
				}

				// Attach middleware log from context
				if ml := GetMiddlewareLog(r.Context()); ml != nil {
					entry.MiddlewareLog = ml
				}

				// Attach handler log from context
				if hl := GetHandlerLog(r.Context()); hl != nil {
					entry.HandlerLog = hl
				}

				globalBuffer.Add(entry)
			}
		})
	}
}

// identityFromContext tries to extract user identity info from context.
// This looks for the vel_identity context key set by auth middleware.
func identityFromContext(ctx context.Context) string {
	// Try to get identity via the same context key used by auth middleware
	v := ctx.Value(identityCtxKey)
	if v == nil {
		return "anonymous"
	}
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return "anonymous"
}

// identityCtxKeyType is used to store identity string in context for logging.
type identityCtxKeyType struct{}

var identityCtxKey = identityCtxKeyType{}

// SetIdentityForLog stores an identity string in context for the request logger.
func SetIdentityForLog(ctx context.Context, identity string) context.Context {
	return context.WithValue(ctx, identityCtxKey, identity)
}
