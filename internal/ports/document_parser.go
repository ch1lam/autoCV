package ports

type DocumentMetadata struct {
	Title string
}

type SourceLocator struct {
	HeadingPath []string `json:"heading_path"`
	Page        int      `json:"page,omitempty"`
	Start       int      `json:"start"`
	End         int      `json:"end"`
}

type ParsedChunk struct {
	Ordinal int
	Kind    string
	Text    string
	Locator SourceLocator
}

type ParseResult struct {
	Metadata DocumentMetadata
	Chunks   []ParsedChunk
	Warnings []string
}

type DocumentParser interface {
	Parse([]byte) (ParseResult, error)
}
