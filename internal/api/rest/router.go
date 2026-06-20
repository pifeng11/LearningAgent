package rest

import (
	"learning-agent/internal/app"

	"github.com/gin-gonic/gin"
)

func NewRouter(service *app.AgentService) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	handler := NewHandler(service)

	router.GET("/api/v1/health", handler.Health)
	router.POST("/api/v1/agent/chat", handler.Chat)

	return router
}
