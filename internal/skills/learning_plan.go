package skills

import (
	"context"
	"fmt"
	"strings"
	"time"

	"learning-agent/internal/agent/state"
	"learning-agent/internal/memory"
	"learning-agent/internal/model"
)

func RegisterBuiltins(registry *Registry, router *model.Router, memoryStore memory.Store) {
	registry.Register(&MemoryLoadSkill{memory: memoryStore})
	registry.Register(&MemorySaveSkill{memory: memoryStore})
	registry.Register(&LearningSkill{name: "learning.plan", description: "生成学习计划", task: "learning_plan", router: router})
	registry.Register(&LearningSkill{name: "learning.qa", description: "学习问答", task: "qa", router: router})
	registry.Register(&LearningSkill{name: "learning.practice", description: "生成练习题", task: "practice", router: router})
	registry.Register(&LearningSkill{name: "learning.review", description: "复盘总结", task: "review", router: router})
}

type MemoryLoadSkill struct {
	memory memory.Store
}

func (s *MemoryLoadSkill) Name() string {
	return "memory.load"
}

func (s *MemoryLoadSkill) Description() string {
	return "读取短期和长期记忆"
}

func (s *MemoryLoadSkill) Run(ctx context.Context, current state.AgentState) (state.AgentState, error) {
	entries, err := s.memory.Load(ctx, current.UserID, current.SessionID)
	if err != nil {
		return current, err
	}
	current.Values["memory"] = entries
	return current, nil
}

type MemorySaveSkill struct {
	memory memory.Store
}

func (s *MemorySaveSkill) Name() string {
	return "memory.save"
}

func (s *MemorySaveSkill) Description() string {
	return "保存会话短期记忆"
}

func (s *MemorySaveSkill) Run(ctx context.Context, current state.AgentState) (state.AgentState, error) {
	content := fmt.Sprintf("input=%s\nintent=%s\nanswer=%s", current.Input, current.Intent, current.Answer)
	return current, s.memory.Save(ctx, memory.Entry{
		UserID:    current.UserID,
		SessionID: current.SessionID,
		Scope:     memory.ScopeShortTerm,
		Content:   content,
		CreatedAt: time.Now(),
	})
}

type LearningSkill struct {
	name        string
	description string
	task        string
	router      *model.Router
}

func (s *LearningSkill) Name() string {
	return s.name
}

func (s *LearningSkill) Description() string {
	return s.description
}

func (s *LearningSkill) Run(ctx context.Context, current state.AgentState) (state.AgentState, error) {
	prompt := buildPrompt(current)
	resp, err := s.router.Generate(ctx, model.Request{Task: s.task, Prompt: prompt})
	if err != nil {
		return current, err
	}
	current.Answer = resp.Text
	return current, nil
}

func buildPrompt(current state.AgentState) string {
	var builder strings.Builder
	builder.WriteString(current.Input)

	if entries, ok := current.Values["memory"].([]memory.Entry); ok && len(entries) > 0 {
		builder.WriteString("\n\n相关记忆：")
		for _, entry := range entries {
			builder.WriteString("\n- ")
			builder.WriteString(entry.Content)
		}
	}

	return builder.String()
}
