package model

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

type Route struct {
	Provider   string
	Capability CapabilityKind
	Model      string
}

type RouterConfig struct {
	DefaultRoute   Route
	TaskRoutes     map[Task]Route
	DefaultTimeout time.Duration
	StreamTimeout  time.Duration
	MaxRetries     int
	RetryBackoff   time.Duration
	CallRecorder   ModelCallRecorder
}

type Router struct {
	providers map[string]Provider
	config    RouterConfig
	recorder  ModelCallRecorder
}

func NewRouter(defaultProvider Provider) *Router {
	return NewRouterWithConfig([]Provider{defaultProvider}, RouterConfig{
		DefaultRoute: Route{
			Provider:   defaultProvider.Name(),
			Capability: CapabilityChat,
		},
		DefaultTimeout: 60 * time.Second,
		StreamTimeout:  120 * time.Second,
		MaxRetries:     0,
		RetryBackoff:   500 * time.Millisecond,
	})
}

func NewRouterWithConfig(providers []Provider, config RouterConfig) *Router {
	if config.DefaultRoute.Capability == "" {
		config.DefaultRoute.Capability = CapabilityChat
	}
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = 60 * time.Second
	}
	if config.StreamTimeout <= 0 {
		config.StreamTimeout = 120 * time.Second
	}
	if config.RetryBackoff <= 0 {
		config.RetryBackoff = 500 * time.Millisecond
	}
	registry := map[string]Provider{}
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		registry[provider.Name()] = provider
		if config.DefaultRoute.Provider == "" {
			config.DefaultRoute.Provider = provider.Name()
		}
	}
	recorder := config.CallRecorder
	if recorder == nil {
		recorder = NoopModelCallRecorder{}
	}
	return &Router{providers: registry, config: config, recorder: recorder}
}

func (r *Router) Generate(ctx context.Context, req Request) (Response, error) {
	route, provider, err := r.resolve(req)
	if err != nil {
		return Response{}, err
	}
	req = r.applyRoute(req, route)
	startedAt := time.Now()
	call := r.startModelCall(ctx, req, route, false, ModelCallStatusRunning, startedAt)

	var lastErr error
	lastAttempt := 0
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		lastAttempt = attempt
		callCtx, cancel := context.WithTimeout(ctx, r.timeoutFor(req, false))
		attemptStartedAt := time.Now()
		resp, err := provider.Chat(callCtx, req)
		cancel()
		if err == nil {
			resp.Metadata.Provider = route.Provider
			resp.Metadata.Model = req.Model
			resp.Metadata.Task = req.Task
			resp.Metadata.Capability = req.Capability
			resp.Metadata.TraceID = req.TraceID
			resp.Metadata.RetryCount = attempt
			resp.Metadata.LatencyMS = time.Since(attemptStartedAt).Milliseconds()
			r.completeModelCall(ctx, call.ID, ModelCallUpdate{
				Status:           ModelCallStatusCompleted,
				Usage:            resp.Usage,
				LatencyMS:        time.Since(startedAt).Milliseconds(),
				RetryCount:       attempt,
				ResponseMetadata: responseMetadataMap(resp.Metadata),
				CompletedAt:      time.Now(),
			})
			return resp, nil
		}
		lastErr = classifyError(err, route.Provider, req.Model)
		if attempt >= r.config.MaxRetries || !IsRetryable(lastErr) {
			break
		}
		sleepWithContext(ctx, r.config.RetryBackoff*time.Duration(attempt+1))
	}
	r.completeModelCall(ctx, call.ID, ModelCallUpdate{
		Status:           ModelCallStatusFailed,
		LatencyMS:        time.Since(startedAt).Milliseconds(),
		RetryCount:       lastAttempt,
		ErrorType:        errorCode(lastErr),
		ErrorMessage:     errorMessage(lastErr),
		ResponseMetadata: errorMetadataMap(lastErr),
		CompletedAt:      time.Now(),
	})
	return Response{}, lastErr
}

