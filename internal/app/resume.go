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

type ResumeService struct {
	resumeRepository       ports.ResumeRepository
	stageRepository        ports.StageResultRepository
	confirmationRepository ports.RunConfirmationRepository
	matchRepository        ports.MatchRepository
	profileRepository      ports.ProfileRepository
	jdRepository           ports.JDRepository
	drafter                ports.ResumeDrafter
	clock                  ports.Clock
}

type ResumeWorkspace struct {
	Status            string                   `json:"status"`
	Message           string                   `json:"message"`
	CanExport         bool                     `json:"canExport"`
	ExportIssues      []string                 `json:"exportIssues"`
	RunID             string                   `json:"runId"`
	ResumeID          string                   `json:"resumeId"`
	Version           int                      `json:"version"`
	Language          string                   `json:"language"`
	TargetRole        string                   `json:"targetRole"`
	PackagingLevel    float64                  `json:"packagingLevel"`
	PackagingLabel    string                   `json:"packagingLabel"`
	PackagingStrategy PackagingStrategySummary `json:"packagingStrategy"`
	Markdown          string                   `json:"markdown"`
	UpdatedAt         string                   `json:"updatedAt"`
	OptimizationNotes []string                 `json:"optimizationNotes"`
	Blocks            []ResumeBlockSummary     `json:"blocks"`
}

type PackagingStrategySummary struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	Description      string   `json:"description"`
	LanguageStrength string   `json:"languageStrength"`
	SelectionPolicy  string   `json:"selectionPolicy"`
	InferencePolicy  string   `json:"inferencePolicy"`
	Guardrails       []string `json:"guardrails"`
}

type ResumeBlockSummary struct {
	ID             string                 `json:"id"`
	Kind           string                 `json:"kind"`
	Label          string                 `json:"label"`
	Content        string                 `json:"content"`
	Locked         bool                   `json:"locked"`
	GroundingLevel string                 `json:"groundingLevel"`
	Optimization   string                 `json:"optimization"`
	Evidence       []MatchEvidenceSummary `json:"evidence"`
}

type preparedResumeInput struct {
	profile       domain.Profile
	jd            domain.JobDescription
	match         domain.MatchAnalysis
	evidence      []domain.Evidence
	confirmations []domain.RunConfirmation
	role          string
}

func (service *ResumeService) currentReadyResume(
	ctx context.Context,
) (domain.ResumeRun, domain.Resume, error) {
	input, blocked, err := service.prepareInput(ctx)
	if err != nil {
		return domain.ResumeRun{}, domain.Resume{}, err
	}
	if blocked.Status != "" {
		return domain.ResumeRun{}, domain.Resume{}, errors.New(blocked.Message)
	}

	run, resume, found, err := service.resumeRepository.GetLatest(
		ctx,
		input.profile.ID,
		input.jd.ID,
	)
	if err != nil {
		return domain.ResumeRun{}, domain.Resume{}, err
	}
	if !found {
		return domain.ResumeRun{}, domain.Resume{}, errors.New(
			"resume has not been generated",
		)
	}
	strategy := resumePackagingStrategyForDisplay(run.PackagingLevel)
	expectedHash, err := hashResumeInput(
		input.match,
		input.confirmations,
		run.Language,
		strategy,
	)
	if err != nil {
		return domain.ResumeRun{}, domain.Resume{}, err
	}
	if resume.InputHash != expectedHash {
		return domain.ResumeRun{}, domain.Resume{}, errors.New(
			"resume inputs changed; regenerate before rendering",
		)
	}
	if err := domain.ValidateResume(resume, input.evidence); err != nil {
		return domain.ResumeRun{}, domain.Resume{}, fmt.Errorf(
			"validate resume for rendering: %w",
			err,
		)
	}
	return run, resume, nil
}

func NewResumeService(
	resumeRepository ports.ResumeRepository,
	stageRepository ports.StageResultRepository,
	confirmationRepository ports.RunConfirmationRepository,
	matchRepository ports.MatchRepository,
	profileRepository ports.ProfileRepository,
	jdRepository ports.JDRepository,
	drafter ports.ResumeDrafter,
	clock ports.Clock,
) *ResumeService {
	return &ResumeService{
		resumeRepository:       resumeRepository,
		stageRepository:        stageRepository,
		confirmationRepository: confirmationRepository,
		matchRepository:        matchRepository,
		profileRepository:      profileRepository,
		jdRepository:           jdRepository,
		drafter:                drafter,
		clock:                  clock,
	}
}

