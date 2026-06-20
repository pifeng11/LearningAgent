package state

import "time"

type Intent string

const (
	IntentLearningPlan Intent = "learning_plan"
	IntentQA           Intent = "qa"
	IntentPractice     Intent = "practice"
	IntentReview       Intent = "review"
)

type AgentState struct {
	UserID    string
	SessionID string
	Input     string
	Intent    Intent
	Answer    string
	Values    map[string]any
	CreatedAt time.Time
}
