package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

var magicLinksBucket = []byte("magic_links")

// MagicLink represents a one-time login link stored in bbolt.
type MagicLink struct {
	ID        string    `json:"id"`
	TokenHash string    `json:"token_hash"` // SHA-256 hash, never plaintext
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

// MagicLinkStore manages magic links in bbolt.
type MagicLinkStore struct {
	db *bolt.DB

	// Rate limiting: userID -> list of creation times
	rateMu    sync.Mutex
	rateTrack map[string][]time.Time
}

// NewMagicLinkStore opens (or reuses) a bbolt database and ensures the
// magic_links bucket exists. Can share the same DB file as sessions.
func NewMagicLinkStore(db *bolt.DB) (*MagicLinkStore, error) {
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(magicLinksBucket)
		return err
	}); err != nil {
		return nil, fmt.Errorf("magiclink store: create bucket: %w", err)
	}

	return &MagicLinkStore{
		db:        db,
		rateTrack: make(map[string][]time.Time),
	}, nil
}

// Create generates a new magic link token for the given user.
// Returns the plaintext token (vel_ml_ + 32 hex chars). Only the SHA-256
// hash is persisted.
func (s *MagicLinkStore) Create(userID string, expiryMinutes int) (string, error) {
	// Rate limiting: max 5 per hour per userID
	if !s.allowCreation(userID) {
		return "", fmt.Errorf("rate limit exceeded: max 5 magic links per hour for user %s", userID)
	}

	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := "vel_ml_" + hex.EncodeToString(tokenBytes)

	// Hash for storage
	hash := sha256.Sum256([]byte(token))
	tokenHash := fmt.Sprintf("sha256:%x", hash)

	// Generate link ID
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	linkID := hex.EncodeToString(idBytes)

	ml := MagicLink{
		ID:        linkID,
		TokenHash: tokenHash,
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Duration(expiryMinutes) * time.Minute),
		Used:      false,
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(ml)
	if err != nil {
		return "", fmt.Errorf("marshal magic link: %w", err)
	}

	if err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(magicLinksBucket)
		if b == nil {
			return fmt.Errorf("magic_links bucket not found")
		}
		return b.Put([]byte(linkID), data)
	}); err != nil {
		return "", fmt.Errorf("save magic link: %w", err)
	}

	return token, nil
}

// Validate checks a plaintext token against stored hashes.
// On success, marks the link as used and returns the userID.
func (s *MagicLinkStore) Validate(token string) (string, error) {
	hash := sha256.Sum256([]byte(token))
	tokenHash := fmt.Sprintf("sha256:%x", hash)

	var foundID string
	var foundLink MagicLink

	// Find the link by hash
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(magicLinksBucket)
		if b == nil {
			return fmt.Errorf("magic_links bucket not found")
		}
		return b.ForEach(func(k, v []byte) error {
			var ml MagicLink
			if err := json.Unmarshal(v, &ml); err != nil {
				return nil // skip corrupt entries
			}
			if ml.TokenHash == tokenHash {
				foundID = string(k)
				foundLink = ml
			}
			return nil
		})
	})
	if err != nil {
		return "", fmt.Errorf("validate magic link: %w", err)
	}

	if foundID == "" {
		return "", fmt.Errorf("invalid magic link token")
	}

	if foundLink.Used {
		return "", fmt.Errorf("magic link already used")
	}

	if time.Now().After(foundLink.ExpiresAt) {
		return "", fmt.Errorf("magic link expired")
	}

	// Mark as used
	foundLink.Used = true
	data, err := json.Marshal(foundLink)
	if err != nil {
		return "", fmt.Errorf("marshal used link: %w", err)
	}

	if err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(magicLinksBucket)
		if b == nil {
			return fmt.Errorf("magic_links bucket not found")
		}
		return b.Put([]byte(foundID), data)
	}); err != nil {
		return "", fmt.Errorf("mark link used: %w", err)
	}

	return foundLink.UserID, nil
}

// Cleanup removes all expired or used magic links.
func (s *MagicLinkStore) Cleanup() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(magicLinksBucket)
		if b == nil {
			return nil
		}
		now := time.Now()
		var toDelete [][]byte
		if err := b.ForEach(func(k, v []byte) error {
			var ml MagicLink
			if err := json.Unmarshal(v, &ml); err != nil {
				toDelete = append(toDelete, append([]byte{}, k...))
				return nil
			}
			if ml.Used || now.After(ml.ExpiresAt) {
				toDelete = append(toDelete, append([]byte{}, k...))
			}
			return nil
		}); err != nil {
			return err
		}
		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

// allowCreation checks rate limiting: max 5 creations per hour per userID.
func (s *MagicLinkStore) allowCreation(userID string) bool {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-1 * time.Hour)

	times := s.rateTrack[userID]
	var valid []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= 5 {
		s.rateTrack[userID] = valid
		return false
	}

	s.rateTrack[userID] = append(valid, now)
	return true
}