func (service *ResumeService) GetWorkspace() (ResumeWorkspace, error) {
	ctx := context.Background()
	input, blocked, err := service.prepareInput(ctx)
	if err != nil {
		return ResumeWorkspace{}, err
	}
	if blocked.Status != "" {
		return blocked, nil
	}

	run, resume, found, err := service.resumeRepository.GetLatest(
		ctx,
		input.profile.ID,
		input.jd.ID,
	)
	if err != nil {
		return ResumeWorkspace{}, err
	}
	if !found {
		return ResumeWorkspace{
			Status:       "pending",
			Message:      "匹配结果已就绪，可以生成第一版结构化简历。",
			ExportIssues: make([]string, 0),
			TargetRole:   input.role,
			Blocks:       make([]ResumeBlockSummary, 0),
		}, nil
	}
	strategy := resumePackagingStrategyForDisplay(run.PackagingLevel)
	expectedHash, err := hashResumeInput(
		input.match,
		input.confirmations,
		run.Language,
		strategy,
	)
	if err != nil {
		return ResumeWorkspace{}, err
	}
	if resume.InputHash != expectedHash {
		message := "资料、JD、匹配结果或追问确认已变化，请重新生成未锁定内容。"
		if lockedCount := lockedResumeBlockCount(resume.Blocks); lockedCount > 0 {
			message = fmt.Sprintf(
				"资料、JD、匹配结果或追问确认已变化；%d 个锁定 Block 不会自动改写，请确认它们仍适合当前岗位后再重新生成。",
				lockedCount,
			)
		}
		return ResumeWorkspace{
			Status:            "stale",
			Message:           message,
			ExportIssues:      []string{"资料、JD、匹配结果或追问确认已变化，请先重新生成当前版本。"},
			RunID:             run.ID,
			ResumeID:          resume.ID,
			Version:           resume.Version,
			Language:          string(run.Language),
			TargetRole:        resume.TargetRole,
			PackagingLevel:    run.PackagingLevel,
			PackagingLabel:    strategy.Label,
			PackagingStrategy: packagingStrategySummary(strategy),
			Markdown:          resume.Markdown,
			UpdatedAt:         resume.CreatedAt.UTC().Format(timeFormat),
			OptimizationNotes: resume.OptimizationNotes,
			Blocks:            resumeBlockSummaries(resume, input.evidence),
		}, nil
	}
	if err := domain.ValidateResume(resume, input.evidence); err != nil {
		return ResumeWorkspace{}, fmt.Errorf(
			"validate stored resume: %w",
			err,
		)
	}
	return resumeWorkspaceFrom(run, resume, input.evidence), nil
}

