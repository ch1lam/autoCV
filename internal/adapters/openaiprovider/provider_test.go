package openaiprovider

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

type clientResult struct {
	response Response
	err      error
}

type queuedClient struct {
	mutex    sync.Mutex
	results  []clientResult
	requests []Request
}

type memoryCallRecorder struct {
	calls []domain.ProviderCall
	err   error
}

func (recorder *memoryCallRecorder) Record(
	_ context.Context,
	call domain.ProviderCall,
) error {
	if recorder.err != nil {
		return recorder.err
	}
	recorder.calls = append(recorder.calls, call)
	return nil
}

func (client *queuedClient) Generate(
	ctx context.Context,
	request Request,
) (Response, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	client.requests = append(client.requests, request)
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	if len(client.results) == 0 {
		return Response{}, errors.New("no queued response")
	}
	result := client.results[0]
	client.results = client.results[1:]
	return result.response, result.err
}

type temporaryError struct{}

func (temporaryError) Error() string   { return "temporary failure" }
func (temporaryError) Temporary() bool { return true }

func TestProviderRepairsInvalidJDOutputOnce(t *testing.T) {
	client := &queuedClient{
		results: []clientResult{
			{
				response: Response{
					Output: []byte(`{"role":""}`),
					Usage:  Usage{InputTokens: 10, OutputTokens: 4, TotalTokens: 14},
				},
			},
			{
				response: Response{
					Output: validJDOutput(),
					Usage:  Usage{InputTokens: 8, OutputTokens: 20, TotalTokens: 28},
				},
			},
		},
	}
	recorder := &memoryCallRecorder{}
	provider := newTestProvider(t, client, recorder, 0)
	analysis, err := provider.AnalyzeJD(
		context.Background(),
		ports.AnalyzeJDRequest{
			Text:         "Senior Backend Engineer",
			LanguageHint: domain.JDLanguageEnglish,
		},
	)
	if err != nil {
		t.Fatalf("analyze JD: %v", err)
	}
	if analysis.Role != "Senior Backend Engineer" {
		t.Fatalf("unexpected analysis %#v", analysis)
	}
	if len(client.requests) != 2 || client.requests[0].Repair != nil ||
		client.requests[1].Repair == nil {
		t.Fatalf("expected one repair request %#v", client.requests)
	}
	if len(recorder.calls) != 1 ||
		!recorder.calls[0].SchemaRepaired ||
		recorder.calls[0].TotalTokens != 42 {
		t.Fatalf("unexpected recorded call %#v", recorder.calls)
	}
}

func TestProviderRetriesOneTemporaryFailure(t *testing.T) {
	client := &queuedClient{
		results: []clientResult{
			{err: temporaryError{}},
			{response: Response{Output: validJDOutput()}},
		},
	}
	provider := newTestProvider(t, client, &memoryCallRecorder{}, 1)
	if _, err := provider.AnalyzeJD(
		context.Background(),
		ports.AnalyzeJDRequest{
			Text:         "Senior Backend Engineer",
			LanguageHint: domain.JDLanguageEnglish,
		},
	); err != nil {
		t.Fatalf("analyze JD after retry: %v", err)
	}
	if len(client.requests) != 2 {
		t.Fatalf("expected two requests, got %d", len(client.requests))
	}
}

