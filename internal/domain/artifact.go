package domain

import "time"

type ArtifactKind string

const (
	ArtifactKindPDF ArtifactKind = "pdf"
)

type Artifact struct {
	ID           string
	RunID        string
	ResumeID     string
	Kind         ArtifactKind
	Path         string
	PreviewPaths []string
	ContentHash  string
	CreatedAt    time.Time
}
