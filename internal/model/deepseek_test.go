package model

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDeepSeekProviderUsesDefaultModelForQA(t *testing.T) {
	var got deepSeekChatRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assertDeepSeekRequest(t, r)
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected authorization header")
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		return jsonResponse(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`), nil
	})}

	provider, err := NewDeepSeekProvider(DeepSeekConfig{
		APIKey:     "test-key",
		BaseURL:    "https://example.test",
		HTTPClient: client,
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := provider.Generate(context.Background(), Request{Task: "qa", Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Text != "ok" {
		t.Fatalf("expected ok, got %q", resp.Text)
	}
	if got.Model != DeepSeekV4Flash {
		t.Fatalf("expected %s, got %s", DeepSeekV4Flash, got.Model)
	}
	if got.ReasoningEffort != "" {
		t.Fatalf("expected no reasoning effort for qa, got %q", got.ReasoningEffort)
	}
}

func TestDeepSeekProviderStreamsDeltas(t *testing.T) {
	var got deepSeekChatRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assertDeepSeekRequest(t, r)
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		return streamResponse(strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"你"}}]}`,
			`data: {"choices":[{"delta":{"content":"好"}}]}`,
			`data: [DONE]`,
			``,
		}, "\n")), nil
	})}

	provider, err := NewDeepSeekProvider(DeepSeekConfig{
		APIKey:     "test-key",
		BaseURL:    "https://example.test",
		HTTPClient: client,
	})
	if err != nil {
		t.Fatal(err)
	}

	chunks, errs := provider.GenerateStream(context.Background(), Request{Task: "qa", Prompt: "hello"})

	var text string
	for chunk := range chunks {
		text += chunk.Text
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
}

func TestDeepSeekProviderUsesReasoningModelForLearningPlan(t *testing.T) {
	var got deepSeekChatRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assertDeepSeekRequest(t, r)
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		return jsonResponse(`{"choices":[{"message":{"role":"assistant","content":"plan"}}]}`), nil
	})}

	provider, err := NewDeepSeekProvider(DeepSeekConfig{
		APIKey:          "test-key",
		BaseURL:         "https://example.test",
		ReasoningEffort: "high",
		ThinkingEnabled: true,
		HTTPClient:      client,
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := provider.Generate(context.Background(), Request{Task: "learning_plan", Prompt: "make plan"})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Text != "plan" {
		t.Fatalf("expected plan, got %q", resp.Text)
	}
	if got.Model != DeepSeekV4Pro {
		t.Fatalf("expected %s, got %s", DeepSeekV4Pro, got.Model)
	}
	if got.ReasoningEffort != "high" {
		t.Fatalf("expected high reasoning effort, got %q", got.ReasoningEffort)
	}
	if got.Thinking == nil || got.Thinking.Type != "enabled" {
		t.Fatalf("expected thinking enabled")
	}
}

type roundTripFunc func(r *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func assertDeepSeekRequest(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Method != http.MethodPost {
		t.Fatalf("unexpected method: %s", r.Method)
	}
	if r.URL.String() != "https://example.test/chat/completions" {
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

func streamResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}
