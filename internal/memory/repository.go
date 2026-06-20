package memory

import (
	"context"
	"sync"
	"time"
)

type Entry struct {
	UserID    string
	SessionID string
	Scope     string
	Content   string
	CreatedAt time.Time
}

type Store interface {
	Load(ctx context.Context, userID string, sessionID string) ([]Entry, error)
	Save(ctx context.Context, entry Entry) error
}

type InMemoryStore struct {
	mu      sync.RWMutex
	entries []Entry
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{entries: []Entry{}}
}

func (s *InMemoryStore) Load(ctx context.Context, userID string, sessionID string) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := []Entry{}
	for _, entry := range s.entries {
		if entry.UserID == userID && (entry.SessionID == sessionID || entry.Scope == "long_term") {
			result = append(result, entry)
		}
	}
	return result, nil
}

func (s *InMemoryStore) Save(ctx context.Context, entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	s.entries = append(s.entries, entry)
	return nil
}
