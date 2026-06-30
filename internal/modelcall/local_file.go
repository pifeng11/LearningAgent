package modelcall

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

	"learning-agent/internal/model"
)

type LocalFileStore struct {
	path    string
	mu      sync.Mutex
	pending map[int64]model.ModelCall
}

func NewLocalFileStore(path string) *LocalFileStore {
	return &LocalFileStore{
		path:    path,
		pending: map[int64]model.ModelCall{},
	}
}

func (s *LocalFileStore) CreateModelCall(ctx context.Context, call model.ModelCall) (model.ModelCall, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if call.ID == 0 {
		call.ID = time.Now().UnixNano()
	}
	if call.StartedAt.IsZero() {
		call.StartedAt = time.Now()
	}
	if call.RequestMetadata == nil {
		call.RequestMetadata = map[string]any{}
	}
	if call.ResponseMetadata == nil {
		call.ResponseMetadata = map[string]any{}
	}
	s.pending[call.ID] = call
	return call, s.appendLocked(ctx, call)
}

func (s *LocalFileStore) CompleteModelCall(ctx context.Context, id int64, update model.ModelCallUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	call, ok := s.pending[id]
	if !ok {
		latest, err := s.loadLatestLocked(ctx)
		if err != nil {
			return err
		}
		call = latest[id]
	}
	call.Status = update.Status
	call.PromptTokens = update.Usage.PromptTokens
	call.CompletionTokens = update.Usage.CompletionTokens
	call.TotalTokens = update.Usage.TotalTokens
	call.LatencyMS = update.LatencyMS
	call.RetryCount = update.RetryCount
	call.ErrorType = update.ErrorType
	call.ErrorMessage = update.ErrorMessage
	call.ResponseMetadata = update.ResponseMetadata
	completedAt := update.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now()
	}
	call.CompletedAt = &completedAt
	delete(s.pending, id)
	return s.appendLocked(ctx, call)
}

func (s *LocalFileStore) ListModelCalls(ctx context.Context, query model.ModelCallQuery) ([]model.ModelCall, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}

	latest, err := s.loadLatestLocked(ctx)
	if err != nil {
		return nil, err
	}

	calls := make([]model.ModelCall, 0, len(latest))
	for _, call := range latest {
		if query.TraceID != "" && call.TraceID != query.TraceID {
			continue
		}
		if query.UserID != "" && call.UserID != query.UserID {
			continue
		}
		if query.SessionID != "" && call.SessionID != query.SessionID {
			continue
		}
		calls = append(calls, call)
	}
	sort.SliceStable(calls, func(i, j int) bool {
		if calls[i].StartedAt.Equal(calls[j].StartedAt) {
			return calls[i].ID > calls[j].ID
		}
		return calls[i].StartedAt.After(calls[j].StartedAt)
	})
	if len(calls) > query.Limit {
		calls = calls[:query.Limit]
	}
	return calls, nil
}

func (s *LocalFileStore) GetModelCall(ctx context.Context, id int64) (model.ModelCall, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	latest, err := s.loadLatestLocked(ctx)
	if err != nil {
		return model.ModelCall{}, false, err
	}
	call, ok := latest[id]
	return call, ok, nil
}

func (s *LocalFileStore) loadLatestLocked(ctx context.Context) (map[int64]model.ModelCall, error) {
	file, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[int64]model.ModelCall{}, nil
		}
		return nil, err
	}
	defer file.Close()

	latest := map[int64]model.ModelCall{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var call model.ModelCall
		if err := json.Unmarshal(scanner.Bytes(), &call); err != nil {
			return nil, err
		}
		latest[call.ID] = call
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return latest, nil
}

func (s *LocalFileStore) appendLocked(ctx context.Context, call model.ModelCall) error {
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

	encoded, err := json.Marshal(call)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return nil
}
