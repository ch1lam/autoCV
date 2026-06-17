package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
	"github.com/ch1lam/autocv/internal/workflow"
	"github.com/google/uuid"
)

const defaultMatchAnalysisErr = "match analysis failed"

type MatchService struct {
	matchRepository         ports.MatchRepository
	scopeRepository         ports.RunScopeRepository
	runRepository           ports.ResumeRunRepository
	stageRepository         ports.StageResultRepository
	clarificationRepository ports.ClarificationRepository
	confirmationRepository  ports.RunConfirmationRepository
	profileRepository       ports.ProfileRepository
	jdRepository            ports.JDRepository
	suggester               ports.MatchSuggester
	clock                   ports.Clock
}

type MatchReview struct {
	Status         string                    `json:"status"`
	Message        string                    `json:"message"`
	Error          string                    `json:"error"`
	JDTitle        string                    `json:"jdTitle"`
	Company        string                    `json:"company"`
	TotalScore     int                       `json:"totalScore"`
	HardCapApplied bool                      `json:"hardCapApplied"`
	UpdatedAt      string                    `json:"updatedAt"`
	Counts         MatchCounts               `json:"counts"`
	Dimensions     []MatchDimensionSummary   `json:"dimensions"`
	Requirements   []RequirementMatchSummary `json:"requirements"`
	Clarifications []ClarificationSummary    `json:"clarifications"`
	Scope          RunScopeSummary           `json:"scope"`
}

type RunScopeSummary struct {
	Mode                string                    `json:"mode"`
	SelectedCount       int                       `json:"selectedCount"`
	AvailableCount      int                       `json:"availableCount"`
	SelectedDocumentIDs []string                  `json:"selectedDocumentIds"`
	Documents           []RunScopeDocumentSummary `json:"documents"`
}

type RunScopeDocumentSummary struct {
	ID           string `json:"id"`
	OriginalName string `json:"originalName"`
	Kind         string `json:"kind"`
	Selected     bool   `json:"selected"`
}

type MatchCounts struct {
	Strong  int `json:"strong"`
	Partial int `json:"partial"`
	Missing int `json:"missing"`
	Unknown int `json:"unknown"`
}

type MatchDimensionSummary struct {
	Category         string  `json:"category"`
	Label            string  `json:"label"`
	Weight           int     `json:"weight"`
	Earned           float64 `json:"earned"`
	RequirementCount int     `json:"requirementCount"`
}

type RequirementMatchSummary struct {
	ID                  string                 `json:"id"`
	Category            string                 `json:"category"`
	Group               string                 `json:"group"`
	Text                string                 `json:"text"`
	Importance          int                    `json:"importance"`
	HardConstraint      bool                   `json:"hardConstraint"`
	Strength            string                 `json:"strength"`
	Explanation         string                 `json:"explanation"`
	ClarificationNeeded bool                   `json:"clarificationNeeded"`
	Evidence            []MatchEvidenceSummary `json:"evidence"`
}

type MatchEvidenceSummary struct {
	ID      string                       `json:"id"`
	Kind    string                       `json:"kind"`
	Title   string                       `json:"title"`
	Content string                       `json:"content"`
	Sources []MatchEvidenceSourceSummary `json:"sources"`
}

type MatchEvidenceSourceSummary struct {
	ChunkID      string `json:"chunkId"`
	DocumentID   string `json:"documentId"`
	DocumentName string `json:"documentName"`
	ChunkText    string `json:"chunkText"`
	LocatorJSON  string `json:"locatorJson"`
	QuoteStart   int    `json:"quoteStart"`
	QuoteEnd     int    `json:"quoteEnd"`
}

type ClarificationSummary struct {
	ID            string `json:"id"`
	RequirementID string `json:"requirementId"`
	Round         int    `json:"round"`
	Ordinal       int    `json:"ordinal"`
	Question      string `json:"question"`
	Reason        string `json:"reason"`
	Status        string `json:"status"`
	Answer        string `json:"answer"`
}

type preparedMatchInput struct {
	profile      domain.Profile
	jd           domain.JobDescription
	requirements []domain.MatchRequirement
	evidence     []domain.Evidence
	inputHash    string
	scope        RunScopeSummary
}

func NewMatchService(
	matchRepository ports.MatchRepository,
	scopeRepository ports.RunScopeRepository,
	runRepository ports.ResumeRunRepository,
	stageRepository ports.StageResultRepository,
	clarificationRepository ports.ClarificationRepository,
	confirmationRepository ports.RunConfirmationRepository,
	profileRepository ports.ProfileRepository,
	jdRepository ports.JDRepository,
	suggester ports.MatchSuggester,
	clock ports.Clock,
) *MatchService {
	return &MatchService{
		matchRepository:         matchRepository,
		scopeRepository:         scopeRepository,
		runRepository:           runRepository,
		stageRepository:         stageRepository,
		clarificationRepository: clarificationRepository,
		confirmationRepository:  confirmationRepository,
		profileRepository:       profileRepository,
		jdRepository:            jdRepository,
		suggester:               suggester,
		clock:                   clock,
	}
}

func (service *MatchService) GetReview() (MatchReview, error) {
	return service.getReview(context.Background())
}

