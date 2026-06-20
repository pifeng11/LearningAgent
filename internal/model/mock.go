package model

import (
	"context"
	"fmt"
)

type MockProvider struct{}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (p *MockProvider) Generate(ctx context.Context, req Request) (Response, error) {
	switch req.Task {
	case "learning_plan":
		return Response{Text: fmt.Sprintf("学习计划草案：\n1. 明确目标：%s\n2. 拆成每周主题。\n3. 每天安排输入、练习、复盘三个环节。\n4. 每周根据薄弱点调整计划。", req.Prompt)}, nil
	case "practice":
		return Response{Text: "练习建议：\n1. 用自己的话解释核心概念。\n2. 完成一个小项目。\n3. 写下卡住的问题并复盘。"}, nil
	case "review":
		return Response{Text: "复盘模板：\n1. 本阶段完成了什么。\n2. 哪些知识点仍不稳定。\n3. 下一阶段减少范围，强化练习。"}, nil
	default:
		return Response{Text: fmt.Sprintf("答疑草案：我会围绕你的问题进行解释，并结合长期记忆和知识库补充上下文。问题：%s", req.Prompt)}, nil
	}
}
