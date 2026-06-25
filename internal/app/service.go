package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"learning-agent/internal/agent/intent"
	"learning-agent/internal/agent/planner"
	"learning-agent/internal/agent/runtime"
	"learning-agent/internal/agent/state"
	"learning-agent/internal/conversation"
	"learning-agent/internal/debugtrace"
	"learning-agent/internal/memory"
	"learning-agent/internal/model"
	"learning-agent/internal/model/deepseek"
	"learning-agent/internal/observability"
	promptbuilder "learning-agent/internal/prompt"
	"learning-agent/internal/skills"
	"learning-agent/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentService struct {
	intentClassifier   *intent.Classifier
	planner            *planner.Planner
	executor           *runtime.Executor
	modelRouter        *model.Router
	memoryStore        memory.Store
	conversationStore  conversation.Store
	memoryExtractor    memory.Extractor
	memoryExtractTTL   time.Duration
	promptBuilder      *promptbuilder.Builder
	traceStore         debugtrace.Store
	debugPrompt        bool
	traceSnapshot      bool
	traceTokenEstimate bool
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
	memoryStore, conversationStore, traceStore, err := newStoresFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	modelRouter := newModelRouterFromConfig(cfg, provider)
	extractor, err := newMemoryExtractorFromConfig(cfg, modelRouter)
	if err != nil {
		return nil, err
	}
	promptBuilder, err := newPromptBuilderFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return newAgentServiceWithStores(modelRouter, memoryStore, conversationStore, extractor, cfg.MemoryExtractTimeout, promptBuilder, traceStore, cfg.TraceCapturePromptText || cfg.DebugPromptEnabled, cfg.TraceContextSnapshot, cfg.TraceTokenEstimation), nil
}

