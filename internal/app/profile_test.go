package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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
		fakeprovider.New(),
		files,
		fakeMarkdownPicker{},
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
	confirmed, err := service.SaveEvidence(SaveEvidenceInput{
		EvidenceID:   overview.Evidence[0].ID,
		Title:        "支付平台职责",
		Content:      "负责支付平台接口交付并改善稳定性。",
		UserVerified: true,
	})
	if err != nil {
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
	if len(overview.Evidence) != 2 ||
		overview.Evidence[0].Content != confirmed.Evidence[0].Content ||
		!overview.Evidence[0].UserVerified ||
		len(overview.Evidence[0].ConflictEvidenceIDs) != 1 ||
		len(overview.Evidence[1].ConflictEvidenceIDs) != 1 {
		t.Fatalf("expected confirmed evidence to remain intact, got %#v", overview.Evidence)
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
		fakeprovider.New(),
		files,
		fakeMarkdownPicker{
			selected: ports.SelectedMarkdown{
				OriginalName: "backend-profile.md",
				Contents:     contents,
			},
			accepted: true,
		},
		fixedClock{now: profileTestTime},
	)
	return service, repository, files, contents, root
}

var _ ports.Clock = fixedClock{}
var _ ports.MarkdownPicker = fakeMarkdownPicker{}
var _ ports.ProfileExtractor = (*sequencedProfileExtractor)(nil)
var _ ports.ProfileRepository = failingSaveRepository{}
