package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type JDLanguage string

const (
	JDLanguageChinese JDLanguage = "zh"
	JDLanguageEnglish JDLanguage = "en"
	JDLanguageMixed   JDLanguage = "mixed"
)

type Requirement struct {
	ID             string `json:"id"`
	Text           string `json:"text"`
	Importance     int    `json:"importance"`
	HardConstraint bool   `json:"hard_constraint"`
}

type JDAnalysis struct {
	Role                 string        `json:"role"`
	Company              *string       `json:"company"`
	Level                *string       `json:"level"`
	Language             JDLanguage    `json:"language"`
	Responsibilities     []Requirement `json:"responsibilities"`
	RequiredSkills       []Requirement `json:"required_skills"`
	PreferredSkills      []Requirement `json:"preferred_skills"`
	DomainSignals        []string      `json:"domain_signals"`
	ScreeningConstraints []string      `json:"screening_constraints"`
	Ambiguities          []string      `json:"ambiguities"`
}

func DecodeJDAnalysis(contents []byte) (JDAnalysis, error) {
	var analysis JDAnalysis
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&analysis); err != nil {
		return JDAnalysis{}, fmt.Errorf("decode JD analysis: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return JDAnalysis{}, errors.New("decode JD analysis: trailing content")
	}
	if err := analysis.Validate(); err != nil {
		return JDAnalysis{}, err
	}
	return analysis, nil
}

func (analysis JDAnalysis) Validate() error {
	if strings.TrimSpace(analysis.Role) == "" {
		return errors.New("JD analysis role is empty")
	}
	switch analysis.Language {
	case JDLanguageChinese, JDLanguageEnglish, JDLanguageMixed:
	default:
		return fmt.Errorf("invalid JD analysis language %q", analysis.Language)
	}

	if analysis.Responsibilities == nil {
		return errors.New("JD analysis responsibilities are missing")
	}
	if analysis.RequiredSkills == nil {
		return errors.New("JD analysis required skills are missing")
	}
	if analysis.PreferredSkills == nil {
		return errors.New("JD analysis preferred skills are missing")
	}
	if analysis.DomainSignals == nil {
		return errors.New("JD analysis domain signals are missing")
	}
	if analysis.ScreeningConstraints == nil {
		return errors.New("JD analysis screening constraints are missing")
	}
	if analysis.Ambiguities == nil {
		return errors.New("JD analysis ambiguities are missing")
	}

	seen := make(map[string]struct{})
	for category, requirements := range map[string][]Requirement{
		"responsibilities": analysis.Responsibilities,
		"required_skills":  analysis.RequiredSkills,
		"preferred_skills": analysis.PreferredSkills,
	} {
		for index, requirement := range requirements {
			if err := validateRequirement(requirement); err != nil {
				return fmt.Errorf("%s[%d]: %w", category, index, err)
			}
			if _, exists := seen[requirement.ID]; exists {
				return fmt.Errorf("duplicate requirement id %q", requirement.ID)
			}
			seen[requirement.ID] = struct{}{}
		}
	}
	return nil
}

func validateRequirement(requirement Requirement) error {
	if strings.TrimSpace(requirement.ID) == "" {
		return errors.New("requirement id is empty")
	}
	if strings.TrimSpace(requirement.Text) == "" {
		return errors.New("requirement text is empty")
	}
	if requirement.Importance < 1 || requirement.Importance > 5 {
		return fmt.Errorf(
			"requirement importance %d is outside 1..5",
			requirement.Importance,
		)
	}
	return nil
}
