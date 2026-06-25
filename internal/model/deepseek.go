package model

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
	"time"
)

const (
	DeepSeekV4Flash = "deepseek-v4-flash"
	DeepSeekV4Pro   = "deepseek-v4-pro"
)

type DeepSeekConfig struct {
	APIKey          string
	BaseURL         string
	Model           string
	ReasoningModel  string
	ReasoningEffort string
	ThinkingEnabled bool
	HTTPClient      *http.Client
}

type DeepSeekProvider struct {
	apiKey          string
	baseURL         string
	model           string
	reasoningModel  string
	reasoningEffort string
	thinkingEnabled bool
	httpClient      *http.Client
}

func NewDeepSeekProvider(cfg DeepSeekConfig) (*DeepSeekProvider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("DEEPSEEK_API_KEY is required when MODEL_PROVIDER=deepseek")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.deepseek.com"
	}
	if cfg.Model == "" {
		cfg.Model = DeepSeekV4Flash
	}
	if cfg.ReasoningModel == "" {
		cfg.ReasoningModel = DeepSeekV4Pro
	}
	if cfg.ReasoningEffort == "" {
		cfg.ReasoningEffort = "medium"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	}

	return &DeepSeekProvider{
		apiKey:          cfg.APIKey,
		baseURL:         strings.TrimRight(cfg.BaseURL, "/"),
		model:           cfg.Model,
		reasoningModel:  cfg.ReasoningModel,
		reasoningEffort: cfg.ReasoningEffort,
		thinkingEnabled: cfg.ThinkingEnabled,
		httpClient:      cfg.HTTPClient,
	}, nil
}

func (p *DeepSeekProvider) Name() string {
	return "deepseek"
}

func (p *DeepSeekProvider) Chat(ctx context.Context, req Request) (Response, error) {
	payload := p.newChatRequest(req, false)
	respBody, err := p.doChatRequest(ctx, payload)
	if err != nil {
		return Response{}, err
	}

	var chatResp deepSeekChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return Response{}, err
	}
	if len(chatResp.Choices) == 0 {
		return Response{}, errors.New("deepseek api returned no choices")
	}

	text := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	return ResponseFromText(text, ResponseMetadata{
		Provider:   p.Name(),
		Model:      payload.Model,
		Task:       req.Task,
		Capability: CapabilityChat,
		TraceID:    req.TraceID,
	}, Usage{Estimated: true}), nil
}

func (p *DeepSeekProvider) Generate(ctx context.Context, req Request) (Response, error) {
	return p.Chat(ctx, req)
}

func (p *DeepSeekProvider) ChatStream(ctx context.Context, req Request) (<-chan StreamEvent, <-chan error) {
	chunks := make(chan StreamEvent)
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
				chunks <- StreamEvent{Type: "completed", Done: true, Metadata: ResponseMetadata{
					Provider:   p.Name(),
					Model:      payload.Model,
					Task:       req.Task,
					Capability: CapabilityChat,
					TraceID:    req.TraceID,
				}}
				return
			}

			var streamResp deepSeekStreamResponse
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
				case chunks <- StreamEvent{Type: "delta", Delta: choice.Delta.Content, Text: choice.Delta.Content, Metadata: ResponseMetadata{
					Provider:   p.Name(),
					Model:      payload.Model,
					Task:       req.Task,
					Capability: CapabilityChat,
					TraceID:    req.TraceID,
				}}:
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errs <- err
			return
		}
		chunks <- StreamEvent{Type: "completed", Done: true, Metadata: ResponseMetadata{
			Provider:   p.Name(),
			Model:      payload.Model,
			Task:       req.Task,
			Capability: CapabilityChat,
			TraceID:    req.TraceID,
		}}
	}()

	return chunks, errs
}

func (p *DeepSeekProvider) GenerateStream(ctx context.Context, req Request) (<-chan StreamEvent, <-chan error) {
	return p.ChatStream(ctx, req)
}

func (p *DeepSeekProvider) newChatRequest(req Request, stream bool) deepSeekChatRequest {
	modelName := req.Model
	if modelName == "" {
		modelName = selectModel(req.Task, p.model, p.reasoningModel)
	}
	payload := deepSeekChatRequest{
		Model: modelName,
		Messages: []deepSeekMessage{
			{Role: "system", Content: systemPromptForTask(req.Task)},
			{Role: "user", Content: req.ChatPrompt()},
		},
		Stream: stream,
	}

	if shouldUseReasoning(req.Task) {
		payload.ReasoningEffort = p.reasoningEffort
		if p.thinkingEnabled {
			payload.Thinking = &deepSeekThinking{Type: "enabled"}
		}
	}

	return payload
}

func (p *DeepSeekProvider) doChatRequest(ctx context.Context, payload deepSeekChatRequest) ([]byte, error) {
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

func selectModel(task Task, defaultModel string, reasoningModel string) string {
	if shouldUseReasoning(task) {
		return reasoningModel
	}
	return defaultModel
}

func shouldUseReasoning(task Task) bool {
	switch task {
	case TaskLearningPlan, TaskReview:
		return true
	default:
		return false
	}
}

func systemPromptForTask(task Task) string {
	switch task {
	case TaskLearningPlan:
		return "你是一个严谨的学习规划 Agent。请根据用户目标输出可执行、分阶段、可复盘的学习计划。回答使用中文。"
	case TaskPractice:
		return "你是一个学习练习 Agent。请根据用户主题生成练习题，并给出简洁的检验标准。回答使用中文。"
	case TaskReview:
		return "你是一个学习复盘 Agent。请帮助用户总结进展、定位薄弱点，并给出下一步行动。回答使用中文。"
	default:
		return "你是一个学习辅导 Agent。请直接、准确地回答用户问题，必要时给出例子。回答使用中文。"
	}
}

type deepSeekChatRequest struct {
	Model           string            `json:"model"`
	Messages        []deepSeekMessage `json:"messages"`
	Thinking        *deepSeekThinking `json:"thinking,omitempty"`
	ReasoningEffort string            `json:"reasoning_effort,omitempty"`
	Stream          bool              `json:"stream"`
}

type deepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepSeekThinking struct {
	Type string `json:"type"`
}

type deepSeekChatResponse struct {
	Choices []struct {
		Message deepSeekMessage `json:"message"`
	} `json:"choices"`
}

type deepSeekStreamResponse struct {
	Choices []struct {
		Delta deepSeekMessage `json:"delta"`
	} `json:"choices"`
}
