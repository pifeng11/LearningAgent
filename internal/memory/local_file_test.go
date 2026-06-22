package memory

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLocalFileStoreSavesAndLoadsEntries(t *testing.T) {
	store := NewLocalFileStore(filepath.Join(t.TempDir(), "memories.jsonl"))

	if err := store.Save(context.Background(), Entry{
		UserID:    "u1",
		SessionID: "s1",
		Scope:     ScopeShortTerm,
		Content:   "hello",
	}); err != nil {
		t.Fatal(err)
	}

	entries, err := store.Load(context.Background(), "u1", "s1")
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Content != "hello" {
		t.Fatalf("expected hello, got %q", entries[0].Content)
	}
}
