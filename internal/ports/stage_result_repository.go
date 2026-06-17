package ports

import (
	"context"

	"github.com/ch1lam/autocv/internal/workflow"
)

type StageResultRepository interface {
	SaveStageResult(context.Context, workflow.StageResult) error
	ListStageResults(
		context.Context,
		string,
	) ([]workflow.StageResult, error)
	LatestStageResult(
		context.Context,
		string,
		workflow.Stage,
	) (workflow.StageResult, bool, error)
	SucceededStageResult(
		context.Context,
		string,
		workflow.Stage,
		string,
	) (workflow.StageResult, bool, error)
}
