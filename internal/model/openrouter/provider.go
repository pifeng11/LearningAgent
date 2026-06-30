package openrouter

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultModel          = "openai/gpt-4o-mini"
	DefaultReasoningModel = "openai/gpt-4o"
)

type Config struct {
	APIKey          string
	BaseURL         string
	Model           string
	ReasoningModel  string
	SiteURL         string
	AppTitle        string
	MetadataEnabled bool
	HTTPClient      *http.Client
}

type Provider struct {
	apiKey          string
	baseURL         string
	model           string
	reasoningModel  string
	siteURL         string
	appTitle        string
	metadataEnabled bool
	httpClient      *http.Client
}

func NewProvider(cfg Config) (*Provider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("OPENROUTER_API_KEY is required when MODEL_PROVIDER=openrouter")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://openrouter.ai/api/v1"
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.ReasoningModel == "" {
		cfg.ReasoningModel = DefaultReasoningModel
	}
	if cfg.AppTitle == "" {
		cfg.AppTitle = "LearningAgent"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	}

	return &Provider{
		apiKey:          cfg.APIKey,
		baseURL:         strings.TrimRight(cfg.BaseURL, "/"),
		model:           cfg.Model,
		reasoningModel:  cfg.ReasoningModel,
		siteURL:         cfg.SiteURL,
		appTitle:        cfg.AppTitle,
		metadataEnabled: cfg.MetadataEnabled,
		httpClient:      cfg.HTTPClient,
	}, nil
}

func (p *Provider) Name() string {
	return "openrouter"
}
