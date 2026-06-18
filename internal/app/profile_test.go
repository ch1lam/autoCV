package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	docxparser "github.com/ch1lam/autocv/internal/adapters/docx"
	"github.com/ch1lam/autocv/internal/adapters/fakeprovider"
	"github.com/ch1lam/autocv/internal/adapters/filesystem"
	markdownparser "github.com/ch1lam/autocv/internal/adapters/markdown"
	sqliteadapter "github.com/ch1lam/autocv/internal/adapters/sqlite"
	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

var profileTestTime = time.Date(2026, 6, 11, 1, 0, 0, 0, time.UTC)

type fixedClock struct {
	now time.Time
}

func (clock fixedClock) Now() time.Time {
	return clock.now
}

type fakeMarkdownPicker struct {
	selected ports.SelectedMarkdown
	accepted bool
	err      error
}

func (picker fakeMarkdownPicker) PickMarkdown() (
	ports.SelectedMarkdown,
	bool,
	error,
) {
	return picker.selected, picker.accepted, picker.err
}

type fakeDOCXPicker struct {
	selected ports.SelectedDOCX
	accepted bool
	err      error
}

func (picker fakeDOCXPicker) PickDOCX() (
	ports.SelectedDOCX,
	bool,
	error,
) {
	return picker.selected, picker.accepted, picker.err
}

type fixedProfileExportPicker struct {
	path     string
	accepted bool
	err      error
}

func (picker fixedProfileExportPicker) PickProfileJSON(string) (
	string,
	bool,
	error,
) {
	if picker.err != nil {
		return "", false, picker.err
	}
	if !picker.accepted {
		return "", false, nil
	}
	return picker.path, true, nil
}

type failingSaveRepository struct {
	ports.ProfileRepository
}

type sequencedProfileExtractor struct {
	contents []string
	call     int
}

func (extractor *sequencedProfileExtractor) ExtractProfile(
	_ context.Context,
	request ports.ExtractProfileRequest,
) ([]domain.ExtractedEvidence, error) {
	content := extractor.contents[extractor.call]
	extractor.call++
	return []domain.ExtractedEvidence{{
		Kind:           domain.EvidenceKindExperience,
		Title:          "支付平台职责",
		Content:        content,
		SourceChunkIDs: []string{request.Chunks[0].ID},
		Confidence:     0.8,
	}}, nil
}

func (repository failingSaveRepository) SaveImportedDocument(
	context.Context,
	ports.ImportedDocument,
) error {
	return errors.New("save failed")
}

func TestProfileServiceImportsAndRestoresMarkdownProfile(t *testing.T) {
	service, repository, files, contents, _ := newProfileServiceTest(t)

	result, err := service.ImportMarkdown()
	if err != nil {
		t.Fatalf("import Markdown: %v", err)
	}
	if result.Cancelled || result.Duplicate {
		t.Fatalf("expected a new import, got %#v", result)
	}
	if result.ChunkCount == 0 || result.EvidenceCount == 0 {
		t.Fatalf("expected chunks and evidence, got %#v", result)
	}

	overview, err := service.GetOverview()
	if err != nil {
		t.Fatalf("get overview: %v", err)
	}
	if overview.Name != defaultProfileName {
		t.Fatalf("unexpected profile name %q", overview.Name)
	}
	if len(overview.Documents) != 1 {
		t.Fatalf("expected one document, got %d", len(overview.Documents))
	}
	if len(overview.Evidence) != result.EvidenceCount {
		t.Fatalf("expected %d evidence items, got %d", result.EvidenceCount, len(overview.Evidence))
	}
	if len(overview.Evidence[0].Sources) == 0 {
		t.Fatal("expected evidence source navigation")
	}

	documents, err := repository.ListDocuments(context.Background(), overview.ProfileID)
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}
	managedContents, err := files.Read(documents[0].ManagedPath)
	if err != nil {
		t.Fatalf("read managed Markdown: %v", err)
	}
	if string(managedContents) != string(contents) {
		t.Fatal("managed Markdown differs from selected file")
	}

	restarted := NewProfileService(
		repository,
		service.search,
		markdownparser.New(),
		docxparser.New(),
		fakeprovider.New(),
		files,
		fakeMarkdownPicker{},
		fakeDOCXPicker{},
		fixedProfileExportPicker{},
		fixedClock{now: profileTestTime.Add(time.Hour)},
	)
	restored, err := restarted.GetOverview()
	if err != nil {
		t.Fatalf("restore overview: %v", err)
	}
	if len(restored.Documents) != 1 ||
		len(restored.Evidence) != result.EvidenceCount {
		t.Fatalf("expected persisted profile data, got %#v", restored)
	}
}

