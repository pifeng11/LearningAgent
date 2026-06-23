package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"learning-agent/internal/model"
)

const minExtractConfidence = 0.5

type LLMExtractor struct {
	router *model.Router
}

func NewLLMExtractor(router *model.Router) *LLMExtractor {
	return &LLMExtractor{router: router}
}

func (e *LLMExtractor) Extract(ctx context.Context, req ExtractRequest) ([]Entry, error) {
	resp, err := e.router.Generate(ctx, model.Request{
		Task:   "memory_extract",
		Prompt: buildMemoryExtractionPrompt(req),
	})
	if err != nil {
		return nil, err
	}

	parsed, err := parseExtractedMemories(resp.Text)
	if err != nil {
		// TODO: 增加 JSON repair / retry，提高 LLM 提取稳定性；失败时不降级为 summary，避免污染长期记忆。
		return nil, err
	}

	entries := []Entry{}
	for _, item := range parsed.Memories {
		entry, ok := item.toEntry(req)
		if ok {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func buildMemoryExtractionPrompt(req ExtractRequest) string {
	return fmt.Sprintf(`你是学习 Agent 的记忆提取器。只提取稳定、可复用、未来对学习辅导有帮助的记忆。
不要保存完整对话，不要保存临时寒暄，不要编造用户没表达的信息。
如果没有值得保存的记忆，返回 {"memories":[]}。

允许的 type: profile, goal, preference, progress, weakness, mistake, fact
允许的 scope: user, session
confidence 范围 0 到 1，低于 0.5 的信息不要输出。

只返回 JSON，不要 markdown，不要解释。
格式:
{
  "memories": [
    {
      "type": "goal",
      "title": "学习 Kubernetes",
      "content": "用户想学习 Kubernetes。",
      "scope": "user",
      "confidence": 0.9
    }
  ]
}

用户输入:
%s

助手回答:
%s`, req.Input, req.Answer)
}

type extractedMemoryList struct {
	Memories []extractedMemory `json:"memories"`
}

type extractedMemory struct {
	Type       string  `json:"type"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Scope      string  `json:"scope"`
	Confidence float64 `json:"confidence"`
}

func parseExtractedMemories(text string) (extractedMemoryList, error) {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var parsed extractedMemoryList
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return extractedMemoryList{}, err
	}
	return parsed, nil
}

func (m extractedMemory) toEntry(req ExtractRequest) (Entry, bool) {
	memoryType := strings.TrimSpace(m.Type)
	scope := strings.TrimSpace(m.Scope)
	if !allowedMemoryType(memoryType) || !allowedMemoryScope(scope) {
		return Entry{}, false
	}
	if strings.TrimSpace(m.Title) == "" || strings.TrimSpace(m.Content) == "" {
		return Entry{}, false
	}
	if m.Confidence < minExtractConfidence {
		return Entry{}, false
	}

	return NormalizeEntry(Entry{
		UserID:           req.UserID,
		SessionID:        req.SessionID,
		Type:             memoryType,
		Title:            strings.TrimSpace(m.Title),
		Content:          strings.TrimSpace(m.Content),
		Scope:            scope,
		Status:           StatusActive,
		Confidence:       m.Confidence,
		SourceMessageIDs: req.SourceMessageIDs,
		Metadata: map[string]any{
			"source":    "llm_extractor",
			"extractor": "llm",
		},
	}), true
}

func allowedMemoryType(memoryType string) bool {
	switch memoryType {
	case "profile", TypeGoal, TypePreference, "progress", TypeWeakness, TypeMistake, TypeFact:
		return true
	default:
		return false
	}
}

func allowedMemoryScope(scope string) bool {
	return scope == ScopeUser || scope == ScopeSession
}
