package observability

import (
	"errors"
	"strings"
	"testing"
)

func TestWrapKeepsErrorCodeAndUserMessage(t *testing.T) {
	err := Wrap(errors.New("network timeout"), "model_stream_failed", "model stream failed", "provider", "deepseek")

	if Code(err) != "model_stream_failed" {
		t.Fatalf("expected model_stream_failed code, got %s", Code(err))
	}
	if Message(err) != "model stream failed" {
		t.Fatalf("expected user message, got %s", Message(err))
	}
}

func TestUserErrorTextIncludesTraceID(t *testing.T) {
	ctx := WithTraceID(t.Context(), "trace-123")
	err := NewError("invalid_request", "message is required")

	got := UserErrorText(ctx, err)

	if !strings.Contains(got, "invalid_request") {
		t.Fatalf("expected code in user error text, got %q", got)
	}
	if !strings.Contains(got, "trace_id=trace-123") {
		t.Fatalf("expected trace id in user error text, got %q", got)
	}
}
