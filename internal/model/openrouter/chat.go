package openrouter

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
	if chatResp.Error != nil {
		return model.Response{}, p.modelError(chatResp.Error, payload.Model)
	}
	if len(chatResp.Choices) == 0 {
		return model.Response{}, errors.New("openrouter api returned no choices")
	}

	text := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	return model.ResponseFromText(text, model.ResponseMetadata{
		Provider:     p.Name(),
		Model:        firstNonEmpty(chatResp.Model, payload.Model),
		Task:         req.Task,
		Capability:   model.CapabilityChat,
		TraceID:      req.TraceID,
		FinishReason: chatResp.Choices[0].FinishReason,
	}, toModelUsage(chatResp.Usage)), nil
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
		p.decorateRequest(httpReq)

		httpResp, err := p.httpClient.Do(httpReq)
		if err != nil {
			errs <- err
			return
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
			apiErr, readErr := readAPIError(httpResp)
			if readErr != nil {
				errs <- readErr
				return
			}
			errs <- p.modelError(apiErr, payload.Model)
			return
		}

		scanner := bufio.NewScanner(httpResp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		lastModel := payload.Model
		var lastUsage model.Usage
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
				chunks <- model.StreamEvent{Type: "completed", Done: true, Usage: lastUsage, Metadata: p.metadata(req, lastModel, "")}
				return
			}

			var streamResp chatResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				errs <- err
				return
			}
			if streamResp.Error != nil {
				errs <- p.modelError(streamResp.Error, firstNonEmpty(streamResp.Model, payload.Model))
				return
			}
			if streamResp.Model != "" {
				lastModel = streamResp.Model
			}
			if streamResp.Usage.TotalTokens > 0 {
				lastUsage = toModelUsage(streamResp.Usage)
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
					Usage:    lastUsage,
					Metadata: p.metadata(req, lastModel, choice.FinishReason),
				}:
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errs <- err
			return
		}
		chunks <- model.StreamEvent{Type: "completed", Done: true, Usage: lastUsage, Metadata: p.metadata(req, lastModel, "")}
	}()

	return chunks, errs
}

func (p *Provider) GenerateStream(ctx context.Context, req model.Request) (<-chan model.StreamEvent, <-chan error) {
	return p.ChatStream(ctx, req)
}

func (p *Provider) newChatRequest(req model.Request, stream bool) chatRequest {
	modelName := req.Model
	if modelName == "" {
		modelName = selectModel(req.Task, p.model, p.reasoningModel)
	}
	userID := metadataString(req.Metadata, model.RequestMetadataUserID)
	sessionID := metadataString(req.Metadata, model.RequestMetadataSessionID)
	payload := chatRequest{
		Model: modelName,
		Messages: []message{
			{Role: "system", Content: systemPromptForTask(req.Task)},
			{Role: "user", Content: req.ChatPrompt()},
		},
		Stream:    stream,
		User:      userID,
		SessionID: sessionID,
	}
	if req.TraceID != "" {
		payload.Trace = map[string]any{
			"trace_id":        req.TraceID,
			"generation_name": string(req.Task),
			"span_name":       "model.chat",
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
	p.decorateRequest(httpReq)

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
		apiErr := parseAPIError(httpResp.StatusCode, respBody)
		return nil, p.modelError(apiErr, payload.Model)
	}

	return respBody, nil
}

func (p *Provider) decorateRequest(httpReq *http.Request) {
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	if p.siteURL != "" {
		httpReq.Header.Set("HTTP-Referer", p.siteURL)
	}
	if p.appTitle != "" {
		httpReq.Header.Set("X-OpenRouter-Title", p.appTitle)
	}
	if p.metadataEnabled {
		httpReq.Header.Set("X-OpenRouter-Metadata", "enabled")
	}
}

func (p *Provider) metadata(req model.Request, modelName string, finishReason string) model.ResponseMetadata {
	return model.ResponseMetadata{
		Provider:     p.Name(),
		Model:        modelName,
		Task:         req.Task,
		Capability:   model.CapabilityChat,
		TraceID:      req.TraceID,
		FinishReason: finishReason,
	}
}

func (p *Provider) modelError(apiErr *apiError, modelName string) error {
	if apiErr == nil {
		return &model.ModelError{Code: "model_provider_error", Provider: p.Name(), Model: modelName, Retryable: false}
	}
	return &model.ModelError{
		Code:       mapErrorCode(apiErr),
		Provider:   p.Name(),
		Model:      modelName,
		Retryable:  isRetryable(apiErr),
		StatusCode: apiErr.Code,
		Metadata:   errorMetadata(apiErr),
		Cause:      fmt.Errorf("openrouter api error: status=%d message=%s", apiErr.Code, apiErr.Message),
	}
}

func readAPIError(httpResp *http.Response) (*apiError, error) {
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}
	return parseAPIError(httpResp.StatusCode, respBody), nil
}

