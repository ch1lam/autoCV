package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

type ResumeLanguage string

const (
	ResumeLanguageChinese ResumeLanguage = "zh"
	ResumeLanguageEnglish ResumeLanguage = "en"
)

type ResumeBlockKind string

const (
	ResumeBlockSummary       ResumeBlockKind = "summary"
	ResumeBlockExperience    ResumeBlockKind = "experience"
	ResumeBlockProject       ResumeBlockKind = "project"
	ResumeBlockSkill         ResumeBlockKind = "skill"
	ResumeBlockEducation     ResumeBlockKind = "education"
	ResumeBlockCertification ResumeBlockKind = "certification"
)

type GroundingLevel string

const (
	GroundingSource        GroundingLevel = "source"
	GroundingDerived       GroundingLevel = "derived"
	GroundingUserConfirmed GroundingLevel = "user_confirmed"
)

type ResumeDraft struct {
	Language          ResumeLanguage     `json:"language"`
	TargetRole        string             `json:"target_role"`
	Blocks            []ResumeBlockDraft `json:"blocks"`
	OptimizationNotes []string           `json:"optimization_notes"`
}

type ResumeRun struct {
	ID             string
	ProfileID      string
	JDID           string
	Status         string
	Stage          string
	PackagingLevel float64
	Language       ResumeLanguage
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type RunScopeMode string

const (
	RunScopeAll      RunScopeMode = "all"
	RunScopeSelected RunScopeMode = "selected"
)

type ResumeRunScope struct {
	ProfileID   string
	JDID        string
	Mode        RunScopeMode
	DocumentIDs []string
	UpdatedAt   time.Time
}

type ResumeBlockDraft struct {
	Kind              ResumeBlockKind `json:"kind"`
	Content           string          `json:"content"`
	SourceEvidenceIDs []string        `json:"source_evidence_ids"`
	GroundingLevel    GroundingLevel  `json:"grounding_level"`
	Optimization      string          `json:"optimization"`
}

type Resume struct {
	ID                string
	RunID             string
	InputHash         string
	Version           int
	SchemaVersion     int
	Language          ResumeLanguage
	TargetRole        string
	Header            ResumeHeader
	Sections          []ResumeSection
	Blocks            []ResumeBlock
	OptimizationNotes []string
	Markdown          string
	CreatedAt         time.Time
}

const (
	ResumeSchemaV1 = 1
	ResumeSchemaV2 = 2
)

type ResumeHeader struct {
	Name       string          `json:"name"`
	TargetRole string          `json:"target_role"`
	Contacts   []ResumeContact `json:"contacts"`
}

type ResumeContact struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Href  string `json:"href,omitempty"`
}

type ResumeSection struct {
	ID     string       `json:"id"`
	Title  string       `json:"title"`
	Intent string       `json:"intent,omitempty"`
	Items  []ResumeItem `json:"items"`
}

type ResumeItem struct {
	ID                string          `json:"id"`
	Kind              ResumeBlockKind `json:"kind"`
	Title             string          `json:"title,omitempty"`
	Subtitle          string          `json:"subtitle,omitempty"`
	Content           string          `json:"content"`
	Locked            bool            `json:"locked"`
	SourceEvidenceIDs []string        `json:"source_evidence_ids"`
	GroundingLevel    GroundingLevel  `json:"grounding_level"`
	Optimization      string          `json:"optimization"`
}

type ResumeBlock struct {
	ID                string          `json:"id"`
	Kind              ResumeBlockKind `json:"kind"`
	Content           string          `json:"content"`
	Locked            bool            `json:"locked"`
	SourceEvidenceIDs []string        `json:"source_evidence_ids"`
	GroundingLevel    GroundingLevel  `json:"grounding_level"`
	Optimization      string          `json:"optimization"`
}

var numericTokenPattern = regexp.MustCompile(
	`(?:^|[^\pL\pN])([+-]?\d+(?:[.,]\d+)*(?:%|\+)?)`,
)

func DecodeResumeDraft(contents []byte) (ResumeDraft, error) {
	var draft ResumeDraft
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&draft); err != nil {
		return ResumeDraft{}, fmt.Errorf("decode resume draft: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ResumeDraft{}, errors.New("decode resume draft: trailing content")
	}
	return draft, nil
}

