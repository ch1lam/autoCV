package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
	"github.com/google/uuid"
)

const (
	defaultFakeModel     = "fixture-v1"
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	defaultOpenAIModel   = "gpt-5.5"
	openAISecretRef      = "openai-api-key"
)

var providerContentSummary = []ProviderDataSummary{
	{
		Label:       "JD 原文",
		Description: "仅在分析岗位时发送当前粘贴的岗位描述。",
	},
	{
		Label:       "相关 Evidence",
		Description: "只发送任务需要的证据内容与来源 ID，不发送整个资料目录。",
	},
	{
		Label:       "Requirement 与匹配上下文",
		Description: "发送结构化岗位要求、证据关联和必要的解释上下文。",
	},
	{
		Label:       "Resume Block 与包装参数",
		Description: "生成或审阅时发送当前结构化内容、锁定状态和包装档位。",
	},
}

var localOnlySummary = []ProviderDataSummary{
	{
		Label:       "API Key",
		Description: "保存在 macOS Keychain，前端和 SQLite 都不会读取明文。",
	},
	{
		Label:       "原始资料文件",
		Description: "Markdown、PDF 与 DOCX 原文件保留在本地受管理目录。",
	},
	{
		Label:       "HTML/PDF 渲染产物",
		Description: "HTML 排版、PDF 预览和导出完全在本机执行。",
	},
}

type SettingsService struct {
	repository ports.ProviderConfigRepository
	secrets    ports.SecretStore
	clock      ports.Clock
}

type ProviderDataSummary struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type ProviderSettings struct {
	Provider          string                `json:"provider"`
	BaseURL           string                `json:"baseUrl"`
	Model             string                `json:"model"`
	APIKeyConfigured  bool                  `json:"apiKeyConfigured"`
	SecretBackend     string                `json:"secretBackend"`
	SentContentTypes  []ProviderDataSummary `json:"sentContentTypes"`
	LocalOnlyTypes    []ProviderDataSummary `json:"localOnlyTypes"`
	ConfigurationNote string                `json:"configurationNote"`
	UpdatedAt         string                `json:"updatedAt"`
}

type SaveProviderInput struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	Model    string `json:"model"`
	APIKey   string `json:"apiKey"`
}

func NewSettingsService(
	repository ports.ProviderConfigRepository,
	secrets ports.SecretStore,
	clock ports.Clock,
) *SettingsService {
	return &SettingsService{
		repository: repository,
		secrets:    secrets,
		clock:      clock,
	}
}

func (service *SettingsService) GetSettings() (ProviderSettings, error) {
	return service.getSettings(context.Background())
}

func (service *SettingsService) SaveProvider(
	input SaveProviderInput,
) (ProviderSettings, error) {
	return service.saveProvider(context.Background(), input)
}

func (service *SettingsService) getSettings(
	ctx context.Context,
) (ProviderSettings, error) {
	config, found, err := service.repository.GetActive(ctx)
	if err != nil {
		return ProviderSettings{}, err
	}
	if !found {
		config = defaultProviderConfig(service.clock.Now().UTC())
	}

	keyConfigured, err := service.openAIKeyConfigured(ctx)
	if err != nil {
		return ProviderSettings{}, err
	}
	return providerSettings(config, keyConfigured), nil
}

