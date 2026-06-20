package app

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	Environment string
}

func LoadConfig() Config {
	return Config{
		HTTPAddr:    envOrDefault("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Environment: envOrDefault("APP_ENV", "dev"),
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
