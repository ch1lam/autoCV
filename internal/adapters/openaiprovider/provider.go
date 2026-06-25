package openaiprovider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
	"github.com/google/uuid"
)

const defaultRetryDelay = 150 * time.Millisecond

type Request struct {
	Task          Task
	PromptVersion string
	Prompt        string
	SchemaName    string
	Schema        json.RawMessage
	Input         json.RawMessage
	Repair        *RepairRequest
}

type RepairRequest struct {
	InvalidOutput   string
	ValidationError string
}

type Response struct {
	Output []byte
	Usage  Usage
}

type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

func (usage *Usage) Add(other Usage) {
	usage.InputTokens += other.InputTokens
	usage.OutputTokens += other.OutputTokens
	usage.TotalTokens += other.TotalTokens
}

type Client interface {
	Generate(context.Context, Request) (Response, error)
}

type Provider struct {
	client     Client
	recorder   ports.ProviderCallRecorder
	model      string
	timeout    time.Duration
	maxRetries int
	retryDelay time.Duration
}

func New(
	client Client,
	recorder ports.ProviderCallRecorder,
	model string,
	timeout time.Duration,
	maxRetries int,
) (*Provider, error) {
	if client == nil {
		return nil, errors.New("OpenAI client is nil")
	}
	if recorder == nil {
		return nil, errors.New("Provider call recorder is nil")
	}
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("OpenAI model is empty")
	}
	if timeout <= 0 {
		return nil, errors.New("OpenAI timeout must be positive")
	}
	if maxRetries < 0 || maxRetries > 1 {
		return nil, errors.New("OpenAI max retries must be 0 or 1")
	}
	return &Provider{
		client:     client,
		recorder:   recorder,
		model:      strings.TrimSpace(model),
		timeout:    timeout,
		maxRetries: maxRetries,
		retryDelay: defaultRetryDelay,
	}, nil
}

func (provider *Provider) ExtractProfile(
	ctx context.Context,
	request ports.ExtractProfileRequest,
) ([]domain.ExtractedEvidence, error) {
	input, err := json.Marshal(profileRequestPayload(request))
	if err != nil {
		return nil, fmt.Errorf("encode profile extraction input: %w", err)
	}
	return executeStructured(
		ctx,
		provider,
		TaskProfileExtraction,
		input,
		func(contents []byte) ([]domain.ExtractedEvidence, error) {
			return DecodeProfileEvidence(contents, request.Chunks)
		},
	)
}

func (provider *Provider) AnalyzeJD(
	ctx context.Context,
	request ports.AnalyzeJDRequest,
) (domain.JDAnalysis, error) {
	input, err := json.Marshal(struct {
		Text         string `json:"text"`
		LanguageHint string `json:"language_hint"`
	}{
		Text:         request.Text,
		LanguageHint: string(request.LanguageHint),
	})
	if err != nil {
		return domain.JDAnalysis{}, fmt.Errorf("encode JD analysis input: %w", err)
	}
	return executeStructured(
		ctx,
		provider,
		TaskJDAnalysis,
		input,
		DecodeJDAnalysis,
	)
}

func (provider *Provider) SuggestMatches(
	ctx context.Context,
	request ports.SuggestMatchesRequest,
) ([]domain.MatchSuggestion, error) {
	input, err := json.Marshal(matchRequestPayload(request))
	if err != nil {
		return nil, fmt.Errorf("encode match suggestion input: %w", err)
	}
	return executeStructured(
		ctx,
		provider,
		TaskMatchSuggestion,
		input,
		func(contents []byte) ([]domain.MatchSuggestion, error) {
			return DecodeMatchSuggestions(
				contents,
				request.Requirements,
				request.Evidence,
			)
		},
	)
}

func (provider *Provider) DraftResume(
	ctx context.Context,
	request ports.DraftResumeRequest,
) (domain.ResumeDraft, error) {
	input, err := json.Marshal(resumeRequestPayload(request))
	if err != nil {
		return domain.ResumeDraft{}, fmt.Errorf(
			"encode resume draft input: %w",
			err,
		)
	}
	return executeStructured(
		ctx,
		provider,
		TaskResumeDraft,
		input,
		func(contents []byte) (domain.ResumeDraft, error) {
			return DecodeResumeDraft(contents, request.Evidence)
		},
	)
}

