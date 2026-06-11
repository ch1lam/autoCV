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
var _ ports.ProfileRepository = failingSaveRepository{}
