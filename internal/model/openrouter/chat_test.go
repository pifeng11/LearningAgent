package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"learning-agent/internal/model"
)

func TestProviderUsesOpenRouterHeadersAndUsage(t *testing.T) {
	var got chatRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assertRequest(t, r)
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected authorization header")
		}
		if r.Header.Get("HTTP-Referer") != "https://example.app" {
			t.Fatalf("unexpected referer header")
		}
		if r.Header.Get("X-OpenRouter-Title") != "LearningAgent Test" {
			t.Fatalf("unexpected title header")
		}
		if r.Header.Get("X-OpenRouter-Metadata") != "enabled" {
			t.Fatalf("expected metadata header")
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		return jsonResponse(`{"model":"deepseek/deepseek-chat","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`), nil
	})}

	provider, err := NewProvider(Config{
		APIKey:          "test-key",
		BaseURL:         "https://example.test/api/v1",
		Model:           "deepseek/deepseek-chat",
		SiteURL:         "https://example.app",
		AppTitle:        "LearningAgent Test",
		MetadataEnabled: true,
		HTTPClient:      client,
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := provider.Chat(context.Background(), model.Request{
		TraceID: "trace-1",
		Task:    model.TaskQA,
		Prompt:  "hello",
		Metadata: map[string]any{
			model.RequestMetadataUserID:    "u1",
			model.RequestMetadataSessionID: "s1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Text != "ok" {
		t.Fatalf("expected ok, got %q", resp.Text)
	}
	if got.Model != "deepseek/deepseek-chat" {
		t.Fatalf("expected routed model, got %s", got.Model)
	}
	if got.User != "u1" || got.SessionID != "s1" {
		t.Fatalf("expected user/session metadata, got %+v", got)
	}
	if got.Trace["trace_id"] != "trace-1" {
		t.Fatalf("expected trace metadata, got %+v", got.Trace)
	}
	if resp.Usage.TotalTokens != 10 || resp.Usage.Estimated {
		t.Fatalf("expected real usage, got %+v", resp.Usage)
	}
}

func TestProviderStreamsDeltasAndUsage(t *testing.T) {
	var got chatRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assertRequest(t, r)
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		return sseResponse(strings.Join([]string{
			`data: {"model":"deepseek/deepseek-chat","choices":[{"delta":{"content":"你"}}]}`,
			`data: {"model":"deepseek/deepseek-chat","choices":[{"delta":{"content":"好"}}]}`,
			`data: {"model":"deepseek/deepseek-chat","choices":[],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}`,
			`data: [DONE]`,
			``,
		}, "\n")), nil
	})}

	provider, err := NewProvider(Config{
		APIKey:     "test-key",
		BaseURL:    "https://example.test/api/v1",
		Model:      "deepseek/deepseek-chat",
		HTTPClient: client,
	})
	if err != nil {
		t.Fatal(err)
	}

	chunks, errs := provider.ChatStream(context.Background(), model.Request{Task: model.TaskQA, Prompt: "hello"})

	var text string
	var usage model.Usage
	for chunk := range chunks {
		text += chunk.Text
		if chunk.Done {
			usage = chunk.Usage
		}
	}
	if err := <-errs; err != nil {
		t.Fatal(err)
	}

	if !got.Stream {
		t.Fatal("expected stream=true")
	}
	if text != "你好" {
		t.Fatalf("expected 你好, got %q", text)
	}
	if usage.TotalTokens != 6 {
		t.Fatalf("expected stream usage, got %+v", usage)
	}
}

func TestProviderUsesReasoningModelForLearningPlan(t *testing.T) {
	var got chatRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assertRequest(t, r)
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		return jsonResponse(`{"choices":[{"message":{"role":"assistant","content":"plan"}}]}`), nil
	})}

	provider, err := NewProvider(Config{
		APIKey:         "test-key",
		BaseURL:        "https://example.test/api/v1",
		Model:          "deepseek/deepseek-chat",
		ReasoningModel: "deepseek/deepseek-r1",
		HTTPClient:     client,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = provider.Chat(context.Background(), model.Request{Task: model.TaskLearningPlan, Prompt: "make plan"})
	if err != nil {
		t.Fatal(err)
	}

	if got.Model != "deepseek/deepseek-r1" {
		t.Fatalf("expected reasoning model, got %s", got.Model)
	}
}

func TestProviderMapsInsufficientCredits(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusPaymentRequired,
			Body:       io.NopCloser(bytes.NewBufferString(`{"error":{"code":402,"message":"insufficient credits","metadata":{"error_type":"insufficient_credits"}}}`)),
			Header:     make(http.Header),
		}, nil
	})}

	provider, err := NewProvider(Config{
		APIKey:     "test-key",
		BaseURL:    "https://example.test/api/v1",
		HTTPClient: client,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = provider.Chat(context.Background(), model.Request{Task: model.TaskQA, Prompt: "hello"})
	if err == nil {
		t.Fatal("expected error")
	}
	var modelErr *model.ModelError
	if !errors.As(err, &modelErr) {
		t.Fatalf("expected model error, got %T", err)
	}
	if modelErr.Code != "model_insufficient_credits" || modelErr.Retryable {
		t.Fatalf("unexpected model error: %+v", modelErr)
	}
}

type roundTripFunc func(r *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func assertRequest(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Method != http.MethodPost {
		t.Fatalf("unexpected method: %s", r.Method)
	}
	if r.URL.String() != "https://example.test/api/v1/chat/completions" {
		t.Fatalf("unexpected url: %s", r.URL.String())
	}
	if r.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected content type: %s", r.Header.Get("Content-Type"))
	}
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}

func sseResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}
