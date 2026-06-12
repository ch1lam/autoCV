package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
	"github.com/google/uuid"
)

const (
	defaultProfileName     = "主资料库"
	defaultProfileLanguage = "zh-CN"
)

type ProfileService struct {
	repository ports.ProfileRepository
	parser     ports.DocumentParser
	extractor  ports.ProfileExtractor
	files      ports.ManagedFileStore
	picker     ports.MarkdownPicker
	clock      ports.Clock
}

type ProfileOverview struct {
	ProfileID       string                  `json:"profileId"`
	Name            string                  `json:"name"`
	DefaultLanguage string                  `json:"defaultLanguage"`
	Documents       []SourceDocumentSummary `json:"documents"`
	Evidence        []EvidenceSummary       `json:"evidence"`
}

type SourceDocumentSummary struct {
	ID           string `json:"id"`
	OriginalName string `json:"originalName"`
	Kind         string `json:"kind"`
	ParseStatus  string `json:"parseStatus"`
	ImportedAt   string `json:"importedAt"`
}

type EvidenceSummary struct {
	ID         string                  `json:"id"`
	Kind       string                  `json:"kind"`
	Title      string                  `json:"title"`
	Content    string                  `json:"content"`
	Confidence float64                 `json:"confidence"`
	Sources    []EvidenceSourceSummary `json:"sources"`
}

type EvidenceSourceSummary struct {
	ChunkID      string `json:"chunkId"`
	DocumentID   string `json:"documentId"`
	DocumentName string `json:"documentName"`
	ChunkText    string `json:"chunkText"`
	LocatorJSON  string `json:"locatorJson"`
	QuoteStart   int    `json:"quoteStart"`
	QuoteEnd     int    `json:"quoteEnd"`
}

type ImportMarkdownResult struct {
	Cancelled     bool                  `json:"cancelled"`
	Duplicate     bool                  `json:"duplicate"`
	Document      SourceDocumentSummary `json:"document"`
	ChunkCount    int                   `json:"chunkCount"`
	EvidenceCount int                   `json:"evidenceCount"`
	Warnings      []string              `json:"warnings"`
}

func NewProfileService(
	repository ports.ProfileRepository,
	parser ports.DocumentParser,
	extractor ports.ProfileExtractor,
	files ports.ManagedFileStore,
	picker ports.MarkdownPicker,
	clock ports.Clock,
) *ProfileService {
	return &ProfileService{
		repository: repository,
		parser:     parser,
		extractor:  extractor,
		files:      files,
		picker:     picker,
		clock:      clock,
	}
}

func (service *ProfileService) GetOverview() (ProfileOverview, error) {
	return service.getOverview(context.Background())
}

func (service *ProfileService) ImportMarkdown() (ImportMarkdownResult, error) {
	selected, accepted, err := service.picker.PickMarkdown()
	if err != nil {
		return ImportMarkdownResult{}, err
	}
	if !accepted {
		return ImportMarkdownResult{Cancelled: true}, nil
	}
	return service.importMarkdown(context.Background(), selected)
}

func (service *ProfileService) getOverview(
	ctx context.Context,
) (ProfileOverview, error) {
	profile, err := service.ensureDefaultProfile(ctx)
	if err != nil {
		return ProfileOverview{}, err
	}
	documents, err := service.repository.ListDocuments(ctx, profile.ID)
	if err != nil {
		return ProfileOverview{}, err
	}
	evidence, err := service.repository.ListEvidence(ctx, profile.ID)
	if err != nil {
		return ProfileOverview{}, err
	}

	overview := ProfileOverview{
		ProfileID:       profile.ID,
		Name:            profile.Name,
		DefaultLanguage: profile.DefaultLanguage,
		Documents:       make([]SourceDocumentSummary, 0, len(documents)),
		Evidence:        make([]EvidenceSummary, 0, len(evidence)),
	}
	for _, document := range documents {
		overview.Documents = append(
			overview.Documents,
			sourceDocumentSummary(document),
		)
	}
	for _, item := range evidence {
		overview.Evidence = append(
			overview.Evidence,
			evidenceSummary(item),
		)
	}
	return overview, nil
}

