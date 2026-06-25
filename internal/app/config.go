package app

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr                string
	DatabaseURL             string
	Environment             string
	MemoryStore             string
	MemoryExtractor         string
	MemoryExtractTimeout    time.Duration
	LocalDataPath           string
	LocalMessagesPath       string
	PromptMaxHistoryTurns   int
	PromptMaxMemories       int
	PromptMaxChars          int
	PromptSystemFile        string
	DebugTraceEnabled       bool
	DebugTraceCapacity      int
	DebugPromptEnabled      bool
	TraceStore              string
	TraceCapturePromptText  bool
	TraceContextSnapshot    bool
	TraceTokenEstimation    bool
	ModelProvider           string
	ModelDefaultModel       string
	ModelTaskQA             string
	ModelTaskLearningPlan   string
	ModelTaskPractice       string
	ModelTaskReview         string
	ModelTaskMemoryExtract  string
	ModelTimeout            time.Duration
	ModelStreamTimeout      time.Duration
	ModelMaxRetries         int
	ModelRetryBackoff       time.Duration
	DeepSeekAPIKey          string
	DeepSeekBaseURL         string
	DeepSeekModel           string
	DeepSeekReasoningModel  string
	DeepSeekReasoningEffort string
	DeepSeekThinkingEnabled bool
}

func LoadConfig() Config {
	return Config{
		HTTPAddr:                envOrDefault("HTTP_ADDR", ":8080"),
		DatabaseURL:             os.Getenv("DATABASE_URL"),
		Environment:             envOrDefault("APP_ENV", "dev"),
		MemoryStore:             envOrDefault("MEMORY_STORE", "local"),
		MemoryExtractor:         envOrDefault("MEMORY_EXTRACTOR", "llm"),
		MemoryExtractTimeout:    envDuration("MEMORY_EXTRACT_TIMEOUT", 30*time.Second),
		LocalDataPath:           envOrDefault("LOCAL_DATA_PATH", "data/memories.jsonl"),
		LocalMessagesPath:       envOrDefault("LOCAL_MESSAGES_PATH", "data/messages.jsonl"),
		PromptMaxHistoryTurns:   envInt("PROMPT_MAX_HISTORY_TURNS", 5),
		PromptMaxMemories:       envInt("PROMPT_MAX_MEMORIES", 8),
		PromptMaxChars:          envInt("PROMPT_MAX_CHARS", 12000),
		PromptSystemFile:        os.Getenv("PROMPT_SYSTEM_FILE"),
		DebugTraceEnabled:       envBool("DEBUG_TRACE_ENABLED", true),
		DebugTraceCapacity:      envInt("DEBUG_TRACE_CAPACITY", 100),
		DebugPromptEnabled:      envBool("DEBUG_PROMPT_ENABLED", false),
		TraceStore:              envOrDefault("TRACE_STORE", "memory"),
		TraceCapturePromptText:  envBool("TRACE_CAPTURE_PROMPT_TEXT", false),
		TraceContextSnapshot:    envBool("TRACE_CONTEXT_SNAPSHOT", true),
		TraceTokenEstimation:    envBool("TRACE_TOKEN_ESTIMATION_ENABLED", true),
		ModelProvider:           envOrDefault("MODEL_PROVIDER", "mock"),
		ModelDefaultModel:       envOrDefault("MODEL_DEFAULT_MODEL", envOrDefault("DEEPSEEK_MODEL", "deepseek-v4-flash")),
		ModelTaskQA:             os.Getenv("MODEL_TASK_QA"),
		ModelTaskLearningPlan:   os.Getenv("MODEL_TASK_LEARNING_PLAN"),
		ModelTaskPractice:       os.Getenv("MODEL_TASK_PRACTICE"),
		ModelTaskReview:         os.Getenv("MODEL_TASK_REVIEW"),
		ModelTaskMemoryExtract:  os.Getenv("MODEL_TASK_MEMORY_EXTRACT"),
		ModelTimeout:            envDuration("MODEL_TIMEOUT", 60*time.Second),
		ModelStreamTimeout:      envDuration("MODEL_STREAM_TIMEOUT", 120*time.Second),
		ModelMaxRetries:         envInt("MODEL_MAX_RETRIES", 0),
		ModelRetryBackoff:       envDuration("MODEL_RETRY_BACKOFF", 500*time.Millisecond),
		DeepSeekAPIKey:          os.Getenv("DEEPSEEK_API_KEY"),
		DeepSeekBaseURL:         envOrDefault("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
		DeepSeekModel:           envOrDefault("DEEPSEEK_MODEL", "deepseek-v4-flash"),
		DeepSeekReasoningModel:  envOrDefault("DEEPSEEK_REASONING_MODEL", "deepseek-v4-pro"),
		DeepSeekReasoningEffort: envOrDefault("DEEPSEEK_REASONING_EFFORT", "medium"),
		DeepSeekThinkingEnabled: envBool("DEEPSEEK_THINKING_ENABLED", false),
	}
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
