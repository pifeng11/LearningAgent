package intent

import (
	"strings"

	"learning-agent/internal/agent/state"
)

type Classifier struct{}

func NewClassifier() *Classifier {
	return &Classifier{}
}

func (c *Classifier) Classify(input string) state.Intent {
	text := strings.ToLower(input)

	// TODO：改为 LLM 处理，这里只是简单示例
	switch {
	case containsAny(text, "计划", "规划", "目标", "学完", "roadmap", "plan"):
		return state.IntentLearningPlan
	case containsAny(text, "题", "练习", "测试", "quiz", "practice"):
		return state.IntentPractice
	case containsAny(text, "复盘", "总结", "回顾", "review", "retrospective"):
		return state.IntentReview
	default:
		return state.IntentQA
	}
}

func containsAny(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}
