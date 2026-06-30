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

func TestRouterRecordsModelCall(t *testing.T) {
	provider := &recordingProvider{name: "mock"}
	recorder := &recordingCallRecorder{}
	router := NewRouterWithConfig([]Provider{provider}, RouterConfig{
		DefaultRoute: Route{Provider: "mock", Capability: CapabilityChat, Model: "flash"},
		CallRecorder: recorder,
	})

	_, err := router.Generate(context.Background(), Request{
		TraceID: "trace-1",
		Task:    TaskQA,
		Prompt:  "hello",
		Metadata: map[string]any{
			RequestMetadataUserID:    "u1",
			RequestMetadataSessionID: "s1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(recorder.created) != 1 {
		t.Fatalf("expected one created call, got %d", len(recorder.created))
	}
	if len(recorder.completed) != 1 {
		t.Fatalf("expected one completed call, got %d", len(recorder.completed))
	}
	created := recorder.created[0]
	if created.TraceID != "trace-1" || created.UserID != "u1" || created.SessionID != "s1" {
		t.Fatalf("unexpected call identity: %+v", created)
	}
	if created.Provider != "mock" || created.Model != "flash" || created.Task != TaskQA {
		t.Fatalf("unexpected call route: %+v", created)
	}
	if recorder.completed[0].Status != ModelCallStatusCompleted {
		t.Fatalf("expected completed status, got %+v", recorder.completed[0])
	}
}

func TestRouterRecordsStreamModelCall(t *testing.T) {
	provider := &recordingProvider{name: "mock"}
	recorder := &recordingCallRecorder{}
	router := NewRouterWithConfig([]Provider{provider}, RouterConfig{
		DefaultRoute: Route{Provider: "mock", Capability: CapabilityChat, Model: "flash"},
		CallRecorder: recorder,
	})

	events, errs := router.GenerateStream(context.Background(), Request{Task: TaskQA, Prompt: "hello"})
	for range events {
	}
	if err, ok := <-errs; ok && err != nil {
		t.Fatal(err)
	}

	if len(recorder.created) != 1 || !recorder.created[0].Stream {
		t.Fatalf("expected one stream call, got %+v", recorder.created)
	}
	if len(recorder.completed) != 1 || recorder.completed[0].Status != ModelCallStatusCompleted {
		t.Fatalf("expected completed stream call, got %+v", recorder.completed)
	}
}

type recordingProvider struct {
	name        string
	calls       int
	failCount   int
	failErr     error
	lastRequest Request
}

type recordingCallRecorder struct {
	nextID    int64
	created   []ModelCall
	completed []ModelCallUpdate
}

func (r *recordingCallRecorder) CreateModelCall(ctx context.Context, call ModelCall) (ModelCall, error) {
	r.nextID++
	call.ID = r.nextID
	r.created = append(r.created, call)
	return call, nil
}

func (r *recordingCallRecorder) CompleteModelCall(ctx context.Context, id int64, update ModelCallUpdate) error {
	r.completed = append(r.completed, update)
	return nil
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
