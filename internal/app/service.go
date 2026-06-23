package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"learning-agent/internal/agent/intent"
	"learning-agent/internal/agent/planner"
	"learning-agent/internal/agent/runtime"
	"learning-agent/internal/agent/state"
	"learning-agent/internal/conversation"
	"learning-agent/internal/memory"
	"learning-agent/internal/model"
	"learning-agent/internal/skills"
	"learning-agent/internal/storage"
)

type AgentService struct {
	intentClassifier  *intent.Classifier
	planner           *planner.Planner
	executor          *runtime.Executor
	modelRouter       *model.Router
	memoryStore       memory.Store
	conversationStore conversation.Store
	memoryExtractor   memory.Extractor
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
	memoryStore, conversationStore, err := newStoresFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	modelRouter := model.NewRouter(provider)
	extractor, err := newMemoryExtractorFromConfig(cfg, modelRouter)
	if err != nil {
		return nil, err
	}
	return newAgentServiceWithStores(modelRouter, memoryStore, conversationStore, extractor), nil
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

func newMemoryExtractorFromConfig(cfg Config, modelRouter *model.Router) (memory.Extractor, error) {
	switch strings.ToLower(cfg.MemoryExtractor) {
	case "", "llm":
		return memory.NewLLMExtractor(modelRouter), nil
	case "rule":
		return memory.NewRuleExtractor(), nil
	default:
		return nil, fmt.Errorf("unsupported MEMORY_EXTRACTOR %q", cfg.MemoryExtractor)
	}
}

func newAgentService(modelRouter *model.Router) *AgentService {
	return newAgentServiceWithStores(modelRouter, memory.NewInMemoryStore(), conversation.NewInMemoryStore(), memory.NewRuleExtractor())
}

func newAgentServiceWithStores(modelRouter *model.Router, memoryStore memory.Store, conversationStore conversation.Store, extractor memory.Extractor) *AgentService {
	registry := skills.NewRegistry()

	skills.RegisterBuiltins(registry, modelRouter, memoryStore)

	return &AgentService{
		intentClassifier:  intent.NewClassifier(),
		planner:           planner.NewPlanner(),
		executor:          runtime.NewExecutor(registry, memoryStore),
		modelRouter:       modelRouter,
		memoryStore:       memoryStore,
		conversationStore: conversationStore,
		memoryExtractor:   extractor,
	}
}

func newStoresFromConfig(cfg Config) (memory.Store, conversation.Store, error) {
	switch strings.ToLower(cfg.MemoryStore) {
	case "", "local":
		return memory.NewLocalFileStore(cfg.LocalDataPath), conversation.NewLocalFileStore(cfg.LocalMessagesPath), nil
	case "memory", "inmemory", "in-memory":
		return memory.NewInMemoryStore(), conversation.NewInMemoryStore(), nil
	case "postgres", "pg":
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pool, err := storage.OpenPostgres(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, nil, err
		}
		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			return nil, nil, fmt.Errorf("ping postgres: %w", err)
		}
		return memory.NewPostgresStore(pool), conversation.NewPostgresStore(pool), nil
	default:
		return nil, nil, fmt.Errorf("unsupported MEMORY_STORE %q", cfg.MemoryStore)
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
		userMessage, err := s.conversationStore.CreateMessage(ctx, conversation.Message{
			UserID:    req.UserID,
			SessionID: req.SessionID,
			Role:      "user",
			Content:   req.Message,
			Status:    "completed",
		})
		if err != nil {
			errs <- err
			return
		}

		assistantMessage, err := s.conversationStore.CreateMessage(ctx, conversation.Message{
			UserID:    req.UserID,
			SessionID: req.SessionID,
			Role:      "assistant",
			Status:    "streaming",
		})
		if err != nil {
			errs <- err
			return
		}

		events <- ChatStreamEvent{
			Type:      "agent.started",
			UserID:    req.UserID,
			SessionID: req.SessionID,
			Intent:    string(detectedIntent),
			Timestamp: time.Now(),
		}

		memories, err := s.memoryStore.Load(ctx, req.UserID, req.SessionID)
		if err != nil {
			errs <- err
			return
		}
		prompt := buildPromptWithMemories(req.Message, memories)

		chunks, streamErrs := s.modelRouter.GenerateStream(ctx, model.Request{
			Task:   skillTaskForIntent(detectedIntent),
			Prompt: prompt,
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

		answerText := answer.String()
		if err := s.conversationStore.CompleteMessage(ctx, assistantMessage.ID, answerText); err != nil {
			errs <- err
			return
		}

		events <- ChatStreamEvent{
			Type:      "agent.completed",
			UserID:    req.UserID,
			SessionID: req.SessionID,
			Intent:    string(detectedIntent),
			Answer:    answerText,
			Timestamp: time.Now(),
		}

		s.saveMemoryAfterCompletion(req.UserID, req.SessionID, req.Message, detectedIntent, answerText, sourceMessageIDs(userMessage.ID, assistantMessage.ID))
	}()

	return events, errs
}

func (s *AgentService) saveMemoryAfterCompletion(userID string, sessionID string, input string, detectedIntent state.Intent, answer string, sourceIDs []int64) {
	// TODO: 将 RuleExtractor 替换为 LLM MemoryExtractor，输出严格 JSON 并做 schema 校验。
	// TODO: 增加 memory upsert/merge，避免同一 goal 或 preference 被重复写入。
	// TODO: 支持流式输出过程中的 periodic checkpoint，降低模型生成中途崩溃时的回答丢失窗口。
	// TODO: 引入持久化后台任务队列后，将这里改为 enqueue，避免请求 goroutine 承担 memory 后处理。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entries, err := s.memoryExtractor.Extract(ctx, memory.ExtractRequest{
		UserID:           userID,
		SessionID:        sessionID,
		Input:            input,
		Answer:           answer,
		SourceMessageIDs: sourceIDs,
	})
	if err != nil {
		slog.Warn("extract memory", "error", err)
		return
	}

	for _, entry := range entries {
		if err := s.memoryStore.Upsert(ctx, entry); err != nil {
			slog.Warn("upsert extracted memory", "error", err, "type", entry.Type, "title", entry.Title)
		}
	}
}

