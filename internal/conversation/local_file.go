package conversation

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type LocalFileStore struct {
	path    string
	mu      sync.Mutex
	pending map[string]Message
}

func NewLocalFileStore(path string) *LocalFileStore {
	return &LocalFileStore{
		path:    path,
		pending: map[string]Message{},
	}
}

func (s *LocalFileStore) CreateMessage(ctx context.Context, message Message) (Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	message = normalizeMessage(message)
	if message.Status == "streaming" {
		s.pending[message.ID] = message
	}
	return message, s.appendLocked(ctx, message)
}

func (s *LocalFileStore) CompleteMessage(ctx context.Context, messageID string, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	message, ok := s.pending[messageID]
	if !ok {
		message = Message{
			ID:        messageID,
			Role:      "assistant",
			CreatedAt: time.Now(),
		}
	}
	message.Content = content
	message.Status = "completed"
	message.UpdatedAt = time.Now()
	delete(s.pending, messageID)
	return s.appendLocked(ctx, message)
}

func (s *LocalFileStore) ListMessages(ctx context.Context, query ListMessagesQuery) ([]Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Message{}, nil
		}
		return nil, err
	}
	defer file.Close()

	latestByID := map[string]Message{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var message Message
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			return nil, err
		}
		if message.UserID == query.UserID && message.SessionID == query.SessionID && beforeMessageID(message.ID, query.BeforeID) {
			latestByID[message.ID] = message
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	messages := make([]Message, 0, len(latestByID))
	for _, message := range latestByID {
		messages = append(messages, message)
	}
	sort.SliceStable(messages, func(i, j int) bool {
		return messages[i].CreatedAt.Before(messages[j].CreatedAt)
	})
	return applyMessageLimit(messages, query.Limit), nil
}

func (s *LocalFileStore) appendLocked(ctx context.Context, message Message) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	encoded, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if _, err := writer.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return writer.Flush()
}
