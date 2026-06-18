package pdftext

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/ch1lam/autocv/internal/ports"
)

func TestParseExtractsTextWithPageLocators(t *testing.T) {
	result, err := NewWithLimits(100, 1).Parse(textPDF(
		t,
		[]string{"Backend Profile", "Built resilient Go APIs."},
		[]string{"Project Notes", "Reduced latency by 35 percent."},
	))
	if err != nil {
		t.Fatalf("parse PDF: %v", err)
	}
	if result.Metadata.Title != "Untitled PDF" {
		t.Fatalf("unexpected title %q", result.Metadata.Title)
	}
	if len(result.Chunks) != 2 {
		t.Fatalf("expected two page text chunks, got %#v", result.Chunks)
	}
	assertPDFChunk(
		t,
		result.Chunks[0],
		"Backend Profile Built resilient Go APIs.",
		1,
		0,
	)
	assertPDFChunk(
		t,
		result.Chunks[1],
		"Project Notes Reduced latency by 35 percent.",
		2,
		0,
	)
}

func TestParseValidatesThreePDFSources(t *testing.T) {
	parser := NewWithLimits(100, 1)
	fixtures := [][]byte{
		textPDF(t, []string{"Source A", "Backend service notes."}),
		textPDF(t, []string{"Source B page one"}, []string{"Source B page two"}),
		textPDF(t, []string{"Source C", "PostgreSQL tuning notes."}),
	}

	for index, fixture := range fixtures {
		result, err := parser.Parse(fixture)
		if err != nil {
			t.Fatalf("parse fixture %d: %v", index+1, err)
		}
		if len(result.Chunks) == 0 {
			t.Fatalf("expected fixture %d to produce chunks", index+1)
		}
		if result.Chunks[0].Locator.Page < 1 {
			t.Fatalf("expected fixture %d to keep page locator: %#v", index+1, result.Chunks[0])
		}
	}
}

func TestParseReportsLowQualityTextLayer(t *testing.T) {
	result, err := NewWithLimits(100, 20).Parse(textPDF(t, []string{"Hi"}))
	if err != nil {
		t.Fatalf("parse PDF: %v", err)
	}
	if len(result.Chunks) != 1 {
		t.Fatalf("expected one short chunk, got %#v", result.Chunks)
	}
	if !containsWarning(result.Warnings, lowQualityTextWarning) {
		t.Fatalf("expected low quality warning, got %#v", result.Warnings)
	}
}

func TestParseReportsMissingTextLayerAndOCRBoundary(t *testing.T) {
	result, err := New().Parse(emptyTextPDF(t))
	if err != nil {
		t.Fatalf("parse PDF: %v", err)
	}
	if len(result.Chunks) != 0 {
		t.Fatalf("expected no chunks, got %#v", result.Chunks)
	}
	if !containsWarning(result.Warnings, noOCRWarning) {
		t.Fatalf("expected OCR boundary warning, got %#v", result.Warnings)
	}
}

func TestParseSplitsOversizedPDFRows(t *testing.T) {
	result, err := NewWithLimits(24, 1).Parse(textPDF(
		t,
		[]string{"Built order APIs with retry budgets and stable observability."},
	))
	if err != nil {
		t.Fatalf("parse PDF: %v", err)
	}
	if len(result.Chunks) < 2 {
		t.Fatalf("expected split chunks, got %#v", result.Chunks)
	}
	if !containsWarning(result.Warnings, "PDF page 1 row 1 was split into 3 chunks") {
		t.Fatalf("expected split warning, got %#v", result.Warnings)
	}
}

func TestParseInvalidPDF(t *testing.T) {
	if _, err := New().Parse([]byte("not a pdf")); err == nil ||
		!strings.Contains(err.Error(), "open PDF") {
		t.Fatalf("expected open PDF error, got %v", err)
	}
}

func assertPDFChunk(
	t *testing.T,
	chunk ports.ParsedChunk,
	text string,
	page int,
	row int,
) {
	t.Helper()
	if chunk.Text != text ||
		chunk.Kind != "page_text" ||
		chunk.Locator.Page != page ||
		chunk.Locator.Start != row ||
		chunk.Locator.End != row+1 {
		t.Fatalf("unexpected PDF chunk %#v", chunk)
	}
}

func textPDF(t *testing.T, pages ...[]string) []byte {
	t.Helper()
	streams := make([]string, 0, len(pages))
	for _, lines := range pages {
		var stream bytes.Buffer
		stream.WriteString("BT\n/F1 12 Tf\n72 720 Td\n")
		for index, line := range lines {
			if index > 0 {
				stream.WriteString("0 -18 Td\n")
			}
			stream.WriteString("(")
			stream.WriteString(escapePDFText(line))
			stream.WriteString(") Tj\n")
		}
		stream.WriteString("ET\n")
		streams = append(streams, stream.String())
	}
	return buildPDF(t, streams)
}

func emptyTextPDF(t *testing.T) []byte {
	t.Helper()
	return buildPDF(t, []string{""})
}

func buildPDF(t *testing.T, streams []string) []byte {
	t.Helper()
	if len(streams) == 0 {
		streams = []string{""}
	}

	totalObjects := 3 + len(streams)*2
	pageRefs := make([]string, 0, len(streams))
	objects := make([]string, totalObjects+1)
	objects[1] = "<< /Type /Catalog /Pages 2 0 R >>"
	for index := range streams {
		pageObjectID := 4 + index*2
		pageRefs = append(pageRefs, fmt.Sprintf("%d 0 R", pageObjectID))
	}
	objects[2] = fmt.Sprintf(
		"<< /Type /Pages /Kids [%s] /Count %d >>",
		strings.Join(pageRefs, " "),
		len(streams),
	)
	objects[3] = "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"
	for index, stream := range streams {
		pageObjectID := 4 + index*2
		contentObjectID := pageObjectID + 1
		objects[pageObjectID] = fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>",
			contentObjectID,
		)
		objects[contentObjectID] = fmt.Sprintf(
			"<< /Length %d >>\nstream\n%s\nendstream",
			len(stream),
			stream,
		)
	}

	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")
	offsets := make([]int, totalObjects+1)
	for objectID := 1; objectID <= totalObjects; objectID++ {
		offsets[objectID] = pdf.Len()
		pdf.WriteString(strconv.Itoa(objectID))
		pdf.WriteString(" 0 obj\n")
		pdf.WriteString(objects[objectID])
		pdf.WriteString("\nendobj\n")
	}
	xrefOffset := pdf.Len()
	pdf.WriteString("xref\n")
	pdf.WriteString(fmt.Sprintf("0 %d\n", totalObjects+1))
	pdf.WriteString("0000000000 65535 f \n")
	for objectID := 1; objectID <= totalObjects; objectID++ {
		pdf.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[objectID]))
	}
	pdf.WriteString("trailer\n")
	pdf.WriteString(fmt.Sprintf("<< /Size %d /Root 1 0 R >>\n", totalObjects+1))
	pdf.WriteString("startxref\n")
	pdf.WriteString(strconv.Itoa(xrefOffset))
	pdf.WriteString("\n%%EOF\n")
	return pdf.Bytes()
}

func escapePDFText(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "(", "\\(")
	value = strings.ReplaceAll(value, ")", "\\)")
	return value
}

func containsWarning(warnings []string, expected string) bool {
	for _, warning := range warnings {
		if warning == expected {
			return true
		}
	}
	return false
}