func (service *ResumeService) Generate(
	language string,
	packagingLevel float64,
) (ResumeWorkspace, error) {
	resumeLanguage := domain.ResumeLanguage(language)
	if resumeLanguage != domain.ResumeLanguageChinese &&
		resumeLanguage != domain.ResumeLanguageEnglish {
		return ResumeWorkspace{}, fmt.Errorf(
			"invalid resume language %q",
			language,
		)
	}
	if packagingLevel < 0 || packagingLevel > 1 {
		return ResumeWorkspace{}, fmt.Errorf(
			"resume packaging level %.2f is outside 0..1",
			packagingLevel,
		)
	}
	strategy, found := domain.ResumePackagingStrategyForLevel(packagingLevel)
	if !found {
		return ResumeWorkspace{}, fmt.Errorf(
			"resume packaging level %.2f is not a supported strategy",
			packagingLevel,
		)
	}

	ctx := context.Background()
	input, blocked, err := service.prepareInput(ctx)
	if err != nil {
		return ResumeWorkspace{}, err
	}
	if blocked.Status != "" {
		return ResumeWorkspace{}, errors.New(blocked.Message)
	}
	inputHash, err := hashResumeInput(
		input.match,
		input.confirmations,
		resumeLanguage,
		strategy,
	)
	if err != nil {
		return ResumeWorkspace{}, err
	}
	now := service.clock.Now().UTC()
	run, previous, found, err := service.resumeRepository.GetLatest(
		ctx,
		input.profile.ID,
		input.jd.ID,
	)
	if err != nil {
		return ResumeWorkspace{}, err
	}
	if !found {
		run = domain.ResumeRun{
			ID:        resumeRunID(input.profile.ID, input.jd.ID),
			ProfileID: input.profile.ID,
			JDID:      input.jd.ID,
			CreatedAt: now,
		}
	}
	run.Status = "active"
	run.Stage = string(workflow.StageDrafted)
	run.PackagingLevel = strategy.Level
	run.Language = resumeLanguage
	run.UpdatedAt = now

	version := 1
	if found {
		version = previous.Version + 1
	}
	if err := service.resumeRepository.SaveRun(ctx, run); err != nil {
		return ResumeWorkspace{}, err
	}
	service.saveResumeDraftStageResult(
		ctx,
		run.ID,
		inputHash,
		workflow.StageStatusRunning,
		"",
		"",
		now,
	)
	draft, err := service.drafter.DraftResume(
		ctx,
		ports.DraftResumeRequest{
			Language:          resumeLanguage,
			TargetRole:        input.role,
			PackagingLevel:    strategy.Level,
			PackagingStrategy: strategy,
			Match:             input.match,
			Evidence:          input.evidence,
			Confirmations:     input.confirmations,
		},
	)
	if err != nil {
		status := workflow.StageStatusFailed
		if errors.Is(err, context.Canceled) {
			status = workflow.StageStatusCancelled
		}
		service.saveResumeDraftStageResult(
			ctx,
			run.ID,
			inputHash,
			status,
			"",
			stageErrorJSON("resume draft failed", err),
			service.clock.Now().UTC(),
		)
		return ResumeWorkspace{}, fmt.Errorf("draft resume: %w", err)
	}
	if err := domain.ValidateResumeDraft(draft, input.evidence); err != nil {
		service.saveResumeDraftStageResult(
			ctx,
			run.ID,
			inputHash,
			workflow.StageStatusFailed,
			"",
			stageErrorJSON("resume draft failed", err),
			service.clock.Now().UTC(),
		)
		return ResumeWorkspace{}, fmt.Errorf("validate resume draft: %w", err)
	}
	resume := domain.Resume{
		ID:                uuid.NewString(),
		RunID:             run.ID,
		InputHash:         inputHash,
		Version:           version,
		Language:          draft.Language,
		TargetRole:        draft.TargetRole,
		OptimizationNotes: append([]string(nil), draft.OptimizationNotes...),
		CreatedAt:         now,
		Blocks:            resumeBlocksFromDraft(run.ID, draft.Blocks),
	}
	if found {
		resume.Blocks = preserveLockedResumeBlocks(
			resume.Blocks,
			previous.Blocks,
			previous.InputHash != inputHash,
			&resume.OptimizationNotes,
		)
	}
	resume.Markdown = domain.RenderResumeMarkdown(resume)
	if err := domain.ValidateResume(resume, input.evidence); err != nil {
		service.saveResumeDraftStageResult(
			ctx,
			run.ID,
			inputHash,
			workflow.StageStatusFailed,
			"",
			stageErrorJSON("resume draft failed", err),
			service.clock.Now().UTC(),
		)
		return ResumeWorkspace{}, fmt.Errorf("validate generated resume: %w", err)
	}
	if err := service.resumeRepository.SaveVersion(ctx, run, resume); err != nil {
		service.saveResumeDraftStageResult(
			ctx,
			run.ID,
			inputHash,
			workflow.StageStatusFailed,
			"",
			stageErrorJSON("resume draft failed", err),
			service.clock.Now().UTC(),
		)
		return ResumeWorkspace{}, err
	}
	service.saveResumeDraftStageResult(
		ctx,
		run.ID,
		inputHash,
		workflow.StageStatusSucceeded,
		resumeDraftStageResultJSON(resume),
		"",
		now,
	)
	return resumeWorkspaceFrom(run, resume, input.evidence), nil
}

