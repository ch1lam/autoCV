package docx

import (
	"archive/zip"
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/ch1lam/autocv/internal/ports"
)

func TestParseChineseDOCXExtractsHeadingBodyListAndTable(t *testing.T) {
	result, err := New().Parse(testDOCX(t, map[string]string{
		documentXMLPath: `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:pPr><w:pStyle w:val="Heading1"/></w:pPr>
      <w:r><w:t>李志林</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t>负责订单系统稳定性优化。</w:t></w:r>
    </w:p>
    <w:p>
      <w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr>
      <w:r><w:t>PostgreSQL 性能优化</w:t></w:r>
    </w:p>
    <w:tbl>
      <w:tr>
        <w:tc><w:p><w:r><w:t>项目</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>结果</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
    <w:p><w:r><w:drawing/></w:r></w:p>
  </w:body>
</w:document>`,
		"word/header1.xml":             `<w:hdr/>`,
		"word/media/image1.png":        "png",
		"docProps/app.xml":             `<Properties/>`,
		"[Content_Types].xml":          `<Types/>`,
		"_rels/.rels":                  `<Relationships/>`,
		"word/_rels/document.xml.rels": `<Relationships/>`,
	}))
	if err != nil {
		t.Fatalf("parse DOCX: %v", err)
	}
	if result.Metadata.Title != "李志林" {
		t.Fatalf("unexpected title %q", result.Metadata.Title)
	}

	assertChunk(t, result.Chunks, 0, ports.ParsedChunk{
		Ordinal: 0,
		Kind:    "heading",
		Text:    "李志林",
		Locator: ports.SourceLocator{
			HeadingPath: []string{"李志林"},
			Start:       0,
			End:         1,
		},
	})
	assertChunk(t, result.Chunks, 1, ports.ParsedChunk{
		Ordinal: 1,
		Kind:    "paragraph",
		Text:    "负责订单系统稳定性优化。",
		Locator: ports.SourceLocator{
			HeadingPath: []string{"李志林"},
			Start:       1,
			End:         2,
		},
	})
	assertChunk(t, result.Chunks, 2, ports.ParsedChunk{
		Ordinal: 2,
		Kind:    "list_item",
		Text:    "PostgreSQL 性能优化",
		Locator: ports.SourceLocator{
			HeadingPath: []string{"李志林"},
			Start:       2,
			End:         3,
		},
	})
	assertChunk(t, result.Chunks, 3, ports.ParsedChunk{
		Ordinal: 3,
		Kind:    "table_row",
		Text:    "项目 | 结果",
		Locator: ports.SourceLocator{
			HeadingPath: []string{"李志林"},
			Start:       3,
			End:         4,
		},
	})

	for _, warning := range []string{
		"DOCX headers are not imported",
		"DOCX embedded images are not imported",
		"DOCX embedded drawings are not imported",
	} {
		if !contains(result.Warnings, warning) {
			t.Fatalf("expected warning %q in %#v", warning, result.Warnings)
		}
	}
}

func TestParseEnglishDOCXUsesCoreTitleWhenNoHeading(t *testing.T) {
	result, err := New().Parse(testDOCX(t, map[string]string{
		documentXMLPath: `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Designed resilient backend APIs.</w:t></w:r></w:p>
    <w:tbl>
      <w:tr>
        <w:tc><w:p><w:r><w:t>Stack</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>Go and PostgreSQL</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`,
		corePropertiesXMLPath: `<?xml version="1.0" encoding="UTF-8"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <dc:title>Backend Profile</dc:title>
</cp:coreProperties>`,
	}))
	if err != nil {
		t.Fatalf("parse DOCX: %v", err)
	}
	if result.Metadata.Title != "Backend Profile" {
		t.Fatalf("unexpected title %q", result.Metadata.Title)
	}
	if len(result.Chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %#v", result.Chunks)
	}
	if result.Chunks[0].Kind != "paragraph" ||
		result.Chunks[0].Text != "Designed resilient backend APIs." {
		t.Fatalf("unexpected paragraph chunk %#v", result.Chunks[0])
	}
	if result.Chunks[1].Kind != "table_row" ||
		result.Chunks[1].Text != "Stack | Go and PostgreSQL" {
		t.Fatalf("unexpected table chunk %#v", result.Chunks[1])
	}
}

func TestParseDOCXReturnsWarningsForEmptyAndTextBoxes(t *testing.T) {
	result, err := New().Parse(testDOCX(t, map[string]string{
		documentXMLPath: `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:txbxContent>
        <w:p><w:r><w:t>Ignored text box</w:t></w:r></w:p>
      </w:txbxContent>
    </w:p>
  </w:body>
</w:document>`,
	}))
	if err != nil {
		t.Fatalf("parse DOCX: %v", err)
	}
	if len(result.Chunks) != 0 {
		t.Fatalf("expected no chunks, got %#v", result.Chunks)
	}
	for _, warning := range []string{
		"DOCX text boxes are not imported",
		"DOCX document has no importable content",
	} {
		if !contains(result.Warnings, warning) {
			t.Fatalf("expected warning %q in %#v", warning, result.Warnings)
		}
	}
}

func TestParseDOCXSplitsOversizedParagraph(t *testing.T) {
	result, err := NewWithMaxChunkRunes(24).Parse(testDOCX(t, map[string]string{
		documentXMLPath: `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Built order APIs with retry budgets and stable observability.</w:t></w:r></w:p>
  </w:body>
</w:document>`,
	}))
	if err != nil {
		t.Fatalf("parse DOCX: %v", err)
	}
	if len(result.Chunks) < 2 {
		t.Fatalf("expected split chunks, got %#v", result.Chunks)
	}
	if !contains(result.Warnings, "paragraph content at block 0 was split into 3 chunks") {
		t.Fatalf("expected split warning, got %#v", result.Warnings)
	}
}

func TestParseInvalidDOCX(t *testing.T) {
	if _, err := New().Parse([]byte("not a zip")); err == nil ||
		!strings.Contains(err.Error(), "open DOCX package") {
		t.Fatalf("expected open package error, got %v", err)
	}

	if _, err := New().Parse(testDOCX(t, map[string]string{
		"[Content_Types].xml": `<Types/>`,
	})); err == nil || !strings.Contains(err.Error(), documentXMLPath) {
		t.Fatalf("expected missing document XML error, got %v", err)
	}
}

func testDOCX(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, contents := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := io.WriteString(file, contents); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buffer.Bytes()
}

func assertChunk(
	t *testing.T,
	chunks []ports.ParsedChunk,
	index int,
	expected ports.ParsedChunk,
) {
	t.Helper()
	if len(chunks) <= index {
		t.Fatalf("expected chunk %d in %#v", index, chunks)
	}
	if !reflect.DeepEqual(chunks[index], expected) {
		t.Fatalf("expected chunk %#v, got %#v", expected, chunks[index])
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
