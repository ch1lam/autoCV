package app

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
	"github.com/ch1lam/autocv/internal/workflow"
)

type WorkflowService struct {
	runRepository   ports.ResumeRunRepository
	stageRepository ports.StageResultRepository
}

type WorkflowStatus struct {
	Status       string                 `json:"status"`
	Message      string                 `json:"message"`
	RunID        string                 `json:"runId"`
	RunStatus    string                 `json:"runStatus"`
	CurrentStage string                 `json:"currentStage"`
	UpdatedAt    string                 `json:"updatedAt"`
	Stages       []WorkflowStageSummary `json:"stages"`
}

type WorkflowStageSummary struct {
	Stage        string `json:"stage"`
	Status       string `json:"status"`
	InputHash    string `json:"inputHash"`
	HasResult    bool   `json:"hasResult"`
	HasError     bool   `json:"hasError"`
	ErrorMessage string `json:"errorMessage"`
	UpdatedAt    string `json:"updatedAt"`
}

func NewWorkflowService(
	runRepository ports.ResumeRunRepository,
	stageRepository ports.StageResultRepository,
) *WorkflowService {
	return &WorkflowService{
		runRepository:   runRepository,
		stageRepository: stageRepository,
	}
}

func (service *WorkflowService) GetStatus() (WorkflowStatus, error) {
	ctx := context.Background()
	run, found, err := service.runRepository.LatestRun(ctx)
	if err != nil {
		return WorkflowStatus{}, err
	}
	if !found {
		return WorkflowStatus{
			Status:  "empty",
			Message: "尚未创建 Resume Run。",
			Stages:  pendingWorkflowStages(),
		}, nil
	}

	results, err := service.stageRepository.ListStageResults(ctx, run.ID)
	if err != nil {
		return WorkflowStatus{}, err
	}
	return workflowStatusFrom(run, results), nil
}

func workflowStatusFrom(
	run domain.ResumeRun,
	results []workflow.StageResult,
) WorkflowStatus {
	latestByStage := make(map[workflow.Stage]workflow.StageResult)
	for _, result := range results {
		if _, exists := latestByStage[result.Stage]; exists {
			continue
		}
		latestByStage[result.Stage] = result
	}

	stages := make([]WorkflowStageSummary, 0, len(workflow.OrderedStages()))
	var newestUpstreamStageResult workflow.StageResult
	var hasUpstreamStageResult bool
	for _, stage := range workflow.OrderedStages() {
		summary := WorkflowStageSummary{
			Stage:  string(stage),
			Status: string(workflow.StageStatusPending),
		}
		if result, found := latestByStage[stage]; found {
			summary.Status = string(result.Status)
			summary.InputHash = result.InputHash
			summary.HasResult = strings.TrimSpace(result.ResultJSON) != ""
			summary.HasError = strings.TrimSpace(result.ErrorJSON) != ""
			summary.ErrorMessage = workflowStageErrorMessage(result.ErrorJSON)
			summary.UpdatedAt = result.UpdatedAt.UTC().Format(timeFormat)
			if hasUpstreamStageResult &&
				result.UpdatedAt.Before(newestUpstreamStageResult.UpdatedAt) {
				summary = WorkflowStageSummary{
					Stage:  string(stage),
					Status: string(workflow.StageStatusPending),
				}
			}
			if !hasUpstreamStageResult ||
				newestUpstreamStageResult.UpdatedAt.Before(result.UpdatedAt) {
				newestUpstreamStageResult = result
				hasUpstreamStageResult = true
			}
		}
		stages = append(stages, summary)
	}

	status := strings.TrimSpace(run.Status)
	if status == "" {
		status = "active"
	}
	return WorkflowStatus{
		Status:       "ready",
		Message:      "已恢复最近一次 Resume Run 状态。",
		RunID:        run.ID,
		RunStatus:    status,
		CurrentStage: run.Stage,
		UpdatedAt:    run.UpdatedAt.UTC().Format(timeFormat),
		Stages:       stages,
	}
}

func pendingWorkflowStages() []WorkflowStageSummary {
	stages := make([]WorkflowStageSummary, 0, len(workflow.OrderedStages()))
	for _, stage := range workflow.OrderedStages() {
		stages = append(stages, WorkflowStageSummary{
			Stage:  string(stage),
			Status: string(workflow.StageStatusPending),
		})
	}
	return stages
}

func workflowStageErrorMessage(errorJSON string) string {
	if strings.TrimSpace(errorJSON) == "" {
		return ""
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(errorJSON), &payload); err != nil {
		return ""
	}
	return payload.Message
}
