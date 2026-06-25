package deepseek

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

const (
	DeepSeekV4Flash = "deepseek-v4-flash"
	DeepSeekV4Pro   = "deepseek-v4-pro"
)

type Config struct {
	APIKey          string
	BaseURL         string
	Model           string
	ReasoningModel  string
	ReasoningEffort string
	ThinkingEnabled bool
	HTTPClient      *http.Client
}

type Provider struct {
	apiKey          string
	baseURL         string
	model           string
	reasoningModel  string
	reasoningEffort string
	thinkingEnabled bool
	httpClient      *http.Client
}

func NewProvider(cfg Config) (*Provider, error) {
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

	return &Provider{
		apiKey:          cfg.APIKey,
		baseURL:         strings.TrimRight(cfg.BaseURL, "/"),
		model:           cfg.Model,
		reasoningModel:  cfg.ReasoningModel,
		reasoningEffort: cfg.ReasoningEffort,
		thinkingEnabled: cfg.ThinkingEnabled,
		httpClient:      cfg.HTTPClient,
	}, nil
}

func (p *Provider) Name() string {
	return "deepseek"
}
