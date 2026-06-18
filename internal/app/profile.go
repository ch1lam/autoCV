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
	"unicode"

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
	search     ports.ProfileSearch
	parser     ports.DocumentParser
	docxParser ports.DocumentParser
	extractor  ports.ProfileExtractor
	files      ports.ManagedFileStore
	picker     ports.MarkdownPicker
	docxPicker ports.DOCXPicker
	exporter   ports.ProfileExportPicker
	clock      ports.Clock
}

type ProfileOverview struct {
	ProfileID       string                  `json:"profileId"`
	Name            string                  `json:"name"`
	DefaultLanguage string                  `json:"defaultLanguage"`
	Profiles        []ProfileSummary        `json:"profiles"`
	Documents       []SourceDocumentSummary `json:"documents"`
	Evidence        []EvidenceSummary       `json:"evidence"`
}

type ProfileSummary struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	DefaultLanguage string `json:"defaultLanguage"`
	Active          bool   `json:"active"`
}

type SourceDocumentSummary struct {
	ID           string `json:"id"`
	OriginalName string `json:"originalName"`
	Kind         string `json:"kind"`
	ParseStatus  string `json:"parseStatus"`
	ImportedAt   string `json:"importedAt"`
}

type EvidenceSummary struct {
	ID                  string                  `json:"id"`
	Kind                string                  `json:"kind"`
	Title               string                  `json:"title"`
	Content             string                  `json:"content"`
	Confidence          float64                 `json:"confidence"`
	UserVerified        bool                    `json:"userVerified"`
	UpdatedAt           string                  `json:"updatedAt"`
	ConflictEvidenceIDs []string                `json:"conflictEvidenceIds"`
	Sources             []EvidenceSourceSummary `json:"sources"`
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

type ProfileSearchResult struct {
	EntityType    string `json:"entityType"`
	EntityID      string `json:"entityId"`
	DocumentID    string `json:"documentId"`
	SourceChunkID string `json:"sourceChunkId"`
	DocumentName  string `json:"documentName"`
	Title         string `json:"title"`
	Snippet       string `json:"snippet"`
}

type SaveEvidenceInput struct {
	EvidenceID   string `json:"evidenceId"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	UserVerified bool   `json:"userVerified"`
}

type ImportMarkdownResult struct {
	Cancelled             bool                  `json:"cancelled"`
	Duplicate             bool                  `json:"duplicate"`
	Document              SourceDocumentSummary `json:"document"`
	ChunkCount            int                   `json:"chunkCount"`
	EvidenceCount         int                   `json:"evidenceCount"`
	MergedEvidenceCount   int                   `json:"mergedEvidenceCount"`
	ConflictEvidenceCount int                   `json:"conflictEvidenceCount"`
	Warnings              []string              `json:"warnings"`
}

type profileExportPayload struct {
	SchemaVersion   int                     `json:"schemaVersion"`
	ExportedAt      string                  `json:"exportedAt"`
	Profile         ProfileSummary          `json:"profile"`
	SourceDocuments []profileExportDocument `json:"sourceDocuments"`
	Evidence        []profileExportEvidence `json:"evidence"`
}

type profileExportDocument struct {
	ID           string `json:"id"`
	OriginalName string `json:"originalName"`
	Kind         string `json:"kind"`
	ContentHash  string `json:"contentHash"`
	ParseStatus  string `json:"parseStatus"`
	ImportedAt   string `json:"importedAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type profileExportEvidence struct {
	ID                  string                        `json:"id"`
	Kind                string                        `json:"kind"`
	Title               string                        `json:"title"`
	Content             string                        `json:"content"`
	Confidence          float64                       `json:"confidence"`
	UserVerified        bool                          `json:"userVerified"`
	UpdatedAt           string                        `json:"updatedAt"`
	ConflictEvidenceIDs []string                      `json:"conflictEvidenceIds"`
	Sources             []profileExportEvidenceSource `json:"sources"`
}

type profileExportEvidenceSource struct {
	ChunkID      string `json:"chunkId"`
	DocumentID   string `json:"documentId"`
	DocumentName string `json:"documentName"`
	LocatorJSON  string `json:"locatorJson"`
	QuoteStart   int    `json:"quoteStart"`
	QuoteEnd     int    `json:"quoteEnd"`
}

func NewProfileService(
	repository ports.ProfileRepository,
	search ports.ProfileSearch,
	parser ports.DocumentParser,
	docxParser ports.DocumentParser,
	extractor ports.ProfileExtractor,
	files ports.ManagedFileStore,
	picker ports.MarkdownPicker,
	docxPicker ports.DOCXPicker,
	exporter ports.ProfileExportPicker,
	clock ports.Clock,
) *ProfileService {
	return &ProfileService{
		repository: repository,
		search:     search,
		parser:     parser,
		docxParser: docxParser,
		extractor:  extractor,
		files:      files,
		picker:     picker,
		docxPicker: docxPicker,
		exporter:   exporter,
		clock:      clock,
	}
}

func (service *ProfileService) GetOverview() (ProfileOverview, error) {
	return service.getOverview(context.Background())
}

func (service *ProfileService) CreateProfile(
	name string,
	defaultLanguage string,
) (ProfileOverview, error) {
	return service.createProfile(
		context.Background(),
		name,
		defaultLanguage,
	)
}

func (service *ProfileService) SelectProfile(
	profileID string,
) (ProfileOverview, error) {
	return service.selectProfile(context.Background(), profileID)
}

func (service *ProfileService) Search(
	query string,
) ([]ProfileSearchResult, error) {
	return service.searchProfile(context.Background(), query)
}

func (service *ProfileService) SaveEvidence(
	input SaveEvidenceInput,
) (ProfileOverview, error) {
	return service.saveEvidence(context.Background(), input)
}

func (service *ProfileService) ResolveEvidenceConflict(
	evidenceID string,
) (ProfileOverview, error) {
	return service.resolveEvidenceConflict(
		context.Background(),
		evidenceID,
	)
}

func (service *ProfileService) ImportMarkdown() (ImportMarkdownResult, error) {
	selected, accepted, err := service.picker.PickMarkdown()
	if err != nil {
		return ImportMarkdownResult{}, err
	}
	if !accepted {
		return ImportMarkdownResult{Cancelled: true}, nil
	}
	return service.importDocument(
		context.Background(),
		profileImportDocument{
			kind:         "markdown",
			label:        "Markdown",
			originalName: selected.OriginalName,
			contents:     selected.Contents,
			parser:       service.parser,
			save:         service.files.SaveMarkdown,
		},
	)
}

func (service *ProfileService) ImportDOCX() (ImportMarkdownResult, error) {
	selected, accepted, err := service.docxPicker.PickDOCX()
	if err != nil {
		return ImportMarkdownResult{}, err
	}
	if !accepted {
		return ImportMarkdownResult{Cancelled: true}, nil
	}
	return service.importDocument(
		context.Background(),
		profileImportDocument{
			kind:         "docx",
			label:        "DOCX",
			originalName: selected.OriginalName,
			contents:     selected.Contents,
			parser:       service.docxParser,
			save:         service.files.SaveDOCX,
		},
	)
}

func (service *ProfileService) ExportProfile() (ExportResult, error) {
	ctx := context.Background()
	profile, err := resolveActiveProfile(
		ctx,
		service.repository,
		service.clock.Now(),
	)
	if err != nil {
		return ExportResult{}, err
	}
	documents, err := service.repository.ListDocuments(ctx, profile.ID)
	if err != nil {
		return ExportResult{}, err
	}
	evidence, err := service.repository.ListEvidence(ctx, profile.ID)
	if err != nil {
		return ExportResult{}, err
	}
	payload := profileExportFrom(
		profile,
		documents,
		evidence,
		service.clock.Now(),
	)
	contents, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return ExportResult{}, fmt.Errorf("encode profile export: %w", err)
	}
	destination, selected, err := service.exporter.PickProfileJSON(
		profileExportFilename(profile),
	)
	if err != nil {
		return ExportResult{}, err
	}
	if !selected {
		return ExportResult{Cancelled: true, Kind: "profile"}, nil
	}
	destination = ensureExtension(destination, ".json")
	if err := service.files.ExportContents(contents, destination); err != nil {
		return ExportResult{}, err
	}
	return ExportResult{Kind: "profile", Path: destination}, nil
}

