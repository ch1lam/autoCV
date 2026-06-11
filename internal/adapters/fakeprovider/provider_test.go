package fakeprovider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ch1lam/autocv/internal/domain"
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

func TestExtractProfileReturnsTraceableEvidence(t *testing.T) {
	evidence, err := New().ExtractProfile(
		context.Background(),
		ports.ExtractProfileRequest{
			Chunks: []domain.SourceChunk{
				{
					ID:          "chunk-experience",
					Text:        "负责交易服务核心链路开发和稳定性治理。",
					LocatorJSON: `{"heading_path":["工作经历"]}`,
				},
				{
					ID:          "chunk-project",
					Text:        "使用 PostgreSQL 保存订单状态。",
					LocatorJSON: `{"heading_path":["项目经验","订单平台"]}`,
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("extract profile: %v", err)
	}
	if len(evidence) != 2 {
		t.Fatalf("expected two evidence items, got %d", len(evidence))
	}
	if evidence[0].Kind != domain.EvidenceKindExperience {
		t.Fatalf("unexpected first evidence kind %q", evidence[0].Kind)
	}
	if evidence[1].Kind != domain.EvidenceKindProject {
		t.Fatalf("unexpected second evidence kind %q", evidence[1].Kind)
	}
	if evidence[1].SourceChunkIDs[0] != "chunk-project" {
		t.Fatalf("expected source chunk traceability")
	}
}

func TestExtractProfileRejectsMissingChunks(t *testing.T) {
	_, err := New().ExtractProfile(
		context.Background(),
		ports.ExtractProfileRequest{},
	)
	if err == nil {
		t.Fatal("expected missing chunks error")
	}
}
