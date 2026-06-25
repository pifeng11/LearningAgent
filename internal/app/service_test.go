package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"learning-agent/internal/agent/state"
	"learning-agent/internal/conversation"
	"learning-agent/internal/debugtrace"
	"learning-agent/internal/memory"
	"learning-agent/internal/model"
	promptbuilder "learning-agent/internal/prompt"
)

func TestAgentServiceChatRunsLearningPlan(t *testing.T) {
	service := NewAgentService()

	resp, err := service.Chat(context.Background(), ChatRequest{
		UserID:    "u1",
		SessionID: "s1",
		Message:   "帮我规划 Rust 学习路线",
	})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Intent != "learning_plan" {
		t.Fatalf("expected learning_plan, got %s", resp.Intent)
	}
	if !strings.Contains(resp.Answer, "学习计划") {
		t.Fatalf("expected answer to contain 学习计划, got %q", resp.Answer)
	}
	if len(resp.Events) == 0 {
		t.Fatal("expected execution events")
	}
}

func TestAgentServiceListMessages(t *testing.T) {
	conversationStore := conversation.NewInMemoryStore()
	service := newAgentServiceWithStores(
		model.NewRouter(model.NewMockProvider()),
		memory.NewInMemoryStore(),
		conversationStore,
		memory.NewRuleExtractor(),
		time.Second,
		promptbuilder.NewBuilder(promptbuilder.Config{}),
		debugtrace.NewRingStore(10),
		false,
		true,
		true,
	)

	_, err := conversationStore.CreateMessage(context.Background(), conversation.Message{
		UserID:    "u1",
		SessionID: "s1",
		Role:      "user",
		Content:   "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = conversationStore.CreateMessage(context.Background(), conversation.Message{
		UserID:    "u1",
		SessionID: "other",
		Role:      "user",
		Content:   "hidden",
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := service.ListMessages(context.Background(), ListMessagesRequest{
		UserID:    "u1",
		SessionID: "s1",
		Turns:     5,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.Messages) != 1 {
		t.Fatalf("expected one message, got %d", len(resp.Messages))
	}
	if resp.Messages[0].Content != "hello" {
		t.Fatalf("expected filtered message, got %+v", resp.Messages[0])
	}
}

func TestAgentServiceListMessagesUsesCursorPagination(t *testing.T) {
	conversationStore := conversation.NewInMemoryStore()
	service := newAgentServiceWithStores(
		model.NewRouter(model.NewMockProvider()),
		memory.NewInMemoryStore(),
		conversationStore,
		memory.NewRuleExtractor(),
		time.Second,
		promptbuilder.NewBuilder(promptbuilder.Config{}),
		debugtrace.NewRingStore(10),
		false,
		true,
		true,
	)

	for index, content := range []string{"m1", "m2", "m3", "m4", "m5"} {
		_, err := conversationStore.CreateMessage(context.Background(), conversation.Message{
			ID:        string(rune('1' + index)),
			UserID:    "u1",
			SessionID: "s1",
			Role:      "user",
			Content:   content,
			CreatedAt: time.Unix(int64(index+1), 0),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	firstPage, err := service.ListMessages(context.Background(), ListMessagesRequest{
		UserID:    "u1",
		SessionID: "s1",
		Turns:     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage.Messages) != 4 {
		t.Fatalf("expected four messages, got %d", len(firstPage.Messages))
	}
	if !firstPage.HasMore {
		t.Fatal("expected more messages")
	}
	if firstPage.NextBeforeID == "" {
		t.Fatal("expected next cursor")
	}

	secondPage, err := service.ListMessages(context.Background(), ListMessagesRequest{
		UserID:    "u1",
		SessionID: "s1",
		Turns:     2,
		BeforeID:  firstPage.NextBeforeID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(secondPage.Messages) != 1 {
		t.Fatalf("expected one older message, got %d", len(secondPage.Messages))
	}
	if secondPage.Messages[0].Content != "m1" {
		t.Fatalf("expected oldest message, got %+v", secondPage.Messages[0])
	}
}

func TestLoadConfigReadsMemoryExtractTimeout(t *testing.T) {
	t.Setenv("MEMORY_EXTRACT_TIMEOUT", "45s")

	cfg := LoadConfig()

	if cfg.MemoryExtractTimeout != 45*time.Second {
		t.Fatalf("expected 45s memory extract timeout, got %s", cfg.MemoryExtractTimeout)
	}
}

func TestLoadConfigFallsBackForInvalidMemoryExtractTimeout(t *testing.T) {
	t.Setenv("MEMORY_EXTRACT_TIMEOUT", "invalid")

	cfg := LoadConfig()

	if cfg.MemoryExtractTimeout != 30*time.Second {
		t.Fatalf("expected fallback memory extract timeout, got %s", cfg.MemoryExtractTimeout)
	}
}

func TestLoadConfigReadsPromptSettings(t *testing.T) {
	t.Setenv("PROMPT_MAX_HISTORY_TURNS", "7")
	t.Setenv("PROMPT_MAX_MEMORIES", "9")
	t.Setenv("PROMPT_MAX_CHARS", "16000")
	t.Setenv("PROMPT_SYSTEM_FILE", "prompts/system.zh.md")

	cfg := LoadConfig()

	if cfg.PromptMaxHistoryTurns != 7 {
		t.Fatalf("expected prompt history turns, got %d", cfg.PromptMaxHistoryTurns)
	}
	if cfg.PromptMaxMemories != 9 {
		t.Fatalf("expected prompt max memories, got %d", cfg.PromptMaxMemories)
	}
	if cfg.PromptMaxChars != 16000 {
		t.Fatalf("expected prompt max chars, got %d", cfg.PromptMaxChars)
	}
	if cfg.PromptSystemFile != "prompts/system.zh.md" {
		t.Fatalf("expected prompt system file, got %s", cfg.PromptSystemFile)
	}
}

func TestLoadConfigReadsDebugTraceSettings(t *testing.T) {
	t.Setenv("DEBUG_TRACE_ENABLED", "false")
	t.Setenv("DEBUG_TRACE_CAPACITY", "42")
	t.Setenv("DEBUG_PROMPT_ENABLED", "true")
	t.Setenv("TRACE_STORE", "postgres")
	t.Setenv("TRACE_CAPTURE_PROMPT_TEXT", "true")
	t.Setenv("TRACE_CONTEXT_SNAPSHOT", "false")
	t.Setenv("TRACE_TOKEN_ESTIMATION_ENABLED", "false")

	cfg := LoadConfig()

	if cfg.DebugTraceEnabled {
		t.Fatal("expected debug trace to be disabled")
	}
	if cfg.DebugTraceCapacity != 42 {
		t.Fatalf("expected debug trace capacity, got %d", cfg.DebugTraceCapacity)
	}
	if !cfg.DebugPromptEnabled {
		t.Fatal("expected debug prompt to be enabled")
	}
	if cfg.TraceStore != "postgres" {
		t.Fatalf("expected postgres trace store, got %s", cfg.TraceStore)
	}
	if !cfg.TraceCapturePromptText {
		t.Fatal("expected prompt text capture to be enabled")
	}
	if cfg.TraceContextSnapshot {
		t.Fatal("expected context snapshot to be disabled")
	}
	if cfg.TraceTokenEstimation {
		t.Fatal("expected token estimation to be disabled")
	}
}

func TestAgentServiceGetPromptTrace(t *testing.T) {
	traceStore := debugtrace.NewRingStore(10)
	service := newAgentServiceWithStores(
		model.NewRouter(model.NewMockProvider()),
		memory.NewInMemoryStore(),
		conversation.NewInMemoryStore(),
		memory.NewRuleExtractor(),
		time.Second,
		promptbuilder.NewBuilder(promptbuilder.Config{}),
		traceStore,
		false,
		true,
		true,
	)

	service.savePromptTrace(context.Background(), "trace-1", "u1", "s1", state.IntentQA, "qa", promptbuilder.BuildResult{
		UsedMemoryIDs:       []int64{1},
		UsedHistoryIDs:      []string{"m1"},
		ContextItems:        []promptbuilder.ContextItem{{ItemType: "current_input", Content: "hello", Ordinal: 0}},
		MemoryCount:         1,
		HistoryMessageCount: 1,
		PromptChars:         123,
		Prompt:              "secret prompt",
	})

	resp, err := service.GetPromptTrace(context.Background(), "trace-1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Prompt != "" {
		t.Fatalf("expected prompt to be hidden by default, got %q", resp.Prompt)
	}
	if resp.PromptChars != 123 || resp.MemoryCount != 1 || resp.HistoryMessageCount != 1 {
		t.Fatalf("unexpected trace response: %+v", resp)
	}
	if resp.EstimatedPromptTokens == 0 {
		t.Fatal("expected estimated tokens")
	}
	if !resp.ContextSnapshotEnabled || len(resp.ContextItems) != 1 {
		t.Fatalf("expected context snapshot, got %+v", resp)
	}
}

func TestAgentServiceGetPromptTraceCanIncludePrompt(t *testing.T) {
	service := newAgentServiceWithStores(
		model.NewRouter(model.NewMockProvider()),
		memory.NewInMemoryStore(),
		conversation.NewInMemoryStore(),
		memory.NewRuleExtractor(),
		time.Second,
		promptbuilder.NewBuilder(promptbuilder.Config{}),
		debugtrace.NewRingStore(10),
		true,
		true,
		true,
	)

	service.savePromptTrace(context.Background(), "trace-1", "u1", "s1", state.IntentQA, "qa", promptbuilder.BuildResult{
		PromptChars: 13,
		Prompt:      "visible prompt",
	})

	resp, err := service.GetPromptTrace(context.Background(), "trace-1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Prompt != "visible prompt" {
		t.Fatalf("expected visible prompt, got %q", resp.Prompt)
	}
}

func TestAgentServiceReconstructPromptAndTokenReport(t *testing.T) {
	service := newAgentServiceWithStores(
		model.NewRouter(model.NewMockProvider()),
		memory.NewInMemoryStore(),
		conversation.NewInMemoryStore(),
		memory.NewRuleExtractor(),
		time.Second,
		promptbuilder.NewBuilder(promptbuilder.Config{}),
		debugtrace.NewRingStore(10),
		false,
		true,
		true,
	)

	service.savePromptTrace(context.Background(), "trace-1", "u1", "s1", state.IntentQA, "qa", promptbuilder.BuildResult{
		ContextItems: []promptbuilder.ContextItem{
			{ItemType: "system_prompt", Content: "系统", Ordinal: 0},
			{ItemType: "current_input", Content: "你好", Ordinal: 1},
		},
		PromptChars: 30,
		Prompt:      "# System Instruction\n系统\n\n# Current User Input\n你好",
	})

	reconstructed, err := service.ReconstructPrompt(context.Background(), "trace-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reconstructed.Prompt, "# Current User Input\n你好") {
		t.Fatalf("expected reconstructed prompt, got %q", reconstructed.Prompt)
	}

	report, err := service.BuildTokenReport(context.Background(), "trace-1")
	if err != nil {
		t.Fatal(err)
	}
	if report.Tokenizer != "estimate" {
		t.Fatalf("expected estimate tokenizer, got %s", report.Tokenizer)
	}
	if report.EstimatedPromptTokens == 0 {
		t.Fatal("expected estimated tokens")
	}
}

func TestNewPromptBuilderFromConfigReadsSystemPromptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "system.md")
	if err := os.WriteFile(path, []byte("自定义系统提示"), 0o644); err != nil {
		t.Fatal(err)
	}

	builder, err := newPromptBuilderFromConfig(Config{
		PromptSystemFile:      path,
		PromptMaxHistoryTurns: 1,
		PromptMaxMemories:     1,
		PromptMaxChars:        1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	result := builder.Build(promptbuilder.BuildRequest{UserInput: "你好"})
	if !strings.Contains(result.Prompt, "自定义系统提示") {
		t.Fatalf("expected custom system prompt, got %q", result.Prompt)
	}
}
