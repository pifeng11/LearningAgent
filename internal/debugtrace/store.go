package debugtrace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const PromptBuilderVersion = "v1"

type PromptTrace struct {
	TraceID                string         `json:"trace_id"`
	UserID                 string         `json:"user_id"`
	SessionID              string         `json:"session_id"`
	Intent                 string         `json:"intent"`
	ModelTask              string         `json:"model_task"`
	UsedMemoryIDs          []int64        `json:"used_memory_ids"`
	UsedHistoryIDs         []string       `json:"used_history_ids"`
	MemoryCount            int            `json:"memory_count"`
	HistoryMessageCount    int            `json:"history_message_count"`
	PromptChars            int            `json:"prompt_chars"`
	EstimatedPromptTokens  int            `json:"estimated_prompt_tokens"`
	PromptBuilderVersion   string         `json:"prompt_builder_version"`
	SystemPromptHash       string         `json:"system_prompt_hash"`
	PromptConfig           map[string]any `json:"prompt_config,omitempty"`
	Prompt                 string         `json:"prompt,omitempty"`
	ContextItems           []ContextItem  `json:"context_items,omitempty"`
	ContextSnapshotEnabled bool           `json:"context_snapshot_enabled"`
	CreatedAt              time.Time      `json:"created_at"`
}

