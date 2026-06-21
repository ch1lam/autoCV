package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ch1lam/autocv/internal/ports"
)

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func TestDataControlRepositoryPreviewsDeletionImpact(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(
		t,
		ctx,
		filepath.Join(t.TempDir(), "data-control.db"),
	)
	defer db.Close()
	seedDataControlGraph(t, ctx, db)

	repository := NewDataControlRepository(db)

	profileImpact, err := repository.PreviewProfileDeletion(ctx, "profile-1")
	if err != nil {
		t.Fatalf("preview profile deletion: %v", err)
	}
	assertDeletionImpact(t, profileImpact, ports.DeletionImpact{
		TargetKind: "profile",
		TargetID:   "profile-1",
		Counts: ports.DeletionCounts{
			Profiles:               1,
			SourceDocuments:        1,
			SourceChunks:           1,
			Evidence:               1,
			EvidenceSources:        1,
			MatchAnalyses:          1,
			MatchRequirements:      1,
			RequirementMatches:     1,
			MatchEvidence:          1,
			RunScopes:              1,
			RunScopeDocuments:      1,
			ResumeRuns:             1,
			StageResults:           1,
			Resumes:                1,
			ResumeBlocks:           1,
			ResumeBlockSources:     1,
			Artifacts:              1,
			ClarificationQuestions: 1,
			RunConfirmations:       1,
		},
		ManagedPaths: []string{
			"sources/profile-1/document-1/source.md",
		},
		ArtifactPaths: []string{
			"runs/run-1/artifacts/artifact-1-page-1.png",
			"runs/run-1/artifacts/artifact-1-page-2.png",
			"runs/run-1/artifacts/artifact-1.pdf",
		},
	})

	jdImpact, err := repository.PreviewJDDeletion(ctx, "jd-1")
	if err != nil {
		t.Fatalf("preview JD deletion: %v", err)
	}
	assertDeletionImpact(t, jdImpact, ports.DeletionImpact{
		TargetKind: "job_description",
		TargetID:   "jd-1",
		Counts: ports.DeletionCounts{
			JobDescriptions:        1,
			MatchAnalyses:          1,
			MatchRequirements:      1,
			RequirementMatches:     1,
			MatchEvidence:          1,
			RunScopes:              1,
			RunScopeDocuments:      1,
			ResumeRuns:             1,
			StageResults:           1,
			Resumes:                1,
			ResumeBlocks:           1,
			ResumeBlockSources:     1,
			Artifacts:              1,
			ClarificationQuestions: 1,
			RunConfirmations:       1,
		},
		ArtifactPaths: []string{
			"runs/run-1/artifacts/artifact-1-page-1.png",
			"runs/run-1/artifacts/artifact-1-page-2.png",
			"runs/run-1/artifacts/artifact-1.pdf",
		},
	})

	runImpact, err := repository.PreviewRunDeletion(ctx, "run-1")
	if err != nil {
		t.Fatalf("preview run deletion: %v", err)
	}
	assertDeletionImpact(t, runImpact, ports.DeletionImpact{
		TargetKind: "resume_run",
		TargetID:   "run-1",
		Counts: ports.DeletionCounts{
			ResumeRuns:             1,
			StageResults:           1,
			Resumes:                1,
			ResumeBlocks:           1,
			ResumeBlockSources:     1,
			Artifacts:              1,
			ClarificationQuestions: 1,
			RunConfirmations:       1,
		},
		ArtifactPaths: []string{
			"runs/run-1/artifacts/artifact-1-page-1.png",
			"runs/run-1/artifacts/artifact-1-page-2.png",
			"runs/run-1/artifacts/artifact-1.pdf",
		},
	})

	artifactImpact, err := repository.PreviewArtifactDeletion(ctx, "artifact-1")
	if err != nil {
		t.Fatalf("preview artifact deletion: %v", err)
	}
	assertDeletionImpact(t, artifactImpact, ports.DeletionImpact{
		TargetKind: "artifact",
		TargetID:   "artifact-1",
		Counts: ports.DeletionCounts{
			Artifacts: 1,
		},
		ArtifactPaths: []string{
			"runs/run-1/artifacts/artifact-1-page-1.png",
			"runs/run-1/artifacts/artifact-1-page-2.png",
			"runs/run-1/artifacts/artifact-1.pdf",
		},
	})
}

func assertDeletionImpact(
	t *testing.T,
	actual ports.DeletionImpact,
	expected ports.DeletionImpact,
) {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("unexpected deletion impact\nexpected: %#v\nactual:   %#v", expected, actual)
	}
}