func (service *ProfileService) resolveEvidenceConflict(
	ctx context.Context,
	evidenceID string,
) (ProfileOverview, error) {
	evidenceID = strings.TrimSpace(evidenceID)
	if evidenceID == "" {
		return ProfileOverview{}, errors.New("Evidence id is empty")
	}

	profile, err := resolveActiveProfile(
		ctx,
		service.repository,
		service.clock.Now(),
	)
	if err != nil {
		return ProfileOverview{}, err
	}
	evidence, err := service.repository.ListEvidence(ctx, profile.ID)
	if err != nil {
		return ProfileOverview{}, err
	}
	var selected bool
	for _, item := range evidence {
		if item.ID == evidenceID {
			selected = true
			break
		}
	}
	if !selected {
		return ProfileOverview{}, fmt.Errorf(
			"evidence %q not found in active profile",
			evidenceID,
		)
	}

	conflictIDs := analyzeEvidenceRelations(evidence).conflictIDs[evidenceID]
	if len(conflictIDs) == 0 {
		return ProfileOverview{}, fmt.Errorf(
			"evidence %q has no conflicting versions",
			evidenceID,
		)
	}
	if err := service.repository.ResolveEvidenceConflict(
		ctx,
		profile.ID,
		evidenceID,
		conflictIDs,
		service.clock.Now().UTC(),
	); err != nil {
		return ProfileOverview{}, err
	}
	slog.Info(
		"evidence.conflict.resolved",
		slog.String("profile_id", profile.ID),
		slog.String("evidence_id", evidenceID),
		slog.Int("excluded_count", len(conflictIDs)),
	)
	return service.getOverview(ctx)
}

