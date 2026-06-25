package prompt

import (
	"fmt"
	"sort"
	"strings"

	"learning-agent/internal/conversation"
	"learning-agent/internal/memory"
)

const DefaultSystemPrompt = `你是 Learning Agent，一个面向个人学习的综合型 AI Agent。

你的职责是帮助用户制定学习计划、解释知识、生成练习、复盘学习过程，并根据上下文持续改进建议。

回答要求：
- 优先结合当前用户问题、最近对话和已知记忆。
- 如果记忆或历史对当前问题无关，请忽略，不要强行引用。
- 不确定时要说明不确定，不能编造用户没有表达过的事实。
- 回答要具体、可执行，适合用户一步步学习。`

type Config struct {
	SystemPrompt    string
	MaxHistoryTurns int
	MaxMemories     int
	MaxPromptChars  int
}

type Builder struct {
	config Config
}

type BuildRequest struct {
	UserInput string
	Memories  []memory.Entry
	History   []conversation.Message
}

type BuildResult struct {
	Prompt              string
	UsedMemoryIDs       []int64
	UsedHistoryIDs      []string
	MemoryCount         int
	HistoryMessageCount int
	PromptChars         int
}

func NewBuilder(config Config) *Builder {
	if strings.TrimSpace(config.SystemPrompt) == "" {
		config.SystemPrompt = DefaultSystemPrompt
	}
	if config.MaxHistoryTurns <= 0 {
		config.MaxHistoryTurns = 5
	}
	if config.MaxMemories <= 0 {
		config.MaxMemories = 8
	}
	if config.MaxPromptChars <= 0 {
		config.MaxPromptChars = 12000
	}
	return &Builder{config: config}
}

func (b *Builder) MaxHistoryMessages() int {
	return b.config.MaxHistoryTurns * 2
}

func (b *Builder) Build(req BuildRequest) BuildResult {
	memories := selectMemories(req.Memories, b.config.MaxMemories)
	history := selectHistory(req.History, b.MaxHistoryMessages())

	// TODO: 当前使用字符数近似 token budget；后续应接入 tokenizer，按模型上下文窗口精确裁剪。
	for {
		result := b.render(req.UserInput, memories, history)
		if result.PromptChars <= b.config.MaxPromptChars {
			return result
		}
		if len(history) > 0 {
			history = history[1:]
			continue
		}
		if len(memories) > 0 {
			memories = memories[:len(memories)-1]
			continue
		}
		result.Prompt = trimPromptKeepCurrentInput(result.Prompt, req.UserInput, b.config.MaxPromptChars)
		result.PromptChars = len([]rune(result.Prompt))
		return result
	}
}

func (b *Builder) render(userInput string, memories []memory.Entry, history []conversation.Message) BuildResult {
	var builder strings.Builder
	builder.WriteString("# System Instruction\n")
	builder.WriteString(strings.TrimSpace(b.config.SystemPrompt))
	builder.WriteString("\n\n")

	if len(memories) > 0 {
		builder.WriteString("# Long-term Memory\n")
		// TODO: memory 当前按规则排序筛选；后续接向量召回、相关性评分和 confidence/decay 权重。
		for _, entry := range memories {
			builder.WriteString("- [")
			builder.WriteString(entry.Type)
			builder.WriteString("/")
			builder.WriteString(entry.Scope)
			builder.WriteString("] ")
			builder.WriteString(entry.Title)
			builder.WriteString(": ")
			builder.WriteString(truncate(entry.Content, 500))
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	if len(history) > 0 {
		builder.WriteString("# Recent Conversation\n")
		// TODO: history 当前只取最近窗口；后续增加 session summary，避免长会话只依赖最近 N 轮。
		for _, message := range history {
			builder.WriteString(formatRole(message.Role))
			builder.WriteString(": ")
			builder.WriteString(truncate(message.Content, 1000))
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	// TODO: system prompt 当前是单模板；后续支持按 task/intent 选择模板，并记录 prompt version。
	builder.WriteString("# Current User Input\n")
	builder.WriteString(strings.TrimSpace(userInput))

	result := BuildResult{
		Prompt:              builder.String(),
		UsedMemoryIDs:       memoryIDs(memories),
		UsedHistoryIDs:      historyIDs(history),
		MemoryCount:         len(memories),
		HistoryMessageCount: len(history),
	}
	result.PromptChars = len([]rune(result.Prompt))
	return result
}

func selectMemories(memories []memory.Entry, limit int) []memory.Entry {
	copied := append([]memory.Entry(nil), memories...)
	sort.SliceStable(copied, func(i, j int) bool {
		leftPriority := memoryPriority(copied[i])
		rightPriority := memoryPriority(copied[j])
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		return copied[i].UpdatedAt.After(copied[j].UpdatedAt)
	})

	selected := make([]memory.Entry, 0, min(limit, len(copied)))
	summaryCount := 0
	for _, entry := range copied {
		if entry.Type == memory.TypeSummary {
			if summaryCount >= 2 {
				continue
			}
			summaryCount++
		}
		selected = append(selected, entry)
		if len(selected) >= limit {
			break
		}
	}
	return selected
}

func selectHistory(history []conversation.Message, limit int) []conversation.Message {
	filtered := make([]conversation.Message, 0, len(history))
	for _, message := range history {
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		filtered = append(filtered, message)
	}
	if limit <= 0 || len(filtered) <= limit {
		return filtered
	}
	return filtered[len(filtered)-limit:]
}

func memoryPriority(entry memory.Entry) int {
	switch entry.Type {
	case memory.TypeGoal:
		return 0
	case memory.TypePreference:
		return 1
	case memory.TypeWeakness, memory.TypeMistake:
		return 2
	case memory.TypeFact:
		return 3
	case memory.TypeSummary:
		return 9
	default:
		return 5
	}
}

func formatRole(role string) string {
	switch strings.ToLower(role) {
	case "user":
		return "User"
	case "assistant":
		return "Assistant"
	default:
		return fmt.Sprintf("%s", role)
	}
}

func memoryIDs(memories []memory.Entry) []int64 {
	ids := make([]int64, 0, len(memories))
	for _, entry := range memories {
		if entry.ID > 0 {
			ids = append(ids, entry.ID)
		}
	}
	return ids
}

func historyIDs(history []conversation.Message) []string {
	ids := make([]string, 0, len(history))
	for _, message := range history {
		if message.ID != "" {
			ids = append(ids, message.ID)
		}
	}
	return ids
}

func truncate(content string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "..."
}

func trimPromptKeepCurrentInput(prompt string, userInput string, maxRunes int) string {
	runes := []rune(prompt)
	if len(runes) <= maxRunes {
		return prompt
	}
	inputSection := "# Current User Input\n" + strings.TrimSpace(userInput)
	inputRunes := []rune(inputSection)
	if len(inputRunes) >= maxRunes {
		return string(inputRunes[len(inputRunes)-maxRunes:])
	}
	prefixBudget := maxRunes - len(inputRunes) - 1
	if prefixBudget <= 0 {
		return inputSection
	}
	prefix := []rune(prompt)
	return string(prefix[:prefixBudget]) + "\n" + inputSection
}
