package ports

import "context"

type DeletionImpact struct {
	TargetKind    string         `json:"targetKind"`
	TargetID      string         `json:"targetId"`
	Counts        DeletionCounts `json:"counts"`
	ManagedPaths  []string       `json:"managedPaths"`
	ArtifactPaths []string       `json:"artifactPaths"`
}

type DeletionCounts struct {
	Profiles               int `json:"profiles"`
	SourceDocuments        int `json:"sourceDocuments"`
	SourceChunks           int `json:"sourceChunks"`
	Evidence               int `json:"evidence"`
	EvidenceSources        int `json:"evidenceSources"`
	JobDescriptions        int `json:"jobDescriptions"`
	MatchAnalyses          int `json:"matchAnalyses"`
	MatchRequirements      int `json:"matchRequirements"`
	RequirementMatches     int `json:"requirementMatches"`
	MatchEvidence          int `json:"matchEvidence"`
	RunScopes              int `json:"runScopes"`
	RunScopeDocuments      int `json:"runScopeDocuments"`
	ResumeRuns             int `json:"resumeRuns"`
	StageResults           int `json:"stageResults"`
	Resumes                int `json:"resumes"`
	ResumeBlocks           int `json:"resumeBlocks"`
	ResumeBlockSources     int `json:"resumeBlockSources"`
	Artifacts              int `json:"artifacts"`
	ClarificationQuestions int `json:"clarificationQuestions"`
	RunConfirmations       int `json:"runConfirmations"`
}

type DataControlRepository interface {
	PreviewProfileDeletion(context.Context, string) (DeletionImpact, error)
	PreviewJDDeletion(context.Context, string) (DeletionImpact, error)
	PreviewRunDeletion(context.Context, string) (DeletionImpact, error)
	PreviewArtifactDeletion(context.Context, string) (DeletionImpact, error)
}