func seedDataControlGraph(
	t *testing.T,
	ctx context.Context,
	db execer,
) {
	t.Helper()
	_, err := db.ExecContext(
		ctx,
		`INSERT INTO profiles(
		    id, name, default_language, created_at, updated_at, is_active
		) VALUES (
		    'profile-1', 'Main profile', 'en', '2026-06-19T00:00:00Z',
		    '2026-06-19T00:00:00Z', 1
		);
		INSERT INTO source_documents(
		    id, profile_id, kind, original_name, managed_path, content_hash,
		    parse_status, created_at, updated_at
		) VALUES (
		    'document-1', 'profile-1', 'markdown', 'source.md',
		    'sources/profile-1/document-1/source.md', 'document-hash',
		    'succeeded', '2026-06-19T00:01:00Z', '2026-06-19T00:01:00Z'
		);
		INSERT INTO source_chunks(
		    id, document_id, ordinal, text, locator_json
		) VALUES (
		    'chunk-1', 'document-1', 0, 'Built reliable Go services.',
		    '{"heading":"Experience"}'
		);
		INSERT INTO evidence(
		    id, profile_id, kind, title, content, confidence, user_verified,
		    created_at, updated_at
		) VALUES (
		    'evidence-1', 'profile-1', 'experience', 'Go reliability',
		    'Built reliable Go services.', 0.9, 1,
		    '2026-06-19T00:02:00Z', '2026-06-19T00:02:00Z'
		);
		INSERT INTO evidence_sources(
		    evidence_id, chunk_id, quote_start, quote_end
		) VALUES (
		    'evidence-1', 'chunk-1', 0, 27
		);
		INSERT INTO job_descriptions(
		    id, title, company, raw_text, language, analysis_json, created_at,
		    updated_at, raw_hash, analysis_status, analysis_error
		) VALUES (
		    'jd-1', 'Backend Engineer', 'Example Co', 'Need Go reliability.',
		    'en', '{"requirements":[]}', '2026-06-19T00:03:00Z',
		    '2026-06-19T00:03:00Z', 'jd-hash', 'succeeded', ''
		);
		INSERT INTO match_analyses(
		    id, profile_id, jd_id, input_hash, status, error, created_at,
		    updated_at
		) VALUES (
		    'analysis-1', 'profile-1', 'jd-1', 'match-hash', 'succeeded', '',
		    '2026-06-19T00:04:00Z', '2026-06-19T00:04:00Z'
		);
		INSERT INTO match_requirements(
		    analysis_id, id, category, text, importance, hard_constraint,
		    ordinal
		) VALUES (
		    'analysis-1', 'requirement-1', 'required', 'Go reliability', 5, 1,
		    0
		);
		INSERT INTO requirement_matches(
		    analysis_id, requirement_id, strength, explanation,
		    clarification_needed
		) VALUES (
		    'analysis-1', 'requirement-1', 'strong', 'Evidence matches.', 0
		);
		INSERT INTO match_evidence(
		    analysis_id, requirement_id, evidence_id, ordinal
		) VALUES (
		    'analysis-1', 'requirement-1', 'evidence-1', 0
		);
		INSERT INTO run_scopes(
		    profile_id, jd_id, mode, updated_at
		) VALUES (
		    'profile-1', 'jd-1', 'selected', '2026-06-19T00:05:00Z'
		);
		INSERT INTO run_scope_documents(
		    profile_id, jd_id, document_id, ordinal
		) VALUES (
		    'profile-1', 'jd-1', 'document-1', 0
		);
		INSERT INTO resume_runs(
		    id, profile_id, jd_id, status, stage, packaging_level, language,
		    created_at, updated_at
		) VALUES (
		    'run-1', 'profile-1', 'jd-1', 'active', 'pdf', 0.5, 'en',
		    '2026-06-19T00:06:00Z', '2026-06-19T00:06:00Z'
		);
		INSERT INTO stage_results(
		    id, run_id, stage, input_hash, status, result_json, error_json,
		    created_at, updated_at
		) VALUES (
		    'stage-1', 'run-1', 'pdf', 'stage-hash', 'succeeded', '{}', '',
		    '2026-06-19T00:07:00Z', '2026-06-19T00:07:00Z'
		);
		INSERT INTO resumes(
		    id, run_id, version, structure_json, markdown, created_at,
		    input_hash
		) VALUES (
		    'resume-1', 'run-1', 1, '{}', '# Backend Engineer',
		    '2026-06-19T00:08:00Z', 'resume-hash'
		);
		INSERT INTO resume_blocks(
		    resume_id, id, kind, ordinal, content, locked, grounding_level,
		    optimization
		) VALUES (
		    'resume-1', 'block-1', 'summary', 0, 'Reliable Go backend engineer.',
		    0, 'source', 'Targeted summary.'
		);
		INSERT INTO block_sources(
		    resume_id, block_id, evidence_id, ordinal, relation, risk_level
		) VALUES (
		    'resume-1', 'block-1', 'evidence-1', 0, 'supports', 'low'
		);
		INSERT INTO artifacts(
		    id, run_id, kind, path, content_hash, created_at, resume_id,
		    preview_paths_json
		) VALUES (
		    'artifact-1', 'run-1', 'pdf',
		    'runs/run-1/artifacts/artifact-1.pdf', 'pdf-hash',
		    '2026-06-19T00:09:00Z', 'resume-1',
		    '["runs/run-1/artifacts/artifact-1-page-1.png","runs/run-1/artifacts/artifact-1-page-2.png"]'
		);
		INSERT INTO clarification_questions(
		    id, run_id, requirement_id, round, ordinal, question, reason,
		    status, answer, created_at, updated_at
		) VALUES (
		    'question-1', 'run-1', 'requirement-1', 1, 0,
		    'Which reliability work matters most?', 'Need detail.', 'answered',
		    'Latency and incident diagnosis.', '2026-06-19T00:10:00Z',
		    '2026-06-19T00:10:00Z'
		);
		INSERT INTO run_confirmations(
		    id, run_id, clarification_question_id, requirement_id, content,
		    created_at, updated_at
		) VALUES (
		    'confirmation-1', 'run-1', 'question-1', 'requirement-1',
		    'Latency and incident diagnosis.', '2026-06-19T00:11:00Z',
		    '2026-06-19T00:11:00Z'
		);`,
	)
	if err != nil {
		t.Fatalf("seed data control graph: %v", err)
	}
}