func (service *ProfileService) importMarkdown(
	ctx context.Context,
	selected ports.SelectedMarkdown,
) (ImportMarkdownResult, error) {
	if strings.TrimSpace(selected.OriginalName) == "" {
		return ImportMarkdownResult{}, errors.New("Markdown file name is empty")
	}
	if len(selected.Contents) == 0 {
		return ImportMarkdownResult{}, errors.New("Markdown file is empty")
	}

	profile, err := service.ensureDefaultProfile(ctx)
	if err != nil {
		return ImportMarkdownResult{}, err
	}

	contentHash := hashContents(selected.Contents)
	if existing, found, err := service.repository.FindDocumentByHash(
		ctx,
		profile.ID,
		contentHash,
	); err != nil {
		return ImportMarkdownResult{}, err
	} else if found {
		return ImportMarkdownResult{
			Duplicate: true,
			Document:  sourceDocumentSummary(existing),
			Warnings:  []string{"相同内容已导入，未创建重复资料。"},
		}, nil
	}

	parsed, err := service.parser.Parse(selected.Contents)
	if err != nil {
		return ImportMarkdownResult{}, fmt.Errorf("parse Markdown: %w", err)
	}
	if len(parsed.Chunks) == 0 {
		return ImportMarkdownResult{}, errors.New(
			"Markdown file has no importable content",
		)
	}

	now := service.clock.Now().UTC()
	documentID := uuid.NewString()
	chunks, err := buildSourceChunks(documentID, parsed.Chunks)
	if err != nil {
		return ImportMarkdownResult{}, err
	}
	extracted, err := service.extractor.ExtractProfile(
		ctx,
		ports.ExtractProfileRequest{Chunks: chunks},
	)
	if err != nil {
		return ImportMarkdownResult{}, fmt.Errorf("extract profile evidence: %w", err)
	}
	evidence, sources, err := buildEvidence(profile.ID, chunks, extracted, now)
	if err != nil {
		return ImportMarkdownResult{}, err
	}

	managedPath, err := service.files.SaveMarkdown(
		profile.ID,
		documentID,
		selected.Contents,
	)
	if err != nil {
		return ImportMarkdownResult{}, err
	}

	document := domain.SourceDocument{
		ID:           documentID,
		ProfileID:    profile.ID,
		Kind:         "markdown",
		OriginalName: selected.OriginalName,
		ManagedPath:  managedPath,
		ContentHash:  contentHash,
		ParseStatus:  "succeeded",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := service.repository.SaveImportedDocument(
		ctx,
		ports.ImportedDocument{
			Document:        document,
			Chunks:          chunks,
			Evidence:        evidence,
			EvidenceSources: sources,
		},
	); err != nil {
		if cleanupErr := service.files.Delete(managedPath); cleanupErr != nil {
			slog.Error(
				"profile.import.cleanup.failed",
				slog.String("document_id", documentID),
				slog.Any("error", cleanupErr),
			)
		}
		return ImportMarkdownResult{}, err
	}

	slog.Info(
		"profile.import.succeeded",
		slog.String("profile_id", profile.ID),
		slog.String("document_id", documentID),
		slog.String("content_hash", contentHash),
		slog.Int("chunk_count", len(chunks)),
		slog.Int("evidence_count", len(evidence)),
	)
	return ImportMarkdownResult{
		Document:      sourceDocumentSummary(document),
		ChunkCount:    len(chunks),
		EvidenceCount: len(evidence),
		Warnings:      parsed.Warnings,
	}, nil
}

func (service *ProfileService) ensureDefaultProfile(
	ctx context.Context,
) (domain.Profile, error) {
	return service.repository.EnsureDefaultProfile(
		ctx,
		defaultProfileName,
		defaultProfileLanguage,
		service.clock.Now(),
	)
}

func buildSourceChunks(
	documentID string,
	parsed []ports.ParsedChunk,
) ([]domain.SourceChunk, error) {
	chunks := make([]domain.SourceChunk, 0, len(parsed))
	for _, item := range parsed {
		locator, err := json.Marshal(item.Locator)
		if err != nil {
			return nil, fmt.Errorf("encode source locator: %w", err)
		}
		chunks = append(chunks, domain.SourceChunk{
			ID:          uuid.NewString(),
			DocumentID:  documentID,
			Ordinal:     item.Ordinal,
			Text:        item.Text,
			LocatorJSON: string(locator),
		})
	}
	return chunks, nil
}

func buildEvidence(
	profileID string,
	chunks []domain.SourceChunk,
	extracted []domain.ExtractedEvidence,
	now time.Time,
) ([]domain.Evidence, []domain.EvidenceSource, error) {
	chunksByID := make(map[string]domain.SourceChunk, len(chunks))
	for _, chunk := range chunks {
		chunksByID[chunk.ID] = chunk
	}

	evidence := make([]domain.Evidence, 0, len(extracted))
	sources := make([]domain.EvidenceSource, 0, len(extracted))
	for _, draft := range extracted {
		if err := draft.Validate(); err != nil {
			return nil, nil, fmt.Errorf("validate extracted evidence: %w", err)
		}
		evidenceID := uuid.NewString()
		evidence = append(evidence, domain.Evidence{
			ID:           evidenceID,
			ProfileID:    profileID,
			Kind:         string(draft.Kind),
			Title:        draft.Title,
			Content:      draft.Content,
			Confidence:   draft.Confidence,
			UserVerified: false,
			CreatedAt:    now,
			UpdatedAt:    now,
		})

		seenSources := make(map[string]struct{}, len(draft.SourceChunkIDs))
		for _, chunkID := range draft.SourceChunkIDs {
			if _, seen := seenSources[chunkID]; seen {
				continue
			}
			chunk, exists := chunksByID[chunkID]
			if !exists {
				return nil, nil, fmt.Errorf(
					"extracted evidence references unknown chunk %q",
					chunkID,
				)
			}
			seenSources[chunkID] = struct{}{}
			sources = append(sources, domain.EvidenceSource{
				EvidenceID: evidenceID,
				ChunkID:    chunkID,
				QuoteStart: 0,
				QuoteEnd:   len(chunk.Text),
			})
		}
	}
	return evidence, sources, nil
}

func sourceDocumentSummary(
	document domain.SourceDocument,
) SourceDocumentSummary {
	return SourceDocumentSummary{
		ID:           document.ID,
		OriginalName: document.OriginalName,
		Kind:         document.Kind,
		ParseStatus:  document.ParseStatus,
		ImportedAt:   document.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func evidenceSummary(item domain.Evidence) EvidenceSummary {
	summary := EvidenceSummary{
		ID:         item.ID,
		Kind:       item.Kind,
		Title:      item.Title,
		Content:    item.Content,
		Confidence: item.Confidence,
		Sources:    make([]EvidenceSourceSummary, 0, len(item.Sources)),
	}
	for _, source := range item.Sources {
		summary.Sources = append(summary.Sources, EvidenceSourceSummary{
			ChunkID:      source.ChunkID,
			DocumentID:   source.DocumentID,
			DocumentName: source.DocumentName,
			ChunkText:    source.ChunkText,
			LocatorJSON:  source.LocatorJSON,
			QuoteStart:   source.QuoteStart,
			QuoteEnd:     source.QuoteEnd,
		})
	}
	return summary
}

func hashContents(contents []byte) string {
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}
