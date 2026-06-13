package ports

import (
	"context"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
)

type ImportedDocument struct {
	Document        domain.SourceDocument
	Chunks          []domain.SourceChunk
	Evidence        []domain.Evidence
	EvidenceSources []domain.EvidenceSource
}

type ProfileRepository interface {
	EnsureDefaultProfile(
		context.Context,
		string,
		string,
		time.Time,
	) (domain.Profile, error)
	CreateProfile(context.Context, domain.Profile) error
	ListProfiles(context.Context) ([]domain.Profile, error)
	GetActiveProfile(context.Context) (domain.Profile, bool, error)
	SetActiveProfile(context.Context, string) (domain.Profile, error)
	FindDocumentByHash(
		context.Context,
		string,
		string,
	) (domain.SourceDocument, bool, error)
	SaveImportedDocument(context.Context, ImportedDocument) error
	ListDocuments(context.Context, string) ([]domain.SourceDocument, error)
	ListEvidence(context.Context, string) ([]domain.Evidence, error)
}
