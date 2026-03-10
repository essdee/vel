package auth

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

var sessionsBucket = []byte("sessions")

// SessionStore defines the interface for session persistence.
type SessionStore interface {
	// Get retrieves a session by ID. Returns nil, nil if not found.
	Get(id string) (*Session, error)
	// Save persists a session (insert or update).
	Save(session *Session) error
	// Delete removes a session by ID.
	Delete(id string) error
	// Cleanup removes all expired sessions.
	Cleanup() error
}

// BoltSessionStore implements SessionStore using bbolt.
type BoltSessionStore struct {
	db *bolt.DB
}

// NewBoltSessionStore opens (or creates) a bbolt database at the given path
// and returns a ready-to-use session store.
func NewBoltSessionStore(path string) (*BoltSessionStore, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("session store: open db: %w", err)
	}

	// Ensure the sessions bucket exists.
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(sessionsBucket)
		return err
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("session store: create bucket: %w", err)
	}

	return &BoltSessionStore{db: db}, nil
}

// Close closes the underlying bbolt database.
func (s *BoltSessionStore) Close() error {
	return s.db.Close()
}

// Get retrieves a session by its ID. Returns (nil, nil) when not found.
func (s *BoltSessionStore) Get(id string) (*Session, error) {
	var sess *Session
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(sessionsBucket)
		if b == nil {
			return nil
		}
		v := b.Get([]byte(id))
		if v == nil {
			return nil // not found — leave sess nil
		}
		var tmp Session
		if err := json.Unmarshal(v, &tmp); err != nil {
			return fmt.Errorf("session decode: %w", err)
		}
		sess = &tmp
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Treat expired sessions as not found.
	if sess != nil && sess.IsExpired() {
		_ = s.Delete(id)
		return nil, nil
	}
	return sess, nil
}

// Save persists a session, overwriting any existing entry with the same ID.
func (s *BoltSessionStore) Save(session *Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("session encode: %w", err)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(sessionsBucket)
		if b == nil {
			return fmt.Errorf("sessions bucket not found")
		}
		return b.Put([]byte(session.ID), data)
	})
}

// Delete removes a session by ID. No-ops if the ID does not exist.
func (s *BoltSessionStore) Delete(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(sessionsBucket)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(id))
	})
}

// Cleanup scans all sessions and deletes any that have expired.
func (s *BoltSessionStore) Cleanup() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(sessionsBucket)
		if b == nil {
			return nil
		}
		now := time.Now()
		var expired [][]byte
		if err := b.ForEach(func(k, v []byte) error {
			var sess Session
			if err := json.Unmarshal(v, &sess); err != nil {
				// Corrupt entry — mark for removal.
				expired = append(expired, append([]byte{}, k...))
				return nil
			}
			if now.After(sess.ExpiresAt) {
				expired = append(expired, append([]byte{}, k...))
			}
			return nil
		}); err != nil {
			return err
		}
		for _, k := range expired {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}
