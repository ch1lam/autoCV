package markdown

import (
	"strings"
	"testing"
)

func TestParsePreservesHeadingContextAndListItems(t *testing.T) {
	source := []byte(`# 李志林

后端工程师，专注 Go 服务开发。

## 工作经历

负责订单服务和稳定性建设。

- 将核心链路拆分为可独立发布的服务
- Wrote English design documents with the team
`)

	result, err := New().Parse(source)
	if err != nil {
		t.Fatalf("parse Markdown: %v", err)
	}
	if result.Metadata.Title != "李志林" {
		t.Fatalf("unexpected title %q", result.Metadata.Title)
	}
	if len(result.Chunks) != 4 {
		t.Fatalf("expected four chunks, got %d", len(result.Chunks))
	}

	if result.Chunks[0].Text != "后端工程师，专注 Go 服务开发。" {
		t.Fatalf("unexpected first chunk %q", result.Chunks[0].Text)
	}
	if len(result.Chunks[0].Locator.HeadingPath) != 1 ||
		result.Chunks[0].Locator.HeadingPath[0] != "李志林" {
		t.Fatalf("unexpected first heading path %#v", result.Chunks[0].Locator.HeadingPath)
	}
	if result.Chunks[2].Kind != "list_item" {
		t.Fatalf("expected list item chunk, got %q", result.Chunks[2].Kind)
	}
	if strings.Join(result.Chunks[2].Locator.HeadingPath, "/") != "李志林/工作经历" {
		t.Fatalf("unexpected list heading path %#v", result.Chunks[2].Locator.HeadingPath)
	}
}

func TestParseEmptyMarkdownReturnsWarning(t *testing.T) {
	result, err := New().Parse(nil)
	if err != nil {
		t.Fatalf("parse empty Markdown: %v", err)
	}
	if len(result.Chunks) != 0 {
		t.Fatalf("expected no chunks")
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected one warning, got %d", len(result.Warnings))
	}
}

func TestParseSplitsOversizedParagraph(t *testing.T) {
	source := []byte("# Profile\n\n" + strings.Repeat("Go 服务优化 ", 20))
	result, err := NewWithMaxChunkRunes(30).Parse(source)
	if err != nil {
		t.Fatalf("parse oversized Markdown: %v", err)
	}
	if len(result.Chunks) < 2 {
		t.Fatalf("expected oversized paragraph to split")
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected split warning, got %#v", result.Warnings)
	}
	for _, chunk := range result.Chunks {
		if len([]rune(chunk.Text)) > 30 {
			t.Fatalf("chunk exceeds maximum length: %q", chunk.Text)
		}
	}
}
