package planner

import (
	"learning-agent/internal/agent/graph"
	"learning-agent/internal/agent/state"
)

type Planner struct{}

func NewPlanner() *Planner {
	return &Planner{}
}

func (p *Planner) Plan(intent state.Intent) graph.Plan {
	return graph.Plan{
		Nodes: []graph.NodeSpec{
			{ID: "load_memory", Skill: "memory.load"},
			{ID: "run_skill", Skill: skillForIntent(intent), DependsOn: []string{"load_memory"}},
			{ID: "save_memory", Skill: "memory.save", DependsOn: []string{"run_skill"}},
		},
	}
}

func skillForIntent(intent state.Intent) string {
	switch intent {
	case state.IntentLearningPlan:
		return "learning.plan"
	case state.IntentPractice:
		return "learning.practice"
	case state.IntentReview:
		return "learning.review"
	default:
		return "learning.qa"
	}
}
