package vel

import (
	"encoding/json"
	"net/http"
	"time"
)

// Error writes a standardized JSON error response and logs the error.
// Response body: {"error": message, "code": code, "hint": hint}
func Error(w http.ResponseWriter, status int, code string, message string, hint string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": message,
		"code":  code,
		"hint":  hint,
	})

	LogError(ErrorEntry{
		Timestamp: time.Now(),
		Status:    status,
		Code:      code,
		Message:   message,
		Hint:      hint,
	})
}

// JSON writes v as a JSON response with Content-Type: application/json.
func JSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
