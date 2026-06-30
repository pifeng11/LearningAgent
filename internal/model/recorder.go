package model

import (
	"context"
	"time"
)

const (
	ModelCallStatusRunning   = "running"
	ModelCallStatusStreaming = "streaming"
	ModelCallStatusCompleted = "completed"
	ModelCallStatusFailed    = "failed"
)

const (
	RequestMetadataUserID    = "user_id"
	RequestMetadataSessionID = "session_id"
)

type ModelCall struct {
	ID               int64
	TraceID          string
	UserID           string
	SessionID        string
	Provider         string
	Model            string
	Capability       CapabilityKind
	Task             Task
	Stream           bool
	Status           string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	LatencyMS        int64
	RetryCount       int
	ErrorType        string
	ErrorMessage     string
	RequestMetadata  map[string]any
	ResponseMetadata map[string]any
	StartedAt        time.Time
	CompletedAt      *time.Time
}

type ModelCallUpdate struct {
	Status           string
	Usage            Usage
	LatencyMS        int64
	RetryCount       int
	ErrorType        string
	ErrorMessage     string
	ResponseMetadata map[string]any
	CompletedAt      time.Time
}

type ModelCallQuery struct {
	TraceID   string
	UserID    string
	SessionID string
	Limit     int
}

type ModelCallRecorder interface {
	CreateModelCall(ctx context.Context, call ModelCall) (ModelCall, error)
	CompleteModelCall(ctx context.Context, id int64, update ModelCallUpdate) error
}

type NoopModelCallRecorder struct{}

func (NoopModelCallRecorder) CreateModelCall(ctx context.Context, call ModelCall) (ModelCall, error) {
	return call, nil
}

func (NoopModelCallRecorder) CompleteModelCall(ctx context.Context, id int64, update ModelCallUpdate) error {
	return nil
}
