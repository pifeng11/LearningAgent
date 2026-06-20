package rest

import (
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