func (provider *Provider) ComposeResumeHTML(
	ctx context.Context,
	request ports.ComposeResumeHTMLRequest,
) (ports.ComposedResumeHTML, error) {
	input, err := json.Marshal(resumeHTMLRequestPayload(request))
	if err != nil {
		return ports.ComposedResumeHTML{}, fmt.Errorf(
			"encode resume HTML compose input: %w",
			err,
		)
	}
	return executeStructured(
		ctx,
		provider,
		TaskResumeHTMLCompose,
		input,
		func(contents []byte) (ports.ComposedResumeHTML, error) {
			return decodeComposedResumeHTML(contents, request, input)
		},
	)
}

func (provider *Provider) CacheKey() string {
	definition, err := Definition(TaskResumeHTMLCompose)
	if err != nil {
		return "openaiprovider/resume-html/unknown"
	}
	return fmt.Sprintf(
		"openaiprovider/resume-html/%s/%s",
		provider.model,
		definition.PromptVersion,
	)
}

func executeStructured[T any](
	ctx context.Context,
	provider *Provider,
	task Task,
	input []byte,
	decode func([]byte) (T, error),
) (result T, err error) {
	startedAt := time.Now()
	inputHash := hashInput(input)
	usage := Usage{}
	repaired := false
	defer func() {
		status, errorKind := providerCallOutcome(err)
		call := domain.ProviderCall{
			ID:             uuid.NewString(),
			Provider:       domain.ProviderOpenAI,
			Model:          provider.model,
			Task:           string(task),
			PromptVersion:  "unknown",
			InputHash:      inputHash,
			Status:         status,
			DurationMS:     time.Since(startedAt).Milliseconds(),
			InputTokens:    usage.InputTokens,
			OutputTokens:   usage.OutputTokens,
			TotalTokens:    usage.TotalTokens,
			SchemaRepaired: repaired,
			ErrorKind:      errorKind,
			CreatedAt:      time.Now().UTC(),
		}
		if definition, definitionErr := Definition(task); definitionErr == nil {
			call.PromptVersion = definition.PromptVersion
		}
		recordContext, cancel := context.WithTimeout(
			context.Background(),
			2*time.Second,
		)
		recordErr := provider.recorder.Record(recordContext, call)
		cancel()
		if recordErr != nil {
			var zero T
			result = zero
			err = errors.Join(err, fmt.Errorf(
				"persist Provider call metadata: %w",
				recordErr,
			))
		}
		slog.Info(
			"provider.call.completed",
			slog.String("provider", domain.ProviderOpenAI),
			slog.String("model", provider.model),
			slog.String("task", string(task)),
			slog.String("status", call.Status),
			slog.Int64("duration_ms", call.DurationMS),
			slog.Int("input_tokens", usage.InputTokens),
			slog.Int("output_tokens", usage.OutputTokens),
			slog.Int("total_tokens", usage.TotalTokens),
			slog.Bool("schema_repaired", repaired),
		)
	}()

	definition, err := Definition(task)
	if err != nil {
		return result, err
	}
	request := Request{
		Task:          task,
		PromptVersion: definition.PromptVersion,
		Prompt:        definition.Prompt,
		SchemaName:    definition.SchemaName,
		Schema:        definition.Schema,
		Input:         input,
	}
	response, err := provider.call(ctx, request)
	if err != nil {
		return result, err
	}
	usage.Add(response.Usage)

	result, repaired, err = ValidateWithSingleRepair(
		ctx,
		response.Output,
		decode,
		func(
			repairContext context.Context,
			invalid []byte,
			validationErr error,
		) ([]byte, error) {
			repairRequest := request
			repairRequest.Repair = &RepairRequest{
				InvalidOutput:   string(invalid),
				ValidationError: validationErr.Error(),
			}
			repairedResponse, repairErr := provider.call(
				repairContext,
				repairRequest,
			)
			if repairErr != nil {
				return nil, repairErr
			}
			usage.Add(repairedResponse.Usage)
			return repairedResponse.Output, nil
		},
	)
	return result, err
}

func (provider *Provider) call(
	ctx context.Context,
	request Request,
) (Response, error) {
	var lastErr error
	for attempt := 0; attempt <= provider.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return Response{}, err
		}
		attemptContext, cancel := context.WithTimeout(ctx, provider.timeout)
		response, err := provider.client.Generate(attemptContext, request)
		cancel()
		if err == nil {
			return response, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return Response{}, ctx.Err()
		}
		if attempt == provider.maxRetries || !isRetryable(err) {
			break
		}
		timer := time.NewTimer(provider.retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return Response{}, ctx.Err()
		case <-timer.C:
		}
	}
	return Response{}, fmt.Errorf("OpenAI request failed: %w", lastErr)
}

type profileChunkPayload struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	LocatorJSON string `json:"locator_json"`
}