func (service *SettingsService) saveProvider(
	ctx context.Context,
	input SaveProviderInput,
) (ProviderSettings, error) {
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if provider != domain.ProviderFake &&
		provider != domain.ProviderOpenAI {
		return ProviderSettings{}, errors.New(
			"Provider must be fake or openai",
		)
	}

	now := service.clock.Now().UTC()
	existing, found, err := service.repository.GetByProvider(ctx, provider)
	if err != nil {
		return ProviderSettings{}, err
	}
	config := domain.ProviderConfig{
		ID:        providerConfigID(provider),
		Provider:  provider,
		BaseURL:   strings.TrimSpace(input.BaseURL),
		Model:     strings.TrimSpace(input.Model),
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if found {
		config.ID = existing.ID
		config.CreatedAt = existing.CreatedAt
		config.SecretRef = existing.SecretRef
	}

	keyConfigured := false
	if provider == domain.ProviderFake {
		config.BaseURL = ""
		if config.Model == "" {
			config.Model = defaultFakeModel
		}
		config.SecretRef = ""
	} else {
		if config.BaseURL == "" {
			config.BaseURL = defaultOpenAIBaseURL
		}
		if config.Model == "" {
			config.Model = defaultOpenAIModel
		}
		config.SecretRef = openAISecretRef

		apiKey := strings.TrimSpace(input.APIKey)
		if apiKey != "" {
			if err := service.secrets.Set(
				ctx,
				config.SecretRef,
				apiKey,
			); err != nil {
				return ProviderSettings{}, fmt.Errorf(
					"save Provider API key: %w",
					err,
				)
			}
			keyConfigured = true
		} else {
			keyConfigured, err = service.secrets.Has(
				ctx,
				config.SecretRef,
			)
			if err != nil {
				return ProviderSettings{}, fmt.Errorf(
					"check Provider API key: %w",
					err,
				)
			}
			if !keyConfigured {
				return ProviderSettings{}, errors.New(
					"OpenAI API key is required",
				)
			}
		}
	}

	if err := config.Validate(); err != nil {
		return ProviderSettings{}, fmt.Errorf(
			"validate Provider settings: %w",
			err,
		)
	}
	if err := service.repository.Save(ctx, config); err != nil {
		return ProviderSettings{}, err
	}
	if provider == domain.ProviderFake {
		keyConfigured, err = service.openAIKeyConfigured(ctx)
		if err != nil {
			return ProviderSettings{}, err
		}
	}
	slog.Info(
		"provider.config.saved",
		slog.String("provider", config.Provider),
		slog.String("model", config.Model),
		slog.Bool("api_key_configured", keyConfigured),
	)
	return providerSettings(config, keyConfigured), nil
}

func (service *SettingsService) openAIKeyConfigured(
	ctx context.Context,
) (bool, error) {
	config, found, err := service.repository.GetByProvider(
		ctx,
		domain.ProviderOpenAI,
	)
	if err != nil {
		return false, err
	}
	if !found || config.SecretRef == "" {
		return false, nil
	}
	configured, err := service.secrets.Has(ctx, config.SecretRef)
	if err != nil {
		return false, fmt.Errorf("check Provider API key: %w", err)
	}
	return configured, nil
}

func defaultProviderConfig(now time.Time) domain.ProviderConfig {
	return domain.ProviderConfig{
		ID:        providerConfigID(domain.ProviderFake),
		Provider:  domain.ProviderFake,
		Model:     defaultFakeModel,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func providerConfigID(provider string) string {
	return uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte("https://autocv.local/providers/"+provider),
	).String()
}

func providerSettings(
	config domain.ProviderConfig,
	keyConfigured bool,
) ProviderSettings {
	note := "Fake Provider 使用固定 Fixture，适合离线演示与回归验证。"
	if config.Provider == domain.ProviderOpenAI {
		note = "OpenAI 配置已保存；请求使用严格结构化输出，发送前会显示内容类型摘要。"
	}
	updatedAt := ""
	if !config.UpdatedAt.IsZero() {
		updatedAt = config.UpdatedAt.UTC().Format(timeFormat)
	}
	return ProviderSettings{
		Provider:          config.Provider,
		BaseURL:           config.BaseURL,
		Model:             config.Model,
		APIKeyConfigured:  keyConfigured,
		SecretBackend:     "macOS Keychain",
		SentContentTypes:  append([]ProviderDataSummary(nil), providerContentSummary...),
		LocalOnlyTypes:    append([]ProviderDataSummary(nil), localOnlySummary...),
		ConfigurationNote: note,
		UpdatedAt:         updatedAt,
	}
}
