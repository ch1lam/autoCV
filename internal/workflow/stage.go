package workflow

type Stage string

const (
	StageProfileReady      Stage = "profile_ready"
	StageJDAnalyzed        Stage = "jd_analyzed"
	StageMatched           Stage = "matched"
	StageRequiresUserInput Stage = "requires_user_input"
	StageDrafted           Stage = "drafted"
	StageReviewed          Stage = "reviewed"
	StageRendered          Stage = "rendered"
	StageCompleted         Stage = "completed"
)

var orderedStages = []Stage{
	StageProfileReady,
	StageJDAnalyzed,
	StageMatched,
	StageRequiresUserInput,
	StageDrafted,
	StageReviewed,
	StageRendered,
	StageCompleted,
}

var stageTransitions = map[Stage][]Stage{
	StageProfileReady: {
		StageJDAnalyzed,
	},
	StageJDAnalyzed: {
		StageMatched,
	},
	StageMatched: {
		StageRequiresUserInput,
		StageDrafted,
	},
	StageRequiresUserInput: {
		StageMatched,
		StageDrafted,
	},
	StageDrafted: {
		StageReviewed,
		StageRendered,
	},
	StageReviewed: {
		StageDrafted,
		StageRendered,
	},
	StageRendered: {
		StageDrafted,
		StageCompleted,
	},
	StageCompleted: {
		StageDrafted,
	},
}

type StageStatus string

const (
	StageStatusPending   StageStatus = "pending"
	StageStatusRunning   StageStatus = "running"
	StageStatusSucceeded StageStatus = "succeeded"
	StageStatusFailed    StageStatus = "failed"
	StageStatusSkipped   StageStatus = "skipped"
	StageStatusCancelled StageStatus = "cancelled"
)

func (stage Stage) Valid() bool {
	switch stage {
	case StageProfileReady,
		StageJDAnalyzed,
		StageMatched,
		StageRequiresUserInput,
		StageDrafted,
		StageReviewed,
		StageRendered,
		StageCompleted:
		return true
	default:
		return false
	}
}

func OrderedStages() []Stage {
	stages := make([]Stage, len(orderedStages))
	copy(stages, orderedStages)
	return stages
}

func StageIndex(stage Stage) (int, bool) {
	for index, candidate := range orderedStages {
		if candidate == stage {
			return index, true
		}
	}
	return -1, false
}

func NextStages(stage Stage) []Stage {
	stages := stageTransitions[stage]
	if len(stages) == 0 {
		return []Stage{}
	}
	next := make([]Stage, len(stages))
	copy(next, stages)
	return next
}

func CanTransition(from Stage, to Stage) bool {
	if !from.Valid() || !to.Valid() {
		return false
	}
	for _, stage := range stageTransitions[from] {
		if stage == to {
			return true
		}
	}
	return false
}

func RerunTarget(stage Stage) (Stage, bool) {
	switch stage {
	case StageMatched:
		return StageMatched, true
	case StageRequiresUserInput:
		return StageMatched, true
	case StageDrafted:
		return StageDrafted, true
	case StageRendered:
		return StageRendered, true
	default:
		return "", false
	}
}

func (status StageStatus) Valid() bool {
	switch status {
	case StageStatusPending,
		StageStatusRunning,
		StageStatusSucceeded,
		StageStatusFailed,
		StageStatusSkipped,
		StageStatusCancelled:
		return true
	default:
		return false
	}
}
