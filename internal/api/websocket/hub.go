package websocket

import "learning-agent/internal/app"

type Message struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

func chatRequestFromMessage(message Message) app.ChatRequest {
	return app.ChatRequest{
		UserID:    message.UserID,
		SessionID: message.SessionID,
		Message:   message.Message,
	}
}
