package conversation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalFileStoreCreatesAndCompletesMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages.jsonl")
	store := NewLocalFileStore(path)

	message, err := store.CreateMessage(context.Background(), Message{
		UserID:    "u1",
		SessionID: "s1",
		Role:      "assistant",
		Status:    "streaming",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.CompleteMessage(context.Background(), message.ID, "hello"); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "streaming") {
		t.Fatalf("expected streaming event in %s", string(content))
	}
	if !strings.Contains(string(content), "hello") {
		t.Fatalf("expected completed content in %s", string(content))
	}
}
