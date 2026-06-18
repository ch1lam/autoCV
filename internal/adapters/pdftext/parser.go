package pdftext

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ch1lam/autocv/internal/ports"
	"github.com/ledongthuc/pdf"
)

const (
	DefaultMaxChunkRunes     = 1200
	DefaultMinimumTextRunes  = 40
	noOCRWarning             = "PDF has no readable text layer; scanned PDFs are not supported and OCR is not available"
	lowQualityTextWarning    = "PDF text layer is too small or low quality; scanned PDFs are not supported and OCR is not available"
	pageWithoutTextWarning   = "One or more PDF pages had no readable text layer and were skipped"
	pageExtractionWarningFmt = "PDF page %d text extraction failed: %v"
)

type Parser struct {
	maxChunkRunes   int
	minimumTextRune int
}

func New() *Parser {
	return NewWithLimits(DefaultMaxChunkRunes, DefaultMinimumTextRunes)
}

func NewWithLimits(maxChunkRunes int, minimumTextRunes int) *Parser {
	if maxChunkRunes < 1 {
		maxChunkRunes = DefaultMaxChunkRunes
	}
	if minimumTextRunes < 1 {
		minimumTextRunes = DefaultMinimumTextRunes
	}
	return &Parser{
		maxChunkRunes:   maxChunkRunes,
		minimumTextRune: minimumTextRunes,
	}
}

func (parser *Parser) Parse(contents []byte) (ports.ParseResult, error) {
	if len(contents) == 0 {
		return ports.ParseResult{}, errors.New("PDF file is empty")
	}

	reader, err := pdf.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		return ports.ParseResult{}, fmt.Errorf("open PDF: %w", err)
	}

	result := ports.ParseResult{
		Metadata: ports.DocumentMetadata{Title: "Untitled PDF"},
		Chunks:   make([]ports.ParsedChunk, 0),
	}
	totalPages := reader.NumPage()
	if totalPages == 0 {
		result.Warnings = append(result.Warnings, noOCRWarning)
		return result, nil
	}

	var totalTextRunes int
	var skippedPages int
	for pageNumber := 1; pageNumber <= totalPages; pageNumber++ {
		page := reader.Page(pageNumber)
		rows, err := page.GetTextByRow()
		if err != nil {
			result.Warnings = appendWarning(
				result.Warnings,
				fmt.Sprintf(pageExtractionWarningFmt, pageNumber, err),
			)
			rows = nil
		}

		pageChunkCount := len(result.Chunks)
		for rowIndex, row := range rows {
			text := rowText(row.Content)
			if text == "" {
				continue
			}
			totalTextRunes += utf8.RuneCountInString(text)
			parser.appendChunks(
				&result,
				text,
				pageNumber,
				rowIndex,
			)
		}
		if len(result.Chunks) == pageChunkCount {
			skippedPages++
		}
	}

	if len(result.Chunks) == 0 {
		result.Warnings = appendWarning(result.Warnings, noOCRWarning)
		return result, nil
	}
	if skippedPages > 0 {
		result.Warnings = appendWarning(result.Warnings, pageWithoutTextWarning)
	}
	if totalTextRunes < parser.minimumTextRune {
		result.Warnings = appendWarning(result.Warnings, lowQualityTextWarning)
	}
	return result, nil
}

func (parser *Parser) appendChunks(
	result *ports.ParseResult,
	text string,
	pageNumber int,
	rowIndex int,
) {
	parts := splitText(text, parser.maxChunkRunes)
	if len(parts) > 1 {
		result.Warnings = appendWarning(
			result.Warnings,
			fmt.Sprintf(
				"PDF page %d row %d was split into %d chunks",
				pageNumber,
				rowIndex+1,
				len(parts),
			),
		)
	}

	for _, part := range parts {
		result.Chunks = append(result.Chunks, ports.ParsedChunk{
			Ordinal: len(result.Chunks),
			Kind:    "page_text",
			Text:    part,
			Locator: ports.SourceLocator{
				Page:  pageNumber,
				Start: rowIndex,
				End:   rowIndex + 1,
			},
		})
	}
}

func rowText(values pdf.TextHorizontal) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		text := strings.TrimSpace(value.S)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return cleanText(strings.Join(parts, " "))
}

func cleanText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func splitText(content string, maxRunes int) []string {
	if utf8.RuneCountInString(content) <= maxRunes {
		return []string{content}
	}

	runes := []rune(content)
	parts := make([]string, 0, len(runes)/maxRunes+1)
	for len(runes) > maxRunes {
		splitAt := maxRunes
		for index := maxRunes; index > maxRunes/2; index-- {
			if unicode.IsSpace(runes[index-1]) {
				splitAt = index
				break
			}
		}
		part := strings.TrimSpace(string(runes[:splitAt]))
		if part != "" {
			parts = append(parts, part)
		}
		runes = runes[splitAt:]
		for len(runes) > 0 && unicode.IsSpace(runes[0]) {
			runes = runes[1:]
		}
	}
	if remaining := strings.TrimSpace(string(runes)); remaining != "" {
		parts = append(parts, remaining)
	}
	return parts
}

func appendWarning(existing []string, warning string) []string {
	warning = strings.TrimSpace(warning)
	if warning == "" {
		return existing
	}
	for _, current := range existing {
		if current == warning {
			return existing
		}
	}
	return append(existing, warning)
}

var _ ports.DocumentParser = (*Parser)(nil)