func buildPromptWithMemories(input string, memories []memory.Entry) string {
	if len(memories) == 0 {
		return input
	}

	sort.SliceStable(memories, func(i, j int) bool {
		leftPriority := memoryPromptPriority(memories[i])
		rightPriority := memoryPromptPriority(memories[j])
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		return memories[i].UpdatedAt.After(memories[j].UpdatedAt)
	})

	var builder strings.Builder
	builder.WriteString("请结合以下已知记忆回答用户问题。记忆可能包含历史摘要、目标、偏好或薄弱点；如果与本次问题无关，请忽略。\n\n")
	builder.WriteString("相关记忆：\n")

	// TODO: 按 token budget、memory type 优先级和相关性筛选，不应长期简单拼接最近所有记忆。
	// TODO: 接入向量检索后，只注入与当前问题语义相关的 memories。
	for _, entry := range selectPromptMemories(memories) {
		builder.WriteString("- [")
		builder.WriteString(entry.Type)
		builder.WriteString("/")
		builder.WriteString(entry.Scope)
		builder.WriteString("] ")
		builder.WriteString(entry.Title)
		builder.WriteString(": ")
		builder.WriteString(truncateMemoryContent(entry.Content, 500))
		builder.WriteString("\n")
	}

	builder.WriteString("\n用户当前问题：\n")
	builder.WriteString(input)
	return builder.String()
}

func selectPromptMemories(memories []memory.Entry) []memory.Entry {
	selected := []memory.Entry{}
	summaryCount := 0
	for _, entry := range memories {
		if entry.Type == memory.TypeSummary {
			if summaryCount >= 2 {
				continue
			}
			summaryCount++
		}
		selected = append(selected, entry)
	}
	return selected
}

func memoryPromptPriority(entry memory.Entry) int {
	switch entry.Type {
	case memory.TypeGoal:
		return 0
	case memory.TypePreference:
		return 1
	case memory.TypeWeakness, memory.TypeMistake:
		return 2
	case memory.TypeFact:
		return 3
	case memory.TypeSummary:
		return 9
	default:
		return 5
	}
}

func truncateMemoryContent(content string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "..."
}

func sourceMessageIDs(ids ...string) []int64 {
	result := []int64{}
	for _, id := range ids {
		parsed, err := strconv.ParseInt(id, 10, 64)
		if err == nil {
			result = append(result, parsed)
		}
	}
	return result
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
