package memory

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type LocalFileStore struct {
	path string
	mu   sync.Mutex
}

func NewLocalFileStore(path string) *LocalFileStore {
	return &LocalFileStore{path: path}
}

func (s *LocalFileStore) Load(ctx context.Context, userID string, sessionID string) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := []Entry{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, err
		}
		if entry.UserID == userID && entry.Status == StatusActive && (entry.SessionID == sessionID || entry.Scope == ScopeUser) {
			result = append(result, entry)
			if len(result) >= DefaultLoadLimit {
				break
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *LocalFileStore) Save(ctx context.Context, entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	entry = NormalizeEntry(entry)
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoded, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return err
	}

	return nil
}

func (s *LocalFileStore) Upsert(ctx context.Context, entry Entry) error {
	// 本地 JSONL 存储采用 append-only 事件格式；upsert 通过追加最新版本表达，读取侧后续会演进为按 identity 合并。
	return s.Save(ctx, entry)
}
