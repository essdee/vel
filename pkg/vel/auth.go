package vel

import (
	"net/http"
	"vel/internal/auth"
)

// User represents an authenticated user.
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username,omitempty"`
}

// Check validates the request and returns the authenticated user, or nil.
func Check(r *http.Request) *User {
	u := auth.Check(r)
	if u == nil {
		return nil
	}
	return &User{ID: u.ID, FirstName: u.FirstName, Username: u.Username}
}

// IsAllowed checks if a Telegram user ID is in the allowed list.
func IsAllowed(id int64) bool {
	return auth.IsAllowed(id)
}

// CheckBotToken validates a token against the configured bot token.
func CheckBotToken(token string) bool {
	return auth.CheckBotToken(token)
}

// GetBotToken returns the configured bot token.
func GetBotToken() string {
	return auth.GetBotToken()
}
