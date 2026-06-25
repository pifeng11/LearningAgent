package debugtrace

import (
	"context"
	"strings"
	"testing"
)

func TestRingStoreEvictsOldestTrace(t *testing.T) {
	store := NewRingStore(2)
	if err := store.Save(context.Background(), PromptTrace{TraceID: "t1"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), PromptTrace{TraceID: "t2"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), PromptTrace{TraceID: "t3"}); err != nil {
		t.Fatal(err)
	}

	if _, ok, err := store.Get(context.Background(), "t1"); err != nil || ok {
		t.Fatal("expected oldest trace to be evicted")
	}
	if _, ok, err := store.Get(context.Background(), "t2"); err != nil || !ok {
		t.Fatal("expected t2 to remain")
	}
	if _, ok, err := store.Get(context.Background(), "t3"); err != nil || !ok {
		t.Fatal("expected t3 to remain")
	}
}

func TestNoopStoreDoesNotSaveTrace(t *testing.T) {
	store := NoopStore{}
	if err := store.Save(context.Background(), PromptTrace{TraceID: "t1"}); err != nil {
		t.Fatal(err)
	}

	if _, ok, err := store.Get(context.Background(), "t1"); err != nil || ok {
		t.Fatal("expected noop store to ignore traces")
	}
}

func TestReconstructPromptFromContextItems(t *testing.T) {
	trace := PromptTrace{
		TraceID: "t1",
		ContextItems: []ContextItem{
			{ItemType: "system_prompt", Content: "系统", Ordinal: 0},
			{ItemType: "memory", Title: "目标", Content: "学 Go", Ordinal: 1, Metadata: map[string]any{"type": "goal", "scope": "user"}},
			{ItemType: "history", Role: "User", Content: "你好", Ordinal: 2},
			{ItemType: "current_input", Content: "继续", Ordinal: 3},
		},
	}

	reconstructed := ReconstructPrompt(trace)

	if reconstructed.Source != "context_snapshot" {
		t.Fatalf("expected context snapshot source, got %s", reconstructed.Source)
	}
	if !strings.Contains(reconstructed.Prompt, "# Long-term Memory") {
		t.Fatalf("expected memory section, got %q", reconstructed.Prompt)
	}
	if !strings.HasSuffix(reconstructed.Prompt, "# Current User Input\n继续") {
		t.Fatalf("expected current input at end, got %q", reconstructed.Prompt)
	}
}

func TestTokenReportUsesEstimate(t *testing.T) {
	report := BuildTokenReport(PromptTrace{
		TraceID: "t1",
		Prompt:  strings.Repeat("a", 9),
	})

	if report.Tokenizer != "estimate" {
		t.Fatalf("expected estimate tokenizer, got %s", report.Tokenizer)
	}
	if report.EstimatedPromptTokens != 3 {
		t.Fatalf("expected estimated tokens, got %d", report.EstimatedPromptTokens)
	}
}
