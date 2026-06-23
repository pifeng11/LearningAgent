package memory

import (
	"context"
	"sync"
	"time"
)

const (
	TypeGoal       = "goal"
	TypePreference = "preference"
	TypeWeakness   = "weakness"
	TypeMistake    = "mistake"
	TypeFact       = "fact"
	TypeSummary    = "summary"

	DefaultLoadLimit = 10

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
	Upsert(ctx context.Context, entry Entry) error
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
			if len(result) >= DefaultLoadLimit {
				break
			}
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

func (s *InMemoryStore) Upsert(ctx context.Context, entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry = NormalizeEntry(entry)
	for i, existing := range s.entries {
		if sameMemoryIdentity(existing, entry) {
			entry.ID = existing.ID
			entry.SourceMessageIDs = mergeInt64s(existing.SourceMessageIDs, entry.SourceMessageIDs)
			entry.CreatedAt = existing.CreatedAt
			entry.UpdatedAt = time.Now()
			if existing.Confidence > entry.Confidence {
				entry.Confidence = existing.Confidence
			}
			s.entries[i] = entry
			return nil
		}
	}
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

func sameMemoryIdentity(left Entry, right Entry) bool {
	return left.UserID == right.UserID &&
		left.Type == right.Type &&
		left.Title == right.Title &&
		left.Scope == right.Scope &&
		left.Status == StatusActive
}

func mergeInt64s(left []int64, right []int64) []int64 {
	seen := map[int64]bool{}
	result := []int64{}
	for _, id := range append(left, right...) {
		if seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}
