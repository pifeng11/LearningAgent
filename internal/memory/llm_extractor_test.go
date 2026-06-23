package memory

import "testing"

func TestParseExtractedMemories(t *testing.T) {
	parsed, err := parseExtractedMemories(`{"memories":[{"type":"goal","title":"学习 Kubernetes","content":"用户想学习 Kubernetes。","scope":"user","confidence":0.9}]}`)
	if err != nil {
		t.Fatal(err)
	}

	if len(parsed.Memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(parsed.Memories))
	}
	if parsed.Memories[0].Type != TypeGoal {
		t.Fatalf("expected goal, got %s", parsed.Memories[0].Type)
	}
}

func TestExtractedMemoryRejectsSummaryFallback(t *testing.T) {
	item := extractedMemory{
		Type:       TypeSummary,
		Title:      "Conversation turn",
		Content:    "full chat",
		Scope:      ScopeSession,
		Confidence: 0.9,
	}

	_, ok := item.toEntry(ExtractRequest{UserID: "u1", SessionID: "s1"})
	if ok {
		t.Fatal("expected summary to be rejected by llm extractor")
	}
}
