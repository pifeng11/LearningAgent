package model

import (
	"errors"
	"fmt"
)

type Task string

const (
	TaskQA            Task = "qa"
	TaskLearningPlan  Task = "learning_plan"
	TaskPractice      Task = "practice"
	TaskReview        Task = "review"
	TaskMemoryExtract Task = "memory_extract"
)

type Request struct {
	TraceID    string
	Task       Task
	Capability CapabilityKind
	Model      string
	Prompt     string
	Input      Input
	Options    RequestOptions
	Metadata   map[string]any
}

type RequestOptions struct {
	TimeoutMS int64
}

type Input struct {
	Chat  *ChatInput
	Image *ImageInput
	Audio *AudioInput
	Video *VideoInput
}

type ImageInput struct{}
type AudioInput struct{}
type VideoInput struct{}

type Output struct {
	Chat  *ChatOutput
	Image *ImageOutput
	Audio *AudioOutput
	Video *VideoOutput
}

type ImageOutput struct{}
type AudioOutput struct{}
type VideoOutput struct{}

type Response struct {
	Text     string
	Output   Output
	Usage    Usage
	Metadata ResponseMetadata
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Estimated        bool
}

type ResponseMetadata struct {
	Provider     string
	Model        string
	Task         Task
	Capability   CapabilityKind
	TraceID      string
	LatencyMS    int64
	RetryCount   int
	FinishReason string
}

type StreamEvent struct {
	Type     string
	Delta    string
	Text     string
	Done     bool
	Response *Response
	Usage    Usage
	Metadata ResponseMetadata
}

type StreamChunk = StreamEvent

type ModelError struct {
	Code       string
	Provider   string
	Model      string
	Retryable  bool
	StatusCode int
	Cause      error
}

func (e *ModelError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return e.Code
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Cause)
}

func (e *ModelError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func IsRetryable(err error) bool {
	var modelErr *ModelError
	if errors.As(err, &modelErr) {
		return modelErr.Retryable
	}
	return false
}
