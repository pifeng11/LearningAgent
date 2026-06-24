package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"learning-agent/internal/conversation"
	"learning-agent/internal/memory"
	"learning-agent/internal/model"
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

func TestBuildPromptWithMemoriesIncludesContext(t *testing.T) {
	prompt := buildPromptWithMemories("我想学什么？", []memory.Entry{
		{
			Type:    memory.TypeSummary,
			Title:   "Conversation turn",
			Scope:   memory.ScopeSession,
			Content: "用户之前说想学习 Go 语言。",
		},
	})

	if !strings.Contains(prompt, "用户之前说想学习 Go 语言") {
		t.Fatalf("expected prompt to include memory, got %q", prompt)
	}
	if !strings.Contains(prompt, "我想学什么？") {
		t.Fatalf("expected prompt to include input, got %q", prompt)
	}
}

func TestTruncateMemoryContent(t *testing.T) {
	got := truncateMemoryContent("一二三四五", 3)

	if got != "一二三..." {
		t.Fatalf("expected truncated content, got %q", got)
	}
}

func TestSelectPromptMemoriesLimitsSummaries(t *testing.T) {
	memories := selectPromptMemories([]memory.Entry{
		{Type: memory.TypeGoal},
		{Type: memory.TypeSummary},
		{Type: memory.TypeSummary},
		{Type: memory.TypeSummary},
	})

	if len(memories) != 3 {
		t.Fatalf("expected 3 memories, got %d", len(memories))
	}
	if memories[0].Type != memory.TypeGoal {
		t.Fatalf("expected goal to be retained")
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
	)

	for _, content := range []string{"m1", "m2", "m3", "m4", "m5"} {
		_, err := conversationStore.CreateMessage(context.Background(), conversation.Message{
			UserID:    "u1",
			SessionID: "s1",
			Role:      "user",
			Content:   content,
		})
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Nanosecond)
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