func (service *MatchService) SaveScope(
	mode string,
	documentIDs []string,
) (MatchReview, error) {
	ctx := context.Background()
	profile, err := resolveActiveProfile(
		ctx,
		service.profileRepository,
		service.clock.Now(),
	)
	if err != nil {
		return MatchReview{}, err
	}
	jd, found, err := service.jdRepository.GetLatest(ctx)
	if err != nil {
		return MatchReview{}, err
	}
	if !found {
		return MatchReview{}, errors.New(
			"save run scope: job description was not found",
		)
	}
	documents, err := service.profileRepository.ListDocuments(ctx, profile.ID)
	if err != nil {
		return MatchReview{}, err
	}
	scope, err := normalizeRunScope(
		profile.ID,
		jd.ID,
		mode,
		documentIDs,
		documents,
		service.clock.Now(),
	)
	if err != nil {
		return MatchReview{}, err
	}
	if err := service.scopeRepository.SaveScope(ctx, scope); err != nil {
		return MatchReview{}, err
	}
	return service.getReview(ctx)
}

func (service *MatchService) Analyze() (MatchReview, error) {
	ctx := context.Background()
	input, blocked, err := service.prepareInput(ctx)
	if err != nil {
		return MatchReview{}, err
	}
	if blocked.Status != "" {
		return blocked, nil
	}

	now := service.clock.Now().UTC()
	if err := service.saveMatchRunStage(
		ctx,
		input,
		workflow.StageMatched,
		now,
	); err != nil {
		return MatchReview{}, err
	}
	service.saveMatchStageResult(
		ctx,
		input,
		workflow.StageStatusRunning,
		"",
		"",
		now,
	)
	suggestions, err := service.suggester.SuggestMatches(
		ctx,
		ports.SuggestMatchesRequest{
			Requirements: input.requirements,
			Evidence:     input.evidence,
		},
	)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			service.saveMatchStageResult(
				ctx,
				input,
				workflow.StageStatusCancelled,
				"",
				stageErrorJSON(err),
				service.clock.Now().UTC(),
			)
			return MatchReview{}, fmt.Errorf("suggest matches: %w", err)
		}
		service.saveFailure(ctx, input, err)
		service.saveMatchStageResult(
			ctx,
			input,
			workflow.StageStatusFailed,
			"",
			stageErrorJSON(err),
			service.clock.Now().UTC(),
		)
		return MatchReview{}, fmt.Errorf("suggest matches: %w", err)
	}
	if err := domain.ValidateMatchSuggestions(
		input.requirements,
		input.evidence,
		suggestions,
	); err != nil {
		service.saveFailure(ctx, input, err)
		service.saveMatchStageResult(
			ctx,
			input,
			workflow.StageStatusFailed,
			"",
			stageErrorJSON(err),
			service.clock.Now().UTC(),
		)
		return MatchReview{}, fmt.Errorf("validate match suggestions: %w", err)
	}
	score, err := domain.CalculateMatchScore(input.requirements, suggestions)
	if err != nil {
		service.saveFailure(ctx, input, err)
		service.saveMatchStageResult(
			ctx,
			input,
			workflow.StageStatusFailed,
			"",
			stageErrorJSON(err),
			service.clock.Now().UTC(),
		)
		return MatchReview{}, fmt.Errorf("calculate match score: %w", err)
	}

	analysis := domain.MatchAnalysis{
		ID:           matchAnalysisID(input.profile.ID, input.jd.ID),
		ProfileID:    input.profile.ID,
		JDID:         input.jd.ID,
		InputHash:    input.inputHash,
		Status:       "succeeded",
		CreatedAt:    now,
		UpdatedAt:    now,
		Requirements: input.requirements,
		Suggestions:  suggestions,
	}
	if existing, found, getErr := service.matchRepository.GetLatest(
		ctx,
		input.profile.ID,
		input.jd.ID,
	); getErr != nil {
		return MatchReview{}, getErr
	} else if found {
		analysis.CreatedAt = existing.CreatedAt
	}
	if err := service.matchRepository.Save(ctx, analysis); err != nil {
		return MatchReview{}, err
	}
	questions, err := service.saveClarificationRound(ctx, input, analysis, now)
	if err != nil {
		return MatchReview{}, err
	}
	service.saveMatchStageResult(
		ctx,
		input,
		workflow.StageStatusSucceeded,
		matchStageResultJSON(analysis, score, len(questions)),
		"",
		now,
	)

	slog.Info(
		"match.analysis.succeeded",
		slog.String("analysis_id", analysis.ID),
		slog.String("input_hash", analysis.InputHash),
		slog.Int("requirement_count", len(analysis.Requirements)),
		slog.Int("clarification_count", len(questions)),
		slog.Int("score", score.Total),
	)
	review := matchReviewFromAnalysis(analysis, input, score)
	review.Clarifications = clarificationSummaries(questions)
	return review, nil
}

func (service *MatchService) AnswerClarification(
	questionID string,
	answer string,
) (MatchReview, error) {
	return service.updateClarification(
		context.Background(),
		questionID,
		domain.ClarificationAnswered,
		answer,
	)
}

func (service *MatchService) SkipClarification(
	questionID string,
) (MatchReview, error) {
	return service.updateClarification(
		context.Background(),
		questionID,
		domain.ClarificationSkipped,
		"",
	)
}