func (service *ResumeService) saveResumeDraftStageResult(
	ctx context.Context,
	runID string,
	inputHash string,
	status workflow.StageStatus,
	resultJSON string,
	errorJSON string,
	now time.Time,
) {
	if service.stageRepository == nil {
		return
	}
	result := workflow.StageResult{
		ID:         stageResultID(runID, workflow.StageDrafted, inputHash),
		RunID:      runID,
		Stage:      workflow.StageDrafted,
		InputHash:  inputHash,
		Status:     status,
		ResultJSON: resultJSON,
		ErrorJSON:  errorJSON,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := service.stageRepository.SaveStageResult(ctx, result); err != nil {
		slog.Error(
			"resume.stage_result.persist.failed",
			slog.String("run_id", runID),
			slog.String("stage", string(workflow.StageDrafted)),
			slog.String("status", string(status)),
			slog.Any("error", err),
		)
	}
}

func resumeDraftStageResultJSON(resume domain.Resume) string {
	return stageResultJSON(struct {
		ResumeID          string `json:"resume_id"`
		InputHash         string `json:"input_hash"`
		Version           int    `json:"version"`
		BlockCount        int    `json:"block_count"`
		OptimizationCount int    `json:"optimization_count"`
	}{
		ResumeID:          resume.ID,
		InputHash:         resume.InputHash,
		Version:           resume.Version,
		BlockCount:        len(resume.Blocks),
		OptimizationCount: len(resume.OptimizationNotes),
	})
}

func (service *ResumeService) UpdateMarkdown(
	markdown string,
) (ResumeWorkspace, error) {
	return service.updateCurrentResume(func(
		resume domain.Resume,
	) (domain.Resume, error) {
		return domain.ApplyResumeMarkdown(resume, markdown)
	})
}

func (service *ResumeService) SetBlockLocked(
	blockID string,
	locked bool,
) (ResumeWorkspace, error) {
	blockID = strings.TrimSpace(blockID)
	if blockID == "" {
		return ResumeWorkspace{}, errors.New("resume block id is empty")
	}
	return service.updateCurrentResume(func(
		resume domain.Resume,
	) (domain.Resume, error) {
		updated := resume
		updated.Blocks = append([]domain.ResumeBlock(nil), resume.Blocks...)
		for index := range updated.Blocks {
			if updated.Blocks[index].ID == blockID {
				updated.Blocks[index].Locked = locked
				return updated, nil
			}
		}
		return domain.Resume{}, fmt.Errorf(
			"resume block %q was not found",
			blockID,
		)
	})
}

func (service *ResumeService) updateCurrentResume(
	update func(domain.Resume) (domain.Resume, error),
) (ResumeWorkspace, error) {
	ctx := context.Background()
	input, blocked, err := service.prepareInput(ctx)
	if err != nil {
		return ResumeWorkspace{}, err
	}
	if blocked.Status != "" {
		return ResumeWorkspace{}, errors.New(blocked.Message)
	}
	run, current, found, err := service.resumeRepository.GetLatest(
		ctx,
		input.profile.ID,
		input.jd.ID,
	)
	if err != nil {
		return ResumeWorkspace{}, err
	}
	if !found {
		return ResumeWorkspace{}, errors.New("resume has not been generated")
	}
	strategy := resumePackagingStrategyForDisplay(run.PackagingLevel)
	expectedHash, err := hashResumeInput(
		input.match,
		input.confirmations,
		run.Language,
		strategy,
	)
	if err != nil {
		return ResumeWorkspace{}, err
	}
	if current.InputHash != expectedHash {
		return ResumeWorkspace{}, errors.New(
			"resume inputs changed; regenerate before editing",
		)
	}

	updated, err := update(current)
	if err != nil {
		return ResumeWorkspace{}, err
	}
	now := service.clock.Now().UTC()
	updated.ID = uuid.NewString()
	updated.Version = current.Version + 1
	updated.CreatedAt = now
	if err := domain.ValidateResume(updated, input.evidence); err != nil {
		return ResumeWorkspace{}, fmt.Errorf("validate updated resume: %w", err)
	}
	run.UpdatedAt = now
	if err := service.resumeRepository.SaveVersion(ctx, run, updated); err != nil {
		return ResumeWorkspace{}, err
	}
	return resumeWorkspaceFrom(run, updated, input.evidence), nil
}

func (service *ResumeService) prepareInput(
	ctx context.Context,
) (preparedResumeInput, ResumeWorkspace, error) {
	profile, err := resolveActiveProfile(
		ctx,
		service.profileRepository,
		service.clock.Now(),
	)
	if err != nil {
		return preparedResumeInput{}, ResumeWorkspace{}, err
	}
	jd, found, err := service.jdRepository.GetLatest(ctx)
	if err != nil {
		return preparedResumeInput{}, ResumeWorkspace{}, err
	}
	if !found || jd.AnalysisStatus != "succeeded" || jd.AnalysisJSON == "" {
		return preparedResumeInput{}, ResumeWorkspace{
			Status:       "blocked",
			Message:      "请先完成目标 JD 的结构化分析。",
			ExportIssues: make([]string, 0),
			Blocks:       make([]ResumeBlockSummary, 0),
		}, nil
	}
	jdAnalysis, err := domain.DecodeJDAnalysis([]byte(jd.AnalysisJSON))
	if err != nil {
		return preparedResumeInput{}, ResumeWorkspace{}, fmt.Errorf(
			"decode JD analysis for resume: %w",
			err,
		)
	}
	requirements, err := buildMatchRequirements(jdAnalysis)
	if err != nil {
		return preparedResumeInput{}, ResumeWorkspace{}, err
	}
	scope, _, err := resolveRunScope(
		ctx,
		service.resumeRepository,
		service.profileRepository,
		profile.ID,
		jd.ID,
		service.clock.Now(),
	)
	if err != nil {
		return preparedResumeInput{}, ResumeWorkspace{}, err
	}
	evidence, err := service.profileRepository.ListEvidence(ctx, profile.ID)
	if err != nil {
		return preparedResumeInput{}, ResumeWorkspace{}, err
	}
	if len(evidence) == 0 {
		return preparedResumeInput{}, ResumeWorkspace{
			Status:       "blocked",
			Message:      "请先导入 Markdown 职业资料。",
			ExportIssues: make([]string, 0),
			Blocks:       make([]ResumeBlockSummary, 0),
		}, nil
	}
	evidence = applyRunScope(selectUsableEvidence(evidence), scope)
	if len(evidence) == 0 {
		return preparedResumeInput{}, ResumeWorkspace{
			Status:       "blocked",
			Message:      "所选资料范围没有可用 Evidence，请调整范围后重新匹配。",
			ExportIssues: make([]string, 0),
			Blocks:       make([]ResumeBlockSummary, 0),
		}, nil
	}
	currentMatchHash, err := hashMatchInput(jd, requirements, evidence)
	if err != nil {
		return preparedResumeInput{}, ResumeWorkspace{}, err
	}
	match, found, err := service.matchRepository.GetLatest(
		ctx,
		profile.ID,
		jd.ID,
	)
	if err != nil {
		return preparedResumeInput{}, ResumeWorkspace{}, err
	}
	if !found || match.Status != "succeeded" {
		return preparedResumeInput{}, ResumeWorkspace{
			Status:       "blocked",
			Message:      "请先完成当前资料与 JD 的匹配分析。",
			ExportIssues: make([]string, 0),
			Blocks:       make([]ResumeBlockSummary, 0),
		}, nil
	}
	if match.InputHash != currentMatchHash {
		return preparedResumeInput{}, ResumeWorkspace{
			Status:       "blocked",
			Message:      "匹配结果已失效，请先重新匹配。",
			ExportIssues: make([]string, 0),
			Blocks:       make([]ResumeBlockSummary, 0),
		}, nil
	}
	if err := domain.ValidateMatchSuggestions(
		match.Requirements,
		evidence,
		match.Suggestions,
	); err != nil {
		return preparedResumeInput{}, ResumeWorkspace{}, fmt.Errorf(
			"validate match analysis for resume: %w",
			err,
		)
	}
	confirmations, err := service.confirmationRepository.ListRunConfirmations(
		ctx,
		resumeRunID(profile.ID, jd.ID),
	)
	if err != nil {
		return preparedResumeInput{}, ResumeWorkspace{}, err
	}
	role := strings.TrimSpace(jdAnalysis.Role)
	if role == "" {
		role = strings.TrimSpace(jd.Title)
	}
	return preparedResumeInput{
		profile:       profile,
		jd:            jd,
		match:         match,
		evidence:      evidence,
		confirmations: confirmations,
		role:          role,
	}, ResumeWorkspace{}, nil
}

func hashResumeInput(
	match domain.MatchAnalysis,
	confirmations []domain.RunConfirmation,
	language domain.ResumeLanguage,
	packagingStrategy domain.ResumePackagingStrategy,
) (string, error) {
	contents, err := json.Marshal(struct {
		MatchID           string
		MatchInput        string
		Requirements      []domain.MatchRequirement
		Suggestions       []domain.MatchSuggestion
		Confirmations     []domain.RunConfirmation
		Language          domain.ResumeLanguage
		PackagingStrategy domain.ResumePackagingStrategy
	}{
		MatchID:           match.ID,
		MatchInput:        match.InputHash,
		Requirements:      match.Requirements,
		Suggestions:       match.Suggestions,
		Confirmations:     confirmations,
		Language:          language,
		PackagingStrategy: packagingStrategy,
	})
	if err != nil {
		return "", fmt.Errorf("encode resume input: %w", err)
	}
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:]), nil
}

