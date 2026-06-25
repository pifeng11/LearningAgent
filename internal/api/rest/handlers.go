package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"learning-agent/internal/app"
	"learning-agent/internal/observability"

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

func (h *Handler) GetPromptTrace(c *gin.Context) {
	resp, err := h.service.GetPromptTrace(c.Request.Context(), c.Param("trace_id"))
	if err != nil {
		observability.LogError(c.Request.Context(), nil, "get prompt trace failed", err)
		c.JSON(http.StatusNotFound, gin.H{"error": observability.UserError(c.Request.Context(), err)})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ReconstructPrompt(c *gin.Context) {
	resp, err := h.service.ReconstructPrompt(c.Request.Context(), c.Param("trace_id"))
	if err != nil {
		observability.LogError(c.Request.Context(), nil, "reconstruct prompt failed", err)
		c.JSON(http.StatusNotFound, gin.H{"error": observability.UserError(c.Request.Context(), err)})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetTokenReport(c *gin.Context) {
	resp, err := h.service.BuildTokenReport(c.Request.Context(), c.Param("trace_id"))
	if err != nil {
		observability.LogError(c.Request.Context(), nil, "get token report failed", err)
		c.JSON(http.StatusNotFound, gin.H{"error": observability.UserError(c.Request.Context(), err)})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListMessages(c *gin.Context) {
	turns, err := strconv.Atoi(c.DefaultQuery("turns", "5"))
	if err != nil {
		appErr := observability.NewError("invalid_request", "invalid turns", "cause", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": observability.UserError(c.Request.Context(), appErr)})
		return
	}

	resp, err := h.service.ListMessages(c.Request.Context(), app.ListMessagesRequest{
		UserID:    c.DefaultQuery("user_id", "anonymous"),
		SessionID: c.DefaultQuery("session_id", "default"),
		BeforeID:  c.Query("before_id"),
		Turns:     turns,
	})
	if err != nil {
		observability.LogError(c.Request.Context(), nil, "list messages failed", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": observability.UserError(c.Request.Context(), err)})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Chat(c *gin.Context) {
	var req app.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := observability.NewError("invalid_request", "invalid chat request", "cause", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": observability.UserError(c.Request.Context(), appErr)})
		return
	}

	resp, err := h.service.Chat(c.Request.Context(), req)
	if err != nil {
		wrapped := observability.Wrap(err, "chat_failed", "chat failed")
		observability.LogError(c.Request.Context(), nil, "chat failed", wrapped)
		c.JSON(http.StatusBadRequest, gin.H{"error": observability.UserError(c.Request.Context(), wrapped)})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ChatStream(c *gin.Context) {
	var req app.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := observability.NewError("invalid_request", "invalid chat request", "cause", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": observability.UserError(c.Request.Context(), appErr)})
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
		observability.LogError(c.Request.Context(), nil, "chat stream failed", err)
		writeSSE(c, "agent.error", gin.H{"error": observability.UserError(c.Request.Context(), err)})
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