func TestProfileServiceImportsDOCXProfile(t *testing.T) {
	service, repository, files, _, _ := newProfileServiceTest(t)
	contents := profileDOCXFixture(t)
	service.docxPicker = fakeDOCXPicker{
		selected: ports.SelectedDOCX{
			OriginalName: "backend-profile.docx",
			Contents:     contents,
		},
		accepted: true,
	}

	result, err := service.ImportDOCX()
	if err != nil {
		t.Fatalf("import DOCX: %v", err)
	}
	if result.Cancelled || result.Duplicate {
		t.Fatalf("expected a new DOCX import, got %#v", result)
	}
	if result.Document.Kind != "docx" ||
		result.Document.OriginalName != "backend-profile.docx" {
		t.Fatalf("unexpected DOCX document summary %#v", result.Document)
	}
	if result.ChunkCount == 0 || result.EvidenceCount == 0 {
		t.Fatalf("expected DOCX chunks and evidence, got %#v", result)
	}

	overview, err := service.GetOverview()
	if err != nil {
		t.Fatalf("get overview: %v", err)
	}
	if len(overview.Documents) != 1 ||
		overview.Documents[0].Kind != "docx" {
		t.Fatalf("expected DOCX document in overview, got %#v", overview.Documents)
	}
	if len(overview.Evidence) == 0 ||
		len(overview.Evidence[0].Sources) == 0 ||
		overview.Evidence[0].Sources[0].DocumentName != "backend-profile.docx" {
		t.Fatalf("expected DOCX source evidence, got %#v", overview.Evidence)
	}

	documents, err := repository.ListDocuments(context.Background(), overview.ProfileID)
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}
	managedContents, err := files.Read(documents[0].ManagedPath)
	if err != nil {
		t.Fatalf("read managed DOCX: %v", err)
	}
	if !bytes.Equal(managedContents, contents) {
		t.Fatal("managed DOCX differs from selected file")
	}
}

func TestProfileServiceExportsStructuredProfileJSON(t *testing.T) {
	service, _, _, _, root := newProfileServiceTest(t)

	if _, err := service.ImportMarkdown(); err != nil {
		t.Fatalf("import Markdown: %v", err)
	}
	exportPath := filepath.Join(root, "exports", "profile-export")
	service.exporter = fixedProfileExportPicker{
		path:     exportPath,
		accepted: true,
	}

	result, err := service.ExportProfile()
	if err != nil {
		t.Fatalf("export profile: %v", err)
	}
	if result.Cancelled || result.Kind != "profile" {
		t.Fatalf("unexpected export result %#v", result)
	}
	if filepath.Ext(result.Path) != ".json" {
		t.Fatalf("expected json extension, got %q", result.Path)
	}
	contents, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("read exported profile: %v", err)
	}
	if strings.Contains(string(contents), "chunkText") {
		t.Fatal("export must not include raw chunk text")
	}

	var payload profileExportPayload
	if err := json.Unmarshal(contents, &payload); err != nil {
		t.Fatalf("decode exported profile: %v", err)
	}
	if payload.SchemaVersion != 1 || payload.Profile.Name != defaultProfileName {
		t.Fatalf("unexpected payload header %#v", payload)
	}
	if len(payload.SourceDocuments) != 1 ||
		payload.SourceDocuments[0].OriginalName != "backend-profile.md" ||
		payload.SourceDocuments[0].ContentHash == "" {
		t.Fatalf("unexpected source document list %#v", payload.SourceDocuments)
	}
	if len(payload.Evidence) == 0 || len(payload.Evidence[0].Sources) == 0 {
		t.Fatalf("expected exported evidence with source locator, got %#v", payload.Evidence)
	}
}

func TestProfileServiceDetectsDuplicateMarkdown(t *testing.T) {
	service, _, _, _, _ := newProfileServiceTest(t)

	if _, err := service.ImportMarkdown(); err != nil {
		t.Fatalf("import first Markdown: %v", err)
	}
	duplicate, err := service.ImportMarkdown()
	if err != nil {
		t.Fatalf("import duplicate Markdown: %v", err)
	}
	if !duplicate.Duplicate {
		t.Fatal("expected duplicate import result")
	}

	overview, err := service.GetOverview()
	if err != nil {
		t.Fatalf("get overview: %v", err)
	}
	if len(overview.Documents) != 1 {
		t.Fatalf("expected one stored document, got %d", len(overview.Documents))
	}
}

