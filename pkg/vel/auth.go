package vel

import (
	"net/http"
	"vel/internal/auth"
)

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

const identityContextKey contextKey = "vel_identity"

// User represents an authenticated user.
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username,omitempty"`
}

// Check validates the request and returns the authenticated user, or nil.
// Checks new session-based Identity (from middleware context) first,
// then falls back to legacy auth.
func Check(r *http.Request) *User {
	// Try new session-based Identity from middleware context first
	if id := GetIdentity(r); id != nil {
		return &User{ID: 0, FirstName: id.Name, Username: id.UserID}
	}
	// Fall back to legacy auth
	u := auth.Check(r)
	if u == nil {
		return nil
	}
	return &User{ID: u.ID, FirstName: u.FirstName, Username: u.Username}
}

// GetIdentity returns the new-style auth Identity from the request context.
// Returns nil if no Identity is set (unauthenticated or legacy auth only).
func GetIdentity(r *http.Request) *auth.Identity {
	v := r.Context().Value(identityContextKey)
	if v == nil {
		return nil
	}
	id, ok := v.(*auth.Identity)
	if !ok {
		return nil
	}
	return id
}

// IsAdmin checks if the request is authenticated with admin role.
func IsAdmin(r *http.Request) bool {
	if id := GetIdentity(r); id != nil {
		return id.Role == "admin"
	}
	// Legacy fallback: non-scoped users are admins
	u := auth.Check(r)
	return u != nil && !auth.IsScopedUser(u)
}

// IsAllowed checks if a Telegram user ID is in the allowed list.
func IsAllowed(id int64) bool {
	return auth.IsAllowed(id)
}

// CheckBotToken validates a token against the configured bot token.
func CheckBotToken(token string) bool {
	return auth.CheckBotToken(token)
}

// IsScopedUser returns true if the user was authenticated via a scoped token.
func IsScopedUser(u *User) bool {
	return u != nil && u.Username != "" && len(u.Username) > 7 && u.Username[:7] == "scoped:"
}

// GetBotToken returns the configured bot token.
func GetBotToken() string {
	return auth.GetBotToken()
}
