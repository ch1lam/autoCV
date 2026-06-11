package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/adapters/fakeprovider"
	sqliteadapter "github.com/ch1lam/autocv/internal/adapters/sqlite"
	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

type failingJDAnalyzer struct{}

func (failingJDAnalyzer) AnalyzeJD(
	context.Context,
	ports.AnalyzeJDRequest,
) (domain.JDAnalysis, error) {
	return domain.JDAnalysis{}, errors.New("invalid structured output")
}

func TestJDServiceAnalyzesAndRestoresWorkspace(t *testing.T) {
	service, repository, rawText := newJDServiceTest(t, fakeprovider.New())

	workspace, err := service.Analyze(rawText)
	if err != nil {
		t.Fatalf("analyze JD: %v", err)
	}
	if workspace.AnalysisStatus != "succeeded" || workspace.Analysis == nil {
		t.Fatalf("expected analyzed workspace, got %#v", workspace)
	}
	if workspace.RawText != strings.TrimSpace(rawText) {
		t.Fatal("expected raw JD to remain visible")
	}
	if workspace.Analysis.Role != "Senior Backend Engineer" {
		t.Fatalf("unexpected role %q", workspace.Analysis.Role)
	}

	restarted := NewJDService(
		repository,
		fakeprovider.New(),
		fixedClock{now: profileTestTime.Add(time.Hour)},
	)
	restored, err := restarted.GetWorkspace()
	if err != nil {
		t.Fatalf("restore JD workspace: %v", err)
	}
	if restored.Analysis == nil ||
		restored.Analysis.Role != workspace.Analysis.Role {
		t.Fatalf("expected persisted analysis, got %#v", restored)
	}
}

func TestJDServiceInvalidatesAnalysisWhenDraftChanges(t *testing.T) {
	service, _, rawText := newJDServiceTest(t, fakeprovider.New())
	if _, err := service.Analyze(rawText); err != nil {
		t.Fatalf("analyze JD: %v", err)
	}

	edited, err := service.SaveDraft(rawText + "\nExperience with Kubernetes.")
	if err != nil {
		t.Fatalf("save edited JD: %v", err)
	}
	if edited.AnalysisStatus != "pending" || edited.Analysis != nil {
		t.Fatalf("expected invalidated analysis, got %#v", edited)
	}
}

func TestJDServicePersistsAnalysisFailure(t *testing.T) {
	service, _, rawText := newJDServiceTest(t, failingJDAnalyzer{})

	if _, err := service.Analyze(rawText); err == nil {
		t.Fatal("expected analysis error")
	}
	workspace, err := service.GetWorkspace()
	if err != nil {
		t.Fatalf("get failed workspace: %v", err)
	}
	if workspace.AnalysisStatus != "failed" ||
		workspace.AnalysisError == "" {
		t.Fatalf("expected persisted analysis failure, got %#v", workspace)
	}
}

func TestJDServiceRejectsEmptyText(t *testing.T) {
	service, _, _ := newJDServiceTest(t, fakeprovider.New())

	if _, err := service.SaveDraft(" \n "); err == nil {
		t.Fatal("expected empty JD error")
	}
	workspace, err := service.GetWorkspace()
	if err != nil {
		t.Fatalf("get empty workspace: %v", err)
	}
	if workspace.AnalysisStatus != "empty" {
		t.Fatalf("expected empty workspace, got %#v", workspace)
	}
}

func newJDServiceTest(
	t *testing.T,
	analyzer ports.JDAnalyzer,
) (*JDService, *sqliteadapter.JDRepository, string) {
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
	rawTextBytes, err := os.ReadFile(filepath.Join(
		"..",
		"..",
		"testdata",
		"synthetic",
		"jd",
		"backend-engineer.txt",
	))
	if err != nil {
		t.Fatalf("read JD fixture: %v", err)
	}
	repository := sqliteadapter.NewJDRepository(db)
	service := NewJDService(
		repository,
		analyzer,
		fixedClock{now: profileTestTime},
	)
	return service, repository, string(rawTextBytes)
}

var _ ports.JDAnalyzer = failingJDAnalyzer{}
