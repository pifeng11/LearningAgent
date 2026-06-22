package model

import "context"

type Router struct {
	defaultProvider Provider
}

func NewRouter(defaultProvider Provider) *Router {
	return &Router{defaultProvider: defaultProvider}
}

func (r *Router) Generate(ctx context.Context, req Request) (Response, error) {
	return r.defaultProvider.Generate(ctx, req)
}

func (r *Router) GenerateStream(ctx context.Context, req Request) (<-chan StreamChunk, <-chan error) {
	return r.defaultProvider.GenerateStream(ctx, req)
}
