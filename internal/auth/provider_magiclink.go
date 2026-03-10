package auth

import (
	"fmt"
	"net/http"
	"strings"
)

// MagicLinkCredentials holds the raw token extracted from the request.
type MagicLinkCredentials struct {
	Token string
}

// MagicLinkProvider authenticates requests using magic link tokens.
// This provider creates sessions (not stateless).
type MagicLinkProvider struct {
	linkStore *MagicLinkStore
	userStore *UserStore
}

// NewMagicLinkProvider creates a Magic Link auth provider.
func NewMagicLinkProvider(linkStore *MagicLinkStore, userStore *UserStore) *MagicLinkProvider {
	return &MagicLinkProvider{
		linkStore: linkStore,
		userStore: userStore,
	}
}

func (p *MagicLinkProvider) Name() string { return "magic_link" }

// Extract checks for ml_token query parameter on requests to /auth/magic.
func (p *MagicLinkProvider) Extract(r *http.Request) (Credentials, bool) {
	// Only extract on /auth/magic path
	if !strings.HasPrefix(r.URL.Path, "/auth/magic") {
		return nil, false
	}

	token := r.URL.Query().Get("ml_token")
	if token == "" || !strings.HasPrefix(token, "vel_ml_") {
		return nil, false
	}

	return MagicLinkCredentials{Token: token}, true
}

// Authenticate validates the magic link token and returns an Identity.
func (p *MagicLinkProvider) Authenticate(creds Credentials) (*Identity, error) {
	mlc, ok := creds.(MagicLinkCredentials)
	if !ok {
		return nil, fmt.Errorf("invalid credentials type for magic_link provider")
	}

	// Validate token (marks it as used)
	userID, err := p.linkStore.Validate(mlc.Token)
	if err != nil {
		return nil, fmt.Errorf("magic link validation failed: %w", err)
	}

	// Look up user
	userData := p.userStore.GetData()
	if userData == nil {
		return nil, fmt.Errorf("user store not available")
	}

	for _, u := range userData.Users {
		if u.ID == userID {
			return &Identity{
				UserID:   u.ID,
				Name:     u.Name,
				Provider: "magic_link",
				Role:     u.Role,
				Scopes:   nil, // full access based on role
				Meta: map[string]string{
					"auth_method": "magic_link",
				},
			}, nil
		}
	}

	return nil, fmt.Errorf("user %s not found in users.json", userID)
}
