package deepseek

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"learning-agent/internal/model"
)

func (p *Provider) Chat(ctx context.Context, req model.Request) (model.Response, error) {
	payload := p.newChatRequest(req, false)
	respBody, err := p.doChatRequest(ctx, payload)
	if err != nil {
		return model.Response{}, err
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return model.Response{}, err
	}
	if len(chatResp.Choices) == 0 {
		return model.Response{}, errors.New("deepseek api returned no choices")
	}

	text := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	return model.ResponseFromText(text, model.ResponseMetadata{
		Provider:   p.Name(),
		Model:      payload.Model,
		Task:       req.Task,
		Capability: model.CapabilityChat,
		TraceID:    req.TraceID,
	}, model.Usage{Estimated: true}), nil
}

func (p *Provider) Generate(ctx context.Context, req model.Request) (model.Response, error) {
	return p.Chat(ctx, req)
}

func (p *Provider) ChatStream(ctx context.Context, req model.Request) (<-chan model.StreamEvent, <-chan error) {
	chunks := make(chan model.StreamEvent)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		payload := p.newChatRequest(req, true)
		body, err := json.Marshal(payload)
		if err != nil {
			errs <- err
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			errs <- err
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

		httpResp, err := p.httpClient.Do(httpReq)
		if err != nil {
			errs <- err
			return
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
			respBody, readErr := io.ReadAll(httpResp.Body)
			if readErr != nil {
				errs <- readErr
				return
			}
			errs <- fmt.Errorf("deepseek api error: status=%d body=%s", httpResp.StatusCode, string(respBody))
			return
		}

		scanner := bufio.NewScanner(httpResp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				chunks <- model.StreamEvent{Type: "completed", Done: true, Metadata: p.metadata(req, payload.Model)}
				return
			}

			var streamResp streamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				errs <- err
				return
			}
			for _, choice := range streamResp.Choices {
				if choice.Delta.Content == "" {
					continue
				}
				select {
				case <-ctx.Done():
					errs <- ctx.Err()
					return
				case chunks <- model.StreamEvent{
					Type:     "delta",
					Delta:    choice.Delta.Content,
					Text:     choice.Delta.Content,
					Metadata: p.metadata(req, payload.Model),
				}:
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errs <- err
			return
		}
		chunks <- model.StreamEvent{Type: "completed", Done: true, Metadata: p.metadata(req, payload.Model)}
	}()

	return chunks, errs
}

func (p *Provider) GenerateStream(ctx context.Context, req model.Request) (<-chan model.StreamEvent, <-chan error) {
	return p.ChatStream(ctx, req)
}

func (p *Provider) metadata(req model.Request, modelName string) model.ResponseMetadata {
	return model.ResponseMetadata{
		Provider:   p.Name(),
		Model:      modelName,
		Task:       req.Task,
		Capability: model.CapabilityChat,
		TraceID:    req.TraceID,
	}
}

func (p *Provider) newChatRequest(req model.Request, stream bool) chatRequest {
	modelName := req.Model
	if modelName == "" {
		modelName = selectModel(req.Task, p.model, p.reasoningModel)
	}
	payload := chatRequest{
		Model: modelName,
		Messages: []message{
			{Role: "system", Content: systemPromptForTask(req.Task)},
			{Role: "user", Content: req.ChatPrompt()},
		},
		Stream: stream,
	}

	if shouldUseReasoning(req.Task) {
		payload.ReasoningEffort = p.reasoningEffort
		if p.thinkingEnabled {
			payload.Thinking = &thinking{Type: "enabled"}
		}
	}

	return payload
}

func (p *Provider) doChatRequest(ctx context.Context, payload chatRequest) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}
	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("deepseek api error: status=%d body=%s", httpResp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func selectModel(task model.Task, defaultModel string, reasoningModel string) string {
	if shouldUseReasoning(task) {
		return reasoningModel
	}
	return defaultModel
}

func shouldUseReasoning(task model.Task) bool {
	switch task {
	case model.TaskLearningPlan, model.TaskReview:
		return true
	default:
		return false
	}
}

func systemPromptForTask(task model.Task) string {
	switch task {
	case model.TaskLearningPlan:
		return "你是一个严谨的学习规划 Agent。请根据用户目标输出可执行、分阶段、可复盘的学习计划。回答使用中文。"
	case model.TaskPractice:
		return "你是一个学习练习 Agent。请根据用户主题生成练习题，并给出简洁的检验标准。回答使用中文。"
	case model.TaskReview:
		return "你是一个学习复盘 Agent。请帮助用户总结进展、定位薄弱点，并给出下一步行动。回答使用中文。"
	default:
		return "你是一个学习辅导 Agent。请直接、准确地回答用户问题，必要时给出例子。回答使用中文。"
	}
}
