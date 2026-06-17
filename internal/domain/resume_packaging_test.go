package domain

import "testing"

func TestResumePackagingStrategyForLevel(t *testing.T) {
	tests := []struct {
		level         float64
		expectedID    string
		expectedLimit int
	}{
		{level: 0, expectedID: "conservative", expectedLimit: 4},
		{level: 0.5, expectedID: "balanced", expectedLimit: 6},
		{level: 1, expectedID: "amplified", expectedLimit: 8},
	}
	for _, test := range tests {
		strategy, found := ResumePackagingStrategyForLevel(test.level)
		if !found {
			t.Fatalf("expected strategy for level %.1f", test.level)
		}
		if strategy.ID != test.expectedID ||
			strategy.EvidenceLimit != test.expectedLimit ||
			len(strategy.Guardrails) == 0 {
			t.Fatalf("unexpected strategy %#v", strategy)
		}
	}

	if _, found := ResumePackagingStrategyForLevel(0.25); found {
		t.Fatal("expected unsupported packaging level")
	}
}
