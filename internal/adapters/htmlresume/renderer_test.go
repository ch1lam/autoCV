package htmlresume

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/ports"
)

type fixtureComposer struct{}

func (fixtureComposer) ComposeResumeHTML(
	_ context.Context,
	request ports.ComposeResumeHTMLRequest,
) (ports.ComposedResumeHTML, error) {
	html, err := replaceBodyForRendererTest(request.Template.HTML, validBody())
	if err != nil {
		return ports.ComposedResumeHTML{}, err
	}
	return ports.ComposedResumeHTML{
		HTML:            html,
		TemplateID:      request.Template.ID,
		TemplateVersion: request.Template.Version,
		Composer:        "fixture",
		ComposerVersion: "v1",
		PromptVersion:   "test",
	}, nil
}

func (fixtureComposer) CacheKey() string {
	return "fixture/v1"
}

func TestRendererComposesValidatesAndRendersHTML(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX shell fake sidecar")
	}
	binary := fakeSidecar(t, `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "autocv-pdf-renderer 0.1.0; weasyprint 69.0; pypdfium2 5.10.1"
  exit 0
fi
while [ "$#" -gt 0 ]; do
  case "$1" in
    --html) html="$2"; shift 2 ;;
    --pdf) pdf="$2"; shift 2 ;;
    --preview-dir) preview_dir="$2"; shift 2 ;;
    *) shift ;;
  esac
done
grep -q "data-autocv-id=\"item-1\"" "$html" || exit 4
mkdir -p "$preview_dir"
printf "%%PDF-1.7\nfake\n" > "$pdf"
printf "\211PNG\r\n\032\nfake\n" > "$preview_dir/page-1.png"
`)
	renderer, err := NewRenderer(
		fixtureComposer{},
		NewSidecar(binary, time.Second),
	)
	if err != nil {
		t.Fatalf("create renderer: %v", err)
	}

	rendered, err := renderer.Render(context.Background(), validResume())
	if err != nil {
		t.Fatalf("render resume: %v", err)
	}
	if !strings.HasPrefix(string(rendered.PDF), "%PDF-") {
		t.Fatalf("expected PDF output")
	}
	if len(rendered.PreviewPages) != 1 {
		t.Fatalf("expected one preview page, got %d", len(rendered.PreviewPages))
	}
	if rendered.Metadata.Renderer != "weasyprint-html" {
		t.Fatalf("unexpected renderer metadata %#v", rendered.Metadata)
	}
	if rendered.Metadata.HTMLTemplateID == "" ||
		rendered.Metadata.HTMLHash == "" ||
		rendered.Metadata.HTMLStyleHash == "" ||
		rendered.Metadata.Composer != "fixture" {
		t.Fatalf("missing HTML metadata %#v", rendered.Metadata)
	}
	if renderer.CacheKey() == "" {
		t.Fatalf("expected cache key")
	}
}

var _ ports.ResumeHTMLComposer = fixtureComposer{}
var _ ports.ResumeRenderer = (*Renderer)(nil)
var _ ports.VersionedResumeRenderer = (*Renderer)(nil)

func replaceBodyForRendererTest(template string, body string) (string, error) {
	start := strings.Index(template, "<body>")
	end := strings.Index(template, "</body>")
	if start < 0 || end < 0 || end < start {
		return "", fmt.Errorf("template body is missing")
	}
	return template[:start+len("<body>")] + "\n" + body + "\n" + template[end:], nil
}
