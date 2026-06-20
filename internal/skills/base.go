package skills

import (
	"context"

	"learning-agent/internal/agent/state"
)

type Skill interface {
	Name() string
	Description() string
	Run(ctx context.Context, current state.AgentState) (state.AgentState, error)
}
