package memory

import (
	"context"
	"sync"
	"time"
)

const (
	TypeSummary = "summary"

	ScopeUser    = "user"
	ScopeSession = "session"

	StatusActive     = "active"
	StatusSuperseded = "superseded"
	StatusDeleted    = "deleted"
	StatusExpired    = "expired"
)

type Entry struct {
	ID               int64
	UserID           string
	SessionID        string
	Type             string
	Title            string
	Content          string
	Scope            string
	Status           string
	Confidence       float64
	SourceMessageIDs []int64
	Metadata         map[string]any
	ValidFrom        *time.Time
	ValidUntil       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
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
		if entry.UserID == userID && entry.Status == StatusActive && (entry.SessionID == sessionID || entry.Scope == ScopeUser) {
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
	entry = NormalizeEntry(entry)
	s.entries = append(s.entries, entry)
	return nil
}

func NormalizeEntry(entry Entry) Entry {
	now := time.Now()
	if entry.Type == "" {
		entry.Type = TypeSummary
	}
	if entry.Title == "" {
		entry.Title = "Conversation turn"
	}
	if entry.Scope == "" {
		entry.Scope = ScopeSession
	}
	if entry.Status == "" {
		entry.Status = StatusActive
	}
	if entry.Confidence == 0 {
		entry.Confidence = 1
	}
	if entry.Metadata == nil {
		entry.Metadata = map[string]any{}
	}
	if entry.SourceMessageIDs == nil {
		entry.SourceMessageIDs = []int64{}
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = entry.CreatedAt
	}
	return entry
}
