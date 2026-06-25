package model

import (
	"context"
	"fmt"
	"strings"
)

type MockProvider struct{}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (p *MockProvider) Name() string {
	return "mock"
}

func (p *MockProvider) Chat(ctx context.Context, req Request) (Response, error) {
	prompt := req.ChatPrompt()
	switch req.Task {
	case TaskLearningPlan:
		return ResponseFromText(fmt.Sprintf("学习计划草案：\n1. 明确目标：%s\n2. 拆成每周主题。\n3. 每天安排输入、练习、复盘三个环节。\n4. 每周根据薄弱点调整计划。", prompt), ResponseMetadata{}, Usage{Estimated: true}), nil
	case TaskPractice:
		return ResponseFromText("练习建议：\n1. 用自己的话解释核心概念。\n2. 完成一个小项目。\n3. 写下卡住的问题并复盘。", ResponseMetadata{}, Usage{Estimated: true}), nil
	case TaskReview:
		return ResponseFromText("复盘模板：\n1. 本阶段完成了什么。\n2. 哪些知识点仍不稳定。\n3. 下一阶段减少范围，强化练习。", ResponseMetadata{}, Usage{Estimated: true}), nil
	default:
		return ResponseFromText(fmt.Sprintf("答疑草案：我会围绕你的问题进行解释，并结合长期记忆和知识库补充上下文。问题：%s", prompt), ResponseMetadata{}, Usage{Estimated: true}), nil
	}
}

func (p *MockProvider) Generate(ctx context.Context, req Request) (Response, error) {
	return p.Chat(ctx, req)
}

func (p *MockProvider) ChatStream(ctx context.Context, req Request) (<-chan StreamEvent, <-chan error) {
	chunks := make(chan StreamEvent)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		resp, err := p.Chat(ctx, req)
		if err != nil {
			errs <- err
			return
		}

		parts := strings.SplitAfter(resp.Text, "")
		for _, part := range parts {
			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			case chunks <- StreamEvent{Type: "delta", Delta: part, Text: part}:
			}
		}
		chunks <- StreamEvent{Type: "completed", Done: true, Response: &resp, Usage: resp.Usage, Metadata: resp.Metadata}
	}()

	return chunks, errs
}

func (p *MockProvider) GenerateStream(ctx context.Context, req Request) (<-chan StreamEvent, <-chan error) {
	return p.ChatStream(ctx, req)
}
