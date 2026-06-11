package domain

import (
	"errors"
	"fmt"
	"strings"
)

type EvidenceKind string

const (
	EvidenceKindExperience    EvidenceKind = "experience"
	EvidenceKindProject       EvidenceKind = "project"
	EvidenceKindSkill         EvidenceKind = "skill"
	EvidenceKindEducation     EvidenceKind = "education"
	EvidenceKindCertification EvidenceKind = "certification"
	EvidenceKindAchievement   EvidenceKind = "achievement"
)

type ExtractedEvidence struct {
	Kind           EvidenceKind
	Title          string
	Content        string
	SourceChunkIDs []string
	Confidence     float64
}

func (evidence ExtractedEvidence) Validate() error {
	switch evidence.Kind {
	case EvidenceKindExperience,
		EvidenceKindProject,
		EvidenceKindSkill,
		EvidenceKindEducation,
		EvidenceKindCertification,
		EvidenceKindAchievement:
	default:
		return fmt.Errorf("invalid evidence kind %q", evidence.Kind)
	}
	if strings.TrimSpace(evidence.Title) == "" {
		return errors.New("evidence title is empty")
	}
	if strings.TrimSpace(evidence.Content) == "" {
		return errors.New("evidence content is empty")
	}
	if len(evidence.SourceChunkIDs) == 0 {
		return errors.New("evidence has no source chunks")
	}
	for _, chunkID := range evidence.SourceChunkIDs {
		if strings.TrimSpace(chunkID) == "" {
			return errors.New("evidence source chunk id is empty")
		}
	}
	if evidence.Confidence < 0 || evidence.Confidence > 1 {
		return fmt.Errorf(
			"evidence confidence %.2f is outside 0..1",
			evidence.Confidence,
		)
	}
	return nil
}