func TestProfileServiceImportsMultipleDocumentsIntoOneProfile(t *testing.T) {
	service, _, _, _, _ := newProfileServiceTest(t)

	first, err := service.ImportMarkdown()
	if err != nil {
		t.Fatalf("import first Markdown: %v", err)
	}
	service.picker = fakeMarkdownPicker{
		selected: ports.SelectedMarkdown{
			OriginalName: "project-notes.md",
			Contents: []byte(
				"# 项目记录\n\n使用 PostgreSQL 和 SQLite 保存任务状态。\n",
			),
		},
		accepted: true,
	}
	second, err := service.ImportMarkdown()
	if err != nil {
		t.Fatalf("import second Markdown: %v", err)
	}
	if second.Duplicate || second.Document.ID == first.Document.ID {
		t.Fatalf("expected a distinct second document, got %#v", second)
	}

	overview, err := service.GetOverview()
	if err != nil {
		t.Fatalf("get multi-document overview: %v", err)
	}
	if len(overview.Documents) != 2 {
		t.Fatalf("expected two documents, got %#v", overview.Documents)
	}
	documentNames := map[string]bool{}
	for _, item := range overview.Evidence {
		for _, source := range item.Sources {
			documentNames[source.DocumentName] = true
		}
	}
	if !documentNames["backend-profile.md"] ||
		!documentNames["project-notes.md"] {
		t.Fatalf(
			"expected evidence from both documents, got %#v",
			documentNames,
		)
	}
}

func TestProfileServiceMergesDuplicateEvidenceAcrossDocuments(t *testing.T) {
	service, _, _, _, _ := newProfileServiceTest(t)
	service.extractor = &sequencedProfileExtractor{
		contents: []string{
			"负责支付平台接口开发。",
			"  负责支付平台接口开发。 ",
		},
	}

	if _, err := service.ImportMarkdown(); err != nil {
		t.Fatalf("import first Markdown: %v", err)
	}
	service.picker = fakeMarkdownPicker{
		selected: ports.SelectedMarkdown{
			OriginalName: "payment-notes.md",
			Contents:     []byte("# 项目补充\n\n支付平台采用 Go 开发。\n"),
		},
		accepted: true,
	}
	result, err := service.ImportMarkdown()
	if err != nil {
		t.Fatalf("import duplicate evidence: %v", err)
	}
	if result.EvidenceCount != 0 ||
		result.MergedEvidenceCount != 1 ||
		result.ConflictEvidenceCount != 0 {
		t.Fatalf("unexpected duplicate merge result %#v", result)
	}

	overview, err := service.GetOverview()
	if err != nil {
		t.Fatalf("get overview: %v", err)
	}
	if len(overview.Evidence) != 1 ||
		len(overview.Evidence[0].Sources) != 2 {
		t.Fatalf("expected one evidence with two sources, got %#v", overview.Evidence)
	}
}

