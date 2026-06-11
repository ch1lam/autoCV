package domain

import "testing"

func TestExtractedEvidenceValidation(t *testing.T) {
	valid := ExtractedEvidence{
		Kind:           EvidenceKindExperience,
		Title:          "Backend delivery",
		Content:        "Built backend services.",
		SourceChunkIDs: []string{"chunk-1"},
		Confidence:     0.75,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("validate evidence: %v", err)
	}

	invalid := valid
	invalid.Kind = "unknown"
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected invalid evidence kind error")
	}

	invalid = valid
	invalid.SourceChunkIDs = nil
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected missing source error")
	}

	invalid = valid
	invalid.Confidence = 1.1
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected confidence error")
	}
}
