package model

import "context"

type CapabilityKind string

const (
	CapabilityChat  CapabilityKind = "chat"
	CapabilityImage CapabilityKind = "image"
	CapabilityAudio CapabilityKind = "audio"
	CapabilityVideo CapabilityKind = "video"
)

type Provider interface {
	Name() string
	Chat(ctx context.Context, req Request) (Response, error)
	ChatStream(ctx context.Context, req Request) (<-chan StreamEvent, <-chan error)
}
