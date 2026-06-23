package memory

import (
	"context"
	"testing"
)

func TestRuleExtractorExtractsGoal(t *testing.T) {
	extractor := NewRuleExtractor()

	entries, err := extractor.Extract(context.Background(), ExtractRequest{
		UserID:           "u1",
		SessionID:        "s1",
		Input:            "I want to learn Go language",
		Answer:           "ok",
		SourceMessageIDs: []int64{1, 2},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Type != TypeGoal {
		t.Fatalf("expected %s, got %s", TypeGoal, entries[0].Type)
	}
	if entries[0].Scope != ScopeUser {
		t.Fatalf("expected %s, got %s", ScopeUser, entries[0].Scope)
	}
	if len(entries[0].SourceMessageIDs) != 2 {
		t.Fatalf("expected source ids")
	}
}

func TestRuleExtractorFallsBackToSummary(t *testing.T) {
	extractor := NewRuleExtractor()

	entries, err := extractor.Extract(context.Background(), ExtractRequest{
		UserID:    "u1",
		SessionID: "s1",
		Input:     "hello",
		Answer:    "hi",
	})
	if err != nil {
		t.Fatal(err)
	}

	if entries[0].Type != TypeSummary {
		t.Fatalf("expected %s, got %s", TypeSummary, entries[0].Type)
	}
}
