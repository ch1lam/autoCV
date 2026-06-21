package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ch1lam/autocv/internal/ports"
)

type DataControlRepository struct {
	db *sql.DB
}

func NewDataControlRepository(db *sql.DB) *DataControlRepository {
	return &DataControlRepository{db: db}
}

func (repository *DataControlRepository) PreviewProfileDeletion(
	ctx context.Context,
	profileID string,
) (ports.DeletionImpact, error) {
	impact := ports.DeletionImpact{
		TargetKind: "profile",
		TargetID:   profileID,
	}
	counts := []countTarget{
		{&impact.Counts.Profiles, `SELECT COUNT(*) FROM profiles WHERE id = ?`, []any{profileID}},
		{&impact.Counts.SourceDocuments, `SELECT COUNT(*) FROM source_documents WHERE profile_id = ?`, []any{profileID}},
		{&impact.Counts.SourceChunks, `SELECT COUNT(*)
		   FROM source_chunks sc
		   JOIN source_documents sd ON sd.id = sc.document_id
		  WHERE sd.profile_id = ?`, []any{profileID}},
		{&impact.Counts.Evidence, `SELECT COUNT(*) FROM evidence WHERE profile_id = ?`, []any{profileID}},
		{&impact.Counts.EvidenceSources, `SELECT COUNT(*)
		   FROM evidence_sources es
		   JOIN evidence e ON e.id = es.evidence_id
		  WHERE e.profile_id = ?`, []any{profileID}},
		{&impact.Counts.MatchAnalyses, `SELECT COUNT(*) FROM match_analyses WHERE profile_id = ?`, []any{profileID}},
		{&impact.Counts.MatchRequirements, `SELECT COUNT(*)
		   FROM match_requirements mr
		   JOIN match_analyses ma ON ma.id = mr.analysis_id
		  WHERE ma.profile_id = ?`, []any{profileID}},
		{&impact.Counts.RequirementMatches, `SELECT COUNT(*)
		   FROM requirement_matches rm
		   JOIN match_analyses ma ON ma.id = rm.analysis_id
		  WHERE ma.profile_id = ?`, []any{profileID}},
		{&impact.Counts.MatchEvidence, `SELECT COUNT(*)
		   FROM match_evidence me
		   JOIN match_analyses ma ON ma.id = me.analysis_id
		  WHERE ma.profile_id = ?`, []any{profileID}},
		{&impact.Counts.RunScopes, `SELECT COUNT(*) FROM run_scopes WHERE profile_id = ?`, []any{profileID}},
		{&impact.Counts.RunScopeDocuments, `SELECT COUNT(*) FROM run_scope_documents WHERE profile_id = ?`, []any{profileID}},
		{&impact.Counts.ResumeRuns, `SELECT COUNT(*) FROM resume_runs WHERE profile_id = ?`, []any{profileID}},
		{&impact.Counts.StageResults, `SELECT COUNT(*)
		   FROM stage_results sr
		   JOIN resume_runs rr ON rr.id = sr.run_id
		  WHERE rr.profile_id = ?`, []any{profileID}},
		{&impact.Counts.Resumes, `SELECT COUNT(*)
		   FROM resumes r
		   JOIN resume_runs rr ON rr.id = r.run_id
		  WHERE rr.profile_id = ?`, []any{profileID}},
		{&impact.Counts.ResumeBlocks, `SELECT COUNT(*)
		   FROM resume_blocks rb
		   JOIN resumes r ON r.id = rb.resume_id
		   JOIN resume_runs rr ON rr.id = r.run_id
		  WHERE rr.profile_id = ?`, []any{profileID}},
		{&impact.Counts.ResumeBlockSources, `SELECT COUNT(*)
		   FROM block_sources bs
		   JOIN resumes r ON r.id = bs.resume_id
		   JOIN resume_runs rr ON rr.id = r.run_id
		  WHERE rr.profile_id = ?`, []any{profileID}},
		{&impact.Counts.Artifacts, `SELECT COUNT(*)
		   FROM artifacts a
		   JOIN resume_runs rr ON rr.id = a.run_id
		  WHERE rr.profile_id = ?`, []any{profileID}},
		{&impact.Counts.ClarificationQuestions, `SELECT COUNT(*)
		   FROM clarification_questions cq
		   JOIN resume_runs rr ON rr.id = cq.run_id
		  WHERE rr.profile_id = ?`, []any{profileID}},
		{&impact.Counts.RunConfirmations, `SELECT COUNT(*)
		   FROM run_confirmations rc
		   JOIN resume_runs rr ON rr.id = rc.run_id
		  WHERE rr.profile_id = ?`, []any{profileID}},
	}
	if err := repository.populateCounts(ctx, counts); err != nil {
		return ports.DeletionImpact{}, err
	}
	managedPaths, err := repository.managedPaths(
		ctx,
		`SELECT managed_path FROM source_documents WHERE profile_id = ?`,
		profileID,
	)
	if err != nil {
		return ports.DeletionImpact{}, err
	}
	artifactPaths, err := repository.artifactPaths(
		ctx,
		`SELECT a.path, a.preview_paths_json
		   FROM artifacts a
		   JOIN resume_runs rr ON rr.id = a.run_id
		  WHERE rr.profile_id = ?`,
		profileID,
	)
	if err != nil {
		return ports.DeletionImpact{}, err
	}
	impact.ManagedPaths = managedPaths
	impact.ArtifactPaths = artifactPaths
	return impact, nil
}

