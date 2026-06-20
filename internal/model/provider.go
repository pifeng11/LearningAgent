package model

import "context"

type Request struct {
	Task   string
	Prompt string
}

type Response struct {
	Text string
}

type Provider interface {
	Generate(ctx context.Context, req Request) (Response, error)
}
