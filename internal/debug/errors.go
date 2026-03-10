package debug

import (
	"encoding/json"
	"net/http"
)

// Error code constants.
const (
	ErrTokenExpired  = "TOKEN_EXPIRED"
	ErrTokenInvalid  = "TOKEN_INVALID"
	ErrRateLimited   = "RATE_LIMITED"
	ErrUnauthorized  = "UNAUTHORIZED"
	ErrForbidden     = "FORBIDDEN"
	ErrNotFound      = "NOT_FOUND"
	ErrInternal      = "INTERNAL_ERROR"
	ErrBadRequest    = "BAD_REQUEST"
	ErrMethodNotAllowed = "METHOD_NOT_ALLOWED"
)

// ErrorResponse is the standard error response structure.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail holds the error detail fields.
type ErrorDetail struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	RequestID  string         `json:"request_id"`
	Details    map[string]any `json:"details,omitempty"`
	CausalChain []string     `json:"causal_chain,omitempty"`
}

// WriteError writes a structured JSON error response.
// When AI debug mode is on and a causal chain is available in context, it's included.
func WriteError(w http.ResponseWriter, r *http.Request, statusCode int, code string, message string, details map[string]any) {
	requestID := RequestID(r.Context())

	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:      code,
			Message:   message,
			RequestID: requestID,
			Details:   details,
		},
	}

	// Include causal chain from middleware log if AI debug mode is on
	if IsAIDebugMode() {
		if ml := GetMiddlewareLog(r.Context()); ml != nil {
			var chain []string
			chain = append(chain, "→ request: "+r.Method+" "+r.URL.RequestURI())
			for _, entry := range ml {
				line := "→ middleware[" + entry.Name + "]: " + entry.Action
				if entry.Fields != nil {
					if reason, ok := entry.Fields["reason"].(string); ok {
						line += " (" + reason + ")"
					}
				}
				chain = append(chain, line)
			}
			resp.Error.CausalChain = chain
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}
