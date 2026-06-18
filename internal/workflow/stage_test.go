package workflow

import (
	"reflect"
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

func TestStageIndex(t *testing.T) {
	for expectedIndex, stage := range OrderedStages() {
		index, found := StageIndex(stage)
		if !found || index != expectedIndex {
			t.Fatalf(
				"expected %q at index %d, got index=%d found=%v",
				stage,
				expectedIndex,
				index,
				found,
			)
		}
	}

	index, found := StageIndex(Stage("unknown"))
	if found || index != -1 {
		t.Fatalf("expected unknown stage to be absent, got index=%d found=%v", index, found)
	}
}

func TestNextStages(t *testing.T) {
	tests := []struct {
		name     string
		stage    Stage
		expected []Stage
	}{
		{
			name:     "profile ready advances to jd analyzed",
			stage:    StageProfileReady,
			expected: []Stage{StageJDAnalyzed},
		},
		{
			name:     "matched can request clarification or draft",
			stage:    StageMatched,
			expected: []Stage{StageRequiresUserInput, StageDrafted},
		},
		{
			name:     "requires user input can restart matching or draft",
			stage:    StageRequiresUserInput,
			expected: []Stage{StageMatched, StageDrafted},
		},
		{
			name:     "drafted can be reviewed or rendered",
			stage:    StageDrafted,
			expected: []Stage{StageReviewed, StageRendered},
		},
		{
			name:     "reviewed can redraft or render",
			stage:    StageReviewed,
			expected: []Stage{StageDrafted, StageRendered},
		},
		{
			name:     "rendered can regenerate or complete",
			stage:    StageRendered,
			expected: []Stage{StageDrafted, StageCompleted},
		},
		{
			name:     "completed can restart from draft",
			stage:    StageCompleted,
			expected: []Stage{StageDrafted},
		},
		{
			name:     "unknown stage has no next stages",
			stage:    Stage("unknown"),
			expected: []Stage{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := NextStages(tt.stage)
			if !reflect.DeepEqual(next, tt.expected) {
				t.Fatalf("expected next stages %#v, got %#v", tt.expected, next)
			}
			if len(next) > 0 {
				next[0] = Stage("mutated")
				if reflect.DeepEqual(NextStages(tt.stage), next) {
					t.Fatal("next stages should return a defensive copy")
				}
			}
		})
	}
}

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name string
		from Stage
		to   Stage
		want bool
	}{
		{name: "profile ready to jd analyzed", from: StageProfileReady, to: StageJDAnalyzed, want: true},
		{name: "jd analyzed to matched", from: StageJDAnalyzed, to: StageMatched, want: true},
		{name: "matched to requires user input", from: StageMatched, to: StageRequiresUserInput, want: true},
		{name: "matched to drafted", from: StageMatched, to: StageDrafted, want: true},
		{name: "requires user input to matched", from: StageRequiresUserInput, to: StageMatched, want: true},
		{name: "drafted to reviewed", from: StageDrafted, to: StageReviewed, want: true},
		{name: "reviewed to rendered", from: StageReviewed, to: StageRendered, want: true},
		{name: "rendered to completed", from: StageRendered, to: StageCompleted, want: true},
		{name: "completed to drafted", from: StageCompleted, to: StageDrafted, want: true},
		{name: "matched cannot skip to rendered", from: StageMatched, to: StageRendered, want: false},
		{name: "rendered cannot move back to matched", from: StageRendered, to: StageMatched, want: false},
		{name: "invalid source cannot transition", from: Stage("unknown"), to: StageMatched, want: false},
		{name: "invalid target cannot transition", from: StageMatched, to: Stage("unknown"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanTransition(tt.from, tt.to); got != tt.want {
				t.Fatalf("expected CanTransition(%q, %q)=%v, got %v", tt.from, tt.to, tt.want, got)
			}
		})
	}
}

func TestRerunTarget(t *testing.T) {
	tests := []struct {
		name     string
		stage    Stage
		expected Stage
		want     bool
	}{
		{name: "matched reruns matching", stage: StageMatched, expected: StageMatched, want: true},
		{name: "clarification reruns matching", stage: StageRequiresUserInput, expected: StageMatched, want: true},
		{name: "drafted reruns drafting", stage: StageDrafted, expected: StageDrafted, want: true},
		{name: "rendered reruns rendering", stage: StageRendered, expected: StageRendered, want: true},
		{name: "profile ready cannot rerun directly", stage: StageProfileReady, want: false},
		{name: "reviewed cannot rerun directly", stage: StageReviewed, want: false},
		{name: "unknown cannot rerun directly", stage: Stage("unknown"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stage, ok := RerunTarget(tt.stage)
			if ok != tt.want || stage != tt.expected {
				t.Fatalf(
					"expected rerun target stage=%q ok=%v, got stage=%q ok=%v",
					tt.expected,
					tt.want,
					stage,
					ok,
				)
			}
		})
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
