package openrouter

type chatRequest struct {
	Model       string         `json:"model"`
	Messages    []message      `json:"messages"`
	Stream      bool           `json:"stream"`
	User        string         `json:"user,omitempty"`
	SessionID   string         `json:"session_id,omitempty"`
	Trace       map[string]any `json:"trace,omitempty"`
	Temperature *float64       `json:"temperature,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost,omitempty"`
}

type chatResponse struct {
	ID                 string         `json:"id"`
	Model              string         `json:"model"`
	Choices            []choice       `json:"choices"`
	Usage              usage          `json:"usage"`
	OpenRouterMetadata map[string]any `json:"openrouter_metadata,omitempty"`
	Error              *apiError      `json:"error,omitempty"`
}

type choice struct {
	Message      message `json:"message"`
	Delta        message `json:"delta"`
	FinishReason string  `json:"finish_reason"`
}

type apiError struct {
	Code     int            `json:"code"`
	Message  string         `json:"message"`
	Metadata map[string]any `json:"metadata,omitempty"`
}
