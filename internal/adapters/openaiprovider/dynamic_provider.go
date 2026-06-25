package openaiprovider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

type DynamicProvider struct {
	configs    ports.ProviderConfigRepository
	secrets    ports.SecretStore
	recorder   ports.ProviderCallRecorder
	timeout    time.Duration
	maxRetries int
}

func NewDynamicProvider(
	configs ports.ProviderConfigRepository,
	secrets ports.SecretStore,
	recorder ports.ProviderCallRecorder,
	timeout time.Duration,
	maxRetries int,
) (*DynamicProvider, error) {
	if configs == nil {
		return nil, errors.New("Provider config repository is nil")
	}
	if secrets == nil {
		return nil, errors.New("Provider secret store is nil")
	}
	if recorder == nil {
		return nil, errors.New("Provider call recorder is nil")
	}
	if timeout <= 0 {
		return nil, errors.New("OpenAI timeout must be positive")
	}
	if maxRetries < 0 || maxRetries > 1 {
		return nil, errors.New("OpenAI max retries must be 0 or 1")
	}
	return &DynamicProvider{
		configs:    configs,
		secrets:    secrets,
		recorder:   recorder,
		timeout:    timeout,
		maxRetries: maxRetries,
	}, nil
}

func (provider *DynamicProvider) ExtractProfile(
	ctx context.Context,
	request ports.ExtractProfileRequest,
) ([]domain.ExtractedEvidence, error) {
	resolved, err := provider.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return resolved.ExtractProfile(ctx, request)
}

func (provider *DynamicProvider) AnalyzeJD(
	ctx context.Context,
	request ports.AnalyzeJDRequest,
) (domain.JDAnalysis, error) {
	resolved, err := provider.resolve(ctx)
	if err != nil {
		return domain.JDAnalysis{}, err
	}
	return resolved.AnalyzeJD(ctx, request)
}

func (provider *DynamicProvider) SuggestMatches(
	ctx context.Context,
	request ports.SuggestMatchesRequest,
) ([]domain.MatchSuggestion, error) {
	resolved, err := provider.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return resolved.SuggestMatches(ctx, request)
}

func (provider *DynamicProvider) DraftResume(
	ctx context.Context,
	request ports.DraftResumeRequest,
) (domain.ResumeDraft, error) {
	resolved, err := provider.resolve(ctx)
	if err != nil {
		return domain.ResumeDraft{}, err
	}
	return resolved.DraftResume(ctx, request)
}

func (provider *DynamicProvider) ComposeResumeHTML(
	ctx context.Context,
	request ports.ComposeResumeHTMLRequest,
) (ports.ComposedResumeHTML, error) {
	resolved, err := provider.resolve(ctx)
	if err != nil {
		return ports.ComposedResumeHTML{}, err
	}
	return resolved.ComposeResumeHTML(ctx, request)
}

func (provider *DynamicProvider) CacheKey() string {
	return "openaiprovider/dynamic/resume-html/v1"
}

func (provider *DynamicProvider) resolve(
	ctx context.Context,
) (*Provider, error) {
	config, found, err := provider.configs.GetActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("load active OpenAI config: %w", err)
	}
	if !found || config.Provider != domain.ProviderOpenAI {
		return nil, errors.New("OpenAI Provider is not active")
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validate OpenAI config: %w", err)
	}
	apiKey, found, err := provider.secrets.Get(ctx, config.SecretRef)
	if err != nil {
		return nil, fmt.Errorf("read OpenAI API key: %w", err)
	}
	if !found {
		return nil, errors.New("OpenAI API key is not configured")
	}

	client, err := NewSDKClient(apiKey, config.BaseURL, config.Model)
	if err != nil {
		return nil, err
	}
	resolved, err := New(
		client,
		provider.recorder,
		config.Model,
		provider.timeout,
		provider.maxRetries,
	)
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

var (
	_ ports.ProfileExtractor   = (*DynamicProvider)(nil)
	_ ports.JDAnalyzer         = (*DynamicProvider)(nil)
	_ ports.MatchSuggester     = (*DynamicProvider)(nil)
	_ ports.ResumeDrafter      = (*DynamicProvider)(nil)
	_ ports.ResumeHTMLComposer = (*DynamicProvider)(nil)
)
