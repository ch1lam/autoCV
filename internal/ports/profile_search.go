package ports

import "context"

type ProfileSearchResult struct {
	EntityType    string
	EntityID      string
	DocumentID    string
	SourceChunkID string
	DocumentName  string
	Title         string
	Snippet       string
}

type ProfileSearch interface {
	Search(
		context.Context,
		string,
		string,
		int,
	) ([]ProfileSearchResult, error)
}
