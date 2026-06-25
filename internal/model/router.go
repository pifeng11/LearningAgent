package model

import (
	"context"
	"errors"
	"fmt"
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
}

type Router struct {
	providers map[string]Provider
	config    RouterConfig
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
	return &Router{providers: registry, config: config}
}

func (r *Router) Generate(ctx context.Context, req Request) (Response, error) {
	route, provider, err := r.resolve(req)
	if err != nil {
		return Response{}, err
	}
	req = r.applyRoute(req, route)

	var lastErr error
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, r.timeoutFor(req, false))
		startedAt := time.Now()
		resp, err := provider.Chat(callCtx, req)
		cancel()
		if err == nil {
			resp.Metadata.Provider = route.Provider
			resp.Metadata.Model = req.Model
			resp.Metadata.Task = req.Task
			resp.Metadata.Capability = req.Capability
			resp.Metadata.TraceID = req.TraceID
			resp.Metadata.RetryCount = attempt
			resp.Metadata.LatencyMS = time.Since(startedAt).Milliseconds()
			return resp, nil
		}
		lastErr = classifyError(err, route.Provider, req.Model)
		if attempt >= r.config.MaxRetries || !IsRetryable(lastErr) {
			break
		}
		sleepWithContext(ctx, r.config.RetryBackoff*time.Duration(attempt+1))
	}
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
	events, errs := provider.ChatStream(ctx, req)
	out := make(chan StreamEvent)
	wrappedErrs := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(wrappedErrs)
		startedAt := time.Now()
		for event := range events {
			event.Metadata.Provider = route.Provider
			event.Metadata.Model = req.Model
			event.Metadata.Task = req.Task
			event.Metadata.Capability = req.Capability
			event.Metadata.TraceID = req.TraceID
			if event.Done || event.Type == "completed" {
				event.Metadata.LatencyMS = time.Since(startedAt).Milliseconds()
			}
			out <- event
		}
		if err, ok := <-errs; ok && err != nil {
			wrappedErrs <- classifyError(err, route.Provider, req.Model)
		}
	}()
	return out, wrappedErrs
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
