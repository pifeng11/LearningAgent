package model

type ChatInput struct {
	Messages []ChatMessage
}

type ChatMessage struct {
	Role  string
	Parts []ContentPart
}

type ContentPart struct {
	Type     string
	Text     string
	URI      string
	MimeType string
	Data     []byte
}

type ChatOutput struct {
	Text string
}

func NewTextChatRequest(task Task, prompt string) Request {
	return Request{
		Task:       task,
		Capability: CapabilityChat,
		Prompt:     prompt,
		Input: Input{Chat: &ChatInput{Messages: []ChatMessage{
			{
				Role: "user",
				Parts: []ContentPart{
					{Type: "text", Text: prompt},
				},
			},
		}}},
	}
}

func (r Request) ChatPrompt() string {
	if r.Prompt != "" {
		return r.Prompt
	}
	if r.Input.Chat == nil {
		return ""
	}
	var text string
	for _, message := range r.Input.Chat.Messages {
		for _, part := range message.Parts {
			if part.Type == "text" {
				if text != "" {
					text += "\n"
				}
				text += part.Text
			}
		}
	}
	return text
}

func ResponseFromText(text string, metadata ResponseMetadata, usage Usage) Response {
	return Response{
		Text: text,
		Output: Output{Chat: &ChatOutput{
			Text: text,
		}},
		Usage:    usage,
		Metadata: metadata,
	}
}