func (repository *DataControlRepository) PreviewJDDeletion(
	ctx context.Context,
	jdID string,
) (ports.DeletionImpact, error) {
	impact := ports.DeletionImpact{
		TargetKind: "job_description",
		TargetID:   jdID,
	}
	counts := []countTarget{
		{&impact.Counts.JobDescriptions, `SELECT COUNT(*) FROM job_descriptions WHERE id = ?`, []any{jdID}},
		{&impact.Counts.MatchAnalyses, `SELECT COUNT(*) FROM match_analyses WHERE jd_id = ?`, []any{jdID}},
		{&impact.Counts.MatchRequirements, `SELECT COUNT(*)
		   FROM match_requirements mr
		   JOIN match_analyses ma ON ma.id = mr.analysis_id
		  WHERE ma.jd_id = ?`, []any{jdID}},
		{&impact.Counts.RequirementMatches, `SELECT COUNT(*)
		   FROM requirement_matches rm
		   JOIN match_analyses ma ON ma.id = rm.analysis_id
		  WHERE ma.jd_id = ?`, []any{jdID}},
		{&impact.Counts.MatchEvidence, `SELECT COUNT(*)
		   FROM match_evidence me
		   JOIN match_analyses ma ON ma.id = me.analysis_id
		  WHERE ma.jd_id = ?`, []any{jdID}},
		{&impact.Counts.RunScopes, `SELECT COUNT(*) FROM run_scopes WHERE jd_id = ?`, []any{jdID}},
		{&impact.Counts.RunScopeDocuments, `SELECT COUNT(*) FROM run_scope_documents WHERE jd_id = ?`, []any{jdID}},
		{&impact.Counts.ResumeRuns, `SELECT COUNT(*) FROM resume_runs WHERE jd_id = ?`, []any{jdID}},
		{&impact.Counts.StageResults, `SELECT COUNT(*)
		   FROM stage_results sr
		   JOIN resume_runs rr ON rr.id = sr.run_id
		  WHERE rr.jd_id = ?`, []any{jdID}},
		{&impact.Counts.Resumes, `SELECT COUNT(*)
		   FROM resumes r
		   JOIN resume_runs rr ON rr.id = r.run_id
		  WHERE rr.jd_id = ?`, []any{jdID}},
		{&impact.Counts.ResumeBlocks, `SELECT COUNT(*)
		   FROM resume_blocks rb
		   JOIN resumes r ON r.id = rb.resume_id
		   JOIN resume_runs rr ON rr.id = r.run_id
		  WHERE rr.jd_id = ?`, []any{jdID}},
		{&impact.Counts.ResumeBlockSources, `SELECT COUNT(*)
		   FROM block_sources bs
		   JOIN resumes r ON r.id = bs.resume_id
		   JOIN resume_runs rr ON rr.id = r.run_id
		  WHERE rr.jd_id = ?`, []any{jdID}},
		{&impact.Counts.Artifacts, `SELECT COUNT(*)
		   FROM artifacts a
		   JOIN resume_runs rr ON rr.id = a.run_id
		  WHERE rr.jd_id = ?`, []any{jdID}},
		{&impact.Counts.ClarificationQuestions, `SELECT COUNT(*)
		   FROM clarification_questions cq
		   JOIN resume_runs rr ON rr.id = cq.run_id
		  WHERE rr.jd_id = ?`, []any{jdID}},
		{&impact.Counts.RunConfirmations, `SELECT COUNT(*)
		   FROM run_confirmations rc
		   JOIN resume_runs rr ON rr.id = rc.run_id
		  WHERE rr.jd_id = ?`, []any{jdID}},
	}
	if err := repository.populateCounts(ctx, counts); err != nil {
		return ports.DeletionImpact{}, err
	}
	artifactPaths, err := repository.artifactPaths(
		ctx,
		`SELECT a.path, a.preview_paths_json
		   FROM artifacts a
		   JOIN resume_runs rr ON rr.id = a.run_id
		  WHERE rr.jd_id = ?`,
		jdID,
	)
	if err != nil {
		return ports.DeletionImpact{}, err
	}
	impact.ArtifactPaths = artifactPaths
	return impact, nil
}

