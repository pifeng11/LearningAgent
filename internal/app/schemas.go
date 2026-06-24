package app

import "time"

type ChatRequest struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

type ChatResponse struct {
	UserID    string       `json:"user_id"`
	SessionID string       `json:"session_id"`
	Intent    string       `json:"intent"`
	Answer    string       `json:"answer"`
	Events    []AgentEvent `json:"events,omitempty"`
}

type ChatStreamEvent struct {
	Type      string    `json:"type"`
	TraceID   string    `json:"trace_id,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	Intent    string    `json:"intent,omitempty"`
	Delta     string    `json:"delta,omitempty"`
	Answer    string    `json:"answer,omitempty"`
	Error     string    `json:"error,omitempty"`
	ErrorCode string    `json:"error_code,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type AgentEvent struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
