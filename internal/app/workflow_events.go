package app

import (
	"time"

	"github.com/ch1lam/autocv/internal/workflow"
)

const WorkflowStageUpdatedEvent = "workflow.stage.updated"

type WorkflowStageEvent struct {
	RunID     string `json:"runId"`
	Stage     string `json:"stage"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	UpdatedAt string `json:"updatedAt"`
}

type WorkflowEventSink interface {
	EmitWorkflowStage(WorkflowStageEvent)
}

type noopWorkflowEventSink struct{}

func (noopWorkflowEventSink) EmitWorkflowStage(WorkflowStageEvent) {}

func workflowEventSinkFrom(sinks []WorkflowEventSink) WorkflowEventSink {
	if len(sinks) == 0 || sinks[0] == nil {
		return noopWorkflowEventSink{}
	}
	return sinks[0]
}

func emitWorkflowStageEvent(
	sink WorkflowEventSink,
	runID string,
	stage workflow.Stage,
	status workflow.StageStatus,
	errorJSON string,
	now time.Time,
) {
	sink.EmitWorkflowStage(WorkflowStageEvent{
		RunID:     runID,
		Stage:     string(stage),
		Status:    string(status),
		Message:   workflowStageErrorMessage(errorJSON),
		UpdatedAt: now.UTC().Format(timeFormat),
	})
}
