package cli

import (
	"fmt"
	"os"

	"learning-agent/internal/app"

	"github.com/spf13/cobra"
)

func Execute() {
	service, err := app.NewAgentServiceFromConfig(app.LoadConfig())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	root := NewRootCommand(service)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func NewRootCommand(service *app.AgentService) *cobra.Command {
	root := &cobra.Command{
		Use:   "learning-agent",
		Short: "综合型学习 Agent",
	}

	root.AddCommand(NewChatCommand(service))
	root.AddCommand(NewServerCommand(service))

	return root
}
