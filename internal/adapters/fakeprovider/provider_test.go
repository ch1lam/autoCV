package fakeprovider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ch1lam/autocv/internal/ports"
)

func TestAnalyzeJDReturnsFixedValidatedFixture(t *testing.T) {
	jd, err := os.ReadFile(filepath.Join(
		"..",
		"..",
		"..",
		"testdata",
		"synthetic",
		"jd",
		"backend-engineer.txt",
	))
	if err != nil {
		t.Fatalf("read synthetic JD: %v", err)
	}

	analysis, err := New().AnalyzeJD(context.Background(), ports.AnalyzeJDRequest{
		Text: string(jd),
	})
	if err != nil {
		t.Fatalf("analyze JD: %v", err)
	}
	if analysis.Role != "Senior Backend Engineer" {
		t.Fatalf("unexpected role %q", analysis.Role)
	}
	if len(analysis.Responsibilities) != 2 {
		t.Fatalf("expected two responsibilities")
	}
	if len(analysis.RequiredSkills) != 3 {
		t.Fatalf("expected three required skills")
	}
}

func TestAnalyzeJDRejectsEmptyInput(t *testing.T) {
	_, err := New().AnalyzeJD(context.Background(), ports.AnalyzeJDRequest{})
	if err == nil {
		t.Fatal("expected empty JD error")
	}
}

func TestAnalyzeJDHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := New().AnalyzeJD(ctx, ports.AnalyzeJDRequest{Text: "JD"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