func (service *MatchService) getReview(
	ctx context.Context,
) (MatchReview, error) {
	input, blocked, err := service.prepareInput(ctx)
	if err != nil {
		return MatchReview{}, err
	}
	if blocked.Status != "" {
		return blocked, nil
	}

	analysis, found, err := service.matchRepository.GetLatest(
		ctx,
		input.profile.ID,
		input.jd.ID,
	)
	if err != nil {
		return MatchReview{}, err
	}
	if !found {
		return emptyMatchReview(
			"pending",
			input.jd,
			"资料和 JD 已就绪，可以开始建立证据关联。",
		).withScope(input.scope), nil
	}
	if analysis.InputHash != input.inputHash {
		return emptyMatchReview(
			"stale",
			input.jd,
			"资料或 JD 已变化，旧匹配结果已失效。",
		).withScope(input.scope), nil
	}
	if analysis.Status == "failed" {
		review := emptyMatchReview(
			"failed",
			input.jd,
			"上一次匹配分析未完成。",
		)
		review.Error = analysis.Error
		review.UpdatedAt = analysis.UpdatedAt.UTC().Format(timeFormat)
		review.Scope = input.scope
		return review, nil
	}
	if err := domain.ValidateMatchSuggestions(
		analysis.Requirements,
		input.evidence,
		analysis.Suggestions,
	); err != nil {
		return MatchReview{}, fmt.Errorf(
			"validate stored match analysis: %w",
			err,
		)
	}
	score, err := domain.CalculateMatchScore(
		analysis.Requirements,
		analysis.Suggestions,
	)
	if err != nil {
		return MatchReview{}, err
	}
	review := matchReviewFromAnalysis(analysis, input, score)
	questions, err := service.clarificationRepository.ListQuestions(
		ctx,
		resumeRunID(input.profile.ID, input.jd.ID),
	)
	if err != nil {
		return MatchReview{}, err
	}
	review.Clarifications = clarificationSummaries(questions)
	return review, nil
}

func (service *MatchService) updateClarification(
	ctx context.Context,
	questionID string,
	status domain.ClarificationQuestionStatus,
	answer string,
) (MatchReview, error) {
	input, analysis, err := service.currentReadyMatchAnalysis(ctx)
	if err != nil {
		return MatchReview{}, err
	}
	runID := resumeRunID(input.profile.ID, input.jd.ID)
	questions, err := service.clarificationRepository.ListQuestions(ctx, runID)
	if err != nil {
		return MatchReview{}, err
	}
	var found bool
	var target domain.ClarificationQuestion
	for _, question := range questions {
		if question.ID != questionID {
			continue
		}
		found = true
		target = question
		if question.Status != domain.ClarificationPending {
			return MatchReview{}, fmt.Errorf(
				"clarification question %q has already been handled",
				questionID,
			)
		}
		break
	}
	if !found {
		return MatchReview{}, fmt.Errorf(
			"clarification question %q not found in current run",
			questionID,
		)
	}

	now := service.clock.Now().UTC()
	if err := service.saveRunConfirmation(
		ctx,
		runID,
		target,
		status,
		answer,
		now,
	); err != nil {
		return MatchReview{}, err
	}
	if _, err := service.clarificationRepository.UpdateQuestionStatus(
		ctx,
		questionID,
		status,
		answer,
		now,
	); err != nil {
		if status == domain.ClarificationAnswered {
			if deleteErr := service.confirmationRepository.DeleteRunConfirmation(
				ctx,
				runID,
				questionID,
			); deleteErr != nil {
				slog.Error(
					"match.confirmation.rollback.failed",
					slog.String("question_id", questionID),
					slog.Any("error", deleteErr),
				)
			}
		}
		return MatchReview{}, err
	}
	if status == domain.ClarificationSkipped {
		var lowered bool
		analysis, lowered = lowerSkippedMatchSuggestion(
			analysis,
			target.RequirementID,
			now,
		)
		if lowered {
			if err := domain.ValidateMatchSuggestions(
				analysis.Requirements,
				input.evidence,
				analysis.Suggestions,
			); err != nil {
				return MatchReview{}, fmt.Errorf(
					"validate lowered match suggestion: %w",
					err,
				)
			}
			if err := service.matchRepository.Save(ctx, analysis); err != nil {
				return MatchReview{}, err
			}
		}
	}
	questions, err = service.clarificationRepository.ListQuestions(ctx, runID)
	if err != nil {
		return MatchReview{}, err
	}
	if err := service.advanceClarificationRound(
		ctx,
		input,
		analysis,
		questions,
		now,
	); err != nil {
		return MatchReview{}, err
	}
	return service.getReview(ctx)
}

func (service *MatchService) saveRunConfirmation(
	ctx context.Context,
	runID string,
	question domain.ClarificationQuestion,
	status domain.ClarificationQuestionStatus,
	answer string,
	now time.Time,
) error {
	switch status {
	case domain.ClarificationAnswered:
		return service.confirmationRepository.SaveRunConfirmation(
			ctx,
			domain.RunConfirmation{
				ID:                      runConfirmationID(runID, question.ID),
				RunID:                   runID,
				ClarificationQuestionID: question.ID,
				RequirementID:           question.RequirementID,
				Content:                 answer,
				CreatedAt:               now,
				UpdatedAt:               now,
			},
		)
	case domain.ClarificationSkipped:
		return service.confirmationRepository.DeleteRunConfirmation(
			ctx,
			runID,
			question.ID,
		)
	default:
		return domain.ValidateClarificationResponse(status, answer)
	}
}

