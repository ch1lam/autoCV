package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

func TestProfileRepositoryEnsuresOneDefaultProfile(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, ctx, filepath.Join(t.TempDir(), "profile.db"))
	defer db.Close()
	repository := NewProfileRepository(db)
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	first, err := repository.EnsureDefaultProfile(ctx, "Main Profile", "zh-CN", now)
	if err != nil {
		t.Fatalf("ensure first profile: %v", err)
	}
	second, err := repository.EnsureDefaultProfile(ctx, "Ignored", "en", now)
	if err != nil {
		t.Fatalf("ensure second profile: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected stable default profile id")
	}
	if second.Name != "Main Profile" {
		t.Fatalf("expected original profile to remain unchanged, got %q", second.Name)
	}
}

func TestProfileRepositorySavesImportedDocumentAtomically(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, ctx, filepath.Join(t.TempDir(), "import.db"))
	defer db.Close()
	repository := NewProfileRepository(db)
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	profile, err := repository.EnsureDefaultProfile(ctx, "Main Profile", "zh-CN", now)
	if err != nil {
		t.Fatalf("ensure profile: %v", err)
	}

	imported := importedDocumentFixture(profile.ID, now)
	if err := repository.SaveImportedDocument(ctx, imported); err != nil {
		t.Fatalf("save imported document: %v", err)
	}

	document, found, err := repository.FindDocumentByHash(
		ctx,
		profile.ID,
		imported.Document.ContentHash,
	)
	if err != nil {
		t.Fatalf("find document: %v", err)
	}
	if !found || document.ID != imported.Document.ID {
		t.Fatalf("expected saved document")
	}

	documents, err := repository.ListDocuments(ctx, profile.ID)
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}
	if len(documents) != 1 {
		t.Fatalf("expected one document, got %d", len(documents))
	}

	evidence, err := repository.ListEvidence(ctx, profile.ID)
	if err != nil {
		t.Fatalf("list evidence: %v", err)
	}
	if len(evidence) != 1 {
		t.Fatalf("expected one evidence item, got %d", len(evidence))
	}
}

func TestProfileRepositoryRejectsDuplicateContentHash(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, ctx, filepath.Join(t.TempDir(), "duplicate.db"))
	defer db.Close()
	repository := NewProfileRepository(db)
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	profile, err := repository.EnsureDefaultProfile(ctx, "Main Profile", "zh-CN", now)
	if err != nil {
		t.Fatalf("ensure profile: %v", err)
	}

	first := importedDocumentFixture(profile.ID, now)
	if err := repository.SaveImportedDocument(ctx, first); err != nil {
		t.Fatalf("save first document: %v", err)
	}
	second := importedDocumentFixture(profile.ID, now)
	second.Document.ID = "document-2"
	second.Chunks[0].ID = "chunk-2"
	second.Chunks[0].DocumentID = second.Document.ID
	second.Evidence[0].ID = "evidence-2"
	second.EvidenceSources[0] = domain.EvidenceSource{
		EvidenceID: second.Evidence[0].ID,
		ChunkID:    second.Chunks[0].ID,
	}

	err = repository.SaveImportedDocument(ctx, second)
	if err == nil {
		t.Fatalf("expected duplicate hash error, got %v", err)
	}

	documents, err := repository.ListDocuments(ctx, profile.ID)
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}
	if len(documents) != 1 {
		t.Fatalf("expected one stored document, got %d", len(documents))
	}
}

func TestProfileRepositoryRollsBackIncompleteImport(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, ctx, filepath.Join(t.TempDir(), "rollback.db"))
	defer db.Close()
	repository := NewProfileRepository(db)
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	profile, err := repository.EnsureDefaultProfile(ctx, "Main Profile", "zh-CN", now)
	if err != nil {
		t.Fatalf("ensure profile: %v", err)
	}

	imported := importedDocumentFixture(profile.ID, now)
	imported.EvidenceSources[0].ChunkID = "missing-chunk"
	if err := repository.SaveImportedDocument(ctx, imported); err == nil {
		t.Fatal("expected invalid evidence source error")
	}

	documents, err := repository.ListDocuments(ctx, profile.ID)
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}
	if len(documents) != 0 {
		t.Fatalf("expected failed import to roll back")
	}
}

func importedDocumentFixture(profileID string, now time.Time) ports.ImportedDocument {
	return ports.ImportedDocument{
		Document: domain.SourceDocument{
			ID:           "document-1",
			ProfileID:    profileID,
			Kind:         "markdown",
			OriginalName: "profile.md",
			ManagedPath:  "sources/profile/document-1/profile.md",
			ContentHash:  "hash-1",
			ParseStatus:  "succeeded",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		Chunks: []domain.SourceChunk{{
			ID:          "chunk-1",
			DocumentID:  "document-1",
			Ordinal:     0,
			Text:        "Built backend services.",
			LocatorJSON: `{"heading_path":["Experience"],"start":0,"end":23}`,
		}},
		Evidence: []domain.Evidence{{
			ID:           "evidence-1",
			ProfileID:    profileID,
			Kind:         "experience",
			Title:        "Backend service delivery",
			Content:      "Built backend services.",
			Confidence:   0.9,
			UserVerified: false,
			CreatedAt:    now,
			UpdatedAt:    now,
		}},
		EvidenceSources: []domain.EvidenceSource{{
			EvidenceID: "evidence-1",
			ChunkID:    "chunk-1",
			QuoteStart: 0,
			QuoteEnd:   23,
		}},
	}
}
