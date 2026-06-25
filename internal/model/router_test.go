package model

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRouterRoutesTaskModel(t *testing.T) {
	provider := &recordingProvider{name: "mock"}
	router := NewRouterWithConfig([]Provider{provider}, RouterConfig{
		DefaultRoute: Route{Provider: "mock", Capability: CapabilityChat, Model: "flash"},
		TaskRoutes: map[Task]Route{
			TaskLearningPlan: {Model: "pro"},
		},
	})

	resp, err := router.Generate(context.Background(), Request{Task: TaskLearningPlan, Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	if provider.lastRequest.Model != "pro" {
		t.Fatalf("expected routed model pro, got %s", provider.lastRequest.Model)
	}
	if resp.Metadata.Model != "pro" || resp.Metadata.Provider != "mock" {
		t.Fatalf("expected response metadata, got %+v", resp.Metadata)
	}
}

func TestRouterRetriesRetryableError(t *testing.T) {
	provider := &recordingProvider{
		name:      "mock",
		failCount: 1,
		failErr:   &ModelError{Code: "model_unavailable", Retryable: true, Cause: errors.New("temporary")},
	}
	router := NewRouterWithConfig([]Provider{provider}, RouterConfig{
		DefaultRoute:   Route{Provider: "mock", Capability: CapabilityChat, Model: "flash"},
		MaxRetries:     1,
		RetryBackoff:   time.Nanosecond,
		DefaultTimeout: time.Second,
	})

	resp, err := router.Generate(context.Background(), Request{Task: TaskQA, Prompt: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	if provider.calls != 2 {
		t.Fatalf("expected two calls, got %d", provider.calls)
	}
	if resp.Metadata.RetryCount != 1 {
		t.Fatalf("expected retry count 1, got %d", resp.Metadata.RetryCount)
	}
}

type recordingProvider struct {
	name        string
	calls       int
	failCount   int
	failErr     error
	lastRequest Request
}

func (p *recordingProvider) Name() string {
	return p.name
}

func (p *recordingProvider) Chat(ctx context.Context, req Request) (Response, error) {
	p.calls++
	p.lastRequest = req
	if p.calls <= p.failCount {
		return Response{}, p.failErr
	}
	return ResponseFromText("ok", ResponseMetadata{}, Usage{Estimated: true}), nil
}

func (p *recordingProvider) ChatStream(ctx context.Context, req Request) (<-chan StreamEvent, <-chan error) {
	events := make(chan StreamEvent, 1)
	errs := make(chan error, 1)
	events <- StreamEvent{Type: "completed", Done: true}
	close(events)
	close(errs)
	return events, errs
}
