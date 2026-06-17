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
