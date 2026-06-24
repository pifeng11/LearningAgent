package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"learning-agent/internal/app"
	"learning-agent/internal/observability"

	"github.com/spf13/cobra"
)

func NewChatCommand() *cobra.Command {
	var userID string
	var sessionID string

	cmd := &cobra.Command{
		Use:   "chat [message]",
		Short: "发送一条学习请求",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := newServiceFromEnv()
			if err != nil {
				return err
			}

			ctx := observability.EnsureTraceID(context.Background())
			events, errs := service.ChatStream(ctx, app.ChatRequest{
				UserID:    userID,
				SessionID: sessionID,
				Message:   strings.Join(args, " "),
			})

			var intentPrinted bool
			for event := range events {
				switch event.Type {
				case "agent.started":
					fmt.Printf("Intent: %s\n\n", event.Intent)
					intentPrinted = true
				case "agent.delta":
					if !intentPrinted {
						fmt.Println()
						intentPrinted = true
					}
					fmt.Print(event.Delta)
				case "agent.completed":
					fmt.Println()
				case "agent.error":
					fmt.Fprintln(os.Stderr, event.Error)
				}
			}

			if err, ok := <-errs; ok {
				observability.LogError(ctx, nil, "cli chat failed", err)
				return fmt.Errorf("%s", observability.UserErrorText(ctx, err))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&userID, "user", "demo", "用户 ID")
	cmd.Flags().StringVar(&sessionID, "session", "default", "会话 ID")

	return cmd
}