func profileRequestPayload(
	request ports.ExtractProfileRequest,
) struct {
	Chunks []profileChunkPayload `json:"chunks"`
} {
	chunks := make([]profileChunkPayload, 0, len(request.Chunks))
	for _, chunk := range request.Chunks {
		chunks = append(chunks, profileChunkPayload{
			ID:          chunk.ID,
			Text:        chunk.Text,
			LocatorJSON: chunk.LocatorJSON,
		})
	}
	return struct {
		Chunks []profileChunkPayload `json:"chunks"`
	}{Chunks: chunks}
}

type matchRequirementPayload struct {
	ID             string `json:"id"`
	Category       string `json:"category"`
	Text           string `json:"text"`
	Importance     int    `json:"importance"`
	HardConstraint bool   `json:"hard_constraint"`
}

type evidencePayload struct {
	ID         string  `json:"id"`
	Kind       string  `json:"kind"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Confidence float64 `json:"confidence"`
}

func matchRequestPayload(
	request ports.SuggestMatchesRequest,
) struct {
	Requirements []matchRequirementPayload `json:"requirements"`
	Evidence     []evidencePayload         `json:"evidence"`
} {
	requirements := make(
		[]matchRequirementPayload,
		0,
		len(request.Requirements),
	)
	for _, requirement := range request.Requirements {
		requirements = append(requirements, matchRequirementPayload{
			ID:             requirement.ID,
			Category:       string(requirement.Category),
			Text:           requirement.Text,
			Importance:     requirement.Importance,
			HardConstraint: requirement.HardConstraint,
		})
	}
	return struct {
		Requirements []matchRequirementPayload `json:"requirements"`
		Evidence     []evidencePayload         `json:"evidence"`
	}{
		Requirements: requirements,
		Evidence:     evidencePayloads(request.Evidence),
	}
}

type matchSuggestionPayload struct {
	RequirementID       string   `json:"requirement_id"`
	Strength            string   `json:"strength"`
	EvidenceIDs         []string `json:"evidence_ids"`
	Explanation         string   `json:"explanation"`
	ClarificationNeeded bool     `json:"clarification_needed"`
}

type runConfirmationPayload struct {
	ID            string `json:"id"`
	RequirementID string `json:"requirement_id"`
	Content       string `json:"content"`
}

type packagingStrategyPayload struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	Description      string   `json:"description"`
	LanguageStrength string   `json:"language_strength"`
	SelectionPolicy  string   `json:"selection_policy"`
	InferencePolicy  string   `json:"inference_policy"`
	Guardrails       []string `json:"guardrails"`
}

func resumeRequestPayload(
	request ports.DraftResumeRequest,
) struct {
	Language          string                    `json:"language"`
	TargetRole        string                    `json:"target_role"`
	PackagingLevel    float64                   `json:"packaging_level"`
	PackagingStrategy packagingStrategyPayload  `json:"packaging_strategy"`
	Requirements      []matchRequirementPayload `json:"requirements"`
	Suggestions       []matchSuggestionPayload  `json:"suggestions"`
	Evidence          []evidencePayload         `json:"evidence"`
	Confirmations     []runConfirmationPayload  `json:"confirmations"`
} {
	requirements := make(
		[]matchRequirementPayload,
		0,
		len(request.Match.Requirements),
	)
	for _, requirement := range request.Match.Requirements {
		requirements = append(requirements, matchRequirementPayload{
			ID:             requirement.ID,
			Category:       string(requirement.Category),
			Text:           requirement.Text,
			Importance:     requirement.Importance,
			HardConstraint: requirement.HardConstraint,
		})
	}
	suggestions := make(
		[]matchSuggestionPayload,
		0,
		len(request.Match.Suggestions),
	)
	for _, suggestion := range request.Match.Suggestions {
		suggestions = append(suggestions, matchSuggestionPayload{
			RequirementID:       suggestion.RequirementID,
			Strength:            string(suggestion.Strength),
			EvidenceIDs:         suggestion.EvidenceIDs,
			Explanation:         suggestion.Explanation,
			ClarificationNeeded: suggestion.ClarificationNeeded,
		})
	}
	confirmations := make(
		[]runConfirmationPayload,
		0,
		len(request.Confirmations),
	)
	for _, confirmation := range request.Confirmations {
		confirmations = append(confirmations, runConfirmationPayload{
			ID:            confirmation.ID,
			RequirementID: confirmation.RequirementID,
			Content:       confirmation.Content,
		})
	}
	return struct {
		Language          string                    `json:"language"`
		TargetRole        string                    `json:"target_role"`
		PackagingLevel    float64                   `json:"packaging_level"`
		PackagingStrategy packagingStrategyPayload  `json:"packaging_strategy"`
		Requirements      []matchRequirementPayload `json:"requirements"`
		Suggestions       []matchSuggestionPayload  `json:"suggestions"`
		Evidence          []evidencePayload         `json:"evidence"`
		Confirmations     []runConfirmationPayload  `json:"confirmations"`
	}{
		Language:       string(request.Language),
		TargetRole:     request.TargetRole,
		PackagingLevel: request.PackagingLevel,
		PackagingStrategy: packagingStrategyPayloadFrom(
			request.PackagingStrategy,
		),
		Requirements:  requirements,
		Suggestions:   suggestions,
		Evidence:      evidencePayloads(request.Evidence),
		Confirmations: confirmations,
	}
}

func resumeHTMLRequestPayload(
	request ports.ComposeResumeHTMLRequest,
) struct {
	Resume   domain.Resume            `json:"resume"`
	Template ports.ResumeHTMLTemplate `json:"template"`
} {
	return struct {
		Resume   domain.Resume            `json:"resume"`
		Template ports.ResumeHTMLTemplate `json:"template"`
	}{
		Resume:   domain.NormalizeResume(request.Resume),
		Template: request.Template,
	}
}

func decodeComposedResumeHTML(
	contents []byte,
	request ports.ComposeResumeHTMLRequest,
	input []byte,
) (ports.ComposedResumeHTML, error) {
	var payload struct {
		HTML string `json:"html"`
	}
	if err := json.Unmarshal(contents, &payload); err != nil {
		return ports.ComposedResumeHTML{}, fmt.Errorf(
			"decode composed resume HTML: %w",
			err,
		)
	}
	payload.HTML = strings.TrimSpace(payload.HTML)
	if payload.HTML == "" {
		return ports.ComposedResumeHTML{}, errors.New(
			"composed resume HTML is empty",
		)
	}
	definition, err := Definition(TaskResumeHTMLCompose)
	if err != nil {
		return ports.ComposedResumeHTML{}, err
	}
	return ports.ComposedResumeHTML{
		HTML:            payload.HTML,
		TemplateID:      request.Template.ID,
		TemplateVersion: request.Template.Version,
		Composer:        "openai",
		ComposerVersion: definition.PromptVersion,
		PromptVersion:   definition.PromptVersion,
		InputHash:       hashInput(input),
		HTMLHash:        hashInput([]byte(payload.HTML)),
	}, nil
}

func packagingStrategyPayloadFrom(
	strategy domain.ResumePackagingStrategy,
) packagingStrategyPayload {
	return packagingStrategyPayload{
		ID:               strategy.ID,
		Label:            strategy.Label,
		Description:      strategy.Description,
		LanguageStrength: strategy.LanguageStrength,
		SelectionPolicy:  strategy.SelectionPolicy,
		InferencePolicy:  strategy.InferencePolicy,
		Guardrails:       append([]string(nil), strategy.Guardrails...),
	}
}

func evidencePayloads(evidence []domain.Evidence) []evidencePayload {
	result := make([]evidencePayload, 0, len(evidence))
	for _, item := range evidence {
		result = append(result, evidencePayload{
			ID:         item.ID,
			Kind:       item.Kind,
			Title:      item.Title,
			Content:    item.Content,
			Confidence: item.Confidence,
		})
	}
	return result
}

type retryable interface {
	Temporary() bool
}

func isRetryable(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return networkError.Timeout() || networkError.Temporary()
	}
	var temporary retryable
	return errors.As(err, &temporary) && temporary.Temporary()
}

func hashInput(input []byte) string {
	digest := sha256.Sum256(input)
	return hex.EncodeToString(digest[:])
}

func providerCallOutcome(err error) (string, string) {
	switch {
	case err == nil:
		return domain.ProviderCallStatusSucceeded, ""
	case errors.Is(err, context.Canceled):
		return domain.ProviderCallStatusCancelled, "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return domain.ProviderCallStatusFailed, "timeout"
	case strings.Contains(err.Error(), "structured output"):
		return domain.ProviderCallStatusFailed, "schema"
	default:
		return domain.ProviderCallStatusFailed, "provider"
	}
}

var (
	_ ports.ProfileExtractor = (*Provider)(nil)
	_ ports.JDAnalyzer       = (*Provider)(nil)
	_ ports.MatchSuggester   = (*Provider)(nil)
	_ ports.ResumeDrafter    = (*Provider)(nil)
)