func resumeBlocksFromDraft(
	runID string,
	drafts []domain.ResumeBlockDraft,
) []domain.ResumeBlock {
	blocks := make([]domain.ResumeBlock, 0, len(drafts))
	seenKeys := make(map[string]int)
	for _, draft := range drafts {
		sourceIDs := append([]string(nil), draft.SourceEvidenceIDs...)
		sort.Strings(sourceIDs)
		key := string(draft.Kind)
		if draft.Kind != domain.ResumeBlockSummary {
			key += ":" + strings.Join(sourceIDs, ",")
		}
		seenKeys[key]++
		if seenKeys[key] > 1 {
			key += fmt.Sprintf(":%d", seenKeys[key])
		}
		blocks = append(blocks, domain.ResumeBlock{
			ID:                resumeBlockID(runID, key),
			Kind:              draft.Kind,
			Content:           strings.TrimSpace(draft.Content),
			SourceEvidenceIDs: append([]string(nil), draft.SourceEvidenceIDs...),
			GroundingLevel:    draft.GroundingLevel,
			Optimization:      strings.TrimSpace(draft.Optimization),
		})
	}
	return blocks
}

func preserveLockedResumeBlocks(
	generated []domain.ResumeBlock,
	previous []domain.ResumeBlock,
	upstreamChanged bool,
	notes *[]string,
) []domain.ResumeBlock {
	lockedByID := make(map[string]domain.ResumeBlock)
	for _, block := range previous {
		if block.Locked {
			lockedByID[block.ID] = block
		}
	}
	if len(lockedByID) == 0 {
		return generated
	}
	preserved := make(map[string]struct{})
	for index := range generated {
		if block, exists := lockedByID[generated[index].ID]; exists {
			generated[index] = block
			preserved[block.ID] = struct{}{}
		}
	}
	for _, block := range previous {
		if !block.Locked {
			continue
		}
		if _, exists := preserved[block.ID]; exists {
			continue
		}
		generated = append(generated, block)
	}
	note := fmt.Sprintf("重新生成时保留了 %d 个锁定内容块。", len(lockedByID))
	if upstreamChanged {
		note = fmt.Sprintf(
			"上游资料或 JD 已变化，仍逐字保留了 %d 个锁定内容块；请确认它们与当前岗位和篇幅没有冲突。",
			len(lockedByID),
		)
	}
	*notes = append(*notes, note)
	return generated
}

