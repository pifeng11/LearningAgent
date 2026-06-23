package rest

import (
	"encoding/json"
	"net/http"

	"learning-agent/internal/app"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *app.AgentService
}

func NewHandler(service *app.AgentService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) Chat(c *gin.Context) {
	var req app.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.service.Chat(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ChatStream(c *gin.Context) {
	var req app.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	events, errs := h.service.ChatStream(c.Request.Context(), req)
	for event := range events {
		writeSSE(c, event.Type, event)
		c.Writer.Flush()
	}

	if err, ok := <-errs; ok && err != nil {
		writeSSE(c, "agent.error", gin.H{"error": err.Error()})
		c.Writer.Flush()
	}
}

func writeSSE(c *gin.Context, eventName string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte(`{"error":"marshal sse payload"}`)
	}

	c.Writer.WriteString("event: ")
	c.Writer.WriteString(eventName)
	c.Writer.WriteString("\n")
	c.Writer.WriteString("data: ")
	c.Writer.Write(data)
	c.Writer.WriteString("\n\n")
}
