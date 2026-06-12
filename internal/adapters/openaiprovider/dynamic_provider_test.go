package openaiprovider

import (
	"context"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
)

type dynamicConfigRepository struct {
	config domain.ProviderConfig
	found  bool
}

func (repository dynamicConfigRepository) GetActive(
	context.Context,
) (domain.ProviderConfig, bool, error) {
	return repository.config, repository.found, nil
}

func (repository dynamicConfigRepository) GetByProvider(
	context.Context,
	string,
) (domain.ProviderConfig, bool, error) {
	return repository.config, repository.found, nil
}

func (dynamicConfigRepository) Save(
	context.Context,
	domain.ProviderConfig,
) error {
	return nil
}

type dynamicSecretStore struct {
	reference string
	secret    string
	found     bool
}

func (*dynamicSecretStore) Set(context.Context, string, string) error {
	return nil
}

func (store *dynamicSecretStore) Get(
	_ context.Context,
	reference string,
) (string, bool, error) {
	store.reference = reference
	return store.secret, store.found, nil
}

func (*dynamicSecretStore) Has(context.Context, string) (bool, error) {
	return false, nil
}

func (*dynamicSecretStore) Delete(context.Context, string) error {
	return nil
}

func TestDynamicProviderResolvesActiveConfigAndSecret(t *testing.T) {
	secrets := &dynamicSecretStore{
		secret: "test-key",
		found:  true,
	}
	provider, err := NewDynamicProvider(
		dynamicConfigRepository{
			found: true,
			config: domain.ProviderConfig{
				Provider:  domain.ProviderOpenAI,
				BaseURL:   "https://api.openai.com/v1",
				Model:     "gpt-test",
				SecretRef: "openai-api-key",
				Enabled:   true,
			},
		},
		secrets,
		&memoryCallRecorder{},
		time.Second,
		1,
	)
	if err != nil {
		t.Fatalf("create dynamic Provider: %v", err)
	}
	resolved, err := provider.resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve dynamic Provider: %v", err)
	}
	if resolved.model != "gpt-test" ||
		secrets.reference != "openai-api-key" {
		t.Fatalf(
			"unexpected resolved Provider model=%q reference=%q",
			resolved.model,
			secrets.reference,
		)
	}
	if client, ok := resolved.client.(*SDKClient); !ok ||
		client.model != "gpt-test" {
		t.Fatalf("unexpected SDK client %#v", resolved.client)
	}
}

func TestDynamicProviderRejectsMissingSecret(t *testing.T) {
	provider, err := NewDynamicProvider(
		dynamicConfigRepository{
			found: true,
			config: domain.ProviderConfig{
				Provider:  domain.ProviderOpenAI,
				Model:     "gpt-test",
				SecretRef: "openai-api-key",
				Enabled:   true,
			},
		},
		&dynamicSecretStore{},
		&memoryCallRecorder{},
		time.Second,
		0,
	)
	if err != nil {
		t.Fatalf("create dynamic Provider: %v", err)
	}
	if _, err := provider.resolve(context.Background()); err == nil {
		t.Fatal("expected missing OpenAI API key error")
	}
}
