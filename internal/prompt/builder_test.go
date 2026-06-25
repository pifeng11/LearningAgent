package prompt

import (
	"strings"
	"testing"
	"time"

	"learning-agent/internal/conversation"
	"learning-agent/internal/memory"
)

func TestBuilderBuildsLayeredPrompt(t *testing.T) {
	builder := NewBuilder(Config{
		SystemPrompt:    "系统提示",
		MaxHistoryTurns: 2,
		MaxMemories:     3,
		MaxPromptChars:  4000,
	})

	result := builder.Build(BuildRequest{
		UserInput: "我现在应该学什么？",
		Memories: []memory.Entry{
			{ID: 1, Type: memory.TypeGoal, Scope: memory.ScopeUser, Title: "目标", Content: "用户想学习 Kubernetes。"},
		},
		History: []conversation.Message{
			{ID: "h1", Role: "user", Content: "我熟悉 Go。"},
			{ID: "h2", Role: "assistant", Content: "可以结合 Kubernetes 学。"},
		},
	})

	if !strings.Contains(result.Prompt, "# System Instruction\n系统提示") {
		t.Fatalf("expected system prompt, got %q", result.Prompt)
	}
	if !strings.Contains(result.Prompt, "用户想学习 Kubernetes") {
		t.Fatalf("expected memory in prompt, got %q", result.Prompt)
	}
	if !strings.Contains(result.Prompt, "User: 我熟悉 Go。") {
		t.Fatalf("expected history in prompt, got %q", result.Prompt)
	}
	if !strings.HasSuffix(result.Prompt, "# Current User Input\n我现在应该学什么？") {
		t.Fatalf("expected current input at end, got %q", result.Prompt)
	}
	if result.MemoryCount != 1 || result.HistoryMessageCount != 2 {
		t.Fatalf("unexpected counts: %+v", result)
	}
}

func TestBuilderLimitsHistoryAndMemories(t *testing.T) {
	now := time.Now()
	builder := NewBuilder(Config{
		SystemPrompt:    "系统提示",
		MaxHistoryTurns: 1,
		MaxMemories:     2,
		MaxPromptChars:  4000,
	})

	result := builder.Build(BuildRequest{
		UserInput: "继续",
		Memories: []memory.Entry{
			{ID: 1, Type: memory.TypeSummary, Title: "摘要1", Content: "summary 1", UpdatedAt: now.Add(-time.Hour)},
			{ID: 2, Type: memory.TypeSummary, Title: "摘要2", Content: "summary 2", UpdatedAt: now},
			{ID: 3, Type: memory.TypeGoal, Title: "目标", Content: "goal", UpdatedAt: now.Add(-2 * time.Hour)},
		},
		History: []conversation.Message{
			{ID: "h1", Role: "user", Content: "old user"},
			{ID: "h2", Role: "assistant", Content: "old assistant"},
			{ID: "h3", Role: "user", Content: "new user"},
			{ID: "h4", Role: "assistant", Content: "new assistant"},
		},
	})

	if strings.Contains(result.Prompt, "old user") {
		t.Fatalf("expected old history to be trimmed, got %q", result.Prompt)
	}
	if !strings.Contains(result.Prompt, "new user") || !strings.Contains(result.Prompt, "new assistant") {
		t.Fatalf("expected newest turn, got %q", result.Prompt)
	}
	if result.MemoryCount != 2 {
		t.Fatalf("expected two memories, got %d", result.MemoryCount)
	}
	if !strings.Contains(result.Prompt, "goal") {
		t.Fatalf("expected goal memory to be prioritized, got %q", result.Prompt)
	}
}

func TestBuilderTrimsToBudgetKeepingCurrentInput(t *testing.T) {
	builder := NewBuilder(Config{
		SystemPrompt:    strings.Repeat("系统", 100),
		MaxHistoryTurns: 2,
		MaxMemories:     2,
		MaxPromptChars:  80,
	})

	result := builder.Build(BuildRequest{
		UserInput: "当前问题必须保留",
		History: []conversation.Message{
			{ID: "h1", Role: "user", Content: strings.Repeat("历史", 100)},
		},
	})

	if result.PromptChars > 80 {
		t.Fatalf("expected prompt within budget, got %d", result.PromptChars)
	}
	if !strings.Contains(result.Prompt, "当前问题必须保留") {
		t.Fatalf("expected current input to be retained, got %q", result.Prompt)
	}
}