func lockedResumeBlockCount(blocks []domain.ResumeBlock) int {
	count := 0
	for _, block := range blocks {
		if block.Locked {
			count++
		}
	}
	return count
}

func resumeWorkspaceFrom(
	run domain.ResumeRun,
	resume domain.Resume,
	evidence []domain.Evidence,
) ResumeWorkspace {
	exportIssues := domain.ResumeExportIssues(resume)
	strategy := resumePackagingStrategyForDisplay(run.PackagingLevel)
	return ResumeWorkspace{
		Status:            "ready",
		Message:           "结构化简历、Markdown 与来源引用已保存到本地。",
		CanExport:         len(exportIssues) == 0,
		ExportIssues:      exportIssues,
		RunID:             run.ID,
		ResumeID:          resume.ID,
		Version:           resume.Version,
		Language:          string(resume.Language),
		TargetRole:        resume.TargetRole,
		PackagingLevel:    run.PackagingLevel,
		PackagingLabel:    strategy.Label,
		PackagingStrategy: packagingStrategySummary(strategy),
		Markdown:          resume.Markdown,
		UpdatedAt:         resume.CreatedAt.UTC().Format(timeFormat),
		OptimizationNotes: append([]string(nil), resume.OptimizationNotes...),
		Blocks:            resumeBlockSummaries(resume, evidence),
	}
}