func ValidateResumeDraft(
	draft ResumeDraft,
	evidence []Evidence,
) error {
	switch draft.Language {
	case ResumeLanguageChinese, ResumeLanguageEnglish:
	default:
		return fmt.Errorf("invalid resume language %q", draft.Language)
	}
	if strings.TrimSpace(draft.TargetRole) == "" {
		return errors.New("resume target role is empty")
	}
	if draft.Blocks == nil || len(draft.Blocks) == 0 {
		return errors.New("resume blocks are empty")
	}
	if draft.OptimizationNotes == nil {
		return errors.New("resume optimization notes are missing")
	}

	evidenceByID := make(map[string]Evidence, len(evidence))
	for _, item := range evidence {
		evidenceByID[item.ID] = item
	}
	for index, block := range draft.Blocks {
		if err := validateResumeBlockDraft(block, evidenceByID); err != nil {
			return fmt.Errorf("resume blocks[%d]: %w", index, err)
		}
	}
	return nil
}

func ValidateResume(resume Resume, evidence []Evidence) error {
	resume = NormalizeResume(resume)
	if strings.TrimSpace(resume.ID) == "" {
		return errors.New("resume id is empty")
	}
	if strings.TrimSpace(resume.RunID) == "" {
		return errors.New("resume run id is empty")
	}
	if strings.TrimSpace(resume.InputHash) == "" {
		return errors.New("resume input hash is empty")
	}
	if resume.Version < 1 {
		return fmt.Errorf("resume version %d is invalid", resume.Version)
	}
	if resume.SchemaVersion != ResumeSchemaV1 &&
		resume.SchemaVersion != ResumeSchemaV2 {
		return fmt.Errorf(
			"resume schema version %d is invalid",
			resume.SchemaVersion,
		)
	}
	if strings.TrimSpace(resume.Header.TargetRole) == "" {
		return errors.New("resume header target role is empty")
	}
	if len(resume.Sections) == 0 {
		return errors.New("resume sections are empty")
	}
	seenSectionIDs := make(map[string]struct{}, len(resume.Sections))
	for sectionIndex, section := range resume.Sections {
		if strings.TrimSpace(section.ID) == "" {
			return fmt.Errorf("resume sections[%d] id is empty", sectionIndex)
		}
		if _, exists := seenSectionIDs[section.ID]; exists {
			return fmt.Errorf("duplicate resume section id %q", section.ID)
		}
		seenSectionIDs[section.ID] = struct{}{}
		if strings.TrimSpace(section.Title) == "" {
			return fmt.Errorf(
				"resume section %q title is empty",
				section.ID,
			)
		}
		if len(section.Items) == 0 {
			return fmt.Errorf(
				"resume section %q items are empty",
				section.ID,
			)
		}
	}
	draft := ResumeDraft{
		Language:          resume.Language,
		TargetRole:        resume.TargetRole,
		Blocks:            make([]ResumeBlockDraft, 0, len(resume.Blocks)),
		OptimizationNotes: resume.OptimizationNotes,
	}
	seenBlockIDs := make(map[string]struct{}, len(resume.Blocks))
	for _, block := range resume.Blocks {
		if strings.TrimSpace(block.ID) == "" {
			return errors.New("resume block id is empty")
		}
		if _, exists := seenBlockIDs[block.ID]; exists {
			return fmt.Errorf("duplicate resume block id %q", block.ID)
		}
		seenBlockIDs[block.ID] = struct{}{}
		draft.Blocks = append(draft.Blocks, ResumeBlockDraft{
			Kind:              block.Kind,
			Content:           block.Content,
			SourceEvidenceIDs: block.SourceEvidenceIDs,
			GroundingLevel:    block.GroundingLevel,
			Optimization:      block.Optimization,
		})
	}
	return ValidateResumeDraft(draft, evidence)
}

func ResumeExportIssues(resume Resume) []string {
	resume = NormalizeResume(resume)
	issues := make([]string, 0)
	for _, block := range resume.Blocks {
		if block.GroundingLevel == GroundingUserConfirmed {
			continue
		}
		if len(block.SourceEvidenceIDs) == 0 {
			issues = append(
				issues,
				fmt.Sprintf(
					"%s内容“%s”没有来源，也未经用户确认",
					resumeBlockLabel(resume.Language, block.Kind),
					resumeBlockIssueExcerpt(block.Content),
				),
			)
		}
	}
	return issues
}

