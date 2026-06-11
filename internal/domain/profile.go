package domain

import "time"

type Profile struct {
	ID              string
	Name            string
	DefaultLanguage string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type SourceDocument struct {
	ID           string
	ProfileID    string
	Kind         string
	OriginalName string
	ManagedPath  string
	ContentHash  string
	ParseStatus  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type SourceChunk struct {
	ID          string
	DocumentID  string
	Ordinal     int
	Text        string
	LocatorJSON string
}

type Evidence struct {
	ID           string
	ProfileID    string
	Kind         string
	Title        string
	Content      string
	Confidence   float64
	UserVerified bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Sources      []EvidenceSource
}

type EvidenceSource struct {
	EvidenceID  string
	ChunkID     string
	DocumentID  string
	ChunkText   string
	LocatorJSON string
	QuoteStart  int
	QuoteEnd    int
}