type ContextItem struct {
	ItemType string         `json:"item_type"`
	SourceID string         `json:"source_id,omitempty"`
	Role     string         `json:"role,omitempty"`
	Title    string         `json:"title,omitempty"`
	Content  string         `json:"content"`
	Ordinal  int            `json:"ordinal"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type ReconstructedPrompt struct {
	TraceID     string `json:"trace_id"`
	Prompt      string `json:"prompt"`
	PromptChars int    `json:"prompt_chars"`
	Source      string `json:"source"`
}

type TokenReport struct {
	TraceID               string  `json:"trace_id"`
	Prompt                string  `json:"prompt"`
	PromptChars           int     `json:"prompt_chars"`
	EstimatedPromptTokens int     `json:"estimated_prompt_tokens"`
	Tokenizer             string  `json:"tokenizer"`
	Tokens                []Token `json:"tokens"`
}

type Token struct {
	Index   int    `json:"index"`
	Text    string `json:"text"`
	TokenID int    `json:"token_id,omitempty"`
}

type Store interface {
	Save(ctx context.Context, trace PromptTrace) error
	Get(ctx context.Context, traceID string) (PromptTrace, bool, error)
}

type NoopStore struct{}

func (NoopStore) Save(ctx context.Context, trace PromptTrace) error {
	return nil
}

func (NoopStore) Get(ctx context.Context, traceID string) (PromptTrace, bool, error) {
	return PromptTrace{}, false, nil
}

type RingStore struct {
	mu       sync.RWMutex
	capacity int
	order    []string
	traces   map[string]PromptTrace
}

func NewRingStore(capacity int) *RingStore {
	if capacity <= 0 {
		capacity = 100
	}
	return &RingStore{
		capacity: capacity,
		order:    make([]string, 0, capacity),
		traces:   map[string]PromptTrace{},
	}
}

func (s *RingStore) Save(ctx context.Context, trace PromptTrace) error {
	if trace.TraceID == "" {
		return nil
	}
	trace = normalizeTrace(trace)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.traces[trace.TraceID]; !exists {
		s.order = append(s.order, trace.TraceID)
	}
	s.traces[trace.TraceID] = trace

	for len(s.order) > s.capacity {
		oldest := s.order[0]
		s.order = s.order[1:]
		delete(s.traces, oldest)
	}
	return nil
}

func (s *RingStore) Get(ctx context.Context, traceID string) (PromptTrace, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trace, ok := s.traces[traceID]
	return trace, ok, nil
}

func ReconstructPrompt(trace PromptTrace) ReconstructedPrompt {
	if strings.TrimSpace(trace.Prompt) != "" {
		return ReconstructedPrompt{
			TraceID:     trace.TraceID,
			Prompt:      trace.Prompt,
			PromptChars: len([]rune(trace.Prompt)),
			Source:      "prompt_text",
		}
	}

	prompt := BuildPromptFromContextItems(trace.ContextItems)
	return ReconstructedPrompt{
		TraceID:     trace.TraceID,
		Prompt:      prompt,
		PromptChars: len([]rune(prompt)),
		Source:      "context_snapshot",
	}
}

func BuildTokenReport(trace PromptTrace) TokenReport {
	reconstructed := ReconstructPrompt(trace)
	return TokenReport{
		TraceID:               trace.TraceID,
		Prompt:                reconstructed.Prompt,
		PromptChars:           reconstructed.PromptChars,
		EstimatedPromptTokens: EstimateTokens(reconstructed.Prompt),
		Tokenizer:             "estimate",
		Tokens:                []Token{},
	}
}

func BuildPromptFromContextItems(items []ContextItem) string {
	if len(items) == 0 {
		return ""
	}

	items = append([]ContextItem(nil), items...)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Ordinal < items[j].Ordinal
	})

	var builder strings.Builder
	systemPrompt := firstItem(items, "system_prompt")
	if systemPrompt != nil {
		builder.WriteString("# System Instruction\n")
		builder.WriteString(strings.TrimSpace(systemPrompt.Content))
		builder.WriteString("\n\n")
	}

	memoryItems := itemsByType(items, "memory")
	if len(memoryItems) > 0 {
		builder.WriteString("# Long-term Memory\n")
		for _, item := range memoryItems {
			builder.WriteString("- [")
			builder.WriteString(metadataString(item.Metadata, "type"))
			builder.WriteString("/")
			builder.WriteString(metadataString(item.Metadata, "scope"))
			builder.WriteString("] ")
			builder.WriteString(item.Title)
			builder.WriteString(": ")
			builder.WriteString(strings.TrimSpace(item.Content))
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	historyItems := itemsByType(items, "history")
	if len(historyItems) > 0 {
		builder.WriteString("# Recent Conversation\n")
		for _, item := range historyItems {
			builder.WriteString(item.Role)
			builder.WriteString(": ")
			builder.WriteString(strings.TrimSpace(item.Content))
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	currentInput := firstItem(items, "current_input")
	if currentInput != nil {
		builder.WriteString("# Current User Input\n")
		builder.WriteString(strings.TrimSpace(currentInput.Content))
	}
	return builder.String()
}

func EstimateTokens(text string) int {
	runeCount := len([]rune(strings.TrimSpace(text)))
	if runeCount == 0 {
		return 0
	}
	// TODO: 接入 DeepSeek 或兼容模型 tokenizer 后，用真实 token 数替换当前估算。
	return (runeCount + 3) / 4
}

func HashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func normalizeTrace(trace PromptTrace) PromptTrace {
	if trace.CreatedAt.IsZero() {
		trace.CreatedAt = time.Now()
	}
	if trace.PromptBuilderVersion == "" {
		trace.PromptBuilderVersion = PromptBuilderVersion
	}
	if trace.EstimatedPromptTokens == 0 {
		promptText := trace.Prompt
		if promptText == "" {
			promptText = BuildPromptFromContextItems(trace.ContextItems)
		}
		trace.EstimatedPromptTokens = EstimateTokens(promptText)
	}
	return trace
}

func firstItem(items []ContextItem, itemType string) *ContextItem {
	for _, item := range items {
		if item.ItemType == itemType {
			copied := item
			return &copied
		}
	}
	return nil
}

func itemsByType(items []ContextItem, itemType string) []ContextItem {
	result := []ContextItem{}
	for _, item := range items {
		if item.ItemType == itemType {
			result = append(result, item)
		}
	}
	return result
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	return fmt.Sprint(value)
}
