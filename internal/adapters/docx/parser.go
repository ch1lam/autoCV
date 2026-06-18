package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ch1lam/autocv/internal/ports"
)

const (
	DefaultMaxChunkRunes = 1200

	documentXMLPath       = "word/document.xml"
	corePropertiesXMLPath = "docProps/core.xml"
)

type Parser struct {
	maxChunkRunes int
}

type parsedDocument struct {
	title    string
	chunks   []ports.ParsedChunk
	warnings []string
}

type paragraph struct {
	text         string
	isHeading    bool
	headingLevel int
	isList       bool
}

type table struct {
	rows []tableRow
}

type tableRow struct {
	cells []string
}

func New() *Parser {
	return NewWithMaxChunkRunes(DefaultMaxChunkRunes)
}

func NewWithMaxChunkRunes(maxChunkRunes int) *Parser {
	if maxChunkRunes < 1 {
		maxChunkRunes = DefaultMaxChunkRunes
	}
	return &Parser{maxChunkRunes: maxChunkRunes}
}

func (parser *Parser) Parse(contents []byte) (ports.ParseResult, error) {
	if len(contents) == 0 {
		return ports.ParseResult{}, errors.New("DOCX file is empty")
	}

	reader, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		return ports.ParseResult{}, fmt.Errorf("open DOCX package: %w", err)
	}
	documentXML, err := readPackageFile(reader, documentXMLPath)
	if err != nil {
		return ports.ParseResult{}, fmt.Errorf("read %s: %w", documentXMLPath, err)
	}

	parsed, err := parser.parseDocumentXML(documentXML)
	if err != nil {
		return ports.ParseResult{}, err
	}

	result := ports.ParseResult{
		Metadata: ports.DocumentMetadata{
			Title: strings.TrimSpace(parsed.title),
		},
		Chunks:   parsed.chunks,
		Warnings: packageWarnings(reader),
	}
	result.Warnings = appendWarnings(result.Warnings, parsed.warnings...)
	if result.Metadata.Title == "" {
		result.Metadata.Title = readCoreTitle(reader)
	}
	if result.Metadata.Title == "" {
		result.Metadata.Title = "Untitled DOCX"
	}
	if len(result.Chunks) == 0 {
		result.Warnings = appendWarnings(
			result.Warnings,
			"DOCX document has no importable content",
		)
	}
	return result, nil
}

func (parser *Parser) parseDocumentXML(contents []byte) (parsedDocument, error) {
	decoder := xmlDecoder(contents)
	document := parsedDocument{
		chunks: make([]ports.ParsedChunk, 0),
	}
	headingPath := make([]string, 0, 6)
	blockIndex := 0

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return parsedDocument{}, fmt.Errorf("parse DOCX document XML: %w", err)
		}

		start, ok := token.(xmlStartElement)
		if !ok {
			continue
		}

		switch localName(start.Name) {
		case "p":
			paragraph, warnings, err := parseParagraph(decoder, start)
			if err != nil {
				return parsedDocument{}, err
			}
			document.warnings = appendWarnings(document.warnings, warnings...)
			text := cleanText(paragraph.text)
			if text == "" {
				blockIndex++
				continue
			}

			kind := "paragraph"
			if paragraph.isHeading {
				kind = "heading"
				headingPath = updateHeadingPath(
					headingPath,
					paragraph.headingLevel,
					text,
				)
				if document.title == "" {
					document.title = text
				}
			} else if paragraph.isList {
				kind = "list_item"
			}
			parser.appendChunks(
				&document,
				kind,
				text,
				headingPath,
				blockIndex,
				blockIndex+1,
			)
			blockIndex++
		case "tbl":
			table, warnings, err := parser.parseTable(decoder, start)
			if err != nil {
				return parsedDocument{}, err
			}
			document.warnings = appendWarnings(document.warnings, warnings...)
			if len(table.rows) == 0 {
				blockIndex++
				continue
			}
			for rowIndex, row := range table.rows {
				text := cleanText(strings.Join(row.cells, " | "))
				if text == "" {
					continue
				}
				parser.appendChunks(
					&document,
					"table_row",
					text,
					headingPath,
					blockIndex+rowIndex,
					blockIndex+rowIndex+1,
				)
			}
			blockIndex += len(table.rows)
		}
	}
	return document, nil
}

