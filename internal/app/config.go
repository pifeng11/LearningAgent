package app

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr                string
	DatabaseURL             string
	Environment             string
	ModelProvider           string
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
		ModelProvider:           envOrDefault("MODEL_PROVIDER", "mock"),
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
