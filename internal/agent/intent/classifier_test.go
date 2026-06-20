package intent

import (
	"testing"

	"learning-agent/internal/agent/state"
)

func TestClassifierClassifiesLearningPlan(t *testing.T) {
	classifier := NewClassifier()

	got := classifier.Classify("我想三个月学完 Go，请帮我做计划")

	if got != state.IntentLearningPlan {
		t.Fatalf("expected %s, got %s", state.IntentLearningPlan, got)
	}
}

func TestClassifierDefaultsToQA(t *testing.T) {
	classifier := NewClassifier()

	got := classifier.Classify("Go 的 interface 应该怎么理解")

	if got != state.IntentQA {
		t.Fatalf("expected %s, got %s", state.IntentQA, got)
	}
}
