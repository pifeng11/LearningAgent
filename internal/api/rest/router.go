package rest

import (
	"learning-agent/internal/app"
	"learning-agent/internal/observability"

	"github.com/gin-gonic/gin"
)

func NewRouter(service *app.AgentService) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(traceMiddleware())

	handler := NewHandler(service)

	router.GET("/api/v1/health", handler.Health)
	router.GET("/api/v1/debug/traces/:trace_id", handler.GetPromptTrace)
	router.GET("/api/v1/debug/traces/:trace_id/reconstructed-prompt", handler.ReconstructPrompt)
	router.GET("/api/v1/debug/traces/:trace_id/tokens", handler.GetTokenReport)
	router.GET("/api/v1/agent/messages", handler.ListMessages)
	router.POST("/api/v1/agent/chat", handler.Chat)
	router.POST("/api/v1/agent/chat/stream", handler.ChatStream)

	return router
}

func traceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := observability.EnsureTraceID(c.Request.Context())
		c.Request = c.Request.WithContext(ctx)
		c.Header("X-Trace-ID", observability.TraceID(ctx))
		c.Next()
	}
}