func (r *Router) GenerateStream(ctx context.Context, req Request) (<-chan StreamEvent, <-chan error) {
	route, provider, err := r.resolve(req)
	if err != nil {
		chunks := make(chan StreamEvent)
		errs := make(chan error, 1)
		close(chunks)
		errs <- err
		close(errs)
		return chunks, errs
	}
	req = r.applyRoute(req, route)
	if req.Options.TimeoutMS <= 0 {
		req.Options.TimeoutMS = r.config.StreamTimeout.Milliseconds()
	}
	startedAt := time.Now()
	call := r.startModelCall(ctx, req, route, true, ModelCallStatusStreaming, startedAt)
	events, errs := provider.ChatStream(ctx, req)
	out := make(chan StreamEvent)
	wrappedErrs := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(wrappedErrs)
		var lastUsage Usage
		var lastMetadata ResponseMetadata
		for event := range events {
			event.Metadata.Provider = route.Provider
			event.Metadata.Model = req.Model
			event.Metadata.Task = req.Task
			event.Metadata.Capability = req.Capability
			event.Metadata.TraceID = req.TraceID
			if event.Response != nil {
				event.Response.Metadata = event.Metadata
				lastUsage = event.Response.Usage
			}
			if event.Usage.TotalTokens > 0 || event.Usage.PromptTokens > 0 || event.Usage.CompletionTokens > 0 {
				lastUsage = event.Usage
			}
			lastMetadata = event.Metadata
			if event.Done || event.Type == "completed" {
				event.Metadata.LatencyMS = time.Since(startedAt).Milliseconds()
				lastMetadata = event.Metadata
			}
			out <- event
		}
		if err, ok := <-errs; ok && err != nil {
			classified := classifyError(err, route.Provider, req.Model)
			r.completeModelCall(ctx, call.ID, ModelCallUpdate{
				Status:           ModelCallStatusFailed,
				Usage:            lastUsage,
				LatencyMS:        time.Since(startedAt).Milliseconds(),
				ErrorType:        errorCode(classified),
				ErrorMessage:     errorMessage(classified),
				ResponseMetadata: errorMetadataMap(classified),
				CompletedAt:      time.Now(),
			})
			wrappedErrs <- classified
			return
		}
		r.completeModelCall(ctx, call.ID, ModelCallUpdate{
			Status:           ModelCallStatusCompleted,
			Usage:            lastUsage,
			LatencyMS:        time.Since(startedAt).Milliseconds(),
			ResponseMetadata: responseMetadataMap(lastMetadata),
			CompletedAt:      time.Now(),
		})
	}()
	return out, wrappedErrs
}

func (r *Router) startModelCall(ctx context.Context, req Request, route Route, stream bool, status string, startedAt time.Time) ModelCall {
	call := ModelCall{
		TraceID:    req.TraceID,
		UserID:     metadataString(req.Metadata, RequestMetadataUserID),
		SessionID:  metadataString(req.Metadata, RequestMetadataSessionID),
		Provider:   route.Provider,
		Model:      req.Model,
		Capability: req.Capability,
		Task:       req.Task,
		Stream:     stream,
		Status:     status,
		StartedAt:  startedAt,
		RetryCount: 0,
		LatencyMS:  0,
		RequestMetadata: map[string]any{
			"prompt_chars": len([]rune(req.ChatPrompt())),
			"timeout_ms":   req.Options.TimeoutMS,
			"route":        route.String(),
		},
		ResponseMetadata: map[string]any{},
	}
	for key, value := range req.Metadata {
		call.RequestMetadata[key] = value
	}

	created, err := r.recorder.CreateModelCall(ctx, call)
	if err != nil {
		// 观测失败不能影响模型主链路；错误保留在日志里，后续可接入告警。
		slog.WarnContext(ctx, "create model call record failed", "trace_id", req.TraceID, "provider", route.Provider, "model", req.Model, "error", err.Error())
		return call
	}
	return created
}