func (service *ProfileService) saveEvidence(
	ctx context.Context,
	input SaveEvidenceInput,
) (ProfileOverview, error) {
	input.EvidenceID = strings.TrimSpace(input.EvidenceID)
	input.Title = strings.TrimSpace(input.Title)
	input.Content = strings.TrimSpace(input.Content)
	if input.EvidenceID == "" {
		return ProfileOverview{}, errors.New("Evidence id is empty")
	}
	if input.Title == "" {
		return ProfileOverview{}, errors.New("Evidence title is empty")
	}
	if len([]rune(input.Title)) > 240 {
		return ProfileOverview{}, errors.New(
			"Evidence title must be 240 characters or fewer",
		)
	}
	if input.Content == "" {
		return ProfileOverview{}, errors.New("Evidence content is empty")
	}
	if len([]rune(input.Content)) > 8000 {
		return ProfileOverview{}, errors.New(
			"Evidence content must be 8000 characters or fewer",
		)
	}

	profile, err := resolveActiveProfile(
		ctx,
		service.repository,
		service.clock.Now(),
	)
	if err != nil {
		return ProfileOverview{}, err
	}
	if err := service.repository.UpdateEvidence(
		ctx,
		profile.ID,
		input.EvidenceID,
		input.Title,
		input.Content,
		input.UserVerified,
		service.clock.Now().UTC(),
	); err != nil {
		return ProfileOverview{}, err
	}
	slog.Info(
		"evidence.updated",
		slog.String("profile_id", profile.ID),
		slog.String("evidence_id", input.EvidenceID),
		slog.Bool("user_verified", input.UserVerified),
	)
	return service.getOverview(ctx)
}

func (service *ProfileService) searchProfile(
	ctx context.Context,
	query string,
) ([]ProfileSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []ProfileSearchResult{}, nil
	}
	if len([]rune(query)) > 200 {
		return nil, errors.New("Search query must be 200 characters or fewer")
	}

	profile, err := resolveActiveProfile(
		ctx,
		service.repository,
		service.clock.Now(),
	)
	if err != nil {
		return nil, err
	}
	results, err := service.search.Search(ctx, profile.ID, query, 20)
	if err != nil {
		return nil, err
	}

	summaries := make([]ProfileSearchResult, 0, len(results))
	for _, result := range results {
		summaries = append(summaries, ProfileSearchResult{
			EntityType:    result.EntityType,
			EntityID:      result.EntityID,
			DocumentID:    result.DocumentID,
			SourceChunkID: result.SourceChunkID,
			DocumentName:  result.DocumentName,
			Title:         result.Title,
			Snippet:       result.Snippet,
		})
	}
	return summaries, nil
}

func (service *ProfileService) getOverview(
	ctx context.Context,
) (ProfileOverview, error) {
	profile, err := resolveActiveProfile(
		ctx,
		service.repository,
		service.clock.Now(),
	)
	if err != nil {
		return ProfileOverview{}, err
	}
	profiles, err := service.repository.ListProfiles(ctx)
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
		Profiles:        make([]ProfileSummary, 0, len(profiles)),
		Documents:       make([]SourceDocumentSummary, 0, len(documents)),
		Evidence:        evidenceSummaries(evidence),
	}
	for _, item := range profiles {
		overview.Profiles = append(
			overview.Profiles,
			profileSummary(item),
		)
	}
	for _, document := range documents {
		overview.Documents = append(
			overview.Documents,
			sourceDocumentSummary(document),
		)
	}
	return overview, nil
}

