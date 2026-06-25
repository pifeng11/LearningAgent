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

type PromptTraceResponse struct {
	TraceID                string             `json:"trace_id"`
	UserID                 string             `json:"user_id"`
	SessionID              string             `json:"session_id"`
	Intent                 string             `json:"intent"`
	ModelTask              string             `json:"model_task"`
	UsedMemoryIDs          []int64            `json:"used_memory_ids"`
	UsedHistoryIDs         []string           `json:"used_history_ids"`
	MemoryCount            int                `json:"memory_count"`
	HistoryMessageCount    int                `json:"history_message_count"`
	PromptChars            int                `json:"prompt_chars"`
	EstimatedPromptTokens  int                `json:"estimated_prompt_tokens"`
	PromptBuilderVersion   string             `json:"prompt_builder_version"`
	SystemPromptHash       string             `json:"system_prompt_hash"`
	PromptConfig           map[string]any     `json:"prompt_config,omitempty"`
	Prompt                 string             `json:"prompt,omitempty"`
	ContextItems           []TraceContextItem `json:"context_items,omitempty"`
	ContextSnapshotEnabled bool               `json:"context_snapshot_enabled"`
	CreatedAt              time.Time          `json:"created_at"`
}

type TraceContextItem struct {
	ItemType string         `json:"item_type"`
	SourceID string         `json:"source_id,omitempty"`
	Role     string         `json:"role,omitempty"`
	Title    string         `json:"title,omitempty"`
	Content  string         `json:"content"`
	Ordinal  int            `json:"ordinal"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type ReconstructedPromptResponse struct {
	TraceID     string `json:"trace_id"`
	Prompt      string `json:"prompt"`
	PromptChars int    `json:"prompt_chars"`
	Source      string `json:"source"`
}

type TokenReportResponse struct {
	TraceID               string        `json:"trace_id"`
	Prompt                string        `json:"prompt"`
	PromptChars           int           `json:"prompt_chars"`
	EstimatedPromptTokens int           `json:"estimated_prompt_tokens"`
	Tokenizer             string        `json:"tokenizer"`
	Tokens                []TokenRecord `json:"tokens"`
}

type TokenRecord struct {
	Index   int    `json:"index"`
	Text    string `json:"text"`
	TokenID int    `json:"token_id,omitempty"`
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