func (repository *DataControlRepository) PreviewRunDeletion(
	ctx context.Context,
	runID string,
) (ports.DeletionImpact, error) {
	impact := ports.DeletionImpact{
		TargetKind: "resume_run",
		TargetID:   runID,
	}
	counts := []countTarget{
		{&impact.Counts.ResumeRuns, `SELECT COUNT(*) FROM resume_runs WHERE id = ?`, []any{runID}},
		{&impact.Counts.StageResults, `SELECT COUNT(*) FROM stage_results WHERE run_id = ?`, []any{runID}},
		{&impact.Counts.Resumes, `SELECT COUNT(*) FROM resumes WHERE run_id = ?`, []any{runID}},
		{&impact.Counts.ResumeBlocks, `SELECT COUNT(*)
		   FROM resume_blocks rb
		   JOIN resumes r ON r.id = rb.resume_id
		  WHERE r.run_id = ?`, []any{runID}},
		{&impact.Counts.ResumeBlockSources, `SELECT COUNT(*)
		   FROM block_sources bs
		   JOIN resumes r ON r.id = bs.resume_id
		  WHERE r.run_id = ?`, []any{runID}},
		{&impact.Counts.Artifacts, `SELECT COUNT(*) FROM artifacts WHERE run_id = ?`, []any{runID}},
		{&impact.Counts.ClarificationQuestions, `SELECT COUNT(*) FROM clarification_questions WHERE run_id = ?`, []any{runID}},
		{&impact.Counts.RunConfirmations, `SELECT COUNT(*) FROM run_confirmations WHERE run_id = ?`, []any{runID}},
	}
	if err := repository.populateCounts(ctx, counts); err != nil {
		return ports.DeletionImpact{}, err
	}
	artifactPaths, err := repository.artifactPaths(
		ctx,
		`SELECT path, preview_paths_json FROM artifacts WHERE run_id = ?`,
		runID,
	)
	if err != nil {
		return ports.DeletionImpact{}, err
	}
	impact.ArtifactPaths = artifactPaths
	return impact, nil
}

func (repository *DataControlRepository) PreviewArtifactDeletion(
	ctx context.Context,
	artifactID string,
) (ports.DeletionImpact, error) {
	impact := ports.DeletionImpact{
		TargetKind: "artifact",
		TargetID:   artifactID,
	}
	counts := []countTarget{
		{&impact.Counts.Artifacts, `SELECT COUNT(*) FROM artifacts WHERE id = ?`, []any{artifactID}},
	}
	if err := repository.populateCounts(ctx, counts); err != nil {
		return ports.DeletionImpact{}, err
	}
	artifactPaths, err := repository.artifactPaths(
		ctx,
		`SELECT path, preview_paths_json FROM artifacts WHERE id = ?`,
		artifactID,
	)
	if err != nil {
		return ports.DeletionImpact{}, err
	}
	impact.ArtifactPaths = artifactPaths
	return impact, nil
}

type countTarget struct {
	destination *int
	query       string
	args        []any
}

func (repository *DataControlRepository) populateCounts(
	ctx context.Context,
	targets []countTarget,
) error {
	for _, target := range targets {
		if err := repository.db.QueryRowContext(
			ctx,
			target.query,
			target.args...,
		).Scan(target.destination); err != nil {
			return fmt.Errorf("count deletion impact: %w", err)
		}
	}
	return nil
}

func (repository *DataControlRepository) managedPaths(
	ctx context.Context,
	query string,
	args ...any,
) ([]string, error) {
	rows, err := repository.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query managed deletion paths: %w", err)
	}
	defer rows.Close()

	paths := make([]string, 0)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("scan managed deletion path: %w", err)
		}
		paths = append(paths, path)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate managed deletion paths: %w", err)
	}
	return uniqueSortedStrings(paths), nil
}

func (repository *DataControlRepository) artifactPaths(
	ctx context.Context,
	query string,
	args ...any,
) ([]string, error) {
	rows, err := repository.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query artifact deletion paths: %w", err)
	}
	defer rows.Close()

	paths := make([]string, 0)
	for rows.Next() {
		var path string
		var previewJSON string
		if err := rows.Scan(&path, &previewJSON); err != nil {
			return nil, fmt.Errorf("scan artifact deletion path: %w", err)
		}
		paths = append(paths, path)
		var previews []string
		if err := json.Unmarshal([]byte(previewJSON), &previews); err != nil {
			return nil, fmt.Errorf("decode artifact preview paths: %w", err)
		}
		paths = append(paths, previews...)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate artifact deletion paths: %w", err)
	}
	return uniqueSortedStrings(paths), nil
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	sort.Strings(unique)
	return unique
}

var _ ports.DataControlRepository = (*DataControlRepository)(nil)