func resumeBlockSummaries(
	resume domain.Resume,
	evidence []domain.Evidence,
) []ResumeBlockSummary {
	evidenceByID := make(map[string]domain.Evidence, len(evidence))
	for _, item := range evidence {
		evidenceByID[item.ID] = item
	}
	summaries := make([]ResumeBlockSummary, 0, len(resume.Blocks))
	for _, block := range resume.Blocks {
		summary := ResumeBlockSummary{
			ID:             block.ID,
			Kind:           string(block.Kind),
			Label:          resumeBlockKindLabel(block.Kind),
			Content:        block.Content,
			Locked:         block.Locked,
			GroundingLevel: string(block.GroundingLevel),
			Optimization:   block.Optimization,
			Evidence:       make([]MatchEvidenceSummary, 0, len(block.SourceEvidenceIDs)),
		}
		for _, evidenceID := range block.SourceEvidenceIDs {
			if item, exists := evidenceByID[evidenceID]; exists {
				summary.Evidence = append(
					summary.Evidence,
					matchEvidenceSummary(item),
				)
			}
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func resumeBlockKindLabel(kind domain.ResumeBlockKind) string {
	switch kind {
	case domain.ResumeBlockSummary:
		return "职业概述"
	case domain.ResumeBlockExperience:
		return "工作经历"
	case domain.ResumeBlockProject:
		return "项目经历"
	case domain.ResumeBlockSkill:
		return "技能"
	case domain.ResumeBlockEducation:
		return "教育经历"
	case domain.ResumeBlockCertification:
		return "认证"
	default:
		return string(kind)
	}
}

func resumePackagingLabel(level float64) string {
	return resumePackagingStrategyForDisplay(level).Label
}

func resumePackagingStrategyForDisplay(
	level float64,
) domain.ResumePackagingStrategy {
	if strategy, found := domain.ResumePackagingStrategyForLevel(level); found {
		return strategy
	}
	return domain.ResumePackagingStrategy{
		ID:               "custom",
		Level:            level,
		Label:            resumePackagingLabelFallback(level),
		Description:      "历史版本使用的自定义包装强度。",
		EvidenceLimit:    6,
		LanguageStrength: "按历史包装参数保持生成结果。",
		SelectionPolicy:  "按历史包装参数选择内容。",
		InferencePolicy:  "仍遵守不新增事实和不写入未确认数字的边界。",
		Guardrails: []string{
			"不新增职责、技术或结果。",
			"不写入未经确认的数字。",
		},
	}
}

func resumePackagingLabelFallback(level float64) string {
	switch {
	case level < 0.34:
		return "保守"
	case level < 0.67:
		return "平衡"
	default:
		return "强化"
	}
}

func packagingStrategySummary(
	strategy domain.ResumePackagingStrategy,
) PackagingStrategySummary {
	return PackagingStrategySummary{
		ID:               strategy.ID,
		Label:            strategy.Label,
		Description:      strategy.Description,
		LanguageStrength: strategy.LanguageStrength,
		SelectionPolicy:  strategy.SelectionPolicy,
		InferencePolicy:  strategy.InferencePolicy,
		Guardrails:       append([]string(nil), strategy.Guardrails...),
	}
}

func resumeRunID(profileID string, jdID string) string {
	return uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte("https://autocv.local/resume-runs/"+profileID+"/"+jdID),
	).String()
}

func stageResultID(
	runID string,
	stage workflow.Stage,
	inputHash string,
) string {
	return uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte(
			fmt.Sprintf(
				"https://autocv.local/stage-results/%s/%s/%s",
				runID,
				stage,
				inputHash,
			),
		),
	).String()
}

func resumeBlockID(runID string, key string) string {
	return uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte("https://autocv.local/resume-blocks/"+runID+"/"+key),
	).String()
}
