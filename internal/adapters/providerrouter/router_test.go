package providerrouter

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ch1lam/autocv/internal/adapters/fakeprovider"
	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

type configRepository struct {
	config domain.ProviderConfig
	found  bool
}

func (repository configRepository) GetActive(
	context.Context,
) (domain.ProviderConfig, bool, error) {
	return repository.config, repository.found, nil
}

func (repository configRepository) GetByProvider(
	context.Context,
	string,
) (domain.ProviderConfig, bool, error) {
	return domain.ProviderConfig{}, false, nil
}

func (repository configRepository) Save(
	context.Context,
	domain.ProviderConfig,
) error {
	return nil
}

func TestRouterDefaultsToFakeProvider(t *testing.T) {
	router := New(configRepository{}, fakeprovider.New(), nil)
	analysis, err := router.AnalyzeJD(
		context.Background(),
		analyzeJDRequest(),
	)
	if err != nil {
		t.Fatalf("analyze JD through router: %v", err)
	}
	if analysis.Role == "" {
		t.Fatalf("unexpected analysis %#v", analysis)
	}
}

func TestRouterUsesActiveFakeProvider(t *testing.T) {
	router := New(
		configRepository{
			found: true,
			config: domain.ProviderConfig{
				Provider: domain.ProviderFake,
			},
		},
		fakeprovider.New(),
		nil,
	)
	if _, err := router.AnalyzeJD(
		context.Background(),
		analyzeJDRequest(),
	); err != nil {
		t.Fatalf("analyze JD through fake Provider: %v", err)
	}
}

func TestRouterDoesNotSilentlyFallbackFromOpenAI(t *testing.T) {
	router := New(
		configRepository{
			found: true,
			config: domain.ProviderConfig{
				Provider: domain.ProviderOpenAI,
			},
		},
		fakeprovider.New(),
		nil,
	)
	_, err := router.AnalyzeJD(
		context.Background(),
		analyzeJDRequest(),
	)
	if !errors.Is(err, ErrOpenAIUnavailable) {
		t.Fatalf("expected unavailable OpenAI error, got %v", err)
	}
}

type blockingProvider struct {
	*fakeprovider.Provider
	mutex   sync.Mutex
	calls   int
	started chan struct{}
}

func (provider *blockingProvider) AnalyzeJD(
	ctx context.Context,
	request ports.AnalyzeJDRequest,
) (domain.JDAnalysis, error) {
	provider.mutex.Lock()
	provider.calls++
	call := provider.calls
	provider.mutex.Unlock()
	if call == 1 {
		close(provider.started)
		<-ctx.Done()
		return domain.JDAnalysis{}, ctx.Err()
	}
	return provider.Provider.AnalyzeJD(ctx, request)
}

func TestRouterCancelsActiveRequestAndAllowsRetry(t *testing.T) {
	provider := &blockingProvider{
		Provider: fakeprovider.New(),
		started:  make(chan struct{}),
	}
	router := New(configRepository{}, provider, nil)
	result := make(chan error, 1)
	go func() {
		_, err := router.AnalyzeJD(
			context.Background(),
			analyzeJDRequest(),
		)
		result <- err
	}()

	<-provider.started
	task, cancelled := router.CancelActive()
	if !cancelled || task != "jd_analysis" {
		t.Fatalf("unexpected cancellation task=%q cancelled=%v", task, cancelled)
	}
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancelled request, got %v", err)
	}

	if _, err := router.AnalyzeJD(
		context.Background(),
		analyzeJDRequest(),
	); err != nil {
		t.Fatalf("retry after cancellation: %v", err)
	}
}

func TestRouterRejectsConcurrentProviderRequests(t *testing.T) {
	provider := &blockingProvider{
		Provider: fakeprovider.New(),
		started:  make(chan struct{}),
	}
	router := New(configRepository{}, provider, nil)
	result := make(chan error, 1)
	go func() {
		_, err := router.AnalyzeJD(
			context.Background(),
			analyzeJDRequest(),
		)
		result <- err
	}()
	<-provider.started

	if _, err := router.AnalyzeJD(
		context.Background(),
		analyzeJDRequest(),
	); !errors.Is(err, ErrProviderBusy) {
		t.Fatalf("expected busy error, got %v", err)
	}
	router.CancelActive()
	<-result
}

func analyzeJDRequest() ports.AnalyzeJDRequest {
	return ports.AnalyzeJDRequest{
		Text:         "Senior Backend Engineer",
		LanguageHint: domain.JDLanguageEnglish,
	}
}