func (service *ProfileService) createProfile(
	ctx context.Context,
	name string,
	defaultLanguage string,
) (ProfileOverview, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ProfileOverview{}, errors.New("Profile name is empty")
	}
	if len([]rune(name)) > 80 {
		return ProfileOverview{}, errors.New(
			"Profile name must be 80 characters or fewer",
		)
	}
	defaultLanguage = strings.TrimSpace(defaultLanguage)
	if defaultLanguage == "" {
		defaultLanguage = defaultProfileLanguage
	}

	now := service.clock.Now().UTC()
	profile := domain.Profile{
		ID:              uuid.NewString(),
		Name:            name,
		DefaultLanguage: defaultLanguage,
		Active:          true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := service.repository.CreateProfile(ctx, profile); err != nil {
		return ProfileOverview{}, err
	}
	slog.Info(
		"profile.created",
		slog.String("profile_id", profile.ID),
		slog.String("default_language", profile.DefaultLanguage),
	)
	return service.getOverview(ctx)
}

func (service *ProfileService) selectProfile(
	ctx context.Context,
	profileID string,
) (ProfileOverview, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return ProfileOverview{}, errors.New("Profile id is empty")
	}
	profile, err := service.repository.SetActiveProfile(ctx, profileID)
	if err != nil {
		return ProfileOverview{}, err
	}
	slog.Info(
		"profile.selected",
		slog.String("profile_id", profile.ID),
	)
	return service.getOverview(ctx)
}

type profileImportDocument struct {
	kind         string
	label        string
	originalName string
	contents     []byte
	parser       ports.DocumentParser
	save         func(string, string, []byte) (string, error)
}

