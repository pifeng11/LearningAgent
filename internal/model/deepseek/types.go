package deepseek

type chatRequest struct {
	Model           string    `json:"model"`
	Messages        []message `json:"messages"`
	Thinking        *thinking `json:"thinking,omitempty"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"`
	Stream          bool      `json:"stream"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type thinking struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

type streamResponse struct {
	Choices []struct {
		Delta message `json:"delta"`
	} `json:"choices"`
}
