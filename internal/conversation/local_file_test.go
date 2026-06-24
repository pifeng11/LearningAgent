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

	messages, err := store.ListMessages(context.Background(), ListMessagesQuery{
		UserID:    "u1",
		SessionID: "s1",
		Limit:     10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected one latest message, got %d", len(messages))
	}
	if messages[0].UserID != "u1" || messages[0].SessionID != "s1" {
		t.Fatalf("expected completed message to keep user and session, got %+v", messages[0])
	}
	if messages[0].Content != "hello" {
		t.Fatalf("expected completed content, got %q", messages[0].Content)
	}
}
