package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	ProviderFake   = "fake"
	ProviderOpenAI = "openai"
)

type ProviderConfig struct {
	ID        string
	Provider  string
	BaseURL   string
	Model     string
	SecretRef string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (config ProviderConfig) Validate() error {
	switch config.Provider {
	case ProviderFake, ProviderOpenAI:
	default:
		return errors.New("provider must be fake or openai")
	}
	if strings.TrimSpace(config.Model) == "" {
		return errors.New("provider model is empty")
	}
	if config.Provider == ProviderOpenAI &&
		strings.TrimSpace(config.SecretRef) == "" {
		return errors.New("OpenAI secret reference is empty")
	}
	return nil
}
