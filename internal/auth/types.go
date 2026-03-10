package auth

import (
	"net/http"
	"time"
)

// Provider extracts credentials from a request and authenticates them.
type Provider interface {
	// Name returns the provider's unique identifier (e.g. "telegram", "api_key").
	Name() string
	// Extract pulls credentials out of the HTTP request.
	// Returns false if this provider's credentials are not present.
	Extract(r *http.Request) (Credentials, bool)
	// Authenticate validates credentials and returns an Identity on success.
	Authenticate(creds Credentials) (*Identity, error)
}

// Credentials are provider-specific credential data (opaque to the framework).
type Credentials interface{}

// Identity is the normalized result of successful authentication.
type Identity struct {
	UserID   string            `json:"user_id"`   // canonical user ID from users.json (e.g. "karthi")
	Name     string            `json:"name"`       // display name
	Provider string            `json:"provider"`   // which provider authenticated them ("telegram", "magic_link", "api_key")
	Role     string            `json:"role"`       // "admin", "user", "viewer"
	Scopes   []string          `json:"scopes,omitempty"` // what they can access (for API keys; nil means full access for role)
	Meta     map[string]string `json:"meta,omitempty"`   // provider-specific extra data
}

// Session represents a server-side session stored in bbolt.
type Session struct {
	ID        string            `json:"id"`
	Identity  *Identity         `json:"identity"`
	CreatedAt time.Time         `json:"created_at"`
	ExpiresAt time.Time         `json:"expires_at"`
	Data      map[string]string `json:"data,omitempty"`
}

// IsExpired returns true if the session has passed its expiry time.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}
