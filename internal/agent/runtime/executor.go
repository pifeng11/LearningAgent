package runtime

import (
	"context"
	"fmt"
	"time"

	"learning-agent/internal/agent/graph"
	"learning-agent/internal/agent/state"
	"learning-agent/internal/memory"
	"learning-agent/internal/skills"
)

type Executor struct {
	registry *skills.Registry
	memory   memory.Store
}

func NewExecutor(registry *skills.Registry, memoryStore memory.Store) *Executor {
	return &Executor{registry: registry, memory: memoryStore}
}

func (e *Executor) Execute(ctx context.Context, plan graph.Plan, current state.AgentState) (Result, error) {
	completed := map[string]bool{}
	events := make([]Event, 0, len(plan.Nodes))

	for len(completed) < len(plan.Nodes) {
		progressed := false

		for _, node := range plan.Nodes {
			if completed[node.ID] || !dependenciesDone(node, completed) {
				continue
			}

			skill, ok := e.registry.Get(node.Skill)
			if !ok {
				return Result{}, fmt.Errorf("skill %q is not registered", node.Skill)
			}

			events = append(events, Event{
				Type:      "node.started",
				Message:   node.ID,
				Timestamp: time.Now(),
			})

			next, err := skill.Run(ctx, current)
			if err != nil {
				return Result{}, fmt.Errorf("run node %q: %w", node.ID, err)
			}
			current = next
			completed[node.ID] = true
			progressed = true

			events = append(events, Event{
				Type:      "node.completed",
				Message:   node.ID,
				Timestamp: time.Now(),
			})
		}

		if !progressed {
			return Result{}, fmt.Errorf("graph contains unresolved dependencies")
		}
	}

	return Result{Answer: current.Answer, Events: events}, nil
}

func dependenciesDone(node graph.NodeSpec, completed map[string]bool) bool {
	for _, dependency := range node.DependsOn {
		if !completed[dependency] {
			return false
		}
	}
	return true
}
