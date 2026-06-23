package conversation

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type LocalFileStore struct {
	path string
	mu   sync.Mutex
}

func NewLocalFileStore(path string) *LocalFileStore {
	return &LocalFileStore{path: path}
}

func (s *LocalFileStore) CreateMessage(ctx context.Context, message Message) (Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	message = normalizeMessage(message)
	return message, s.appendLocked(ctx, message)
}

func (s *LocalFileStore) CompleteMessage(ctx context.Context, messageID string, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	message := Message{
		ID:        messageID,
		Role:      "assistant",
		Content:   content,
		Status:    "completed",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return s.appendLocked(ctx, message)
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
