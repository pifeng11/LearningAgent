package memory

import (
	"context"
	"strings"
)

type ExtractRequest struct {
	UserID           string
	SessionID        string
	Input            string
	Answer           string
	SourceMessageIDs []int64
}

type Extractor interface {
	Extract(ctx context.Context, req ExtractRequest) ([]Entry, error)
}

type RuleExtractor struct{}

func NewRuleExtractor() *RuleExtractor {
	return &RuleExtractor{}
}

func (e *RuleExtractor) Extract(ctx context.Context, req ExtractRequest) ([]Entry, error) {
	input := strings.TrimSpace(req.Input)
	if input == "" {
		return []Entry{}, nil
	}

	content := strings.ToLower(input)
	switch {
	case containsAny(content, "want to learn", "i want to learn", "想学", "学习", "学会", "掌握"):
		return []Entry{newExtractedEntry(req, TypeGoal, "学习目标："+truncateForMemory(input, 80), "用户表达了学习目标："+input, ScopeUser)}, nil
	case containsAny(content, "prefer", "preference", "喜欢", "偏好", "更愿意"):
		return []Entry{newExtractedEntry(req, TypePreference, "学习偏好："+truncateForMemory(input, 80), "用户表达了学习偏好："+input, ScopeUser)}, nil
	default:
		return []Entry{newExtractedEntry(req, TypeSummary, "Conversation turn", "用户提问："+input+"\n助手回答："+truncateForMemory(req.Answer, 500), ScopeSession)}, nil
	}
}

func newExtractedEntry(req ExtractRequest, memoryType string, title string, content string, scope string) Entry {
	return NormalizeEntry(Entry{
		UserID:           req.UserID,
		SessionID:        req.SessionID,
		Type:             memoryType,
		Title:            title,
		Content:          content,
		Scope:            scope,
		Status:           StatusActive,
		Confidence:       0.8,
		SourceMessageIDs: req.SourceMessageIDs,
		Metadata: map[string]any{
			"source":    "rule_extractor",
			"extractor": "rule",
		},
	})
}

func containsAny(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func truncateForMemory(content string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "..."
}
