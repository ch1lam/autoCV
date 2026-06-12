package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

type MatchRepository struct {
	db *sql.DB
}

func NewMatchRepository(db *sql.DB) *MatchRepository {
	return &MatchRepository{db: db}
}

func (repository *MatchRepository) GetLatest(
	ctx context.Context,
	profileID string,
	jdID string,
) (domain.MatchAnalysis, bool, error) {
	var analysis domain.MatchAnalysis
	var createdAt string
	var updatedAt string
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT id, profile_id, jd_id, input_hash, status, error,
		        created_at, updated_at
		   FROM match_analyses
		  WHERE profile_id = ? AND jd_id = ?`,
		profileID,
		jdID,
	).Scan(
		&analysis.ID,
		&analysis.ProfileID,
		&analysis.JDID,
		&analysis.InputHash,
		&analysis.Status,
		&analysis.Error,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.MatchAnalysis{}, false, nil
	}
	if err != nil {
		return domain.MatchAnalysis{}, false, fmt.Errorf(
			"get match analysis: %w",
			err,
		)
	}
	analysis.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.MatchAnalysis{}, false, err
	}
	analysis.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.MatchAnalysis{}, false, err
	}

	analysis.Requirements, err = repository.listRequirements(ctx, analysis.ID)
	if err != nil {
		return domain.MatchAnalysis{}, false, err
	}
	analysis.Suggestions, err = repository.listSuggestions(ctx, analysis.ID)
	if err != nil {
		return domain.MatchAnalysis{}, false, err
	}
	return analysis, true, nil
}

func (repository *MatchRepository) Save(
	ctx context.Context,
	analysis domain.MatchAnalysis,
) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin match analysis transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO match_analyses(
			id, profile_id, jd_id, input_hash, status, error,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id, jd_id) DO UPDATE SET
			id = excluded.id,
			input_hash = excluded.input_hash,
			status = excluded.status,
			error = excluded.error,
			updated_at = excluded.updated_at`,
		analysis.ID,
		analysis.ProfileID,
		analysis.JDID,
		analysis.InputHash,
		analysis.Status,
		analysis.Error,
		formatTime(analysis.CreatedAt),
		formatTime(analysis.UpdatedAt),
	); err != nil {
		return fmt.Errorf("save match analysis: %w", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		"DELETE FROM match_requirements WHERE analysis_id = ?",
		analysis.ID,
	); err != nil {
		return fmt.Errorf("clear match requirements: %w", err)
	}

	for _, requirement := range analysis.Requirements {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO match_requirements(
				analysis_id, id, category, text, importance,
				hard_constraint, ordinal
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			analysis.ID,
			requirement.ID,
			requirement.Category,
			requirement.Text,
			requirement.Importance,
			requirement.HardConstraint,
			requirement.Ordinal,
		); err != nil {
			return fmt.Errorf("insert match requirement: %w", err)
		}
	}
	for _, suggestion := range analysis.Suggestions {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO requirement_matches(
				analysis_id, requirement_id, strength, explanation,
				clarification_needed
			) VALUES (?, ?, ?, ?, ?)`,
			analysis.ID,
			suggestion.RequirementID,
			suggestion.Strength,
			suggestion.Explanation,
			suggestion.ClarificationNeeded,
		); err != nil {
			return fmt.Errorf("insert requirement match: %w", err)
		}
		for ordinal, evidenceID := range suggestion.EvidenceIDs {
			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO match_evidence(
					analysis_id, requirement_id, evidence_id, ordinal
				) VALUES (?, ?, ?, ?)`,
				analysis.ID,
				suggestion.RequirementID,
				evidenceID,
				ordinal,
			); err != nil {
				return fmt.Errorf("insert match evidence: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit match analysis: %w", err)
	}
	return nil
}

func (repository *MatchRepository) listRequirements(
	ctx context.Context,
	analysisID string,
) ([]domain.MatchRequirement, error) {
	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT id, category, text, importance, hard_constraint, ordinal
		   FROM match_requirements
		  WHERE analysis_id = ?
		  ORDER BY ordinal, id`,
		analysisID,
	)
	if err != nil {
		return nil, fmt.Errorf("list match requirements: %w", err)
	}
	defer rows.Close()

	requirements := make([]domain.MatchRequirement, 0)
	for rows.Next() {
		var requirement domain.MatchRequirement
		if err := rows.Scan(
			&requirement.ID,
			&requirement.Category,
			&requirement.Text,
			&requirement.Importance,
			&requirement.HardConstraint,
			&requirement.Ordinal,
		); err != nil {
			return nil, fmt.Errorf("scan match requirement: %w", err)
		}
		requirements = append(requirements, requirement)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate match requirements: %w", err)
	}
	return requirements, nil
}

func (repository *MatchRepository) listSuggestions(
	ctx context.Context,
	analysisID string,
) ([]domain.MatchSuggestion, error) {
	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT rm.requirement_id, rm.strength, rm.explanation,
		        rm.clarification_needed, me.evidence_id
		   FROM requirement_matches rm
		   LEFT JOIN match_evidence me
		     ON me.analysis_id = rm.analysis_id
		    AND me.requirement_id = rm.requirement_id
		  WHERE rm.analysis_id = ?
		  ORDER BY rm.requirement_id, me.ordinal`,
		analysisID,
	)
	if err != nil {
		return nil, fmt.Errorf("list match suggestions: %w", err)
	}
	defer rows.Close()

	suggestions := make([]domain.MatchSuggestion, 0)
	indexes := make(map[string]int)
	for rows.Next() {
		var suggestion domain.MatchSuggestion
		var evidenceID sql.NullString
		if err := rows.Scan(
			&suggestion.RequirementID,
			&suggestion.Strength,
			&suggestion.Explanation,
			&suggestion.ClarificationNeeded,
			&evidenceID,
		); err != nil {
			return nil, fmt.Errorf("scan match suggestion: %w", err)
		}
		index, exists := indexes[suggestion.RequirementID]
		if !exists {
			suggestion.EvidenceIDs = make([]string, 0)
			suggestions = append(suggestions, suggestion)
			index = len(suggestions) - 1
			indexes[suggestion.RequirementID] = index
		}
		if evidenceID.Valid {
			suggestions[index].EvidenceIDs = append(
				suggestions[index].EvidenceIDs,
				evidenceID.String,
			)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate match suggestions: %w", err)
	}
	return suggestions, nil
}

var _ ports.MatchRepository = (*MatchRepository)(nil)
