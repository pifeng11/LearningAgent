package app

import (
	"context"
	"errors"
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
}

func NewAgentService() *AgentService {
	modelRouter := model.NewRouter(model.NewMockProvider())
	memoryStore := memory.NewInMemoryStore()
	registry := skills.NewRegistry()

	skills.RegisterBuiltins(registry, modelRouter, memoryStore)

	return &AgentService{
		intentClassifier: intent.NewClassifier(),
		planner:          planner.NewPlanner(),
		executor:         runtime.NewExecutor(registry, memoryStore),
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
