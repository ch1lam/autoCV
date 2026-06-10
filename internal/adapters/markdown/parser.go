package markdown

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ch1lam/autocv/internal/ports"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

const DefaultMaxChunkRunes = 1200

type Parser struct {
	markdown      goldmark.Markdown
	maxChunkRunes int
}

func New() *Parser {
	return NewWithMaxChunkRunes(DefaultMaxChunkRunes)
}

func NewWithMaxChunkRunes(maxChunkRunes int) *Parser {
	if maxChunkRunes < 1 {
		maxChunkRunes = DefaultMaxChunkRunes
	}
	return &Parser{
		markdown:      goldmark.New(goldmark.WithExtensions(extension.GFM)),
		maxChunkRunes: maxChunkRunes,
	}
}

func (parser *Parser) Parse(contents []byte) (ports.ParseResult, error) {
	document := parser.markdown.Parser().Parse(text.NewReader(contents))
	result := ports.ParseResult{
		Chunks: make([]ports.ParsedChunk, 0),
	}
	headingPath := make([]string, 0, 6)

	for node := document.FirstChild(); node != nil; node = node.NextSibling() {
		if heading, ok := node.(*ast.Heading); ok {
			title := extractText(heading, contents)
			headingPath = updateHeadingPath(headingPath, heading.Level, title)
			if result.Metadata.Title == "" {
				result.Metadata.Title = title
			}
			continue
		}

		if list, ok := node.(*ast.List); ok {
			for item := list.FirstChild(); item != nil; item = item.NextSibling() {
				parser.appendNodeChunks(
					&result,
					item,
					"list_item",
					headingPath,
					contents,
				)
			}
			continue
		}

		parser.appendNodeChunks(
			&result,
			node,
			nodeKind(node),
			headingPath,
			contents,
		)
	}

	if result.Metadata.Title == "" {
		result.Metadata.Title = "Untitled Markdown"
	}
	if len(result.Chunks) == 0 {
		result.Warnings = append(result.Warnings, "Markdown document has no content")
	}
	return result, nil
}

func (parser *Parser) appendNodeChunks(
	result *ports.ParseResult,
	node ast.Node,
	kind string,
	headingPath []string,
	source []byte,
) {
	content := strings.TrimSpace(extractText(node, source))
	if content == "" {
		return
	}

	start, end := sourceRange(node)
	parts := splitText(content, parser.maxChunkRunes)
	if len(parts) > 1 {
		result.Warnings = append(
			result.Warnings,
			fmt.Sprintf(
				"%s content at byte %d was split into %d chunks",
				kind,
				start,
				len(parts),
			),
		)
	}

	for _, part := range parts {
		result.Chunks = append(result.Chunks, ports.ParsedChunk{
			Ordinal: len(result.Chunks),
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

func nodeKind(node ast.Node) string {
	name := strings.ToLower(node.Kind().String())
	if name == "" {
		return "block"
	}
	return name
}

func extractText(root ast.Node, source []byte) string {
	var builder strings.Builder
	_ = ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch value := node.(type) {
		case *ast.Text:
			builder.Write(value.Segment.Value(source))
			if value.SoftLineBreak() || value.HardLineBreak() {
				builder.WriteByte(' ')
			}
		case *ast.String:
			builder.Write(value.Value)
		case *ast.Paragraph:
			writeSeparator(&builder)
		case *ast.ListItem:
			if node != root {
				writeSeparator(&builder)
			}
		}
		return ast.WalkContinue, nil
	})
	return strings.Join(strings.Fields(builder.String()), " ")
}

func writeSeparator(builder *strings.Builder) {
	if builder.Len() > 0 {
		builder.WriteByte('\n')
	}
}

func sourceRange(root ast.Node) (int, int) {
	start := -1
	end := -1
	_ = ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if value, ok := node.(*ast.Text); ok {
			start, end = includeRange(start, end, value.Segment.Start, value.Segment.Stop)
		}
		if node.Type() == ast.TypeBlock {
			lines := node.Lines()
			for index := 0; index < lines.Len(); index++ {
				segment := lines.At(index)
				start, end = includeRange(start, end, segment.Start, segment.Stop)
			}
		}
		return ast.WalkContinue, nil
	})
	if start < 0 {
		return 0, 0
	}
	return start, end
}

func includeRange(start, end, candidateStart, candidateEnd int) (int, int) {
	if start < 0 || candidateStart < start {
		start = candidateStart
	}
	if candidateEnd > end {
		end = candidateEnd
	}
	return start, end
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

var _ ports.DocumentParser = (*Parser)(nil)
