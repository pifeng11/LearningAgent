package cli

import (
	"fmt"
	"log/slog"
	"os"

	"learning-agent/internal/app"
	"learning-agent/internal/observability"

	"github.com/spf13/cobra"
)

func Execute() {
	slog.SetDefault(observability.NewLogger())

	root := NewRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "learning-agent",
		Short: "综合型学习 Agent",
	}

	root.AddCommand(NewChatCommand())
	root.AddCommand(NewServerCommand())
	root.AddCommand(NewMigrateCommand())

	return root
}

func newServiceFromEnv() (*app.AgentService, error) {
	return app.NewAgentServiceFromConfig(app.LoadConfig())
}
