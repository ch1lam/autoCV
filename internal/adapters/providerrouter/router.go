package providerrouter

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
	"github.com/google/uuid"
)

var ErrOpenAIUnavailable = errors.New(
	"OpenAI Provider is configured but its official SDK adapter is unavailable",
)
var ErrProviderBusy = errors.New("another Provider request is already running")

type Provider interface {
	ports.ProfileExtractor
	ports.JDAnalyzer
	ports.MatchSuggester
	ports.ResumeDrafter
}

type Router struct {
	configs ports.ProviderConfigRepository
	fake    Provider
	openAI  Provider
	mutex   sync.Mutex
	active  *activeRequest
}

type activeRequest struct {
	id     string
	task   string
	cancel context.CancelFunc
}

func New(
	configs ports.ProviderConfigRepository,
	fake Provider,
	openAI Provider,
) *Router {
	return &Router{
		configs: configs,
		fake:    fake,
		openAI:  openAI,
	}
}

func (router *Router) ExtractProfile(
	ctx context.Context,
	request ports.ExtractProfileRequest,
) ([]domain.ExtractedEvidence, error) {
	requestContext, done, err := router.begin(ctx, "profile_extraction")
	if err != nil {
		return nil, err
	}
	defer done()
	provider, err := router.provider(requestContext)
	if err != nil {
		return nil, err
	}
	return provider.ExtractProfile(requestContext, request)
}

func (router *Router) AnalyzeJD(
	ctx context.Context,
	request ports.AnalyzeJDRequest,
) (domain.JDAnalysis, error) {
	requestContext, done, err := router.begin(ctx, "jd_analysis")
	if err != nil {
		return domain.JDAnalysis{}, err
	}
	defer done()
	provider, err := router.provider(requestContext)
	if err != nil {
		return domain.JDAnalysis{}, err
	}
	return provider.AnalyzeJD(requestContext, request)
}

func (router *Router) SuggestMatches(
	ctx context.Context,
	request ports.SuggestMatchesRequest,
) ([]domain.MatchSuggestion, error) {
	requestContext, done, err := router.begin(ctx, "match_suggestion")
	if err != nil {
		return nil, err
	}
	defer done()
	provider, err := router.provider(requestContext)
	if err != nil {
		return nil, err
	}
	return provider.SuggestMatches(requestContext, request)
}

func (router *Router) DraftResume(
	ctx context.Context,
	request ports.DraftResumeRequest,
) (domain.ResumeDraft, error) {
	requestContext, done, err := router.begin(ctx, "resume_draft")
	if err != nil {
		return domain.ResumeDraft{}, err
	}
	defer done()
	provider, err := router.provider(requestContext)
	if err != nil {
		return domain.ResumeDraft{}, err
	}
	return provider.DraftResume(requestContext, request)
}

func (router *Router) CancelActive() (string, bool) {
	router.mutex.Lock()
	defer router.mutex.Unlock()
	if router.active == nil {
		return "", false
	}
	task := router.active.task
	router.active.cancel()
	return task, true
}

func (router *Router) begin(
	ctx context.Context,
	task string,
) (context.Context, func(), error) {
	router.mutex.Lock()
	defer router.mutex.Unlock()
	if router.active != nil {
		return nil, nil, ErrProviderBusy
	}

	requestContext, cancel := context.WithCancel(ctx)
	id := uuid.NewString()
	router.active = &activeRequest{
		id:     id,
		task:   task,
		cancel: cancel,
	}
	done := func() {
		cancel()
		router.mutex.Lock()
		defer router.mutex.Unlock()
		if router.active != nil && router.active.id == id {
			router.active = nil
		}
	}
	return requestContext, done, nil
}

func (router *Router) provider(ctx context.Context) (Provider, error) {
	config, found, err := router.configs.GetActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve active Provider: %w", err)
	}
	if !found || config.Provider == domain.ProviderFake {
		if router.fake == nil {
			return nil, errors.New("Fake Provider is unavailable")
		}
		return router.fake, nil
	}
	if config.Provider == domain.ProviderOpenAI {
		if router.openAI == nil {
			return nil, ErrOpenAIUnavailable
		}
		return router.openAI, nil
	}
	return nil, fmt.Errorf("unsupported Provider %q", config.Provider)
}

var (
	_ ports.ProfileExtractor          = (*Router)(nil)
	_ ports.JDAnalyzer                = (*Router)(nil)
	_ ports.MatchSuggester            = (*Router)(nil)
	_ ports.ResumeDrafter             = (*Router)(nil)
	_ ports.ProviderRequestController = (*Router)(nil)
)
