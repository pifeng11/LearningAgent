package cli

import (
	"learning-agent/internal/api/rest"
	wsapi "learning-agent/internal/api/websocket"
	"learning-agent/internal/app"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

func NewServerCommand() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "server",
		Short: "启动 REST 和 WebSocket 服务",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := app.LoadConfig()
			service, err := app.NewAgentServiceFromConfig(cfg)
			if err != nil {
				return err
			}
			if addr == "" {
				addr = cfg.HTTPAddr
			}

			router := rest.NewRouter(service)
			websocketHandler := wsapi.NewHandler(service)
			router.GET("/ws/v1/agent", func(c *gin.Context) {
				websocketHandler.ServeHTTP(c.Writer, c.Request)
			})

			return router.Run(addr)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "", "监听地址，例如 :8080")
	return cmd
}