func (parser *Parser) parseTable(
	decoder *xmlDecoderType,
	start xmlStartElement,
) (table, []string, error) {
	var parsed table
	var warnings []string
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return table{}, nil, errors.New("parse DOCX table: unexpected EOF")
		}
		if err != nil {
			return table{}, nil, fmt.Errorf("parse DOCX table: %w", err)
		}

		switch value := token.(type) {
		case xmlStartElement:
			if localName(value.Name) == "tr" {
				row, rowWarnings, err := parser.parseTableRow(decoder, value)
				if err != nil {
					return table{}, nil, err
				}
				warnings = appendWarnings(warnings, rowWarnings...)
				if len(row.cells) > 0 {
					parsed.rows = append(parsed.rows, row)
				}
			}
		case xmlEndElement:
			if sameElement(value.Name, start.Name) {
				return parsed, warnings, nil
			}
		}
	}
}

func (parser *Parser) parseTableRow(
	decoder *xmlDecoderType,
	start xmlStartElement,
) (tableRow, []string, error) {
	var row tableRow
	var warnings []string
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return tableRow{}, nil, errors.New("parse DOCX table row: unexpected EOF")
		}
		if err != nil {
			return tableRow{}, nil, fmt.Errorf("parse DOCX table row: %w", err)
		}

		switch value := token.(type) {
		case xmlStartElement:
			if localName(value.Name) == "tc" {
				cellText, cellWarnings, err := parser.parseTableCell(decoder, value)
				if err != nil {
					return tableRow{}, nil, err
				}
				warnings = appendWarnings(warnings, cellWarnings...)
				if cellText != "" {
					row.cells = append(row.cells, cellText)
				}
			}
		case xmlEndElement:
			if sameElement(value.Name, start.Name) {
				return row, warnings, nil
			}
		}
	}
}

func (parser *Parser) parseTableCell(
	decoder *xmlDecoderType,
	start xmlStartElement,
) (string, []string, error) {
	var parts []string
	var warnings []string
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return "", nil, errors.New("parse DOCX table cell: unexpected EOF")
		}
		if err != nil {
			return "", nil, fmt.Errorf("parse DOCX table cell: %w", err)
		}

		switch value := token.(type) {
		case xmlStartElement:
			switch localName(value.Name) {
			case "p":
				paragraph, paragraphWarnings, err := parseParagraph(
					decoder,
					value,
				)
				if err != nil {
					return "", nil, err
				}
				warnings = appendWarnings(warnings, paragraphWarnings...)
				if text := cleanText(paragraph.text); text != "" {
					parts = append(parts, text)
				}
			case "tbl":
				nested, nestedWarnings, err := parser.parseTable(decoder, value)
				if err != nil {
					return "", nil, err
				}
				warnings = appendWarnings(warnings, nestedWarnings...)
				for _, row := range nested.rows {
					if text := cleanText(strings.Join(row.cells, " | ")); text != "" {
						parts = append(parts, text)
					}
				}
			}
		case xmlEndElement:
			if sameElement(value.Name, start.Name) {
				return cleanText(strings.Join(parts, " ")), warnings, nil
			}
		}
	}
}

func parseParagraph(
	decoder *xmlDecoderType,
	start xmlStartElement,
) (paragraph, []string, error) {
	var parsed paragraph
	var text strings.Builder
	var warnings []string

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return paragraph{}, nil, errors.New("parse DOCX paragraph: unexpected EOF")
		}
		if err != nil {
			return paragraph{}, nil, fmt.Errorf("parse DOCX paragraph: %w", err)
		}

		switch value := token.(type) {
		case xmlStartElement:
			switch localName(value.Name) {
			case "pStyle":
				if level, ok := headingLevel(attrValue(value, "val")); ok {
					parsed.isHeading = true
					parsed.headingLevel = level
				}
			case "numPr":
				parsed.isList = true
			case "t":
				var runText string
				if err := decoder.DecodeElement(&runText, &value); err != nil {
					return paragraph{}, nil, fmt.Errorf(
						"parse DOCX text run: %w",
						err,
					)
				}
				text.WriteString(runText)
			case "tab":
				text.WriteByte('\t')
			case "br", "cr":
				text.WriteByte('\n')
			case "drawing":
				warnings = appendWarnings(
					warnings,
					"DOCX embedded drawings are not imported",
				)
				if err := decoder.Skip(); err != nil {
					return paragraph{}, nil, fmt.Errorf(
						"skip DOCX drawing: %w",
						err,
					)
				}
			case "pict", "object":
				warnings = appendWarnings(
					warnings,
					"DOCX embedded objects are not imported",
				)
				if err := decoder.Skip(); err != nil {
					return paragraph{}, nil, fmt.Errorf(
						"skip DOCX object: %w",
						err,
					)
				}
			case "txbxContent":
				warnings = appendWarnings(
					warnings,
					"DOCX text boxes are not imported",
				)
				if err := decoder.Skip(); err != nil {
					return paragraph{}, nil, fmt.Errorf(
						"skip DOCX text box: %w",
						err,
					)
				}
			}
		case xmlEndElement:
			if sameElement(value.Name, start.Name) {
				parsed.text = text.String()
				if parsed.headingLevel == 0 {
					parsed.headingLevel = 1
				}
				return parsed, warnings, nil
			}
		}
	}
}

