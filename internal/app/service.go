package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"learning-agent/internal/agent/intent"
	"learning-agent/internal/agent/planner"
	"learning-agent/internal/agent/runtime"
	"learning-agent/internal/agent/state"
	"learning-agent/internal/memory"
	"learning-agent/internal/model"
	"learning-agent/internal/skills"
	"learning-agent/internal/storage"
)

type AgentService struct {
	intentClassifier *intent.Classifier
	planner          *planner.Planner
	executor         *runtime.Executor
	modelRouter      *model.Router
	memoryStore      memory.Store
}

func NewAgentService() *AgentService {
	modelRouter := model.NewRouter(model.NewMockProvider())
	return newAgentService(modelRouter)
}

func NewAgentServiceFromConfig(cfg Config) (*AgentService, error) {
	provider, err := newModelProviderFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	memoryStore, err := newMemoryStoreFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return newAgentServiceWithStore(model.NewRouter(provider), memoryStore), nil
}

func newModelProviderFromConfig(cfg Config) (model.Provider, error) {
	switch strings.ToLower(cfg.ModelProvider) {
	case "", "mock":
		return model.NewMockProvider(), nil
	case "deepseek":
		return model.NewDeepSeekProvider(model.DeepSeekConfig{
			APIKey:          cfg.DeepSeekAPIKey,
			BaseURL:         cfg.DeepSeekBaseURL,
			Model:           cfg.DeepSeekModel,
			ReasoningModel:  cfg.DeepSeekReasoningModel,
			ReasoningEffort: cfg.DeepSeekReasoningEffort,
			ThinkingEnabled: cfg.DeepSeekThinkingEnabled,
		})
	default:
		return nil, fmt.Errorf("unsupported MODEL_PROVIDER %q", cfg.ModelProvider)
	}
}

func newAgentService(modelRouter *model.Router) *AgentService {
	return newAgentServiceWithStore(modelRouter, memory.NewInMemoryStore())
}

func newAgentServiceWithStore(modelRouter *model.Router, memoryStore memory.Store) *AgentService {
	registry := skills.NewRegistry()

	skills.RegisterBuiltins(registry, modelRouter, memoryStore)

	return &AgentService{
		intentClassifier: intent.NewClassifier(),
		planner:          planner.NewPlanner(),
		executor:         runtime.NewExecutor(registry, memoryStore),
		modelRouter:      modelRouter,
		memoryStore:      memoryStore,
	}
}

func newMemoryStoreFromConfig(cfg Config) (memory.Store, error) {
	switch strings.ToLower(cfg.MemoryStore) {
	case "", "local":
		return memory.NewLocalFileStore(cfg.LocalDataPath), nil
	case "memory", "inmemory", "in-memory":
		return memory.NewInMemoryStore(), nil
	case "postgres", "pg":
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pool, err := storage.OpenPostgres(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, err
		}
		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			return nil, fmt.Errorf("ping postgres: %w", err)
		}
		return memory.NewPostgresStore(pool), nil
	default:
		return nil, fmt.Errorf("unsupported MEMORY_STORE %q", cfg.MemoryStore)
	}
}

func (s *AgentService) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	if strings.TrimSpace(req.Message) == "" {
		return ChatResponse{}, errors.New("message is required")
	}
	if req.UserID == "" {
		req.UserID = "anonymous"
	}
	if req.SessionID == "" {
		req.SessionID = "default"
	}

	detectedIntent := s.intentClassifier.Classify(req.Message)
	plan := s.planner.Plan(detectedIntent)
	agentState := state.AgentState{
		UserID:    req.UserID,
		SessionID: req.SessionID,
		Input:     req.Message,
		Intent:    detectedIntent,
		CreatedAt: time.Now(),
		Values:    map[string]any{},
	}

	result, err := s.executor.Execute(ctx, plan, agentState)
	if err != nil {
		return ChatResponse{}, err
	}

	events := make([]AgentEvent, 0, len(result.Events))
	for _, event := range result.Events {
		events = append(events, AgentEvent{
			Type:      event.Type,
			Message:   event.Message,
			Timestamp: event.Timestamp,
		})
	}

	return ChatResponse{
		UserID:    req.UserID,
		SessionID: req.SessionID,
		Intent:    string(detectedIntent),
		Answer:    result.Answer,
		Events:    events,
	}, nil
}

func (s *AgentService) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatStreamEvent, <-chan error) {
	events := make(chan ChatStreamEvent)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)

		if strings.TrimSpace(req.Message) == "" {
			errs <- errors.New("message is required")
			return
		}
		if req.UserID == "" {
			req.UserID = "anonymous"
		}
		if req.SessionID == "" {
			req.SessionID = "default"
		}

		detectedIntent := s.intentClassifier.Classify(req.Message)
		events <- ChatStreamEvent{
			Type:      "agent.started",
			UserID:    req.UserID,
			SessionID: req.SessionID,
			Intent:    string(detectedIntent),
			Timestamp: time.Now(),
		}

		chunks, streamErrs := s.modelRouter.GenerateStream(ctx, model.Request{
			Task:   skillTaskForIntent(detectedIntent),
			Prompt: req.Message,
		})

		var answer strings.Builder
		for chunk := range chunks {
			if chunk.Text != "" {
				answer.WriteString(chunk.Text)
				events <- ChatStreamEvent{
					Type:      "agent.delta",
					UserID:    req.UserID,
					SessionID: req.SessionID,
					Intent:    string(detectedIntent),
					Delta:     chunk.Text,
					Timestamp: time.Now(),
				}
			}
			if chunk.Done {
				break
			}
		}

		if err, ok := <-streamErrs; ok && err != nil {
			errs <- err
			return
		}

		if err := s.memoryStore.Save(ctx, memory.Entry{
			UserID:    req.UserID,
			SessionID: req.SessionID,
			Scope:     memory.ScopeShortTerm,
			Content:   fmt.Sprintf("input=%s\nintent=%s\nanswer=%s", req.Message, detectedIntent, answer.String()),
			CreatedAt: time.Now(),
		}); err != nil {
			errs <- err
			return
		}

		events <- ChatStreamEvent{
			Type:      "agent.completed",
			UserID:    req.UserID,
			SessionID: req.SessionID,
			Intent:    string(detectedIntent),
			Answer:    answer.String(),
			Timestamp: time.Now(),
		}
	}()

	return events, errs
}

func skillTaskForIntent(intent state.Intent) string {
	switch intent {
	case state.IntentLearningPlan:
		return "learning_plan"
	case state.IntentPractice:
		return "practice"
	case state.IntentReview:
		return "review"
	default:
		return "qa"
	}
}
