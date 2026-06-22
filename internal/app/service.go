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
)

type AgentService struct {
	intentClassifier *intent.Classifier
	planner          *planner.Planner
	executor         *runtime.Executor
	modelRouter      *model.Router
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
	return newAgentService(model.NewRouter(provider)), nil
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
	memoryStore := memory.NewInMemoryStore()
	registry := skills.NewRegistry()

	skills.RegisterBuiltins(registry, modelRouter, memoryStore)

	return &AgentService{
		intentClassifier: intent.NewClassifier(),
		planner:          planner.NewPlanner(),
		executor:         runtime.NewExecutor(registry, memoryStore),
		modelRouter:      modelRouter,
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
