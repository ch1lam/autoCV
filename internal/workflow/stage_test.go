package workflow

import "testing"

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
