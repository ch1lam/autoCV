package workflow

import (
	"strings"
	"testing"
	"time"
)

func TestStageValidity(t *testing.T) {
	for _, stage := range []Stage{
		StageProfileReady,
		StageJDAnalyzed,
		StageMatched,
		StageRequiresUserInput,
		StageDrafted,
		StageReviewed,
		StageRendered,
		StageCompleted,
	} {
		if !stage.Valid() {
			t.Fatalf("expected stage %q to be valid", stage)
		}
	}
	if Stage("unknown").Valid() {
		t.Fatal("expected unknown stage to be invalid")
	}
}

func TestOrderedStagesReturnsStateMachineOrder(t *testing.T) {
	stages := OrderedStages()
	expected := []Stage{
		StageProfileReady,
		StageJDAnalyzed,
		StageMatched,
		StageRequiresUserInput,
		StageDrafted,
		StageReviewed,
		StageRendered,
		StageCompleted,
	}
	if len(stages) != len(expected) {
		t.Fatalf("expected %d stages, got %d", len(expected), len(stages))
	}
	for index := range expected {
		if stages[index] != expected[index] {
			t.Fatalf("unexpected stage order %#v", stages)
		}
	}

	stages[0] = Stage("mutated")
	if OrderedStages()[0] != StageProfileReady {
		t.Fatal("ordered stages should return a defensive copy")
	}
}

func TestStageStatusValidity(t *testing.T) {
	for _, status := range []StageStatus{
		StageStatusPending,
		StageStatusRunning,
		StageStatusSucceeded,
		StageStatusFailed,
		StageStatusSkipped,
		StageStatusCancelled,
	} {
		if !status.Valid() {
			t.Fatalf("expected status %q to be valid", status)
		}
	}
	if StageStatus("unknown").Valid() {
		t.Fatal("expected unknown status to be invalid")
	}
}

func TestStageResultValidate(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	result := StageResult{
		ID:         "stage-result-1",
		RunID:      "run-1",
		Stage:      StageMatched,
		InputHash:  "input-hash",
		Status:     StageStatusSucceeded,
		ResultJSON: `{"ok":true}`,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("expected valid stage result: %v", err)
	}

	result.Stage = Stage("unknown")
	if err := result.Validate(); err == nil ||
		!strings.Contains(err.Error(), "invalid workflow stage") {
		t.Fatalf("expected invalid stage error, got %v", err)
	}
}