func lowerSkippedMatchSuggestion(
	analysis domain.MatchAnalysis,
	requirementID string,
	now time.Time,
) (domain.MatchAnalysis, bool) {
	updated := analysis
	updated.Suggestions = append(
		[]domain.MatchSuggestion(nil),
		analysis.Suggestions...,
	)
	for index, suggestion := range updated.Suggestions {
		if suggestion.RequirementID != requirementID {
			continue
		}
		if suggestion.Strength == domain.MatchStrengthMissing &&
			!suggestion.ClarificationNeeded &&
			len(suggestion.EvidenceIDs) == 0 &&
			strings.Contains(suggestion.Explanation, skippedClarificationNote) {
			return analysis, false
		}
		suggestion.Strength = domain.MatchStrengthMissing
		suggestion.EvidenceIDs = nil
		suggestion.ClarificationNeeded = false
		suggestion.Explanation = appendSkippedClarificationNote(
			suggestion.Explanation,
		)
		updated.Suggestions[index] = suggestion
		updated.UpdatedAt = now
		return updated, true
	}
	return analysis, false
}

const skippedClarificationNote = "用户跳过了该要求的追问，系统按未确认缺口处理。"

func appendSkippedClarificationNote(explanation string) string {
	explanation = strings.TrimSpace(explanation)
	if strings.Contains(explanation, skippedClarificationNote) {
		return explanation
	}
	if explanation == "" {
		return skippedClarificationNote
	}
	return strings.TrimRight(explanation, "。.!！?？;；") +
		"；" + skippedClarificationNote
}

func (service *MatchService) currentReadyMatchAnalysis(
	ctx context.Context,
) (preparedMatchInput, domain.MatchAnalysis, error) {
	input, blocked, err := service.prepareInput(ctx)
	if err != nil {
		return preparedMatchInput{}, domain.MatchAnalysis{}, err
	}
	if blocked.Status != "" {
		return preparedMatchInput{}, domain.MatchAnalysis{}, errors.New(
			blocked.Message,
		)
	}
	analysis, found, err := service.matchRepository.GetLatest(
		ctx,
		input.profile.ID,
		input.jd.ID,
	)
	if err != nil {
		return preparedMatchInput{}, domain.MatchAnalysis{}, err
	}
	if !found || analysis.InputHash != input.inputHash ||
		analysis.Status != "succeeded" {
		return preparedMatchInput{}, domain.MatchAnalysis{}, errors.New(
			"match analysis is not ready for clarification",
		)
	}
	return input, analysis, nil
}

func (service *MatchService) prepareInput(
	ctx context.Context,
) (preparedMatchInput, MatchReview, error) {
	profile, err := resolveActiveProfile(
		ctx,
		service.profileRepository,
		service.clock.Now(),
	)
	if err != nil {
		return preparedMatchInput{}, MatchReview{}, err
	}
	jd, found, err := service.jdRepository.GetLatest(ctx)
	if err != nil {
		return preparedMatchInput{}, MatchReview{}, err
	}
	if !found {
		return preparedMatchInput{}, MatchReview{
			Status:  "blocked",
			Message: "请先在 JD 工作区粘贴并分析目标岗位。",
			Scope:   emptyRunScopeSummary(),
		}, nil
	}
	scope, documents, err := resolveRunScope(
		ctx,
		service.scopeRepository,
		service.profileRepository,
		profile.ID,
		jd.ID,
		service.clock.Now(),
	)
	if err != nil {
		return preparedMatchInput{}, MatchReview{}, err
	}
	scopeSummary := runScopeSummary(scope, documents)
	if jd.AnalysisStatus != "succeeded" || jd.AnalysisJSON == "" {
		return preparedMatchInput{}, emptyMatchReview(
			"blocked",
			jd,
			"请先完成当前 JD 的结构化分析。",
		).withScope(scopeSummary), nil
	}
	analysis, err := domain.DecodeJDAnalysis([]byte(jd.AnalysisJSON))
	if err != nil {
		return preparedMatchInput{}, MatchReview{}, fmt.Errorf(
			"decode JD analysis for matching: %w",
			err,
		)
	}
	requirements, err := buildMatchRequirements(analysis)
	if err != nil {
		return preparedMatchInput{}, MatchReview{}, err
	}
	storedEvidence, err := service.profileRepository.ListEvidence(ctx, profile.ID)
	if err != nil {
		return preparedMatchInput{}, MatchReview{}, err
	}
	if len(storedEvidence) == 0 {
		return preparedMatchInput{}, emptyMatchReview(
			"blocked",
			jd,
			"请先导入 Markdown 职业资料，生成可追溯 Evidence。",
		).withScope(scopeSummary), nil
	}
	evidence := applyRunScope(selectUsableEvidence(storedEvidence), scope)
	if len(evidence) == 0 {
		message := "现有 Evidence 均存在未解决冲突，请先在资料库中确认采用版本。"
		if scope.Mode == domain.RunScopeSelected {
			message = "所选资料范围没有可用 Evidence，请调整范围或先解决冲突。"
		}
		return preparedMatchInput{}, emptyMatchReview(
			"blocked",
			jd,
			message,
		).withScope(scopeSummary), nil
	}
	inputHash, err := hashMatchInput(jd, requirements, evidence)
	if err != nil {
		return preparedMatchInput{}, MatchReview{}, err
	}
	return preparedMatchInput{
		profile:      profile,
		jd:           jd,
		requirements: requirements,
		evidence:     evidence,
		inputHash:    inputHash,
		scope:        scopeSummary,
	}, MatchReview{}, nil
}

