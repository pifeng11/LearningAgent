package app

import "time"

type ChatRequest struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

type ListMessagesRequest struct {
	UserID    string
	SessionID string
	BeforeID  string
	Turns     int
}

type ListMessagesResponse struct {
	Messages     []ConversationMessage `json:"messages"`
	NextBeforeID string                `json:"next_before_id,omitempty"`
	HasMore      bool                  `json:"has_more"`
}

type ConversationMessage struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
