package app

import (
	"context"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/adapters/fakeprovider"
	"github.com/ch1lam/autocv/internal/workflow"
)

func TestWorkflowServiceRestoresLatestRunStages(t *testing.T) {
	fixture := newMatchServiceFixture(t, fakeprovider.New())
	fixture.importProfile(t)
	fixture.analyzeJD(t, fixture.jdText)
	if _, err := fixture.service.Analyze(); err != nil {
		t.Fatalf("analyze match: %v", err)
	}
	run, found, err := fixture.scopeRepository.LatestRun(context.Background())
	if err != nil {
		t.Fatalf("read latest run: %v", err)
	}
	if !found {
		t.Fatal("expected latest run after match analysis")
	}

	failed := workflow.StageResult{
		ID:        "stage-result-draft-failed",
		RunID:     run.ID,
		Stage:     workflow.StageDrafted,
		InputHash: "draft-hash",
		Status:    workflow.StageStatusFailed,
		ErrorJSON: `{"message":"resume draft failed","detail":"provider timed out"}`,
		CreatedAt: profileTestTime.Add(time.Hour),
		UpdatedAt: profileTestTime.Add(time.Hour),
	}
	staleRenderedResult := workflow.StageResult{
		ID:         "stage-result-rendered-stale",
		RunID:      run.ID,
		Stage:      workflow.StageRendered,
		InputHash:  "render-hash",
		Status:     workflow.StageStatusSucceeded,
		ResultJSON: `{"artifact_id":"artifact-1"}`,
		CreatedAt:  profileTestTime.Add(30 * time.Minute),
		UpdatedAt:  profileTestTime.Add(30 * time.Minute),
	}
	if err := fixture.stageRepository.SaveStageResult(
		context.Background(),
		staleRenderedResult,
	); err != nil {
		t.Fatalf("save rendered stage result: %v", err)
	}
	if err := fixture.stageRepository.SaveStageResult(
		context.Background(),
		failed,
	); err != nil {
		t.Fatalf("save failed stage result: %v", err)
	}

	service := NewWorkflowService(
		fixture.scopeRepository,
		fixture.stageRepository,
	)
	status, err := service.GetStatus()
	if err != nil {
		t.Fatalf("get workflow status: %v", err)
	}
	if status.Status != "ready" ||
		status.RunID != run.ID ||
		status.CurrentStage != string(workflow.StageRequiresUserInput) {
		t.Fatalf("unexpected workflow status %#v", status)
	}
	if len(status.Stages) != len(workflow.OrderedStages()) {
		t.Fatalf("unexpected stage count %#v", status.Stages)
	}
	matched := findWorkflowStage(t, status, workflow.StageMatched)
	if matched.Status != string(workflow.StageStatusSucceeded) ||
		!matched.HasResult ||
		matched.HasError {
		t.Fatalf("unexpected matched stage %#v", matched)
	}
	drafted := findWorkflowStage(t, status, workflow.StageDrafted)
	if drafted.Status != string(workflow.StageStatusFailed) ||
		!drafted.HasError ||
		drafted.ErrorMessage != "resume draft failed" {
		t.Fatalf("unexpected drafted stage %#v", drafted)
	}
	rendered := findWorkflowStage(t, status, workflow.StageRendered)
	if rendered.Status != string(workflow.StageStatusPending) ||
		rendered.HasResult ||
		rendered.HasError {
		t.Fatalf("unexpected rendered stage %#v", rendered)
	}
}

func TestWorkflowServiceReportsEmptyStatus(t *testing.T) {
	fixture := newMatchServiceFixture(t, fakeprovider.New())
	service := NewWorkflowService(
		fixture.scopeRepository,
		fixture.stageRepository,
	)

	status, err := service.GetStatus()
	if err != nil {
		t.Fatalf("get empty workflow status: %v", err)
	}
	if status.Status != "empty" || status.RunID != "" {
		t.Fatalf("unexpected empty workflow status %#v", status)
	}
	if len(status.Stages) != len(workflow.OrderedStages()) {
		t.Fatalf("unexpected empty stage count %#v", status.Stages)
	}
}

func TestWorkflowServiceRerunsDraftStage(t *testing.T) {
	fixture := newMatchServiceFixture(t, fakeprovider.New())
	fixture.importProfile(t)
	fixture.analyzeJD(t, fixture.jdText)
	if _, err := fixture.service.Analyze(); err != nil {
		t.Fatalf("analyze match: %v", err)
	}

	resumeService := NewResumeService(
		fixture.scopeRepository,
		fixture.stageRepository,
		fixture.confirmationRepository,
		fixture.matchRepository,
		fixture.profileRepository,
		fixture.jdRepository,
		fakeprovider.New(),
		fixedClock{now: profileTestTime.Add(2 * time.Hour)},
	)
	generated, err := resumeService.Generate("zh", 0.5)
	if err != nil {
		t.Fatalf("generate resume: %v", err)
	}
	if generated.Version != 1 {
		t.Fatalf("expected first resume version, got %#v", generated)
	}

	rerunService := NewResumeService(
		fixture.scopeRepository,
		fixture.stageRepository,
		fixture.confirmationRepository,
		fixture.matchRepository,
		fixture.profileRepository,
		fixture.jdRepository,
		fakeprovider.New(),
		fixedClock{now: profileTestTime.Add(3 * time.Hour)},
	)
	workflowService := NewWorkflowService(
		fixture.scopeRepository,
		fixture.stageRepository,
		WorkflowStageRunners{Resume: rerunService},
	)
	status, err := workflowService.RerunStage(string(workflow.StageDrafted))
	if err != nil {
		t.Fatalf("rerun drafted stage: %v", err)
	}
	if status.CurrentStage != string(workflow.StageDrafted) {
		t.Fatalf("expected drafted current stage, got %#v", status)
	}
	drafted := findWorkflowStage(t, status, workflow.StageDrafted)
	if drafted.Status != string(workflow.StageStatusSucceeded) ||
		!drafted.HasResult {
		t.Fatalf("unexpected drafted stage after rerun %#v", drafted)
	}

	run, found, err := fixture.scopeRepository.LatestRun(context.Background())
	if err != nil {
		t.Fatalf("read latest run: %v", err)
	}
	if !found {
		t.Fatal("expected latest run after rerun")
	}
	_, resume, found, err := fixture.scopeRepository.GetLatest(
		context.Background(),
		run.ProfileID,
		run.JDID,
	)
	if err != nil {
		t.Fatalf("read latest resume: %v", err)
	}
	if !found || resume.Version != 2 {
		t.Fatalf("expected forced rerun to create version 2, found=%v %#v", found, resume)
	}
}

func findWorkflowStage(
	t *testing.T,
	status WorkflowStatus,
	stage workflow.Stage,
) WorkflowStageSummary {
	t.Helper()
	for _, summary := range status.Stages {
		if summary.Stage == string(stage) {
			return summary
		}
	}
	t.Fatalf("stage %q was not found in %#v", stage, status.Stages)
	return WorkflowStageSummary{}
}