func (service *MatchService) saveFailure(
	ctx context.Context,
	input preparedMatchInput,
	matchErr error,
) {
	message := defaultMatchAnalysisErr
	if matchErr != nil {
		message = matchErr.Error()
	}
	now := service.clock.Now().UTC()
	analysis := domain.MatchAnalysis{
		ID:           matchAnalysisID(input.profile.ID, input.jd.ID),
		ProfileID:    input.profile.ID,
		JDID:         input.jd.ID,
		InputHash:    input.inputHash,
		Status:       "failed",
		Error:        message,
		CreatedAt:    now,
		UpdatedAt:    now,
		Requirements: input.requirements,
	}
	if existing, found, err := service.matchRepository.GetLatest(
		ctx,
		input.profile.ID,
		input.jd.ID,
	); err == nil && found {
		analysis.CreatedAt = existing.CreatedAt
	}
	if err := service.matchRepository.Save(ctx, analysis); err != nil {
		slog.Error(
			"match.analysis.failure.persist.failed",
			slog.String("analysis_id", analysis.ID),
			slog.Any("error", err),
		)
	}
}

func (service *MatchService) saveMatchStageResult(
	ctx context.Context,
	input preparedMatchInput,
	status workflow.StageStatus,
	resultJSON string,
	errorJSON string,
	now time.Time,
) {
	if service.stageRepository == nil {
		return
	}
	runID := resumeRunID(input.profile.ID, input.jd.ID)
	result := workflow.StageResult{
		ID:         stageResultID(runID, workflow.StageMatched, input.inputHash),
		RunID:      runID,
		Stage:      workflow.StageMatched,
		InputHash:  input.inputHash,
		Status:     status,
		ResultJSON: resultJSON,
		ErrorJSON:  errorJSON,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := service.stageRepository.SaveStageResult(ctx, result); err != nil {
		slog.Error(
			"match.stage_result.persist.failed",
			slog.String("run_id", runID),
			slog.String("stage", string(workflow.StageMatched)),
			slog.String("status", string(status)),
			slog.Any("error", err),
		)
	}
}

func matchStageResultJSON(
	analysis domain.MatchAnalysis,
	score domain.MatchScore,
	clarificationCount int,
) string {
	return stageResultJSON(struct {
		AnalysisID         string `json:"analysis_id"`
		InputHash          string `json:"input_hash"`
		RequirementCount   int    `json:"requirement_count"`
		ClarificationCount int    `json:"clarification_count"`
		TotalScore         int    `json:"total_score"`
		HardCapApplied     bool   `json:"hard_cap_applied"`
	}{
		AnalysisID:         analysis.ID,
		InputHash:          analysis.InputHash,
		RequirementCount:   len(analysis.Requirements),
		ClarificationCount: clarificationCount,
		TotalScore:         score.Total,
		HardCapApplied:     score.HardCapApplied,
	})
}

func stageErrorJSON(err error) string {
	message := defaultMatchAnalysisErr
	if err != nil {
		message = err.Error()
	}
	return stageResultJSON(struct {
		Message string `json:"message"`
	}{Message: message})
}

func stageResultJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func (service *MatchService) saveClarificationRound(
	ctx context.Context,
	input preparedMatchInput,
	analysis domain.MatchAnalysis,
	now time.Time,
) ([]domain.ClarificationQuestion, error) {
	runID := resumeRunID(input.profile.ID, input.jd.ID)
	questions := buildClarificationQuestions(runID, analysis, now)
	stage := workflow.StageMatched
	if len(questions) > 0 {
		stage = workflow.StageRequiresUserInput
	}
	if err := service.saveMatchRunStage(ctx, input, stage, now); err != nil {
		return nil, err
	}
	if err := service.clarificationRepository.ReplaceRoundQuestions(
		ctx,
		runID,
		1,
		questions,
	); err != nil {
		return nil, err
	}
	return questions, nil
}

func (service *MatchService) advanceClarificationRound(
	ctx context.Context,
	input preparedMatchInput,
	analysis domain.MatchAnalysis,
	questions []domain.ClarificationQuestion,
	now time.Time,
) error {
	runID := resumeRunID(input.profile.ID, input.jd.ID)
	if hasPendingClarifications(questions) {
		return service.saveMatchRunStage(
			ctx,
			input,
			workflow.StageRequiresUserInput,
			now,
		)
	}

	maxRound := maxClarificationRound(questions)
	if maxRound < domain.MaxClarificationRounds {
		askedRequirementIDs := make(map[string]struct{}, len(questions))
		for _, question := range questions {
			askedRequirementIDs[question.RequirementID] = struct{}{}
		}
		nextRound := maxRound + 1
		nextQuestions := buildClarificationQuestionsForRound(
			runID,
			analysis,
			nextRound,
			askedRequirementIDs,
			now,
		)
		if len(nextQuestions) > 0 {
			if err := service.saveMatchRunStage(
				ctx,
				input,
				workflow.StageRequiresUserInput,
				now,
			); err != nil {
				return err
			}
			return service.clarificationRepository.ReplaceRoundQuestions(
				ctx,
				runID,
				nextRound,
				nextQuestions,
			)
		}
	}
	return service.saveMatchRunStage(ctx, input, workflow.StageMatched, now)
}

func (service *MatchService) saveMatchRunStage(
	ctx context.Context,
	input preparedMatchInput,
	stage workflow.Stage,
	now time.Time,
) error {
	return service.runRepository.SaveRun(ctx, domain.ResumeRun{
		ID:             resumeRunID(input.profile.ID, input.jd.ID),
		ProfileID:      input.profile.ID,
		JDID:           input.jd.ID,
		Status:         "active",
		Stage:          string(stage),
		PackagingLevel: 0.5,
		Language:       resumeLanguageFromJD(input.jd),
		CreatedAt:      now,
		UpdatedAt:      now,
	})
}

type clarificationCandidate struct {
	requirement domain.MatchRequirement
	suggestion  domain.MatchSuggestion
}

func buildClarificationQuestions(
	runID string,
	analysis domain.MatchAnalysis,
	now time.Time,
) []domain.ClarificationQuestion {
	return buildClarificationQuestionsForRound(
		runID,
		analysis,
		1,
		nil,
		now,
	)
}

func buildClarificationQuestionsForRound(
	runID string,
	analysis domain.MatchAnalysis,
	round int,
	excludedRequirementIDs map[string]struct{},
	now time.Time,
) []domain.ClarificationQuestion {
	requirementsByID := make(
		map[string]domain.MatchRequirement,
		len(analysis.Requirements),
	)
	for _, requirement := range analysis.Requirements {
		requirementsByID[requirement.ID] = requirement
	}

	candidates := make([]clarificationCandidate, 0)
	for _, suggestion := range analysis.Suggestions {
		if !suggestion.ClarificationNeeded {
			continue
		}
		if _, excluded := excludedRequirementIDs[suggestion.RequirementID]; excluded {
			continue
		}
		requirement, exists := requirementsByID[suggestion.RequirementID]
		if !exists {
			continue
		}
		candidates = append(candidates, clarificationCandidate{
			requirement: requirement,
			suggestion:  suggestion,
		})
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		leftRequirement := candidates[left].requirement
		rightRequirement := candidates[right].requirement
		if leftRequirement.HardConstraint != rightRequirement.HardConstraint {
			return leftRequirement.HardConstraint
		}
		if leftRequirement.Importance != rightRequirement.Importance {
			return leftRequirement.Importance > rightRequirement.Importance
		}
		return leftRequirement.Ordinal < rightRequirement.Ordinal
	})
	if len(candidates) > domain.MaxClarificationQuestionsPerRound {
		candidates = candidates[:domain.MaxClarificationQuestionsPerRound]
	}

	questions := make([]domain.ClarificationQuestion, 0, len(candidates))
	for ordinal, candidate := range candidates {
		questions = append(questions, domain.ClarificationQuestion{
			ID:            clarificationQuestionID(runID, round, candidate.requirement.ID),
			RunID:         runID,
			RequirementID: candidate.requirement.ID,
			Round:         round,
			Ordinal:       ordinal,
			Question: clarificationQuestionText(
				candidate.requirement,
				candidate.suggestion,
			),
			Reason: clarificationReasonText(
				candidate.requirement,
				candidate.suggestion,
			),
			Status:    domain.ClarificationPending,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	return questions
}

func hasPendingClarifications(
	questions []domain.ClarificationQuestion,
) bool {
	for _, question := range questions {
		if question.Status == domain.ClarificationPending {
			return true
		}
	}
	return false
}

func maxClarificationRound(
	questions []domain.ClarificationQuestion,
) int {
	var maxRound int
	for _, question := range questions {
		if question.Round > maxRound {
			maxRound = question.Round
		}
	}
	return maxRound
}

func resumeLanguageFromJD(jd domain.JobDescription) domain.ResumeLanguage {
	if jd.Language == string(domain.JDLanguageEnglish) {
		return domain.ResumeLanguageEnglish
	}
	return domain.ResumeLanguageChinese
}

func clarificationQuestionText(
	requirement domain.MatchRequirement,
	suggestion domain.MatchSuggestion,
) string {
	switch suggestion.Strength {
	case domain.MatchStrengthPartial:
		return fmt.Sprintf(
			"关于“%s”，请补充具体职责范围、技术细节、规模或结果。",
			requirement.Text,
		)
	case domain.MatchStrengthMissing, domain.MatchStrengthUnknown:
		return fmt.Sprintf(
			"请确认你是否具备“%s”相关经历；如果有，请补充可验证的职责、项目或结果。",
			requirement.Text,
		)
	default:
		return fmt.Sprintf(
			"关于“%s”，还有哪些可验证信息需要补充？",
			requirement.Text,
		)
	}
}

func clarificationReasonText(
	requirement domain.MatchRequirement,
	suggestion domain.MatchSuggestion,
) string {
	prefix := "该要求会影响简历表达强度"
	if requirement.HardConstraint {
		prefix = "该要求是 JD 硬性条件"
	}
	return fmt.Sprintf(
		"%s，当前匹配为%s；%s",
		prefix,
		matchStrengthLabel(suggestion.Strength),
		suggestion.Explanation,
	)
}

func buildMatchRequirements(
	analysis domain.JDAnalysis,
) ([]domain.MatchRequirement, error) {
	requirements := make([]domain.MatchRequirement, 0)
	seen := make(map[string]struct{})
	appendRequirement := func(requirement domain.MatchRequirement) error {
		if _, exists := seen[requirement.ID]; exists {
			return fmt.Errorf(
				"duplicate match requirement id %q",
				requirement.ID,
			)
		}
		requirement.Ordinal = len(requirements)
		seen[requirement.ID] = struct{}{}
		requirements = append(requirements, requirement)
		return nil
	}

	for _, requirement := range analysis.RequiredSkills {
		if err := appendRequirement(domain.MatchRequirement{
			ID:             requirement.ID,
			Category:       domain.RequirementCategoryRequired,
			Text:           requirement.Text,
			Importance:     requirement.Importance,
			HardConstraint: requirement.HardConstraint,
		}); err != nil {
			return nil, err
		}
	}
	for _, constraint := range analysis.ScreeningConstraints {
		constraint = strings.TrimSpace(constraint)
		if constraint == "" {
			continue
		}
		if err := appendRequirement(domain.MatchRequirement{
			ID:             derivedRequirementID("screening", constraint),
			Category:       domain.RequirementCategoryRequired,
			Text:           constraint,
			Importance:     5,
			HardConstraint: true,
		}); err != nil {
			return nil, err
		}
	}
	for _, requirement := range analysis.Responsibilities {
		if err := appendRequirement(domain.MatchRequirement{
			ID:             requirement.ID,
			Category:       domain.RequirementCategoryResponsibility,
			Text:           requirement.Text,
			Importance:     requirement.Importance,
			HardConstraint: requirement.HardConstraint,
		}); err != nil {
			return nil, err
		}
	}
	if analysis.Level != nil && strings.TrimSpace(*analysis.Level) != "" {
		level := strings.TrimSpace(*analysis.Level)
		if err := appendRequirement(domain.MatchRequirement{
			ID:         derivedRequirementID("level", level),
			Category:   domain.RequirementCategoryLevel,
			Text:       level,
			Importance: 3,
		}); err != nil {
			return nil, err
		}
	}
	for _, signal := range analysis.DomainSignals {
		signal = strings.TrimSpace(signal)
		if signal == "" {
			continue
		}
		if err := appendRequirement(domain.MatchRequirement{
			ID:         derivedRequirementID("domain", signal),
			Category:   domain.RequirementCategoryDomain,
			Text:       signal,
			Importance: 3,
		}); err != nil {
			return nil, err
		}
	}
	for _, requirement := range analysis.PreferredSkills {
		if err := appendRequirement(domain.MatchRequirement{
			ID:             requirement.ID,
			Category:       domain.RequirementCategoryPreferred,
			Text:           requirement.Text,
			Importance:     requirement.Importance,
			HardConstraint: requirement.HardConstraint,
		}); err != nil {
			return nil, err
		}
	}
	if len(requirements) == 0 {
		return nil, errors.New("JD analysis has no matchable requirements")
	}
	return requirements, nil
}

func hashMatchInput(
	jd domain.JobDescription,
	requirements []domain.MatchRequirement,
	evidence []domain.Evidence,
) (string, error) {
	contents, err := json.Marshal(struct {
		JDRawHash    string
		Requirements []domain.MatchRequirement
		Evidence     []domain.Evidence
	}{
		JDRawHash:    jd.RawHash,
		Requirements: requirements,
		Evidence:     evidence,
	})
	if err != nil {
		return "", fmt.Errorf("encode match input: %w", err)
	}
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:]), nil
}

func matchReviewFromAnalysis(
	analysis domain.MatchAnalysis,
	input preparedMatchInput,
	score domain.MatchScore,
) MatchReview {
	evidenceByID := make(map[string]domain.Evidence, len(input.evidence))
	for _, item := range input.evidence {
		evidenceByID[item.ID] = item
	}
	suggestionsByID := make(
		map[string]domain.MatchSuggestion,
		len(analysis.Suggestions),
	)
	for _, suggestion := range analysis.Suggestions {
		suggestionsByID[suggestion.RequirementID] = suggestion
	}

	review := MatchReview{
		Status:         "ready",
		Message:        "匹配分只表示当前资料与 JD 的证据覆盖度。",
		JDTitle:        input.jd.Title,
		Company:        input.jd.Company,
		TotalScore:     score.Total,
		HardCapApplied: score.HardCapApplied,
		UpdatedAt:      analysis.UpdatedAt.UTC().Format(timeFormat),
		Dimensions:     make([]MatchDimensionSummary, 0, len(score.Dimensions)),
		Requirements:   make([]RequirementMatchSummary, 0, len(analysis.Requirements)),
		Clarifications: make([]ClarificationSummary, 0),
		Scope:          input.scope,
	}
	for _, dimension := range score.Dimensions {
		review.Dimensions = append(
			review.Dimensions,
			MatchDimensionSummary{
				Category:         string(dimension.Category),
				Label:            matchCategoryLabel(dimension.Category),
				Weight:           dimension.Weight,
				Earned:           dimension.Earned,
				RequirementCount: dimension.RequirementCount,
			},
		)
	}
	for _, requirement := range analysis.Requirements {
		suggestion := suggestionsByID[requirement.ID]
		summary := RequirementMatchSummary{
			ID:                  requirement.ID,
			Category:            string(requirement.Category),
			Group:               matchCategoryLabel(requirement.Category),
			Text:                requirement.Text,
			Importance:          requirement.Importance,
			HardConstraint:      requirement.HardConstraint,
			Strength:            string(suggestion.Strength),
			Explanation:         suggestion.Explanation,
			ClarificationNeeded: suggestion.ClarificationNeeded,
			Evidence:            make([]MatchEvidenceSummary, 0, len(suggestion.EvidenceIDs)),
		}
		for _, evidenceID := range suggestion.EvidenceIDs {
			if item, exists := evidenceByID[evidenceID]; exists {
				summary.Evidence = append(
					summary.Evidence,
					matchEvidenceSummary(item),
				)
			}
		}
		switch suggestion.Strength {
		case domain.MatchStrengthStrong:
			review.Counts.Strong++
		case domain.MatchStrengthPartial:
			review.Counts.Partial++
		case domain.MatchStrengthMissing:
			review.Counts.Missing++
		case domain.MatchStrengthUnknown:
			review.Counts.Unknown++
		}
		review.Requirements = append(review.Requirements, summary)
	}
	return review
}

func matchEvidenceSummary(item domain.Evidence) MatchEvidenceSummary {
	summary := MatchEvidenceSummary{
		ID:      item.ID,
		Kind:    item.Kind,
		Title:   item.Title,
		Content: item.Content,
		Sources: make([]MatchEvidenceSourceSummary, 0, len(item.Sources)),
	}
	for _, source := range item.Sources {
		summary.Sources = append(
			summary.Sources,
			MatchEvidenceSourceSummary{
				ChunkID:      source.ChunkID,
				DocumentID:   source.DocumentID,
				DocumentName: source.DocumentName,
				ChunkText:    source.ChunkText,
				LocatorJSON:  source.LocatorJSON,
				QuoteStart:   source.QuoteStart,
				QuoteEnd:     source.QuoteEnd,
			},
		)
	}
	return summary
}

func clarificationSummaries(
	questions []domain.ClarificationQuestion,
) []ClarificationSummary {
	summaries := make([]ClarificationSummary, 0, len(questions))
	for _, question := range questions {
		summaries = append(summaries, ClarificationSummary{
			ID:            question.ID,
			RequirementID: question.RequirementID,
			Round:         question.Round,
			Ordinal:       question.Ordinal,
			Question:      question.Question,
			Reason:        question.Reason,
			Status:        string(question.Status),
			Answer:        question.Answer,
		})
	}
	return summaries
}

func emptyMatchReview(
	status string,
	jd domain.JobDescription,
	message string,
) MatchReview {
	return MatchReview{
		Status:       status,
		Message:      message,
		JDTitle:      jd.Title,
		Company:      jd.Company,
		Dimensions:   make([]MatchDimensionSummary, 0),
		Requirements: make([]RequirementMatchSummary, 0),
		Clarifications: make(
			[]ClarificationSummary,
			0,
		),
		Scope: emptyRunScopeSummary(),
	}
}

func (review MatchReview) withScope(scope RunScopeSummary) MatchReview {
	review.Scope = scope
	return review
}

func emptyRunScopeSummary() RunScopeSummary {
	return RunScopeSummary{
		Mode:                string(domain.RunScopeAll),
		SelectedDocumentIDs: make([]string, 0),
		Documents:           make([]RunScopeDocumentSummary, 0),
	}
}

func runScopeSummary(
	scope domain.ResumeRunScope,
	documents []domain.SourceDocument,
) RunScopeSummary {
	summary := RunScopeSummary{
		Mode:                string(scope.Mode),
		AvailableCount:      len(documents),
		SelectedDocumentIDs: append([]string(nil), scope.DocumentIDs...),
		Documents:           make([]RunScopeDocumentSummary, 0, len(documents)),
	}
	for _, document := range documents {
		selected := runScopeContainsDocument(scope, document.ID)
		if selected {
			summary.SelectedCount++
		}
		summary.Documents = append(summary.Documents, RunScopeDocumentSummary{
			ID:           document.ID,
			OriginalName: document.OriginalName,
			Kind:         document.Kind,
			Selected:     selected,
		})
	}
	return summary
}

func matchCategoryLabel(category domain.RequirementCategory) string {
	switch category {
	case domain.RequirementCategoryRequired:
		return "必要技能与硬性条件"
	case domain.RequirementCategoryResponsibility:
		return "主要职责证据"
	case domain.RequirementCategoryLevel:
		return "岗位级别与责任范围"
	case domain.RequirementCategoryDomain:
		return "领域与业务经验"
	case domain.RequirementCategoryPreferred:
		return "加分项"
	default:
		return string(category)
	}
}

func matchStrengthLabel(strength domain.MatchStrength) string {
	switch strength {
	case domain.MatchStrengthStrong:
		return "强匹配"
	case domain.MatchStrengthPartial:
		return "部分匹配"
	case domain.MatchStrengthMissing:
		return "缺失"
	case domain.MatchStrengthUnknown:
		return "未知"
	default:
		return string(strength)
	}
}

func derivedRequirementID(kind string, value string) string {
	return "derived-" + kind + "-" + uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte(kind+":"+value),
	).String()
}

func matchAnalysisID(profileID string, jdID string) string {
	return uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte("https://autocv.local/matches/"+profileID+"/"+jdID),
	).String()
}

func clarificationQuestionID(
	runID string,
	round int,
	requirementID string,
) string {
	return uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte(fmt.Sprintf(
			"https://autocv.local/clarifications/%s/%d/%s",
			runID,
			round,
			requirementID,
		)),
	).String()
}

func runConfirmationID(runID string, questionID string) string {
	return uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte(fmt.Sprintf(
			"https://autocv.local/run-confirmations/%s/%s",
			runID,
			questionID,
		)),
	).String()
}
