package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

// FindUserByEmail looks up a user by email address (case-insensitive).
// Returns nil if not found.
func (us *UserStore) FindUserByEmail(email string) *UserRecord {
	us.mu.RLock()
	defer us.mu.RUnlock()
	if us.data == nil {
		return nil
	}
	emailLower := strings.ToLower(email)
	for i := range us.data.Users {
		u := &us.data.Users[i]
		if strings.ToLower(u.Email) == emailLower {
			return u
		}
	}
	return nil
}

// FindUserByID looks up a user by their canonical ID.
// Returns nil if not found.
func (us *UserStore) FindUserByID(id string) *UserRecord {
	us.mu.RLock()
	defer us.mu.RUnlock()
	if us.data == nil {
		return nil
	}
	for i := range us.data.Users {
		u := &us.data.Users[i]
		if u.ID == id {
			return u
		}
	}
	return nil
}

// AddAPIKey adds an API key entry and persists to disk.
func (us *UserStore) AddAPIKey(key APIKey) error {
	us.mu.Lock()
	defer us.mu.Unlock()
	if us.data == nil {
		return fmt.Errorf("user store not loaded")
	}
	us.data.APIKeys = append(us.data.APIKeys, key)
	return SaveUsers(us.path, us.data)
}

// RemoveAPIKey removes an API key by ID and persists to disk.
// Returns true if the key was found and removed.
func (us *UserStore) RemoveAPIKey(id string) (bool, error) {
	us.mu.Lock()
	defer us.mu.Unlock()
	if us.data == nil {
		return false, fmt.Errorf("user store not loaded")
	}
	for i, k := range us.data.APIKeys {
		if k.ID == id {
			us.data.APIKeys = append(us.data.APIKeys[:i], us.data.APIKeys[i+1:]...)
			return true, SaveUsers(us.path, us.data)
		}
	}
	return false, nil
}

// AddUser adds a user record and persists to disk.
func (us *UserStore) AddUser(user UserRecord) error {
	us.mu.Lock()
	defer us.mu.Unlock()
	if us.data == nil {
		return fmt.Errorf("user store not loaded")
	}
	// Check for duplicate ID
	for _, u := range us.data.Users {
		if u.ID == user.ID {
			return fmt.Errorf("user %s already exists", user.ID)
		}
	}
	us.data.Users = append(us.data.Users, user)
	return SaveUsers(us.path, us.data)
}

// RemoveUser removes a user by ID and persists to disk.
// Returns true if the user was found and removed.
func (us *UserStore) RemoveUser(id string) (bool, error) {
	us.mu.Lock()
	defer us.mu.Unlock()
	if us.data == nil {
		return false, fmt.Errorf("user store not loaded")
	}
	for i, u := range us.data.Users {
		if u.ID == id {
			us.data.Users = append(us.data.Users[:i], us.data.Users[i+1:]...)
			return true, SaveUsers(us.path, us.data)
		}
	}
	return false, nil
}

// GetAllUsers returns a copy of all user records.
func (us *UserStore) GetAllUsers() []UserRecord {
	us.mu.RLock()
	defer us.mu.RUnlock()
	if us.data == nil {
		return nil
	}
	result := make([]UserRecord, len(us.data.Users))
	copy(result, us.data.Users)
	return result
}

// GetAllAPIKeys returns a copy of all API key records.
func (us *UserStore) GetAllAPIKeys() []APIKey {
	us.mu.RLock()
	defer us.mu.RUnlock()
	if us.data == nil {
		return nil
	}
	result := make([]APIKey, len(us.data.APIKeys))
	copy(result, us.data.APIKeys)
	return result
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
	// Ensure non-nil slices so JSON serializes as [] not null
	if uf.Users == nil {
		uf.Users = []UserRecord{}
	}
	if uf.APIKeys == nil {
		uf.APIKeys = []APIKey{}
	}
	data, err := json.MarshalIndent(uf, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
