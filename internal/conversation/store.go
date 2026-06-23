package conversation

import (
	"context"
	"sync"
	"time"
)

type Message struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store interface {
	CreateMessage(ctx context.Context, message Message) (Message, error)
	CompleteMessage(ctx context.Context, messageID string, content string) error
}

type InMemoryStore struct {
	mu       sync.Mutex
	messages map[string]Message
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{messages: map[string]Message{}}
}

func (s *InMemoryStore) CreateMessage(ctx context.Context, message Message) (Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	message = normalizeMessage(message)
	s.messages[message.ID] = message
	return message, nil
}

func (s *InMemoryStore) CompleteMessage(ctx context.Context, messageID string, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	message := s.messages[messageID]
	message.Content = content
	message.Status = "completed"
	message.UpdatedAt = time.Now()
	s.messages[messageID] = message
	return nil
}

func normalizeMessage(message Message) Message {
	now := time.Now()
	if message.ID == "" {
		message.ID = newMessageID()
	}
	if message.Status == "" {
		message.Status = "completed"
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = now
	}
	if message.UpdatedAt.IsZero() {
		message.UpdatedAt = message.CreatedAt
	}
	return message
}

func newMessageID() string {
	return time.Now().UTC().Format("20060102150405.000000000")
}