func TestProviderDoesNotRetryCancelledRequest(t *testing.T) {
	client := &queuedClient{
		results: []clientResult{{response: Response{Output: validJDOutput()}}},
	}
	recorder := &memoryCallRecorder{}
	provider := newTestProvider(t, client, recorder, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.AnalyzeJD(
		ctx,
		ports.AnalyzeJDRequest{
			Text:         "Senior Backend Engineer",
			LanguageHint: domain.JDLanguageEnglish,
		},
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if len(client.requests) != 0 {
		t.Fatalf("cancelled request should not reach client, got %d calls", len(client.requests))
	}
	if len(recorder.calls) != 1 ||
		recorder.calls[0].Status != domain.ProviderCallStatusCancelled {
		t.Fatalf("unexpected cancelled call %#v", recorder.calls)
	}
}

func TestProviderRejectsSecondInvalidJDOutput(t *testing.T) {
	client := &queuedClient{
		results: []clientResult{
			{response: Response{Output: []byte(`{"role":""}`)}},
			{response: Response{Output: []byte(`{"role":""}`)}},
		},
	}
	provider := newTestProvider(t, client, &memoryCallRecorder{}, 0)
	if _, err := provider.AnalyzeJD(
		context.Background(),
		ports.AnalyzeJDRequest{
			Text:         "Senior Backend Engineer",
			LanguageHint: domain.JDLanguageEnglish,
		},
	); err == nil {
		t.Fatal("expected repaired output validation failure")
	}
	if len(client.requests) != 2 {
		t.Fatalf("expected one repair, got %d calls", len(client.requests))
	}
}

func TestProviderRejectsResultWhenMetadataCannotPersist(t *testing.T) {
	client := &queuedClient{
		results: []clientResult{{
			response: Response{Output: validJDOutput()},
		}},
	}
	recorder := &memoryCallRecorder{
		err: errors.New("database unavailable"),
	}
	provider := newTestProvider(t, client, recorder, 0)
	if _, err := provider.AnalyzeJD(
		context.Background(),
		ports.AnalyzeJDRequest{
			Text:         "Senior Backend Engineer",
			LanguageHint: domain.JDLanguageEnglish,
		},
	); err == nil {
		t.Fatal("expected metadata persistence failure")
	}
}

func TestResumeRequestPayloadIncludesConfirmations(t *testing.T) {
	strategy, found := domain.ResumePackagingStrategyForLevel(1)
	if !found {
		t.Fatal("expected amplified packaging strategy")
	}
	payload := resumeRequestPayload(ports.DraftResumeRequest{
		Language:          domain.ResumeLanguageChinese,
		TargetRole:        "后端工程师",
		PackagingLevel:    strategy.Level,
		PackagingStrategy: strategy,
		Match: domain.MatchAnalysis{
			Requirements: []domain.MatchRequirement{{
				ID:         "required-team",
				Text:       "团队协作",
				Importance: 4,
			}},
			Suggestions: []domain.MatchSuggestion{{
				RequirementID:       "required-team",
				Strength:            domain.MatchStrengthMissing,
				Explanation:         "资料中缺少团队规模。",
				ClarificationNeeded: true,
			}},
		},
		Confirmations: []domain.RunConfirmation{{
			ID:            "confirmation-1",
			RequirementID: "required-team",
			Content:       "负责 8 人后端团队。",
		}},
	})
	contents, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode resume payload: %v", err)
	}
	var decoded struct {
		PackagingStrategy struct {
			ID         string   `json:"id"`
			Label      string   `json:"label"`
			Guardrails []string `json:"guardrails"`
		} `json:"packaging_strategy"`
		Confirmations []struct {
			ID            string `json:"id"`
			RequirementID string `json:"requirement_id"`
			Content       string `json:"content"`
		} `json:"confirmations"`
	}
	if err := json.Unmarshal(contents, &decoded); err != nil {
		t.Fatalf("decode resume payload: %v", err)
	}
	if decoded.PackagingStrategy.ID != "amplified" ||
		decoded.PackagingStrategy.Label != "强化" ||
		len(decoded.PackagingStrategy.Guardrails) == 0 {
		t.Fatalf("unexpected packaging strategy payload %#v", decoded.PackagingStrategy)
	}
	if len(decoded.Confirmations) != 1 ||
		decoded.Confirmations[0].ID != "confirmation-1" ||
		decoded.Confirmations[0].RequirementID != "required-team" ||
		decoded.Confirmations[0].Content != "负责 8 人后端团队。" {
		t.Fatalf("unexpected confirmations payload %#v", decoded.Confirmations)
	}
}

func newTestProvider(
	t *testing.T,
	client Client,
	recorder ports.ProviderCallRecorder,
	maxRetries int,
) *Provider {
	t.Helper()
	provider, err := New(
		client,
		recorder,
		"gpt-5.5",
		time.Second,
		maxRetries,
	)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	provider.retryDelay = 0
	return provider
}

func validJDOutput() []byte {
	return []byte(`{
		"role": "Senior Backend Engineer",
		"company": null,
		"level": "senior",
		"language": "en",
		"responsibilities": [],
		"required_skills": [{
			"id": "required-go",
			"text": "Go",
			"importance": 5,
			"hard_constraint": true
		}],
		"preferred_skills": [],
		"domain_signals": [],
		"screening_constraints": [],
		"ambiguities": []
	}`)
}
