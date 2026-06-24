package conversation

import (
	"context"
	"sort"
	"strconv"
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

type ListMessagesQuery struct {
	UserID    string
	SessionID string
	BeforeID  string
	Limit     int
}

type Store interface {
	CreateMessage(ctx context.Context, message Message) (Message, error)
	CompleteMessage(ctx context.Context, messageID string, content string) error
	ListMessages(ctx context.Context, query ListMessagesQuery) ([]Message, error)
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

func (s *InMemoryStore) ListMessages(ctx context.Context, query ListMessagesQuery) ([]Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	messages := make([]Message, 0)
	for _, message := range s.messages {
		if message.UserID == query.UserID && message.SessionID == query.SessionID && beforeMessageID(message.ID, query.BeforeID) {
			messages = append(messages, message)
		}
	}
	sort.SliceStable(messages, func(i, j int) bool {
		return messages[i].CreatedAt.Before(messages[j].CreatedAt)
	})
	return applyMessageLimit(messages, query.Limit), nil
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

func applyMessageLimit(messages []Message, limit int) []Message {
	if limit <= 0 || len(messages) <= limit {
		return messages
	}
	return messages[len(messages)-limit:]
}

func beforeMessageID(messageID string, beforeID string) bool {
	if beforeID == "" {
		return true
	}
	messageNumber, messageErr := strconv.ParseInt(messageID, 10, 64)
	beforeNumber, beforeErr := strconv.ParseInt(beforeID, 10, 64)
	if messageErr == nil && beforeErr == nil {
		return messageNumber < beforeNumber
	}
	return messageID < beforeID
}