func (parser *Parser) appendChunks(
	document *parsedDocument,
	kind string,
	text string,
	headingPath []string,
	start int,
	end int,
) {
	parts := splitText(text, parser.maxChunkRunes)
	if len(parts) > 1 {
		document.warnings = appendWarnings(
			document.warnings,
			fmt.Sprintf(
				"%s content at block %d was split into %d chunks",
				kind,
				start,
				len(parts),
			),
		)
	}

	for _, part := range parts {
		document.chunks = append(document.chunks, ports.ParsedChunk{
			Ordinal: len(document.chunks),
			Kind:    kind,
			Text:    part,
			Locator: ports.SourceLocator{
				HeadingPath: append([]string(nil), headingPath...),
				Start:       start,
				End:         end,
			},
		})
	}
}

func readPackageFile(reader *zip.Reader, name string) ([]byte, error) {
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		opened, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer opened.Close()
		return io.ReadAll(opened)
	}
	return nil, fmt.Errorf("file %q not found", name)
}

type xmlDecoderType = xml.Decoder
type xmlStartElement = xml.StartElement
type xmlEndElement = xml.EndElement

func xmlDecoder(contents []byte) *xml.Decoder {
	return xml.NewDecoder(bytes.NewReader(contents))
}

func localName(name xml.Name) string {
	return name.Local
}

func sameElement(left xml.Name, right xml.Name) bool {
	return left.Local == right.Local
}

func attrValue(start xml.StartElement, local string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

func packageWarnings(reader *zip.Reader) []string {
	var warnings []string
	for _, file := range reader.File {
		name := filepathSlash(file.Name)
		if strings.HasPrefix(name, "word/header") &&
			strings.HasSuffix(name, ".xml") {
			warnings = appendWarnings(
				warnings,
				"DOCX headers are not imported",
			)
		}
		if strings.HasPrefix(name, "word/footer") &&
			strings.HasSuffix(name, ".xml") {
			warnings = appendWarnings(
				warnings,
				"DOCX footers are not imported",
			)
		}
		if strings.HasPrefix(name, "word/media/") {
			warnings = appendWarnings(
				warnings,
				"DOCX embedded images are not imported",
			)
		}
	}
	return warnings
}

func readCoreTitle(reader *zip.Reader) string {
	contents, err := readPackageFile(reader, corePropertiesXMLPath)
	if err != nil {
		return ""
	}

	decoder := xmlDecoder(contents)
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		start, ok := token.(xmlStartElement)
		if !ok || localName(start.Name) != "title" {
			continue
		}
		var title string
		if err := decoder.DecodeElement(&title, &start); err != nil {
			return ""
		}
		return strings.TrimSpace(title)
	}
}

func headingLevel(style string) (int, bool) {
	normalized := strings.Map(func(value rune) rune {
		if unicode.IsSpace(value) || value == '-' || value == '_' {
			return -1
		}
		return unicode.ToLower(value)
	}, strings.TrimSpace(style))
	if normalized == "title" {
		return 1, true
	}
	if !strings.HasPrefix(normalized, "heading") {
		return 0, false
	}

	suffix := strings.TrimPrefix(normalized, "heading")
	if suffix == "" {
		return 1, true
	}
	level, err := strconv.Atoi(suffix)
	if err != nil || level < 1 {
		return 1, true
	}
	if level > 6 {
		return 6, true
	}
	return level, true
}

func updateHeadingPath(current []string, level int, title string) []string {
	if level < 1 {
		level = 1
	}
	if level > len(current)+1 {
		level = len(current) + 1
	}
	next := append([]string(nil), current[:level-1]...)
	return append(next, title)
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

func appendWarnings(existing []string, warnings ...string) []string {
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		if warning == "" {
			continue
		}
		alreadyAdded := false
		for _, current := range existing {
			if current == warning {
				alreadyAdded = true
				break
			}
		}
		if !alreadyAdded {
			existing = append(existing, warning)
		}
	}
	return existing
}

func filepathSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

var _ ports.DocumentParser = (*Parser)(nil)
