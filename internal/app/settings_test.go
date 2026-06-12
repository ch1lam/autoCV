package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
)

type memoryProviderConfigRepository struct {
	configs map[string]domain.ProviderConfig
	active  string
	saveErr error
}

func newMemoryProviderConfigRepository() *memoryProviderConfigRepository {
	return &memoryProviderConfigRepository{
		configs: make(map[string]domain.ProviderConfig),
	}
}

func (repository *memoryProviderConfigRepository) GetActive(
	_ context.Context,
) (domain.ProviderConfig, bool, error) {
	config, found := repository.configs[repository.active]
	return config, found, nil
}

func (repository *memoryProviderConfigRepository) GetByProvider(
	_ context.Context,
	provider string,
) (domain.ProviderConfig, bool, error) {
	config, found := repository.configs[provider]
	return config, found, nil
}

func (repository *memoryProviderConfigRepository) Save(
	_ context.Context,
	config domain.ProviderConfig,
) error {
	if repository.saveErr != nil {
		return repository.saveErr
	}
	for provider, item := range repository.configs {
		item.Enabled = false
		repository.configs[provider] = item
	}
	repository.configs[config.Provider] = config
	if config.Enabled {
		repository.active = config.Provider
	}
	return nil
}

type memorySecretStore struct {
	values map[string]string
	setErr error
}

func newMemorySecretStore() *memorySecretStore {
	return &memorySecretStore{values: make(map[string]string)}
}

func (store *memorySecretStore) Set(
	_ context.Context,
	reference string,
	secret string,
) error {
	if store.setErr != nil {
		return store.setErr
	}
	store.values[reference] = secret
	return nil
}

func (store *memorySecretStore) Get(
	_ context.Context,
	reference string,
) (string, bool, error) {
	value, found := store.values[reference]
	return value, found, nil
}

func (store *memorySecretStore) Has(
	_ context.Context,
	reference string,
) (bool, error) {
	_, found := store.values[reference]
	return found, nil
}

func (store *memorySecretStore) Delete(
	_ context.Context,
	reference string,
) error {
	delete(store.values, reference)
	return nil
}

func TestSettingsServiceDefaultsToFakeProvider(t *testing.T) {
	service := NewSettingsService(
		newMemoryProviderConfigRepository(),
		newMemorySecretStore(),
		fixedClock{now: time.Date(2026, 6, 12, 8, 0, 0, 0, time.UTC)},
	)
	settings, err := service.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if settings.Provider != domain.ProviderFake ||
		settings.Model != defaultFakeModel ||
		settings.APIKeyConfigured {
		t.Fatalf("unexpected default settings %#v", settings)
	}
	if len(settings.SentContentTypes) != 4 ||
		len(settings.LocalOnlyTypes) != 3 {
		t.Fatalf("unexpected privacy summary %#v", settings)
	}
}

func TestSettingsServiceStoresOpenAIKeyOutsideRepository(t *testing.T) {
	repository := newMemoryProviderConfigRepository()
	secrets := newMemorySecretStore()
	service := NewSettingsService(
		repository,
		secrets,
		fixedClock{now: time.Date(2026, 6, 12, 8, 0, 0, 0, time.UTC)},
	)
	settings, err := service.SaveProvider(SaveProviderInput{
		Provider: domain.ProviderOpenAI,
		Model:    "gpt-5.5",
		APIKey:   "sk-test-secret",
	})
	if err != nil {
		t.Fatalf("save OpenAI settings: %v", err)
	}

	config := repository.configs[domain.ProviderOpenAI]
	if config.SecretRef != openAISecretRef ||
		config.BaseURL != defaultOpenAIBaseURL {
		t.Fatalf("unexpected stored config %#v", config)
	}
	if _, found := secrets.values[openAISecretRef]; !found {
		t.Fatal("expected API key in secret store")
	}
	if settings.APIKeyConfigured != true {
		t.Fatalf("unexpected settings %#v", settings)
	}
}

func TestSettingsServicePreservesExistingOpenAIKey(t *testing.T) {
	repository := newMemoryProviderConfigRepository()
	secrets := newMemorySecretStore()
	secrets.values[openAISecretRef] = "sk-existing"
	now := time.Date(2026, 6, 12, 8, 0, 0, 0, time.UTC)
	repository.configs[domain.ProviderOpenAI] = domain.ProviderConfig{
		ID:        "provider-openai",
		Provider:  domain.ProviderOpenAI,
		BaseURL:   defaultOpenAIBaseURL,
		Model:     "gpt-5.5",
		SecretRef: openAISecretRef,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	repository.active = domain.ProviderOpenAI
	service := NewSettingsService(
		repository,
		secrets,
		fixedClock{now: now.Add(time.Hour)},
	)

	settings, err := service.SaveProvider(SaveProviderInput{
		Provider: domain.ProviderOpenAI,
		Model:    "gpt-5.5",
	})
	if err != nil {
		t.Fatalf("save existing OpenAI settings: %v", err)
	}
	if !settings.APIKeyConfigured ||
		secrets.values[openAISecretRef] != "sk-existing" {
		t.Fatalf("expected existing key to be preserved %#v", settings)
	}
}

func TestSettingsServiceReportsOpenAIKeyWhileFakeProviderIsActive(t *testing.T) {
	repository := newMemoryProviderConfigRepository()
	secrets := newMemorySecretStore()
	secrets.values[openAISecretRef] = "sk-existing"
	now := time.Date(2026, 6, 12, 8, 0, 0, 0, time.UTC)
	repository.configs[domain.ProviderOpenAI] = domain.ProviderConfig{
		ID:        "provider-openai",
		Provider:  domain.ProviderOpenAI,
		BaseURL:   defaultOpenAIBaseURL,
		Model:     "gpt-5.5",
		SecretRef: openAISecretRef,
		CreatedAt: now,
		UpdatedAt: now,
	}
	repository.configs[domain.ProviderFake] = defaultProviderConfig(now)
	repository.active = domain.ProviderFake
	service := NewSettingsService(
		repository,
		secrets,
		fixedClock{now: now},
	)

	settings, err := service.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if settings.Provider != domain.ProviderFake ||
		!settings.APIKeyConfigured {
		t.Fatalf("unexpected settings %#v", settings)
	}
}

func TestSettingsServiceRequiresOpenAIKey(t *testing.T) {
	service := NewSettingsService(
		newMemoryProviderConfigRepository(),
		newMemorySecretStore(),
		fixedClock{now: time.Date(2026, 6, 12, 8, 0, 0, 0, time.UTC)},
	)
	if _, err := service.SaveProvider(SaveProviderInput{
		Provider: domain.ProviderOpenAI,
		Model:    "gpt-5.5",
	}); err == nil {
		t.Fatal("expected missing OpenAI API key error")
	}
}

func TestSettingsServiceDoesNotPersistConfigWhenKeychainFails(t *testing.T) {
	repository := newMemoryProviderConfigRepository()
	secrets := newMemorySecretStore()
	secrets.setErr = errors.New("keychain unavailable")
	service := NewSettingsService(
		repository,
		secrets,
		fixedClock{now: time.Date(2026, 6, 12, 8, 0, 0, 0, time.UTC)},
	)
	if _, err := service.SaveProvider(SaveProviderInput{
		Provider: domain.ProviderOpenAI,
		Model:    "gpt-5.5",
		APIKey:   "sk-test",
	}); err == nil {
		t.Fatal("expected keychain failure")
	}
	if len(repository.configs) != 0 {
		t.Fatalf("config should not persist after keychain failure %#v", repository.configs)
	}
}
