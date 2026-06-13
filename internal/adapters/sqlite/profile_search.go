package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ch1lam/autocv/internal/ports"
)

const defaultProfileSearchLimit = 20

type ProfileSearch struct {
	db *sql.DB
}

func NewProfileSearch(db *sql.DB) *ProfileSearch {
	return &ProfileSearch{db: db}
}

func (search *ProfileSearch) Search(
	ctx context.Context,
	profileID string,
	query string,
	limit int,
) ([]ports.ProfileSearchResult, error) {
	query = strings.TrimSpace(query)
	if profileID == "" || query == "" {
		return []ports.ProfileSearchResult{}, nil
	}
	if limit <= 0 {
		limit = defaultProfileSearchLimit
	}

	if utf8.RuneCountInString(query) < 3 {
		return search.searchShortQuery(ctx, profileID, query, limit)
	}
	rows, err := search.db.QueryContext(
		ctx,
		`SELECT entity_type,
		        entity_id,
		        document_id,
		        source_chunk_id,
		        document_name,
		        title,
		        snippet(profile_search, 7, '', '', ' … ', 18)
		   FROM profile_search
		  WHERE profile_search MATCH ?
		    AND profile_id = ?
		  ORDER BY bm25(profile_search, 0, 0, 0, 0, 0, 0, 5, 1),
		           document_name,
		           entity_type
		  LIMIT ?`,
		quoteFTSQuery(query),
		profileID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search profile index: %w", err)
	}
	return scanProfileSearchResults(rows)
}

func (search *ProfileSearch) searchShortQuery(
	ctx context.Context,
	profileID string,
	query string,
	limit int,
) ([]ports.ProfileSearchResult, error) {
	pattern := "%" + escapeLikePattern(query) + "%"
	rows, err := search.db.QueryContext(
		ctx,
		`SELECT entity_type,
		        entity_id,
		        document_id,
		        source_chunk_id,
		        document_name,
		        title,
		        CASE
		            WHEN length(body) <= 160 THEN body
		            ELSE substr(body, 1, 157) || ' …'
		        END
		   FROM profile_search
		  WHERE profile_id = ?
		    AND (
		        title LIKE ? ESCAPE '\'
		        OR body LIKE ? ESCAPE '\'
		    )
		  ORDER BY CASE WHEN title LIKE ? ESCAPE '\' THEN 0 ELSE 1 END,
		           document_name,
		           entity_type
		  LIMIT ?`,
		profileID,
		pattern,
		pattern,
		pattern,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search short profile query: %w", err)
	}
	return scanProfileSearchResults(rows)
}

func quoteFTSQuery(query string) string {
	return `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
}

func escapeLikePattern(query string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	)
	return replacer.Replace(query)
}

func scanProfileSearchResults(
	rows *sql.Rows,
) ([]ports.ProfileSearchResult, error) {
	defer rows.Close()

	results := make([]ports.ProfileSearchResult, 0)
	for rows.Next() {
		var result ports.ProfileSearchResult
		if err := rows.Scan(
			&result.EntityType,
			&result.EntityID,
			&result.DocumentID,
			&result.SourceChunkID,
			&result.DocumentName,
			&result.Title,
			&result.Snippet,
		); err != nil {
			return nil, fmt.Errorf("scan profile search result: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate profile search results: %w", err)
	}
	return results, nil
}
