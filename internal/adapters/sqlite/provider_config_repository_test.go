package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
)

func TestProviderConfigRepositorySwitchesActiveProvider(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, ctx, filepath.Join(t.TempDir(), "provider.db"))
	defer db.Close()

	repository := NewProviderConfigRepository(db)
	now := time.Date(2026, 6, 12, 6, 0, 0, 0, time.UTC)
	fake := domain.ProviderConfig{
		ID:        "provider-fake",
		Provider:  domain.ProviderFake,
		Model:     "fixture-v1",
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repository.Save(ctx, fake); err != nil {
		t.Fatalf("save fake provider: %v", err)
	}

	openAI := domain.ProviderConfig{
		ID:        "provider-openai",
		Provider:  domain.ProviderOpenAI,
		BaseURL:   "https://api.openai.com/v1",
		Model:     "gpt-5.5",
		SecretRef: "openai-api-key",
		Enabled:   true,
		CreatedAt: now.Add(time.Minute),
		UpdatedAt: now.Add(time.Minute),
	}
	if err := repository.Save(ctx, openAI); err != nil {
		t.Fatalf("save OpenAI provider: %v", err)
	}

	active, found, err := repository.GetActive(ctx)
	if err != nil {
		t.Fatalf("get active provider: %v", err)
	}
	if !found || active.Provider != domain.ProviderOpenAI {
		t.Fatalf("unexpected active provider %#v", active)
	}

	storedFake, found, err := repository.GetByProvider(ctx, domain.ProviderFake)
	if err != nil {
		t.Fatalf("get fake provider: %v", err)
	}
	if !found || storedFake.Enabled {
		t.Fatalf("expected fake provider to be disabled, got %#v", storedFake)
	}
}

func TestProviderConfigRepositoryDoesNotPersistAPIKey(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, ctx, filepath.Join(t.TempDir(), "provider.db"))
	defer db.Close()

	repository := NewProviderConfigRepository(db)
	now := time.Date(2026, 6, 12, 6, 0, 0, 0, time.UTC)
	config := domain.ProviderConfig{
		ID:        "provider-openai",
		Provider:  domain.ProviderOpenAI,
		BaseURL:   "https://api.openai.com/v1",
		Model:     "gpt-5.5",
		SecretRef: "openai-api-key",
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repository.Save(ctx, config); err != nil {
		t.Fatalf("save OpenAI provider: %v", err)
	}

	var secretRef string
	if err := db.QueryRow(
		"SELECT secret_ref FROM provider_configs WHERE provider = 'openai'",
	).Scan(&secretRef); err != nil {
		t.Fatalf("read secret reference: %v", err)
	}
	if secretRef != "openai-api-key" {
		t.Fatalf("unexpected secret reference %q", secretRef)
	}
}
