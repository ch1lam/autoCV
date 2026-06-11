package domain

import "time"

type JobDescription struct {
	ID             string
	Title          string
	Company        string
	RawText        string
	Language       string
	RawHash        string
	AnalysisJSON   string
	AnalysisStatus string
	AnalysisError  string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
