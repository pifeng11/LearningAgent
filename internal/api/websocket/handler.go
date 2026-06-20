package websocket

import (
	"net/http"

	"learning-agent/internal/app"

	gorilla "github.com/gorilla/websocket"
)

type Handler struct {
	service  *app.AgentService
	upgrader gorilla.Upgrader
}

func NewHandler(service *app.AgentService) *Handler {
	return &Handler{
		service: service,
		upgrader: gorilla.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		var message Message
		if err := conn.ReadJSON(&message); err != nil {
			return
		}

		if err := conn.WriteJSON(Event{Type: "agent.started", Data: message}); err != nil {
			return
		}

		resp, err := h.service.Chat(r.Context(), chatRequestFromMessage(message))
		if err != nil {
			_ = conn.WriteJSON(Event{Type: "agent.error", Data: err.Error()})
			continue
		}

		for _, event := range resp.Events {
			if err := conn.WriteJSON(Event{Type: event.Type, Data: event}); err != nil {
				return
			}
		}

		if err := conn.WriteJSON(Event{Type: "agent.completed", Data: resp}); err != nil {
			return
		}
	}
}
