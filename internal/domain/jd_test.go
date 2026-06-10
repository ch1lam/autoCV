package domain

import (
	"strings"
	"testing"
)

const validAnalysis = `{
  "role": "Backend Engineer",
  "company": null,
  "level": "Senior",
  "language": "en",
  "responsibilities": [
    {
      "id": "responsibility-1",
      "text": "Build backend services",
      "importance": 5,
      "hard_constraint": false
    }
  ],
  "required_skills": [],
  "preferred_skills": [],
  "domain_signals": [],
  "screening_constraints": [],
  "ambiguities": []
}`

func TestDecodeJDAnalysis(t *testing.T) {
	analysis, err := DecodeJDAnalysis([]byte(validAnalysis))
	if err != nil {
		t.Fatalf("decode valid analysis: %v", err)
	}
	if analysis.Role != "Backend Engineer" {
		t.Fatalf("unexpected role %q", analysis.Role)
	}
	if len(analysis.Responsibilities) != 1 {
		t.Fatalf("expected one responsibility")
	}
}

func TestDecodeJDAnalysisRejectsUnknownFields(t *testing.T) {
	invalid := strings.Replace(
		validAnalysis,
		`"ambiguities": []`,
		`"ambiguities": [], "score": 99`,
		1,
	)

	_, err := DecodeJDAnalysis([]byte(invalid))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestDecodeJDAnalysisRejectsMissingArrays(t *testing.T) {
	invalid := strings.Replace(
		validAnalysis,
		`"preferred_skills": [],`,
		"",
		1,
	)

	_, err := DecodeJDAnalysis([]byte(invalid))
	if err == nil || !strings.Contains(err.Error(), "preferred skills are missing") {
		t.Fatalf("expected missing field error, got %v", err)
	}
}

func TestDecodeJDAnalysisRejectsDuplicateRequirementIDs(t *testing.T) {
	invalid := strings.Replace(
		validAnalysis,
		`"required_skills": []`,
		`"required_skills": [{
		  "id": "responsibility-1",
		  "text": "Use Go",
		  "importance": 5,
		  "hard_constraint": true
		}]`,
		1,
	)

	_, err := DecodeJDAnalysis([]byte(invalid))
	if err == nil || !strings.Contains(err.Error(), "duplicate requirement id") {
		t.Fatalf("expected duplicate id error, got %v", err)
	}
}

func TestDecodeJDAnalysisRejectsInvalidImportance(t *testing.T) {
	invalid := strings.Replace(
		validAnalysis,
		`"importance": 5`,
		`"importance": 6`,
		1,
	)

	_, err := DecodeJDAnalysis([]byte(invalid))
	if err == nil || !strings.Contains(err.Error(), "outside 1..5") {
		t.Fatalf("expected importance error, got %v", err)
	}
}