func TestProfileServiceKeepsConfirmedEvidenceWhenNewSourceConflicts(t *testing.T) {
	service, _, _, _, _ := newProfileServiceTest(t)
	service.extractor = &sequencedProfileExtractor{
		contents: []string{
			"负责支付平台接口开发。",
			"负责支付平台架构设计。",
		},
	}

	if _, err := service.ImportMarkdown(); err != nil {
		t.Fatalf("import first Markdown: %v", err)
	}
	overview, err := service.GetOverview()
	if err != nil {
		t.Fatalf("get overview: %v", err)
	}
	confirmedID := overview.Evidence[0].ID
	confirmedContent := "负责支付平台接口交付并改善稳定性。"
	if _, err := service.SaveEvidence(SaveEvidenceInput{
		EvidenceID:   confirmedID,
		Title:        "支付平台职责",
		Content:      confirmedContent,
		UserVerified: true,
	}); err != nil {
		t.Fatalf("confirm evidence: %v", err)
	}

	service.picker = fakeMarkdownPicker{
		selected: ports.SelectedMarkdown{
			OriginalName: "architecture-notes.md",
			Contents:     []byte("# 项目补充\n\n参与支付平台架构工作。\n"),
		},
		accepted: true,
	}
	result, err := service.ImportMarkdown()
	if err != nil {
		t.Fatalf("import conflicting evidence: %v", err)
	}
	if result.EvidenceCount != 1 ||
		result.MergedEvidenceCount != 0 ||
		result.ConflictEvidenceCount != 1 {
		t.Fatalf("unexpected conflict result %#v", result)
	}

	overview, err = service.GetOverview()
	if err != nil {
		t.Fatalf("get conflicted overview: %v", err)
	}
	confirmedEvidence := findEvidenceSummary(t, overview.Evidence, confirmedID)
	if len(overview.Evidence) != 2 ||
		confirmedEvidence.Content != confirmedContent ||
		!confirmedEvidence.UserVerified ||
		len(confirmedEvidence.ConflictEvidenceIDs) != 1 {
		t.Fatalf("expected confirmed evidence to remain intact, got %#v", overview.Evidence)
	}
	conflictingID := confirmedEvidence.ConflictEvidenceIDs[0]
	conflictingEvidence := findEvidenceSummary(t, overview.Evidence, conflictingID)
	if len(conflictingEvidence.ConflictEvidenceIDs) != 1 {
		t.Fatalf("expected reciprocal conflict, got %#v", conflictingEvidence)
	}

	service.clock = fixedClock{now: profileTestTime.Add(2 * time.Hour)}
	resolved, err := service.ResolveEvidenceConflict(conflictingID)
	if err != nil {
		t.Fatalf("resolve evidence conflict: %v", err)
	}
	resolvedConfirmed := findEvidenceSummary(t, resolved.Evidence, confirmedID)
	resolvedConflict := findEvidenceSummary(t, resolved.Evidence, conflictingID)
	if resolvedConfirmed.UserVerified ||
		!resolvedConflict.UserVerified ||
		resolvedConflict.UpdatedAt !=
			profileTestTime.Add(2*time.Hour).Format(time.RFC3339) {
		t.Fatalf("expected selected conflict version only, got %#v", resolved.Evidence)
	}

	switched, err := service.ResolveEvidenceConflict(confirmedID)
	if err != nil {
		t.Fatalf("switch evidence conflict version: %v", err)
	}
	switchedConfirmed := findEvidenceSummary(t, switched.Evidence, confirmedID)
	switchedConflict := findEvidenceSummary(t, switched.Evidence, conflictingID)
	if !switchedConfirmed.UserVerified ||
		switchedConflict.UserVerified {
		t.Fatalf("expected conflict selection to switch, got %#v", switched.Evidence)
	}
}

func TestProfileServiceRejectsInvalidEvidenceConflictResolution(t *testing.T) {
	service, _, _, _, _ := newProfileServiceTest(t)
	if _, err := service.ResolveEvidenceConflict(" "); err == nil {
		t.Fatal("expected blank evidence id error")
	}
	if _, err := service.ImportMarkdown(); err != nil {
		t.Fatalf("import Markdown: %v", err)
	}
	overview, err := service.GetOverview()
	if err != nil {
		t.Fatalf("get overview: %v", err)
	}
	if _, err := service.ResolveEvidenceConflict(overview.Evidence[0].ID); err == nil {
		t.Fatal("expected non-conflicting evidence error")
	}
	if _, err := service.ResolveEvidenceConflict("missing-evidence"); err == nil {
		t.Fatal("expected missing evidence error")
	}
}