func parseAPIError(statusCode int, body []byte) *apiError {
	var parsed struct {
		Error apiError `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error.Code != 0 {
		return &parsed.Error
	}
	return &apiError{Code: statusCode, Message: strings.TrimSpace(string(body))}
}

func mapErrorCode(apiErr *apiError) string {
	if apiErr != nil {
		if value, ok := apiErr.Metadata["error_type"].(string); ok && value != "" {
			switch value {
			case "rate_limit_exceeded":
				return "model_rate_limited"
			case "insufficient_credits":
				return "model_insufficient_credits"
			case "context_length_exceeded":
				return "model_context_too_long"
			case "policy_violation":
				return "model_policy_violation"
			}
		}
	}
	switch apiErr.Code {
	case http.StatusBadRequest:
		if strings.Contains(strings.ToLower(apiErr.Message), "context") {
			return "model_context_too_long"
		}
		return "model_bad_request"
	case http.StatusUnauthorized:
		return "model_auth_failed"
	case http.StatusForbidden:
		if isPolicyViolation(apiErr) {
			return "model_policy_violation"
		}
		return "model_provider_forbidden"
	case http.StatusPaymentRequired:
		return "model_insufficient_credits"
	case http.StatusRequestTimeout:
		return "model_timeout"
	case http.StatusTooManyRequests:
		return "model_rate_limited"
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		return "model_provider_unavailable"
	default:
		return "model_provider_error"
	}
}

func errorMetadata(apiErr *apiError) map[string]any {
	metadata := map[string]any{
		"provider_status_code": apiErr.Code,
		"provider_message":     apiErr.Message,
	}
	for key, value := range apiErr.Metadata {
		metadata[key] = value
	}
	if _, ok := metadata["provider_error_type"]; !ok {
		if value, ok := apiErr.Metadata["error_type"]; ok {
			metadata["provider_error_type"] = value
		}
	}
	return metadata
}

func isPolicyViolation(apiErr *apiError) bool {
	if apiErr == nil {
		return false
	}
	if value, ok := apiErr.Metadata["error_type"].(string); ok {
		switch value {
		case "policy_violation", "terms_of_service":
			return true
		}
	}
	lowerMessage := strings.ToLower(apiErr.Message)
	return strings.Contains(lowerMessage, "terms of service") ||
		strings.Contains(lowerMessage, "policy") ||
		strings.Contains(lowerMessage, "prohibited")
}

func isRetryable(apiErr *apiError) bool {
	switch mapErrorCode(apiErr) {
	case "model_timeout", "model_rate_limited", "model_provider_unavailable":
		return true
	default:
		return false
	}
}

func toModelUsage(value usage) model.Usage {
	return model.Usage{
		PromptTokens:     value.PromptTokens,
		CompletionTokens: value.CompletionTokens,
		TotalTokens:      value.TotalTokens,
		Estimated:        false,
	}
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

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
