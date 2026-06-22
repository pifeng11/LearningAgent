package model

import "context"

type Request struct {
	Task   string
	Prompt string
}

type Response struct {
	Text string
}

type StreamChunk struct {
	Text string
	Done bool
}

type Provider interface {
	Generate(ctx context.Context, req Request) (Response, error)
	GenerateStream(ctx context.Context, req Request) (<-chan StreamChunk, <-chan error)
}
