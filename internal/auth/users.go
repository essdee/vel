package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// UserIdentity links a user to an external provider account.
type UserIdentity struct {
	Provider   string `json:"provider"`
	ProviderID string `json:"provider_id"`
}

// UserRecord represents a user entry in users.json.
type UserRecord struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Email      string         `json:"email,omitempty"`
	Role       string         `json:"role"`
	Identities []UserIdentity `json:"identities,omitempty"`
}

// APIKey represents an API key entry in users.json.
type APIKey struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	KeyHash   string   `json:"key_hash"`
	Role      string   `json:"role"`
	Scopes    []string `json:"scopes,omitempty"`
	CreatedBy string   `json:"created_by,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
}

// UsersFile is the top-level structure of users.json.
type UsersFile struct {
	Users   []UserRecord `json:"users"`
	APIKeys []APIKey     `json:"api_keys"`
}

// UserStore manages the users.json file with auto-reload on change.
type UserStore struct {
	mu       sync.RWMutex
	path     string
	data     *UsersFile
	lastMod  time.Time
	stopCh   chan struct{}
}

// NewUserStore creates a UserStore and loads the file immediately.
// It starts a background goroutine that polls for file changes every 30s.
func NewUserStore(path string) (*UserStore, error) {
	us := &UserStore{
		path:   path,
		stopCh: make(chan struct{}),
	}
	if err := us.load(); err != nil {
		return nil, fmt.Errorf("userstore: initial load failed: %w", err)
	}
	go us.watch()
	return us, nil
}

// load reads the users.json file from disk and replaces in-memory state.
func (us *UserStore) load() error {
	info, err := os.Stat(us.path)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(us.path)
	if err != nil {
		return err
	}

	var uf UsersFile
	if err := json.Unmarshal(data, &uf); err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	us.mu.Lock()
	us.data = &uf
	us.lastMod = info.ModTime()
	us.mu.Unlock()
	return nil
}

// watch polls the file's mtime every 30s and reloads on change.
func (us *UserStore) watch() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			info, err := os.Stat(us.path)
			if err != nil {
				continue
			}
			us.mu.RLock()
			lastMod := us.lastMod
			us.mu.RUnlock()
			if info.ModTime().After(lastMod) {
				if err := us.load(); err == nil {
					fmt.Printf("[auth] users.json reloaded\n")
				}
			}
		case <-us.stopCh:
			return
		}
	}
}

// Stop halts the background watcher goroutine.
func (us *UserStore) Stop() {
	close(us.stopCh)
}

// FindUserByIdentity looks up a user by provider name and provider-specific ID.
// Returns nil if not found.
func (us *UserStore) FindUserByIdentity(provider, providerID string) *UserRecord {
	us.mu.RLock()
	defer us.mu.RUnlock()
	if us.data == nil {
		return nil
	}
	for i := range us.data.Users {
		u := &us.data.Users[i]
		for _, ident := range u.Identities {
			if ident.Provider == provider && ident.ProviderID == providerID {
				return u
			}
		}
	}
	return nil
}

// FindAPIKey looks up an API key by its SHA-256 hash (hex string with "sha256:" prefix).
// Returns nil if not found.
func (us *UserStore) FindAPIKey(keyHash string) *APIKey {
	us.mu.RLock()
	defer us.mu.RUnlock()
	if us.data == nil {
		return nil
	}
	for i := range us.data.APIKeys {
		k := &us.data.APIKeys[i]
		if k.KeyHash == keyHash {
			return k
		}
	}
	return nil
}

// GetData returns a snapshot of the current users file (copy-safe for read).
func (us *UserStore) GetData() *UsersFile {
	us.mu.RLock()
	defer us.mu.RUnlock()
	return us.data
}

// LoadUsers reads and parses a users.json file. Does not start a watcher.
func LoadUsers(path string) (*UsersFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var uf UsersFile
	if err := json.Unmarshal(data, &uf); err != nil {
		return nil, err
	}
	return &uf, nil
}

// SaveUsers marshals and writes a UsersFile to disk (indented JSON).
func SaveUsers(path string, uf *UsersFile) error {
	data, err := json.MarshalIndent(uf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