func newModelProviderFromConfig(cfg Config) (model.Provider, error) {
	switch strings.ToLower(cfg.ModelProvider) {
	case "", "mock":
		return model.NewMockProvider(), nil
	case "deepseek":
		return deepseek.NewProvider(deepseek.Config{
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

func newModelRouterFromConfig(cfg Config, provider model.Provider) *model.Router {
	defaultRoute := model.Route{
		Provider:   provider.Name(),
		Capability: model.CapabilityChat,
		Model:      cfg.ModelDefaultModel,
	}
	taskRoutes := map[model.Task]model.Route{}
	addTaskRoute(taskRoutes, model.TaskQA, cfg.ModelTaskQA)
	addTaskRoute(taskRoutes, model.TaskLearningPlan, firstNonEmptyString(cfg.ModelTaskLearningPlan, cfg.DeepSeekReasoningModel))
	addTaskRoute(taskRoutes, model.TaskPractice, cfg.ModelTaskPractice)
	addTaskRoute(taskRoutes, model.TaskReview, firstNonEmptyString(cfg.ModelTaskReview, cfg.DeepSeekReasoningModel))
	addTaskRoute(taskRoutes, model.TaskMemoryExtract, cfg.ModelTaskMemoryExtract)

	return model.NewRouterWithConfig([]model.Provider{provider}, model.RouterConfig{
		DefaultRoute:   defaultRoute,
		TaskRoutes:     taskRoutes,
		DefaultTimeout: cfg.ModelTimeout,
		StreamTimeout:  cfg.ModelStreamTimeout,
		MaxRetries:     cfg.ModelMaxRetries,
		RetryBackoff:   cfg.ModelRetryBackoff,
	})
}

func addTaskRoute(routes map[model.Task]model.Route, task model.Task, modelName string) {
	if modelName == "" {
		return
	}
	routes[task] = model.Route{
		Capability: model.CapabilityChat,
		Model:      modelName,
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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
	return newAgentServiceWithStores(modelRouter, memory.NewInMemoryStore(), conversation.NewInMemoryStore(), memory.NewRuleExtractor(), 30*time.Second, defaultPromptBuilder(), debugtrace.NoopStore{}, false, true, true)
}

func newAgentServiceWithStores(modelRouter *model.Router, memoryStore memory.Store, conversationStore conversation.Store, extractor memory.Extractor, memoryExtractTTL time.Duration, promptBuilder *promptbuilder.Builder, traceStore debugtrace.Store, debugPrompt bool, traceContextSnapshot bool, traceTokenEstimation bool) *AgentService {
	if memoryExtractTTL <= 0 {
		memoryExtractTTL = 30 * time.Second
	}
	if promptBuilder == nil {
		promptBuilder = defaultPromptBuilder()
	}
	if traceStore == nil {
		traceStore = debugtrace.NoopStore{}
	}

	registry := skills.NewRegistry()

	skills.RegisterBuiltins(registry, modelRouter, memoryStore)

	return &AgentService{
		intentClassifier:   intent.NewClassifier(),
		planner:            planner.NewPlanner(),
		executor:           runtime.NewExecutor(registry, memoryStore),
		modelRouter:        modelRouter,
		memoryStore:        memoryStore,
		conversationStore:  conversationStore,
		memoryExtractor:    extractor,
		memoryExtractTTL:   memoryExtractTTL,
		promptBuilder:      promptBuilder,
		traceStore:         traceStore,
		debugPrompt:        debugPrompt,
		traceSnapshot:      traceContextSnapshot,
		traceTokenEstimate: traceTokenEstimation,
	}
}

func newTraceStoreFromConfig(cfg Config, pool *pgxpool.Pool) (debugtrace.Store, error) {
	if !cfg.DebugTraceEnabled {
		return debugtrace.NoopStore{}, nil
	}
	switch strings.ToLower(cfg.TraceStore) {
	case "", "memory":
		return debugtrace.NewRingStore(cfg.DebugTraceCapacity), nil
	case "postgres", "pg":
		if pool == nil {
			return nil, fmt.Errorf("TRACE_STORE=postgres requires MEMORY_STORE=postgres in this version")
		}
		return debugtrace.NewPostgresStore(pool), nil
	default:
		return nil, fmt.Errorf("unsupported TRACE_STORE %q", cfg.TraceStore)
	}
}

func newPromptBuilderFromConfig(cfg Config) (*promptbuilder.Builder, error) {
	systemPrompt := ""
	if cfg.PromptSystemFile != "" {
		content, err := os.ReadFile(cfg.PromptSystemFile)
		if err != nil {
			return nil, fmt.Errorf("read PROMPT_SYSTEM_FILE: %w", err)
		}
		systemPrompt = string(content)
	}

	return promptbuilder.NewBuilder(promptbuilder.Config{
		SystemPrompt:    systemPrompt,
		MaxHistoryTurns: cfg.PromptMaxHistoryTurns,
		MaxMemories:     cfg.PromptMaxMemories,
		MaxPromptChars:  cfg.PromptMaxChars,
	}), nil
}

func defaultPromptBuilder() *promptbuilder.Builder {
	return promptbuilder.NewBuilder(promptbuilder.Config{})
}

func newStoresFromConfig(cfg Config) (memory.Store, conversation.Store, debugtrace.Store, error) {
	switch strings.ToLower(cfg.MemoryStore) {
	case "", "local":
		traceStore, err := newTraceStoreFromConfig(cfg, nil)
		if err != nil {
			return nil, nil, nil, err
		}
		return memory.NewLocalFileStore(cfg.LocalDataPath), conversation.NewLocalFileStore(cfg.LocalMessagesPath), traceStore, nil
	case "memory", "inmemory", "in-memory":
		traceStore, err := newTraceStoreFromConfig(cfg, nil)
		if err != nil {
			return nil, nil, nil, err
		}
		return memory.NewInMemoryStore(), conversation.NewInMemoryStore(), traceStore, nil
	case "postgres", "pg":
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pool, err := storage.OpenPostgres(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, nil, nil, err
		}
		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			return nil, nil, nil, fmt.Errorf("ping postgres: %w", err)
		}
		traceStore, err := newTraceStoreFromConfig(cfg, pool)
		if err != nil {
			pool.Close()
			return nil, nil, nil, err
		}
		return memory.NewPostgresStore(pool), conversation.NewPostgresStore(pool), traceStore, nil
	default:
		return nil, nil, nil, fmt.Errorf("unsupported MEMORY_STORE %q", cfg.MemoryStore)
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

func (s *AgentService) ListMessages(ctx context.Context, req ListMessagesRequest) (ListMessagesResponse, error) {
	if req.UserID == "" {
		req.UserID = "anonymous"
	}
	if req.SessionID == "" {
		req.SessionID = "default"
	}
	if req.Turns <= 0 {
		req.Turns = 5
	}
	if req.Turns > 50 {
		req.Turns = 50
	}

	limit := req.Turns * 2
	messages, err := s.conversationStore.ListMessages(ctx, conversation.ListMessagesQuery{
		UserID:    req.UserID,
		SessionID: req.SessionID,
		BeforeID:  req.BeforeID,
		Limit:     limit + 1,
	})
	if err != nil {
		return ListMessagesResponse{}, observability.Wrap(err, "conversation_list_failed", "list conversation messages failed", "user_id", req.UserID, "session_id", req.SessionID)
	}

	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[1:]
	}

	resp := ListMessagesResponse{
		Messages: make([]ConversationMessage, 0, len(messages)),
		HasMore:  hasMore,
	}
	if len(messages) > 0 {
		resp.NextBeforeID = messages[0].ID
	}
	for _, message := range messages {
		resp.Messages = append(resp.Messages, ConversationMessage{
			ID:        message.ID,
			UserID:    message.UserID,
			SessionID: message.SessionID,
			Role:      message.Role,
			Content:   message.Content,
			Status:    message.Status,
			CreatedAt: message.CreatedAt,
			UpdatedAt: message.UpdatedAt,
		})
	}
	return resp, nil
}

func (s *AgentService) GetPromptTrace(ctx context.Context, traceID string) (PromptTraceResponse, error) {
	trace, ok, err := s.traceStore.Get(ctx, traceID)
	if err != nil {
		return PromptTraceResponse{}, observability.Wrap(err, "trace_load_failed", "load prompt trace failed", "trace_id", traceID)
	}
	if !ok {
		return PromptTraceResponse{}, observability.NewError("trace_not_found", "prompt trace not found", "trace_id", traceID)
	}

	return promptTraceResponse(trace), nil
}

func (s *AgentService) ReconstructPrompt(ctx context.Context, traceID string) (ReconstructedPromptResponse, error) {
	trace, ok, err := s.traceStore.Get(ctx, traceID)
	if err != nil {
		return ReconstructedPromptResponse{}, observability.Wrap(err, "trace_load_failed", "load prompt trace failed", "trace_id", traceID)
	}
	if !ok {
		return ReconstructedPromptResponse{}, observability.NewError("trace_not_found", "prompt trace not found", "trace_id", traceID)
	}

	reconstructed := debugtrace.ReconstructPrompt(trace)
	return ReconstructedPromptResponse{
		TraceID:     reconstructed.TraceID,
		Prompt:      reconstructed.Prompt,
		PromptChars: reconstructed.PromptChars,
		Source:      reconstructed.Source,
	}, nil
}

func (s *AgentService) BuildTokenReport(ctx context.Context, traceID string) (TokenReportResponse, error) {
	trace, ok, err := s.traceStore.Get(ctx, traceID)
	if err != nil {
		return TokenReportResponse{}, observability.Wrap(err, "trace_load_failed", "load prompt trace failed", "trace_id", traceID)
	}
	if !ok {
		return TokenReportResponse{}, observability.NewError("trace_not_found", "prompt trace not found", "trace_id", traceID)
	}

	report := debugtrace.BuildTokenReport(trace)
	resp := TokenReportResponse{
		TraceID:               report.TraceID,
		Prompt:                report.Prompt,
		PromptChars:           report.PromptChars,
		EstimatedPromptTokens: report.EstimatedPromptTokens,
		Tokenizer:             report.Tokenizer,
		Tokens:                make([]TokenRecord, 0, len(report.Tokens)),
	}
	for _, token := range report.Tokens {
		resp.Tokens = append(resp.Tokens, TokenRecord{
			Index:   token.Index,
			Text:    token.Text,
			TokenID: token.TokenID,
		})
	}
	return resp, nil
}

func promptTraceResponse(trace debugtrace.PromptTrace) PromptTraceResponse {
	return PromptTraceResponse{
		TraceID:                trace.TraceID,
		UserID:                 trace.UserID,
		SessionID:              trace.SessionID,
		Intent:                 trace.Intent,
		ModelTask:              trace.ModelTask,
		UsedMemoryIDs:          trace.UsedMemoryIDs,
		UsedHistoryIDs:         trace.UsedHistoryIDs,
		MemoryCount:            trace.MemoryCount,
		HistoryMessageCount:    trace.HistoryMessageCount,
		PromptChars:            trace.PromptChars,
		EstimatedPromptTokens:  trace.EstimatedPromptTokens,
		PromptBuilderVersion:   trace.PromptBuilderVersion,
		SystemPromptHash:       trace.SystemPromptHash,
		PromptConfig:           trace.PromptConfig,
		Prompt:                 trace.Prompt,
		ContextItems:           traceContextItemsResponse(trace.ContextItems),
		ContextSnapshotEnabled: trace.ContextSnapshotEnabled,
		CreatedAt:              trace.CreatedAt,
	}
}

func traceContextItemsResponse(items []debugtrace.ContextItem) []TraceContextItem {
	result := make([]TraceContextItem, 0, len(items))
	for _, item := range items {
		result = append(result, TraceContextItem{
			ItemType: item.ItemType,
			SourceID: item.SourceID,
			Role:     item.Role,
			Title:    item.Title,
			Content:  item.Content,
			Ordinal:  item.Ordinal,
			Metadata: item.Metadata,
		})
	}
	return result
}

func (s *AgentService) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatStreamEvent, <-chan error) {
	events := make(chan ChatStreamEvent)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)

		ctx = observability.EnsureTraceID(ctx)
		traceID := observability.TraceID(ctx)
		startedAt := time.Now()

		if strings.TrimSpace(req.Message) == "" {
			errs <- observability.NewError("invalid_request", "message is required")
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
			errs <- observability.Wrap(err, "conversation_create_user_failed", "create user message failed", "user_id", req.UserID, "session_id", req.SessionID)
			return
		}

		assistantMessage, err := s.conversationStore.CreateMessage(ctx, conversation.Message{
			UserID:    req.UserID,
			SessionID: req.SessionID,
			Role:      "assistant",
			Status:    "streaming",
		})
		if err != nil {
			errs <- observability.Wrap(err, "conversation_create_assistant_failed", "create assistant message failed", "user_id", req.UserID, "session_id", req.SessionID)
			return
		}

		// 对话主链路只在边界打日志：开始、模型完成、记忆提取完成/失败。
		// 内部函数通过 AppError 补充上下文，避免多层重复打印同一个错误。
		slog.InfoContext(ctx, "chat stream started",
			"trace_id", traceID,
			"user_id", req.UserID,
			"session_id", req.SessionID,
			"intent", detectedIntent,
		)

		events <- ChatStreamEvent{
			Type:      "agent.started",
			TraceID:   traceID,
			UserID:    req.UserID,
			SessionID: req.SessionID,
			Intent:    string(detectedIntent),
			Timestamp: time.Now(),
		}

		memories, err := s.memoryStore.Load(ctx, req.UserID, req.SessionID)
		if err != nil {
			errs <- observability.Wrap(err, "memory_load_failed", "load memories failed", "user_id", req.UserID, "session_id", req.SessionID)
			return
		}

		history, err := s.conversationStore.ListMessages(ctx, conversation.ListMessagesQuery{
			UserID:    req.UserID,
			SessionID: req.SessionID,
			Limit:     s.promptBuilder.MaxHistoryMessages() + 2,
		})
		if err != nil {
			errs <- observability.Wrap(err, "conversation_history_load_failed", "load conversation history failed", "user_id", req.UserID, "session_id", req.SessionID)
			return
		}

		promptResult := s.promptBuilder.Build(promptbuilder.BuildRequest{
			UserInput: req.Message,
			Memories:  memories,
			History:   filterPromptHistory(history, userMessage.ID, assistantMessage.ID),
		})
		modelTask := skillTaskForIntent(detectedIntent)
		s.savePromptTrace(ctx, traceID, req.UserID, req.SessionID, detectedIntent, modelTask, promptResult)
		slog.InfoContext(ctx, "prompt context built",
			"trace_id", traceID,
			"user_id", req.UserID,
			"session_id", req.SessionID,
			"intent", detectedIntent,
			"memory_count", len(memories),
			"used_memory_count", promptResult.MemoryCount,
			"history_message_count", len(history),
			"used_history_count", promptResult.HistoryMessageCount,
			"prompt_chars", promptResult.PromptChars,
		)

		chunks, streamErrs := s.modelRouter.GenerateStream(ctx, model.Request{
			TraceID: traceID,
			Task:    modelTask,
			Prompt:  promptResult.Prompt,
		})

		var answer strings.Builder
		var modelMetadata model.ResponseMetadata
		for chunk := range chunks {
			if chunk.Metadata.Provider != "" || chunk.Metadata.Model != "" {
				modelMetadata = chunk.Metadata
			}
			if chunk.Text != "" {
				answer.WriteString(chunk.Text)
				events <- ChatStreamEvent{
					Type:      "agent.delta",
					TraceID:   traceID,
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
			wrapped := observability.Wrap(err, "model_stream_failed", "model stream failed", "user_id", req.UserID, "session_id", req.SessionID, "intent", detectedIntent)
			errs <- wrapped
			return
		}

		answerText := answer.String()
		if err := s.conversationStore.CompleteMessage(ctx, assistantMessage.ID, answerText); err != nil {
			errs <- observability.Wrap(err, "conversation_complete_assistant_failed", "complete assistant message failed", "message_id", assistantMessage.ID)
			return
		}

		slog.InfoContext(ctx, "model stream completed",
			"trace_id", traceID,
			"user_id", req.UserID,
			"session_id", req.SessionID,
			"intent", detectedIntent,
			"provider", modelMetadata.Provider,
			"model", modelMetadata.Model,
			"model_task", modelMetadata.Task,
			"model_latency_ms", modelMetadata.LatencyMS,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"answer_chars", len([]rune(answerText)),
		)

		events <- ChatStreamEvent{
			Type:      "agent.completed",
			TraceID:   traceID,
			UserID:    req.UserID,
			SessionID: req.SessionID,
			Intent:    string(detectedIntent),
			Answer:    answerText,
			Timestamp: time.Now(),
		}

		s.saveMemoryAfterCompletion(ctx, req.UserID, req.SessionID, req.Message, detectedIntent, answerText, sourceMessageIDs(userMessage.ID, assistantMessage.ID))
	}()

	return events, errs
}

func (s *AgentService) saveMemoryAfterCompletion(parentCtx context.Context, userID string, sessionID string, input string, detectedIntent state.Intent, answer string, sourceIDs []int64) {
	// TODO: 提高 LLM 记忆提取稳定性：增加重试、JSON repair 和结构化校验错误统计。
	// TODO: 支持流式输出过程中的 periodic checkpoint，降低模型生成中途崩溃时的回答丢失窗口。
	// TODO: 引入持久化后台任务队列后，将这里改为 enqueue，避免请求 goroutine 承担 memory 后处理。
	startedAt := time.Now()
	parentCtx = observability.EnsureTraceID(parentCtx)
	// 记忆提取发生在用户已收到 completed 之后，保留 trace 但不再受请求取消影响。
	parentCtx = context.WithoutCancel(parentCtx)
	ctx, cancel := context.WithTimeout(context.Background(), s.memoryExtractTTL)
	defer cancel()
	ctx = observability.WithTraceID(ctx, observability.TraceID(parentCtx))

	slog.InfoContext(ctx, "memory extraction started",
		"trace_id", observability.TraceID(ctx),
		"user_id", userID,
		"session_id", sessionID,
		"intent", detectedIntent,
		"timeout_ms", s.memoryExtractTTL.Milliseconds(),
		"source_message_ids", sourceIDs,
	)

	entries, err := s.memoryExtractor.Extract(ctx, memory.ExtractRequest{
		UserID:           userID,
		SessionID:        sessionID,
		Input:            input,
		Answer:           answer,
		SourceMessageIDs: sourceIDs,
	})
	if err != nil {
		wrapped := observability.Wrap(err, "memory_extract_failed", "extract memory failed", "user_id", userID, "session_id", sessionID, "intent", detectedIntent)
		observability.LogError(ctx, slog.Default(), "memory extraction failed", wrapped, "duration_ms", time.Since(startedAt).Milliseconds())
		return
	}

	upserted := 0
	for _, entry := range entries {
		if err := s.memoryStore.Upsert(ctx, entry); err != nil {
			wrapped := observability.Wrap(err, "memory_upsert_failed", "upsert extracted memory failed", "type", entry.Type, "title", entry.Title)
			observability.LogError(ctx, slog.Default(), "memory upsert failed", wrapped)
			continue
		}
		upserted++
	}

	slog.InfoContext(ctx, "memory extraction completed",
		"trace_id", observability.TraceID(ctx),
		"user_id", userID,
		"session_id", sessionID,
		"intent", detectedIntent,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"extracted_count", len(entries),
		"upserted_count", upserted,
	)
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

func filterPromptHistory(history []conversation.Message, excludedIDs ...string) []conversation.Message {
	excluded := map[string]struct{}{}
	for _, id := range excludedIDs {
		if id != "" {
			excluded[id] = struct{}{}
		}
	}

	result := make([]conversation.Message, 0, len(history))
	for _, message := range history {
		if _, ok := excluded[message.ID]; ok {
			continue
		}
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		result = append(result, message)
	}
	return result
}

func (s *AgentService) savePromptTrace(ctx context.Context, traceID string, userID string, sessionID string, detectedIntent state.Intent, modelTask model.Task, promptResult promptbuilder.BuildResult) {
	promptText := ""
	if s.debugPrompt {
		promptText = promptResult.Prompt
	}
	contextItems := []debugtrace.ContextItem{}
	if s.traceSnapshot {
		contextItems = toDebugTraceContextItems(promptResult.ContextItems)
	}
	estimatedTokens := 0
	if s.traceTokenEstimate {
		estimatedTokens = debugtrace.EstimateTokens(promptResult.Prompt)
	}

	// TODO: 线上环境必须给 debug trace 接口加认证和权限校验；prompt 可能包含用户隐私。
	// TODO: 当前 trace store 是进程内 ring buffer；后续可接入 TTL、持久化或 OpenTelemetry span。
	err := s.traceStore.Save(ctx, debugtrace.PromptTrace{
		TraceID:                traceID,
		UserID:                 userID,
		SessionID:              sessionID,
		Intent:                 string(detectedIntent),
		ModelTask:              string(modelTask),
		UsedMemoryIDs:          promptResult.UsedMemoryIDs,
		UsedHistoryIDs:         promptResult.UsedHistoryIDs,
		MemoryCount:            promptResult.MemoryCount,
		HistoryMessageCount:    promptResult.HistoryMessageCount,
		PromptChars:            promptResult.PromptChars,
		EstimatedPromptTokens:  estimatedTokens,
		PromptBuilderVersion:   debugtrace.PromptBuilderVersion,
		SystemPromptHash:       debugtrace.HashText(s.promptBuilder.SystemPrompt()),
		PromptConfig:           s.promptBuilder.ConfigSnapshot(),
		Prompt:                 promptText,
		ContextItems:           contextItems,
		ContextSnapshotEnabled: s.traceSnapshot,
		CreatedAt:              time.Now(),
	})
	if err != nil {
		wrapped := observability.Wrap(err, "prompt_trace_save_failed", "save prompt trace failed", "trace_id", traceID)
		observability.LogError(ctx, slog.Default(), "prompt trace save failed", wrapped)
	}
}

func toDebugTraceContextItems(items []promptbuilder.ContextItem) []debugtrace.ContextItem {
	result := make([]debugtrace.ContextItem, 0, len(items))
	for _, item := range items {
		result = append(result, debugtrace.ContextItem{
			ItemType: item.ItemType,
			SourceID: item.SourceID,
			Role:     item.Role,
			Title:    item.Title,
			Content:  item.Content,
			Ordinal:  item.Ordinal,
			Metadata: item.Metadata,
		})
	}
	return result
}

func skillTaskForIntent(intent state.Intent) model.Task {
	switch intent {
	case state.IntentLearningPlan:
		return model.TaskLearningPlan
	case state.IntentPractice:
		return model.TaskPractice
	case state.IntentReview:
		return model.TaskReview
	default:
		return model.TaskQA
	}
}
