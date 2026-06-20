package app

import (
	"context"
	"strings"
	"testing"
)

func TestAgentServiceChatRunsLearningPlan(t *testing.T) {
	service := NewAgentService()

	resp, err := service.Chat(context.Background(), ChatRequest{
		UserID:    "u1",
		SessionID: "s1",
		Message:   "帮我规划 Rust 学习路线",
	})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Intent != "learning_plan" {
		t.Fatalf("expected learning_plan, got %s", resp.Intent)
	}
	if !strings.Contains(resp.Answer, "学习计划") {
		t.Fatalf("expected answer to contain 学习计划, got %q", resp.Answer)
	}
	if len(resp.Events) == 0 {
		t.Fatal("expected execution events")
	}
}
