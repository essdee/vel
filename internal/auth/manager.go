package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

// AuthManager coordinates authentication providers, sessions, and users.
type AuthManager struct {
	providers    []Provider
	userStore    *UserStore
	sessionStore SessionStore
	maxAge       time.Duration
	cookieName   string
}

// AuthManagerConfig holds configuration for the auth manager.
type AuthManagerConfig struct {
	MaxAgeHours int    // session max age in hours (default 168 = 7 days)
	CookieName  string // session cookie name (default "vel_session")
}

// NewAuthManager creates a new AuthManager.
func NewAuthManager(userStore *UserStore, sessionStore SessionStore, cfg AuthManagerConfig) *AuthManager {
	maxAge := 168 // 7 days default
	if cfg.MaxAgeHours > 0 {
		maxAge = cfg.MaxAgeHours
	}
	cookieName := "vel_session"
	if cfg.CookieName != "" {
		cookieName = cfg.CookieName
	}
	return &AuthManager{
		userStore:    userStore,
		sessionStore: sessionStore,
		maxAge:       time.Duration(maxAge) * time.Hour,
		cookieName:   cookieName,
	}
}

// RegisterProvider adds an auth provider to the chain.
func (m *AuthManager) RegisterProvider(p Provider) {
	m.providers = append(m.providers, p)
}

// GetProvider returns a provider by name, or nil if not found.
func (m *AuthManager) GetProvider(name string) Provider {
	for _, p := range m.providers {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

// CookieName returns the configured session cookie name.
func (m *AuthManager) CookieName() string {
	return m.cookieName
}

// MaxAge returns the configured session max age.
func (m *AuthManager) MaxAge() time.Duration {
	return m.maxAge
}

// Authenticate tries session cookie first, then each provider.
// Returns (identity, session, error). Session may be nil for stateless providers.
func (m *AuthManager) Authenticate(r *http.Request) (*Identity, *Session, error) {
	// 1. Try session cookie
	sess, err := m.GetSession(r)
	if err == nil && sess != nil && !sess.IsExpired() {
		return sess.Identity, sess, nil
	}

	// 2. Try each provider
	for _, p := range m.providers {
		creds, ok := p.Extract(r)
		if !ok {
			continue
		}
		identity, err := p.Authenticate(creds)
		if err != nil {
			continue // try next provider
		}
		// Stateless providers (api_key) don't create sessions
		if p.Name() == "api_key" {
			return identity, nil, nil
		}
		return identity, nil, nil
	}

	return nil, nil, fmt.Errorf("no valid authentication found")
}

// CreateSession creates a new session for the given identity and persists it.
func (m *AuthManager) CreateSession(identity *Identity) (*Session, error) {
	id, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("generate session ID: %w", err)
	}

	sess := &Session{
		ID:        id,
		Identity:  identity,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(m.maxAge),
	}

	if err := m.sessionStore.Save(sess); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	return sess, nil
}

// GetSession loads a session from the request's session cookie.
func (m *AuthManager) GetSession(r *http.Request) (*Session, error) {
	cookie, err := r.Cookie(m.cookieName)
	if err != nil {
		return nil, fmt.Errorf("no session cookie")
	}
	sess, err := m.sessionStore.Get(cookie.Value)
	if err != nil {
		return nil, fmt.Errorf("session lookup: %w", err)
	}
	if sess == nil {
		return nil, fmt.Errorf("session not found or expired")
	}
	return sess, nil
}

// DestroySession removes a session by ID.
func (m *AuthManager) DestroySession(sessionID string) error {
	return m.sessionStore.Delete(sessionID)
}

// UserStore returns the user store.
func (m *AuthManager) UserStore() *UserStore {
	return m.userStore
}

// SessionStore returns the session store.
func (m *AuthManager) SessionStore() SessionStore {
	return m.sessionStore
}

// Cleanup removes expired sessions.
func (m *AuthManager) Cleanup() error {
	return m.sessionStore.Cleanup()
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