func (service *ProfileService) importDocument(
	ctx context.Context,
	selected profileImportDocument,
) (ImportMarkdownResult, error) {
	if strings.TrimSpace(selected.originalName) == "" {
		return ImportMarkdownResult{}, fmt.Errorf("%s file name is empty", selected.label)
	}
	if len(selected.contents) == 0 {
		return ImportMarkdownResult{}, fmt.Errorf("%s file is empty", selected.label)
	}
	if selected.parser == nil {
		return ImportMarkdownResult{}, fmt.Errorf("%s parser is not configured", selected.label)
	}
	if selected.save == nil {
		return ImportMarkdownResult{}, fmt.Errorf("%s file store is not configured", selected.label)
	}

	profile, err := resolveActiveProfile(
		ctx,
		service.repository,
		service.clock.Now(),
	)
	if err != nil {
		return ImportMarkdownResult{}, err
	}

	contentHash := hashContents(selected.contents)
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

	parsed, err := selected.parser.Parse(selected.contents)
	if err != nil {
		return ImportMarkdownResult{}, fmt.Errorf("parse %s: %w", selected.label, err)
	}
	if len(parsed.Chunks) == 0 {
		return ImportMarkdownResult{}, fmt.Errorf(
			"%s file has no importable content",
			selected.label,
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
	existingEvidence, err := service.repository.ListEvidence(ctx, profile.ID)
	if err != nil {
		return ImportMarkdownResult{}, err
	}
	evidence, sources, mergedEvidenceCount, conflictEvidenceCount :=
		reconcileImportedEvidence(
			existingEvidence,
			evidence,
			sources,
		)

	managedPath, err := selected.save(
		profile.ID,
		documentID,
		selected.contents,
	)
	if err != nil {
		return ImportMarkdownResult{}, err
	}

	document := domain.SourceDocument{
		ID:           documentID,
		ProfileID:    profile.ID,
		Kind:         selected.kind,
		OriginalName: selected.originalName,
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
		slog.String("kind", selected.kind),
		slog.String("content_hash", contentHash),
		slog.Int("chunk_count", len(chunks)),
		slog.Int("evidence_count", len(evidence)),
	)
	warnings := append([]string(nil), parsed.Warnings...)
	if mergedEvidenceCount > 0 {
		warnings = append(
			warnings,
			fmt.Sprintf(
				"已合并 %d 条重复 Evidence，并保留新增来源。",
				mergedEvidenceCount,
			),
		)
	}
	if conflictEvidenceCount > 0 {
		warnings = append(
			warnings,
			fmt.Sprintf(
				"发现 %d 条冲突 Evidence，请在资料库中确认采用版本。",
				conflictEvidenceCount,
			),
		)
	}
	return ImportMarkdownResult{
		Document:              sourceDocumentSummary(document),
		ChunkCount:            len(chunks),
		EvidenceCount:         len(evidence),
		MergedEvidenceCount:   mergedEvidenceCount,
		ConflictEvidenceCount: conflictEvidenceCount,
		Warnings:              warnings,
	}, nil
}

func resolveActiveProfile(
	ctx context.Context,
	repository ports.ProfileRepository,
	now time.Time,
) (domain.Profile, error) {
	defaultProfile, err := repository.EnsureDefaultProfile(
		ctx,
		defaultProfileName,
		defaultProfileLanguage,
		now,
	)
	if err != nil {
		return domain.Profile{}, err
	}
	active, found, err := repository.GetActiveProfile(ctx)
	if err != nil {
		return domain.Profile{}, err
	}
	if found {
		return active, nil
	}
	return repository.SetActiveProfile(ctx, defaultProfile.ID)
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

func profileSummary(profile domain.Profile) ProfileSummary {
	return ProfileSummary{
		ID:              profile.ID,
		Name:            profile.Name,
		DefaultLanguage: profile.DefaultLanguage,
		Active:          profile.Active,
	}
}

func profileExportFrom(
	profile domain.Profile,
	documents []domain.SourceDocument,
	evidence []domain.Evidence,
	now time.Time,
) profileExportPayload {
	relations := analyzeEvidenceRelations(evidence)
	payload := profileExportPayload{
		SchemaVersion:   1,
		ExportedAt:      now.UTC().Format(time.RFC3339),
		Profile:         profileSummary(profile),
		SourceDocuments: make([]profileExportDocument, 0, len(documents)),
		Evidence:        make([]profileExportEvidence, 0, len(evidence)),
	}
	for _, document := range documents {
		payload.SourceDocuments = append(
			payload.SourceDocuments,
			profileExportDocument{
				ID:           document.ID,
				OriginalName: document.OriginalName,
				Kind:         document.Kind,
				ContentHash:  document.ContentHash,
				ParseStatus:  document.ParseStatus,
				ImportedAt:   document.CreatedAt.UTC().Format(time.RFC3339),
				UpdatedAt:    document.UpdatedAt.UTC().Format(time.RFC3339),
			},
		)
	}
	for _, item := range evidence {
		exported := profileExportEvidence{
			ID:                  item.ID,
			Kind:                item.Kind,
			Title:               item.Title,
			Content:             item.Content,
			Confidence:          item.Confidence,
			UserVerified:        item.UserVerified,
			UpdatedAt:           item.UpdatedAt.UTC().Format(time.RFC3339),
			ConflictEvidenceIDs: append([]string(nil), relations.conflictIDs[item.ID]...),
			Sources:             make([]profileExportEvidenceSource, 0, len(item.Sources)),
		}
		for _, source := range item.Sources {
			exported.Sources = append(
				exported.Sources,
				profileExportEvidenceSource{
					ChunkID:      source.ChunkID,
					DocumentID:   source.DocumentID,
					DocumentName: source.DocumentName,
					LocatorJSON:  source.LocatorJSON,
					QuoteStart:   source.QuoteStart,
					QuoteEnd:     source.QuoteEnd,
				},
			)
		}
		payload.Evidence = append(payload.Evidence, exported)
	}
	return payload
}

func profileExportFilename(profile domain.Profile) string {
	return safeFilenameBase(profile.Name, "profile") + "-profile.json"
}

func safeFilenameBase(value string, fallback string) string {
	var builder strings.Builder
	for _, character := range strings.TrimSpace(value) {
		switch {
		case unicode.IsLetter(character), unicode.IsNumber(character):
			builder.WriteRune(character)
		case unicode.IsSpace(character), character == '-', character == '_':
			if builder.Len() > 0 {
				builder.WriteRune('-')
			}
		}
		if builder.Len() >= 48 {
			break
		}
	}
	base := strings.Trim(builder.String(), "-")
	if base == "" {
		return fallback
	}
	return base
}

func evidenceSummary(item domain.Evidence) EvidenceSummary {
	summary := EvidenceSummary{
		ID:                  item.ID,
		Kind:                item.Kind,
		Title:               item.Title,
		Content:             item.Content,
		Confidence:          item.Confidence,
		UserVerified:        item.UserVerified,
		UpdatedAt:           item.UpdatedAt.UTC().Format(time.RFC3339),
		ConflictEvidenceIDs: []string{},
		Sources:             make([]EvidenceSourceSummary, 0, len(item.Sources)),
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

func evidenceSummaries(items []domain.Evidence) []EvidenceSummary {
	relations := analyzeEvidenceRelations(items)
	summaries := make([]EvidenceSummary, 0, len(items))
	for _, item := range items {
		summary := evidenceSummary(item)
		summary.ConflictEvidenceIDs = append(
			summary.ConflictEvidenceIDs,
			relations.conflictIDs[item.ID]...,
		)
		summaries = append(summaries, summary)
	}
	return summaries
}

func hashContents(contents []byte) string {
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}