func (r *Router) completeModelCall(ctx context.Context, id int64, update ModelCallUpdate) {
	if id == 0 {
		return
	}
	if update.CompletedAt.IsZero() {
		update.CompletedAt = time.Now()
	}
	if err := r.recorder.CompleteModelCall(ctx, id, update); err != nil {
		// 完成记录失败同样不能打断用户响应，避免观测系统反向影响可用性。
		slog.WarnContext(ctx, "complete model call record failed", "model_call_id", id, "status", update.Status, "error", err.Error())
	}
}

func (r *Router) resolve(req Request) (Route, Provider, error) {
	route := r.config.DefaultRoute
	if taskRoute, ok := r.config.TaskRoutes[req.Task]; ok {
		route = mergeRoute(route, taskRoute)
	}
	if req.Model != "" {
		route.Model = req.Model
	}
	if req.Capability != "" {
		route.Capability = req.Capability
	}
	if route.Capability == "" {
		route.Capability = CapabilityChat
	}
	provider := r.providers[route.Provider]
	if provider == nil {
		return Route{}, nil, &ModelError{Code: "model_provider_not_found", Provider: route.Provider, Model: route.Model, Retryable: false}
	}
	if route.Capability != CapabilityChat {
		return Route{}, nil, &ModelError{Code: "model_capability_unsupported", Provider: route.Provider, Model: route.Model, Retryable: false}
	}
	return route, provider, nil
}

func (r *Router) applyRoute(req Request, route Route) Request {
	req.Capability = route.Capability
	req.Model = route.Model
	if req.Input.Chat == nil && req.Prompt != "" {
		req.Input = NewTextChatRequest(req.Task, req.Prompt).Input
	}
	return req
}

func (r *Router) timeoutFor(req Request, stream bool) time.Duration {
	if req.Options.TimeoutMS > 0 {
		return time.Duration(req.Options.TimeoutMS) * time.Millisecond
	}
	if stream {
		return r.config.StreamTimeout
	}
	return r.config.DefaultTimeout
}

func mergeRoute(base Route, override Route) Route {
	if override.Provider != "" {
		base.Provider = override.Provider
	}
	if override.Capability != "" {
		base.Capability = override.Capability
	}
	if override.Model != "" {
		base.Model = override.Model
	}
	return base
}

func classifyError(err error, provider string, model string) error {
	if err == nil {
		return nil
	}
	var modelErr *ModelError
	if errors.As(err, &modelErr) {
		if modelErr.Provider == "" {
			modelErr.Provider = provider
		}
		if modelErr.Model == "" {
			modelErr.Model = model
		}
		return modelErr
	}
	return &ModelError{Code: "model_provider_error", Provider: provider, Model: model, Retryable: false, Cause: err}
}

func errorCode(err error) string {
	var modelErr *ModelError
	if errors.As(err, &modelErr) && modelErr.Code != "" {
		return modelErr.Code
	}
	if err != nil {
		return "model_error"
	}
	return ""
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func responseMetadataMap(metadata ResponseMetadata) map[string]any {
	return map[string]any{
		"provider":      metadata.Provider,
		"model":         metadata.Model,
		"task":          metadata.Task,
		"capability":    metadata.Capability,
		"trace_id":      metadata.TraceID,
		"latency_ms":    metadata.LatencyMS,
		"retry_count":   metadata.RetryCount,
		"finish_reason": metadata.FinishReason,
	}
}

func errorMetadataMap(err error) map[string]any {
	metadata := map[string]any{}
	var modelErr *ModelError
	if errors.As(err, &modelErr) {
		metadata["provider"] = modelErr.Provider
		metadata["model"] = modelErr.Model
		metadata["status_code"] = modelErr.StatusCode
		metadata["retryable"] = modelErr.Retryable
		for key, value := range modelErr.Metadata {
			metadata[key] = value
		}
	}
	return metadata
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func sleepWithContext(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (r Route) String() string {
	return fmt.Sprintf("%s/%s/%s", r.Provider, r.Capability, r.Model)
}
