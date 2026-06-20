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

type AgentEvent struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
