package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
)

func TestResumeRepositoryAppendsVersionsAndLoadsBlockSources(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, ctx, filepath.Join(t.TempDir(), "resume.db"))
	defer db.Close()
	now := time.Date(2026, 6, 12, 2, 0, 0, 0, time.UTC)
	seedResumeDependencies(t, db, now)
	repository := NewResumeRepository(db)

	run := domain.ResumeRun{
		ID:             "run-1",
		ProfileID:      "profile-1",
		JDID:           "jd-1",
		Status:         "active",
		Stage:          "drafted",
		PackagingLevel: 0.5,
		Language:       domain.ResumeLanguageChinese,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	first := resumeRepositoryFixture(now)
	if err := repository.SaveVersion(ctx, run, first); err != nil {
		t.Fatalf("save first resume version: %v", err)
	}

	second := first
	second.ID = "resume-2"
	second.Version = 2
	second.CreatedAt = now.Add(time.Minute)
	second.Blocks = append([]domain.ResumeBlock(nil), first.Blocks...)
	second.Blocks[0].Locked = true
	second.Markdown = domain.RenderResumeMarkdown(second)
	run.UpdatedAt = second.CreatedAt
	if err := repository.SaveVersion(ctx, run, second); err != nil {
		t.Fatalf("save second resume version: %v", err)
	}

	savedRun, saved, found, err := repository.GetLatest(
		ctx,
		run.ProfileID,
		run.JDID,
	)
	if err != nil {
		t.Fatalf("get latest resume: %v", err)
	}
	if !found || savedRun.ID != run.ID || saved.Version != 2 {
		t.Fatalf("unexpected latest resume: %#v %#v", savedRun, saved)
	}
	if !saved.Blocks[0].Locked {
		t.Fatal("expected latest block lock state")
	}
	if len(saved.Blocks[0].SourceEvidenceIDs) != 1 ||
		saved.Blocks[0].SourceEvidenceIDs[0] != "evidence-1" {
		t.Fatalf("unexpected block sources: %#v", saved.Blocks[0].SourceEvidenceIDs)
	}

	latestRun, found, err := repository.LatestRun(ctx)
	if err != nil {
		t.Fatalf("get latest run: %v", err)
	}
	if !found || latestRun.ID != run.ID || latestRun.Stage != run.Stage {
		t.Fatalf("unexpected latest run found=%v %#v", found, latestRun)
	}
}

func TestResumeRepositoryReplacesRunScopeDocuments(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, ctx, filepath.Join(t.TempDir(), "scope.db"))
	defer db.Close()
	now := time.Date(2026, 6, 16, 2, 0, 0, 0, time.UTC)
	seedResumeDependencies(t, db, now)
	for _, documentID := range []string{"document-1", "document-2"} {
		if _, err := db.Exec(
			`INSERT INTO source_documents(
				id, profile_id, kind, original_name, managed_path,
				content_hash, parse_status, created_at, updated_at
			) VALUES (?, 'profile-1', 'markdown', ?, ?, ?, 'succeeded', ?, ?)`,
			documentID,
			documentID+".md",
			"/managed/"+documentID+".md",
			"hash-"+documentID,
			formatTime(now),
			formatTime(now),
		); err != nil {
			t.Fatalf("seed source document: %v", err)
		}
	}
	repository := NewResumeRepository(db)
	selected := domain.ResumeRunScope{
		ProfileID:   "profile-1",
		JDID:        "jd-1",
		Mode:        domain.RunScopeSelected,
		DocumentIDs: []string{"document-2"},
		UpdatedAt:   now,
	}
	if err := repository.SaveScope(ctx, selected); err != nil {
		t.Fatalf("save selected run scope: %v", err)
	}
	saved, found, err := repository.GetScope(ctx, "profile-1", "jd-1")
	if err != nil {
		t.Fatalf("get selected run scope: %v", err)
	}
	if !found || saved.Mode != domain.RunScopeSelected ||
		len(saved.DocumentIDs) != 1 || saved.DocumentIDs[0] != "document-2" {
		t.Fatalf("unexpected selected run scope %#v", saved)
	}

	all := selected
	all.Mode = domain.RunScopeAll
	all.DocumentIDs = nil
	all.UpdatedAt = now.Add(time.Minute)
	if err := repository.SaveScope(ctx, all); err != nil {
		t.Fatalf("save all-documents run scope: %v", err)
	}
	saved, found, err = repository.GetScope(ctx, "profile-1", "jd-1")
	if err != nil {
		t.Fatalf("get all-documents run scope: %v", err)
	}
	if !found || saved.Mode != domain.RunScopeAll ||
		len(saved.DocumentIDs) != 0 {
		t.Fatalf("unexpected all-documents run scope %#v", saved)
	}
}

func seedResumeDependencies(
	t *testing.T,
	db *sql.DB,
	now time.Time,
) {
	t.Helper()
	timestamp := formatTime(now)
	if _, err := db.Exec(
		`INSERT INTO profiles(
			id, name, default_language, created_at, updated_at
		) VALUES ('profile-1', 'Main', 'zh-CN', ?, ?)`,
		timestamp,
		timestamp,
	); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO job_descriptions(
			id, title, raw_text, language, analysis_json,
			created_at, updated_at, raw_hash, analysis_status
		) VALUES (
			'jd-1', 'Backend Engineer', 'Go required', 'en', '{}',
			?, ?, 'hash-1', 'succeeded'
		)`,
		timestamp,
		timestamp,
	); err != nil {
		t.Fatalf("seed JD: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO evidence(
			id, profile_id, kind, title, content, confidence,
			created_at, updated_at
		) VALUES (
			'evidence-1', 'profile-1', 'experience', 'Backend',
			'Built Go services.', 0.9, ?, ?
		)`,
		timestamp,
		timestamp,
	); err != nil {
		t.Fatalf("seed evidence: %v", err)
	}
}

func resumeRepositoryFixture(now time.Time) domain.Resume {
	resume := domain.Resume{
		ID:                "resume-1",
		RunID:             "run-1",
		InputHash:         "input-1",
		Version:           1,
		Language:          domain.ResumeLanguageChinese,
		TargetRole:        "后端工程师",
		OptimizationNotes: []string{"优先展示 Go 经验。"},
		CreatedAt:         now,
		Blocks: []domain.ResumeBlock{{
			ID:                "block-1",
			Kind:              domain.ResumeBlockExperience,
			Content:           "Built Go services.",
			SourceEvidenceIDs: []string{"evidence-1"},
			GroundingLevel:    domain.GroundingSource,
			Optimization:      "对应必要技能。",
		}},
	}
	resume.Markdown = domain.RenderResumeMarkdown(resume)
	return resume
}
