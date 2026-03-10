package auth

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
)

// APIKeyCredentials holds the raw API key extracted from the request.
type APIKeyCredentials struct {
	Key string
}

// APIKeyProvider authenticates requests using API keys (Bearer or X-API-Key header).
// This provider is stateless — it does NOT create sessions.
type APIKeyProvider struct {
	userStore *UserStore
}

// NewAPIKeyProvider creates an API Key auth provider.
func NewAPIKeyProvider(userStore *UserStore) *APIKeyProvider {
	return &APIKeyProvider{userStore: userStore}
}

func (p *APIKeyProvider) Name() string { return "api_key" }

// Extract checks for vel_ak_ prefixed keys in Authorization Bearer or X-API-Key headers.
// Intentionally ignores ?token= query params.
func (p *APIKeyProvider) Extract(r *http.Request) (Credentials, bool) {
	// Try Authorization: Bearer vel_ak_...
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if strings.HasPrefix(token, "vel_ak_") {
			return APIKeyCredentials{Key: token}, true
		}
	}

	// Try X-API-Key header
	if apiKey := r.Header.Get("X-API-Key"); strings.HasPrefix(apiKey, "vel_ak_") {
		return APIKeyCredentials{Key: apiKey}, true
	}

	return nil, false
}

// Authenticate hashes the key and looks it up in the UserStore.
func (p *APIKeyProvider) Authenticate(creds Credentials) (*Identity, error) {
	akc, ok := creds.(APIKeyCredentials)
	if !ok {
		return nil, fmt.Errorf("invalid credentials type for api_key provider")
	}

	// SHA-256 hash with prefix
	hash := sha256.Sum256([]byte(akc.Key))
	keyHash := fmt.Sprintf("sha256:%x", hash)

	apiKey := p.userStore.FindAPIKey(keyHash)
	if apiKey == nil {
		return nil, fmt.Errorf("invalid API key")
	}

	return &Identity{
		UserID:   apiKey.ID,
		Name:     apiKey.Name,
		Provider: "api_key",
		Role:     apiKey.Role,
		Scopes:   apiKey.Scopes,
		Meta: map[string]string{
			"key_id":   apiKey.ID,
			"key_name": apiKey.Name,
		},
	}, nil
}
