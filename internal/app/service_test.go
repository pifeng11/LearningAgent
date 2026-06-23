package app

import (
	"context"
	"strings"
	"testing"

	"learning-agent/internal/memory"
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
