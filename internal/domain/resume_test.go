package domain

import (
	"strings"
	"testing"
)

func TestValidateResumeDraftRejectsUngroundedNumbers(t *testing.T) {
	evidence := []Evidence{{
		ID:      "evidence-1",
		Title:   "性能优化",
		Content: "优化了订单服务的稳定性。",
	}}
	draft := validResumeDraft()
	draft.Blocks[0].Content = "将接口延迟降低 35%。"

	err := ValidateResumeDraft(draft, evidence)
	if err == nil || !strings.Contains(err.Error(), "ungrounded numeric") {
		t.Fatalf("expected ungrounded numeric error, got %v", err)
	}

	evidence[0].Content = "将接口延迟降低 35%。"
	if err := ValidateResumeDraft(draft, evidence); err != nil {
		t.Fatalf("expected grounded numeric value: %v", err)
	}
}

func TestDecodeResumeDraftRejectsUnknownFields(t *testing.T) {
	_, err := DecodeResumeDraft([]byte(`{
		"language":"zh",
		"target_role":"后端工程师",
		"blocks":[],
		"optimization_notes":[],
		"score":99
	}`))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestApplyResumeMarkdownPreservesBlockIdentity(t *testing.T) {
	resume := validResume()
	markdown := RenderResumeMarkdown(resume)
	edited := strings.Replace(
		markdown,
		"负责 Go 服务开发。",
		"负责 Go 服务开发与稳定性治理。",
		1,
	)

	updated, err := ApplyResumeMarkdown(resume, edited)
	if err != nil {
		t.Fatalf("apply resume Markdown: %v", err)
	}
	if updated.Blocks[0].ID != resume.Blocks[0].ID {
		t.Fatal("expected stable resume block id")
	}
	if updated.Blocks[0].Content != "负责 Go 服务开发与稳定性治理。" {
		t.Fatalf("unexpected edited content %q", updated.Blocks[0].Content)
	}
	if updated.Blocks[0].GroundingLevel != GroundingUserConfirmed {
		t.Fatalf(
			"expected edited content to become user confirmed, got %q",
			updated.Blocks[0].GroundingLevel,
		)
	}
}

func TestResumeV2SupportsDynamicSectionTitles(t *testing.T) {
	resume := validResume()
	resume.SchemaVersion = ResumeSchemaV2
	resume.Sections = []ResumeSection{{
		ID:    "section-selected-work",
		Title: "代表作品",
		Items: []ResumeItem{{
			ID:                "block-1",
			Kind:              ResumeBlockProject,
			Content:           "负责 Go 服务开发。",
			SourceEvidenceIDs: []string{"evidence-1"},
			GroundingLevel:    GroundingSource,
			Optimization:      "按作品集岗位突出交付产物。",
		}},
	}}
	resume.Blocks = nil

	normalized := NormalizeResume(resume)
	if normalized.Blocks[0].ID != "block-1" {
		t.Fatalf("expected v2 item to keep stable review id")
	}
	if normalized.Blocks[0].Kind != ResumeBlockProject {
		t.Fatalf("unexpected compatibility block kind %q", normalized.Blocks[0].Kind)
	}
	markdown := RenderResumeMarkdown(resume)
	if !strings.Contains(markdown, "## 代表作品") {
		t.Fatalf("expected dynamic section title in Markdown:\n%s", markdown)
	}
	if strings.Contains(markdown, "## 项目经历") {
		t.Fatalf("did not expect fixed project title in Markdown:\n%s", markdown)
	}

	edited := strings.Replace(
		markdown,
		"负责 Go 服务开发。",
		"负责 Go 服务开发与作品交付。",
		1,
	)
	updated, err := ApplyResumeMarkdown(resume, edited)
	if err != nil {
		t.Fatalf("apply v2 resume Markdown: %v", err)
	}
	if updated.Sections[0].Title != "代表作品" {
		t.Fatalf("expected dynamic title to survive edit")
	}
	if updated.Sections[0].Items[0].Content != "负责 Go 服务开发与作品交付。" {
		t.Fatalf(
			"expected v2 item content to update, got %q",
			updated.Sections[0].Items[0].Content,
		)
	}
	if updated.Blocks[0].GroundingLevel != GroundingUserConfirmed {
		t.Fatalf("expected edited v2 item to become user confirmed")
	}
}

func TestApplyResumeMarkdownRejectsStructureAndLockedChanges(t *testing.T) {
	resume := validResume()
	markdown := RenderResumeMarkdown(resume)

	withoutMarker := strings.Replace(
		markdown,
		resumeBlockStart(resume.Blocks[0].ID),
		"",
		1,
	)
	if _, err := ApplyResumeMarkdown(resume, withoutMarker); err == nil {
		t.Fatal("expected missing marker error")
	}

	resume.Blocks[0].Locked = true
	edited := strings.Replace(
		markdown,
		"负责 Go 服务开发。",
		"修改后的内容。",
		1,
	)
	if _, err := ApplyResumeMarkdown(resume, edited); err == nil ||
		!strings.Contains(err.Error(), "locked") {
		t.Fatalf("expected locked block error, got %v", err)
	}

	resume.Blocks[0].Locked = false
	injectedMarker := strings.Replace(
		markdown,
		"负责 Go 服务开发。",
		"负责 Go 服务开发。\n<!-- autocv:block:unknown:start -->",
		1,
	)
	if _, err := ApplyResumeMarkdown(resume, injectedMarker); err == nil ||
		!strings.Contains(err.Error(), "reserved block marker") {
		t.Fatalf("expected reserved marker error, got %v", err)
	}
}

func TestValidateResumeForExportRequiresSourceOrConfirmation(t *testing.T) {
	resume := validResume()
	if err := ValidateResumeForExport(resume); err != nil {
		t.Fatalf("expected grounded resume to be exportable: %v", err)
	}

	resume.Blocks[0].SourceEvidenceIDs = nil
	resume.Blocks[0].GroundingLevel = GroundingDerived
	draft := validResumeDraft()
	draft.Blocks[0].SourceEvidenceIDs = nil
	draft.Blocks[0].GroundingLevel = GroundingDerived
	if err := ValidateResumeDraft(draft, nil); err != nil {
		t.Fatalf("expected unconfirmed derived block to remain reviewable: %v", err)
	}
	err := ValidateResumeForExport(resume)
	if err == nil || !strings.Contains(err.Error(), "没有来源") {
		t.Fatalf("expected ungrounded export error, got %v", err)
	}

	resume.Blocks[0].GroundingLevel = GroundingUserConfirmed
	if err := ValidateResumeForExport(resume); err != nil {
		t.Fatalf("expected user-confirmed block to be exportable: %v", err)
	}
}

func validResumeDraft() ResumeDraft {
	return ResumeDraft{
		Language:   ResumeLanguageChinese,
		TargetRole: "后端工程师",
		Blocks: []ResumeBlockDraft{{
			Kind:              ResumeBlockExperience,
			Content:           "负责 Go 服务开发。",
			SourceEvidenceIDs: []string{"evidence-1"},
			GroundingLevel:    GroundingSource,
			Optimization:      "优先展示与目标岗位直接相关的经历。",
		}},
		OptimizationNotes: []string{},
	}
}

func validResume() Resume {
	return Resume{
		ID:                "resume-1",
		RunID:             "run-1",
		InputHash:         "input-1",
		Version:           1,
		Language:          ResumeLanguageChinese,
		TargetRole:        "后端工程师",
		OptimizationNotes: []string{},
		Blocks: []ResumeBlock{{
			ID:                "block-1",
			Kind:              ResumeBlockExperience,
			Content:           "负责 Go 服务开发。",
			SourceEvidenceIDs: []string{"evidence-1"},
			GroundingLevel:    GroundingSource,
			Optimization:      "优先展示与目标岗位直接相关的经历。",
		}},
	}
}