func ValidateResumeForExport(resume Resume) error {
	issues := ResumeExportIssues(resume)
	if len(issues) == 0 {
		return nil
	}
	return fmt.Errorf(
		"resume export blocked: %s",
		strings.Join(issues, "；"),
	)
}

func RenderResumeMarkdown(resume Resume) string {
	resume = NormalizeResume(resume)
	var builder strings.Builder
	builder.WriteString("# ")
	builder.WriteString(strings.TrimSpace(resume.TargetRole))
	builder.WriteString("\n")
	for _, section := range resume.Sections {
		builder.WriteString("\n## ")
		builder.WriteString(strings.TrimSpace(section.Title))
		builder.WriteString("\n\n")
		for _, item := range section.Items {
			block := resumeBlockFromItem(item)
			builder.WriteString(resumeBlockStart(block.ID))
			builder.WriteString("\n")
			builder.WriteString(renderResumeBlockContent(block))
			builder.WriteString("\n")
			builder.WriteString(resumeBlockEnd(block.ID))
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func resumeBlockIssueExcerpt(content string) string {
	content = strings.TrimSpace(content)
	const limit = 24
	runes := []rune(content)
	if len(runes) <= limit {
		return content
	}
	return string(runes[:limit]) + "…"
}

func ApplyResumeMarkdown(
	existing Resume,
	markdown string,
) (Resume, error) {
	existing = NormalizeResume(existing)
	if strings.TrimSpace(markdown) == "" {
		return Resume{}, errors.New("resume Markdown is empty")
	}
	expectedSkeleton, err := resumeMarkdownSkeleton(
		RenderResumeMarkdown(existing),
		existing.Blocks,
	)
	if err != nil {
		return Resume{}, err
	}
	actualSkeleton, err := resumeMarkdownSkeleton(markdown, existing.Blocks)
	if err != nil {
		return Resume{}, err
	}
	if actualSkeleton != expectedSkeleton {
		return Resume{}, errors.New(
			"resume Markdown structure changed outside editable block markers",
		)
	}

	updated := existing
	updated.Blocks = append([]ResumeBlock(nil), existing.Blocks...)
	for index, block := range existing.Blocks {
		body, err := resumeBlockBody(markdown, block.ID)
		if err != nil {
			return Resume{}, err
		}
		content := parseResumeBlockContent(block.Kind, body)
		if strings.TrimSpace(content) == "" {
			return Resume{}, fmt.Errorf(
				"resume block %q content is empty",
				block.ID,
			)
		}
		if block.Locked && content != block.Content {
			return Resume{}, fmt.Errorf(
				"locked resume block %q cannot be changed",
				block.ID,
			)
		}
		if strings.Contains(content, "<!-- autocv:block:") {
			return Resume{}, fmt.Errorf(
				"resume block %q contains a reserved block marker",
				block.ID,
			)
		}
		updated.Blocks[index].Content = content
		if content != block.Content {
			updated.Blocks[index].GroundingLevel = GroundingUserConfirmed
		}
	}
	updated.Sections = updateResumeSectionsFromBlocks(
		updated.Sections,
		updated.Blocks,
	)
	updated.Markdown = markdown
	return updated, nil
}

func NormalizeResume(resume Resume) Resume {
	if resume.SchemaVersion == 0 {
		resume.SchemaVersion = ResumeSchemaV1
	}
	resume.TargetRole = strings.TrimSpace(resume.TargetRole)
	resume.Header.TargetRole = strings.TrimSpace(resume.Header.TargetRole)
	if resume.Header.TargetRole == "" {
		resume.Header.TargetRole = resume.TargetRole
	}
	if resume.TargetRole == "" {
		resume.TargetRole = resume.Header.TargetRole
	}
	if len(resume.Sections) == 0 && len(resume.Blocks) > 0 {
		resume.Sections = resumeSectionsFromBlocks(
			resume.Language,
			resume.Blocks,
		)
	}
	if len(resume.Sections) > 0 && len(resume.Blocks) > 0 {
		resume.Sections = updateResumeSectionsFromBlocks(
			resume.Sections,
			resume.Blocks,
		)
	}
	if len(resume.Blocks) == 0 && len(resume.Sections) > 0 {
		resume.Blocks = resumeBlocksFromSections(resume.Sections)
	}
	if len(resume.Sections) > 0 {
		resume.SchemaVersion = ResumeSchemaV2
	}
	return resume
}

func resumeSectionsFromBlocks(
	language ResumeLanguage,
	blocks []ResumeBlock,
) []ResumeSection {
	sections := make([]ResumeSection, 0, len(blocks))
	for _, block := range blocks {
		sections = append(sections, ResumeSection{
			ID:    "section-" + block.ID,
			Title: resumeBlockLabel(language, block.Kind),
			Items: []ResumeItem{resumeItemFromBlock(block)},
		})
	}
	return sections
}

func resumeBlocksFromSections(sections []ResumeSection) []ResumeBlock {
	blocks := make([]ResumeBlock, 0)
	for _, section := range sections {
		for _, item := range section.Items {
			blocks = append(blocks, resumeBlockFromItem(item))
		}
	}
	return blocks
}

func updateResumeSectionsFromBlocks(
	sections []ResumeSection,
	blocks []ResumeBlock,
) []ResumeSection {
	blocksByID := make(map[string]ResumeBlock, len(blocks))
	for _, block := range blocks {
		blocksByID[block.ID] = block
	}
	updated := append([]ResumeSection(nil), sections...)
	for sectionIndex := range updated {
		updated[sectionIndex].Items = append(
			[]ResumeItem(nil),
			updated[sectionIndex].Items...,
		)
		for itemIndex := range updated[sectionIndex].Items {
			item := updated[sectionIndex].Items[itemIndex]
			block, exists := blocksByID[item.ID]
			if !exists {
				continue
			}
			updated[sectionIndex].Items[itemIndex].Content = block.Content
			updated[sectionIndex].Items[itemIndex].Locked = block.Locked
			updated[sectionIndex].Items[itemIndex].SourceEvidenceIDs = append(
				[]string(nil),
				block.SourceEvidenceIDs...,
			)
			updated[sectionIndex].Items[itemIndex].GroundingLevel =
				block.GroundingLevel
			updated[sectionIndex].Items[itemIndex].Optimization =
				block.Optimization
		}
	}
	return updated
}

func resumeItemFromBlock(block ResumeBlock) ResumeItem {
	return ResumeItem{
		ID:                block.ID,
		Kind:              block.Kind,
		Content:           block.Content,
		Locked:            block.Locked,
		SourceEvidenceIDs: append([]string(nil), block.SourceEvidenceIDs...),
		GroundingLevel:    block.GroundingLevel,
		Optimization:      block.Optimization,
	}
}

func resumeBlockFromItem(item ResumeItem) ResumeBlock {
	return ResumeBlock{
		ID:                item.ID,
		Kind:              item.Kind,
		Content:           item.Content,
		Locked:            item.Locked,
		SourceEvidenceIDs: append([]string(nil), item.SourceEvidenceIDs...),
		GroundingLevel:    item.GroundingLevel,
		Optimization:      item.Optimization,
	}
}

func validateResumeBlockDraft(
	block ResumeBlockDraft,
	evidenceByID map[string]Evidence,
) error {
	switch block.Kind {
	case ResumeBlockSummary,
		ResumeBlockExperience,
		ResumeBlockProject,
		ResumeBlockSkill,
		ResumeBlockEducation,
		ResumeBlockCertification:
	default:
		return fmt.Errorf("invalid resume block kind %q", block.Kind)
	}
	if strings.TrimSpace(block.Content) == "" {
		return errors.New("resume block content is empty")
	}
	switch block.GroundingLevel {
	case GroundingSource:
		if len(block.SourceEvidenceIDs) == 0 {
			return errors.New("source-grounded resume block has no evidence")
		}
	case GroundingDerived:
	case GroundingUserConfirmed:
	default:
		return fmt.Errorf(
			"invalid resume grounding level %q",
			block.GroundingLevel,
		)
	}
	if strings.TrimSpace(block.Optimization) == "" {
		return errors.New("resume block optimization is empty")
	}

	seen := make(map[string]struct{}, len(block.SourceEvidenceIDs))
	sourceText := strings.Builder{}
	for _, evidenceID := range block.SourceEvidenceIDs {
		item, exists := evidenceByID[evidenceID]
		if !exists {
			return fmt.Errorf(
				"resume block references unknown evidence %q",
				evidenceID,
			)
		}
		if _, exists := seen[evidenceID]; exists {
			return fmt.Errorf(
				"resume block repeats evidence %q",
				evidenceID,
			)
		}
		seen[evidenceID] = struct{}{}
		sourceText.WriteString(item.Title)
		sourceText.WriteString("\n")
		sourceText.WriteString(item.Content)
		sourceText.WriteString("\n")
	}
	if block.GroundingLevel != GroundingUserConfirmed {
		for _, match := range numericTokenPattern.FindAllStringSubmatch(
			block.Content,
			-1,
		) {
			token := match[1]
			if !strings.Contains(sourceText.String(), token) {
				return fmt.Errorf(
					"resume block contains ungrounded numeric value %q",
					token,
				)
			}
		}
	}
	return nil
}

func resumeMarkdownSkeleton(
	markdown string,
	blocks []ResumeBlock,
) (string, error) {
	var builder strings.Builder
	cursor := 0
	for _, block := range blocks {
		startMarker := resumeBlockStart(block.ID)
		endMarker := resumeBlockEnd(block.ID)
		if strings.Count(markdown, startMarker) != 1 ||
			strings.Count(markdown, endMarker) != 1 {
			return "", fmt.Errorf(
				"resume block %q markers are missing or duplicated",
				block.ID,
			)
		}
		start := strings.Index(markdown[cursor:], startMarker)
		if start < 0 {
			return "", fmt.Errorf(
				"resume block %q order changed",
				block.ID,
			)
		}
		start += cursor
		bodyStart := start + len(startMarker)
		end := strings.Index(markdown[bodyStart:], endMarker)
		if end < 0 {
			return "", fmt.Errorf(
				"resume block %q end marker is missing",
				block.ID,
			)
		}
		end += bodyStart
		builder.WriteString(markdown[cursor:bodyStart])
		builder.WriteString("\n<autocv-editable-block>\n")
		cursor = end
	}
	builder.WriteString(markdown[cursor:])
	return builder.String(), nil
}

func resumeBlockBody(markdown string, blockID string) (string, error) {
	startMarker := resumeBlockStart(blockID)
	endMarker := resumeBlockEnd(blockID)
	start := strings.Index(markdown, startMarker)
	if start < 0 {
		return "", fmt.Errorf("resume block %q start marker is missing", blockID)
	}
	start += len(startMarker)
	end := strings.Index(markdown[start:], endMarker)
	if end < 0 {
		return "", fmt.Errorf("resume block %q end marker is missing", blockID)
	}
	return strings.TrimSpace(markdown[start : start+end]), nil
}

func renderResumeBlockContent(block ResumeBlock) string {
	content := strings.TrimSpace(block.Content)
	switch block.Kind {
	case ResumeBlockExperience, ResumeBlockProject:
		return "- " + content
	default:
		return content
	}
}

func parseResumeBlockContent(
	kind ResumeBlockKind,
	body string,
) string {
	body = strings.TrimSpace(body)
	switch kind {
	case ResumeBlockExperience, ResumeBlockProject:
		return strings.TrimSpace(strings.TrimPrefix(body, "- "))
	default:
		return body
	}
}

func resumeBlockStart(blockID string) string {
	return "<!-- autocv:block:" + blockID + ":start -->"
}

func resumeBlockEnd(blockID string) string {
	return "<!-- autocv:block:" + blockID + ":end -->"
}

func resumeBlockLabel(
	language ResumeLanguage,
	kind ResumeBlockKind,
) string {
	if language == ResumeLanguageEnglish {
		switch kind {
		case ResumeBlockSummary:
			return "Professional Summary"
		case ResumeBlockExperience:
			return "Experience"
		case ResumeBlockProject:
			return "Projects"
		case ResumeBlockSkill:
			return "Skills"
		case ResumeBlockEducation:
			return "Education"
		case ResumeBlockCertification:
			return "Certifications"
		}
	}
	switch kind {
	case ResumeBlockSummary:
		return "职业概述"
	case ResumeBlockExperience:
		return "工作经历"
	case ResumeBlockProject:
		return "项目经历"
	case ResumeBlockSkill:
		return "技能"
	case ResumeBlockEducation:
		return "教育经历"
	case ResumeBlockCertification:
		return "认证"
	default:
		return string(kind)
	}
}