func TestProfileServiceSearchesActiveProfileContent(t *testing.T) {
	service, _, _, _, _ := newProfileServiceTest(t)
	if _, err := service.ImportMarkdown(); err != nil {
		t.Fatalf("import Markdown: %v", err)
	}

	results, err := service.Search("Postgre")
	if err != nil {
		t.Fatalf("search profile: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	for _, result := range results {
		if result.DocumentName != "backend-profile.md" ||
			result.SourceChunkID == "" ||
			result.Snippet == "" {
			t.Fatalf("unexpected search result %#v", result)
		}
	}

	other, err := service.CreateProfile("其他岗位", "zh-CN")
	if err != nil {
		t.Fatalf("create second profile: %v", err)
	}
	if other.ProfileID == "" {
		t.Fatal("expected created profile id")
	}
	isolated, err := service.Search("Postgre")
	if err != nil {
		t.Fatalf("search empty profile: %v", err)
	}
	if len(isolated) != 0 {
		t.Fatalf("expected active profile isolation, got %#v", isolated)
	}
}

func TestProfileServiceValidatesSearchQuery(t *testing.T) {
	service, _, _, _, _ := newProfileServiceTest(t)

	empty, err := service.Search("  ")
	if err != nil {
		t.Fatalf("search empty query: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected no empty query results, got %#v", empty)
	}
	if _, err := service.Search(string(make([]rune, 201))); err == nil {
		t.Fatal("expected long search query error")
	}
}

func TestProfileServiceEditsAndConfirmsEvidence(t *testing.T) {
	service, _, _, _, _ := newProfileServiceTest(t)
	if _, err := service.ImportMarkdown(); err != nil {
		t.Fatalf("import Markdown: %v", err)
	}
	overview, err := service.GetOverview()
	if err != nil {
		t.Fatalf("get overview: %v", err)
	}
	if len(overview.Evidence) == 0 {
		t.Fatal("expected imported evidence")
	}

	service.clock = fixedClock{now: profileTestTime.Add(time.Hour)}
	updated, err := service.SaveEvidence(SaveEvidenceInput{
		EvidenceID:   overview.Evidence[0].ID,
		Title:        "  已确认的后端交付能力  ",
		Content:      "  负责核心服务交付并改善稳定性。  ",
		UserVerified: true,
	})
	if err != nil {
		t.Fatalf("save evidence: %v", err)
	}
	item := updated.Evidence[0]
	if item.Title != "已确认的后端交付能力" ||
		item.Content != "负责核心服务交付并改善稳定性。" ||
		!item.UserVerified ||
		item.UpdatedAt != profileTestTime.Add(time.Hour).Format(time.RFC3339) {
		t.Fatalf("unexpected saved evidence %#v", item)
	}

	if _, err := service.CreateProfile("其他岗位", "zh-CN"); err != nil {
		t.Fatalf("create second profile: %v", err)
	}
	if _, err := service.SaveEvidence(SaveEvidenceInput{
		EvidenceID:   item.ID,
		Title:        item.Title,
		Content:      item.Content,
		UserVerified: true,
	}); err == nil {
		t.Fatal("expected active profile isolation")
	}
}

func TestProfileServiceValidatesEvidenceChanges(t *testing.T) {
	service, _, _, _, _ := newProfileServiceTest(t)

	cases := []SaveEvidenceInput{
		{Title: "Title", Content: "Content"},
		{EvidenceID: "evidence", Content: "Content"},
		{EvidenceID: "evidence", Title: "Title"},
		{
			EvidenceID: "evidence",
			Title:      string(make([]rune, 241)),
			Content:    "Content",
		},
		{
			EvidenceID: "evidence",
			Title:      "Title",
			Content:    string(make([]rune, 8001)),
		},
	}
	for _, input := range cases {
		if _, err := service.SaveEvidence(input); err == nil {
			t.Fatalf("expected invalid evidence input error for %#v", input)
		}
	}
}

func TestProfileServiceCreatesSelectsAndIsolatesProfiles(t *testing.T) {
	service, _, _, _, _ := newProfileServiceTest(t)

	mainProfile, err := service.GetOverview()
	if err != nil {
		t.Fatalf("get default profile: %v", err)
	}
	if len(mainProfile.Profiles) != 1 || !mainProfile.Profiles[0].Active {
		t.Fatalf("expected one active default profile, got %#v", mainProfile.Profiles)
	}
	if _, err := service.ImportMarkdown(); err != nil {
		t.Fatalf("import default profile: %v", err)
	}

	englishProfile, err := service.CreateProfile("English applications", "en")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if englishProfile.Name != "English applications" ||
		englishProfile.DefaultLanguage != "en" ||
		len(englishProfile.Profiles) != 2 ||
		len(englishProfile.Documents) != 0 {
		t.Fatalf("unexpected created profile overview %#v", englishProfile)
	}
	if _, err := service.ImportMarkdown(); err != nil {
		t.Fatalf("import same Markdown into second profile: %v", err)
	}

	restoredMain, err := service.SelectProfile(mainProfile.ProfileID)
	if err != nil {
		t.Fatalf("select default profile: %v", err)
	}
	if restoredMain.ProfileID != mainProfile.ProfileID ||
		len(restoredMain.Documents) != 1 {
		t.Fatalf("expected isolated default profile data, got %#v", restoredMain)
	}
}

func TestProfileServiceRejectsInvalidProfileInput(t *testing.T) {
	service, _, _, _, _ := newProfileServiceTest(t)

	if _, err := service.CreateProfile("   ", "en"); err == nil {
		t.Fatal("expected blank profile name error")
	}
	if _, err := service.CreateProfile(
		string(make([]rune, 81)),
		"en",
	); err == nil {
		t.Fatal("expected long profile name error")
	}
	if _, err := service.SelectProfile(" "); err == nil {
		t.Fatal("expected blank profile id error")
	}
}

func TestProfileServiceHandlesCancelledAndEmptySelection(t *testing.T) {
	service, _, _, _, _ := newProfileServiceTest(t)
	service.picker = fakeMarkdownPicker{}

	cancelled, err := service.ImportMarkdown()
	if err != nil {
		t.Fatalf("cancel import: %v", err)
	}
	if !cancelled.Cancelled {
		t.Fatal("expected cancelled result")
	}

	service.picker = fakeMarkdownPicker{
		selected: ports.SelectedMarkdown{OriginalName: "empty.md"},
		accepted: true,
	}
	if _, err := service.ImportMarkdown(); err == nil {
		t.Fatal("expected empty Markdown error")
	}
}

func TestProfileServiceRemovesManagedFileWhenDatabaseSaveFails(t *testing.T) {
	service, repository, _, _, root := newProfileServiceTest(t)
	service.repository = failingSaveRepository{ProfileRepository: repository}

	if _, err := service.ImportMarkdown(); err == nil {
		t.Fatal("expected database save error")
	}

	entries, err := os.ReadDir(filepath.Join(root, "sources"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read sources directory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one profile directory, got %d", len(entries))
	}
	profileEntries, err := os.ReadDir(filepath.Join(root, "sources", entries[0].Name()))
	if err != nil {
		t.Fatalf("read profile directory: %v", err)
	}
	if len(profileEntries) != 0 {
		t.Fatalf("expected failed document directory cleanup")
	}
}

func newProfileServiceTest(t *testing.T) (
	*ProfileService,
	*sqliteadapter.ProfileRepository,
	*filesystem.ManagedFiles,
	[]byte,
	string,
) {
	t.Helper()

	root := t.TempDir()
	db, err := sqliteadapter.Open(
		context.Background(),
		filepath.Join(root, "autocv.db"),
	)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	repository := sqliteadapter.NewProfileRepository(db)
	files, err := filesystem.NewManagedFiles(root)
	if err != nil {
		t.Fatalf("create managed file store: %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(
		"..",
		"..",
		"testdata",
		"synthetic",
		"profile",
		"backend-profile.md",
	))
	if err != nil {
		t.Fatalf("read profile fixture: %v", err)
	}

	service := NewProfileService(
		repository,
		sqliteadapter.NewProfileSearch(db),
		markdownparser.New(),
		docxparser.New(),
		fakeprovider.New(),
		files,
		fakeMarkdownPicker{
			selected: ports.SelectedMarkdown{
				OriginalName: "backend-profile.md",
				Contents:     contents,
			},
			accepted: true,
		},
		fakeDOCXPicker{},
		fixedProfileExportPicker{},
		fixedClock{now: profileTestTime},
	)
	return service, repository, files, contents, root
}

func profileDOCXFixture(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	document, err := writer.Create("word/document.xml")
	if err != nil {
		t.Fatalf("create DOCX document XML: %v", err)
	}
	if _, err := io.WriteString(document, `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:pPr><w:pStyle w:val="Heading1"/></w:pPr>
      <w:r><w:t>李志林</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t>负责订单服务稳定性优化，支持核心交易链路。</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`); err != nil {
		t.Fatalf("write DOCX document XML: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close DOCX fixture: %v", err)
	}
	return buffer.Bytes()
}

func findEvidenceSummary(
	t *testing.T,
	items []EvidenceSummary,
	evidenceID string,
) EvidenceSummary {
	t.Helper()
	for _, item := range items {
		if item.ID == evidenceID {
			return item
		}
	}
	t.Fatalf("evidence %q not found in %#v", evidenceID, items)
	return EvidenceSummary{}
}

var _ ports.Clock = fixedClock{}
var _ ports.DOCXPicker = fakeDOCXPicker{}
var _ ports.MarkdownPicker = fakeMarkdownPicker{}
var _ ports.ProfileExtractor = (*sequencedProfileExtractor)(nil)
var _ ports.ProfileRepository = failingSaveRepository{}
