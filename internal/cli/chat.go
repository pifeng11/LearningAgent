package cli

import (
	"context"
	"fmt"
	"strings"

	"learning-agent/internal/app"

	"github.com/spf13/cobra"
)

func NewChatCommand(service *app.AgentService) *cobra.Command {
	var userID string
	var sessionID string

	cmd := &cobra.Command{
		Use:   "chat [message]",
		Short: "发送一条学习请求",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := service.Chat(context.Background(), app.ChatRequest{
				UserID:    userID,
				SessionID: sessionID,
				Message:   strings.Join(args, " "),
			})
			if err != nil {
				return err
			}

			fmt.Printf("Intent: %s\n\n%s\n", resp.Intent, resp.Answer)
			return nil
		},
	}

	cmd.Flags().StringVar(&userID, "user", "demo", "用户 ID")
	cmd.Flags().StringVar(&sessionID, "session", "default", "会话 ID")

	return cmd
}
