package htmlresume

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSidecarRenderHTMLReadsPDFAndPreview(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX shell fake sidecar")
	}
	binary := fakeSidecar(t, `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "autocv-pdf-renderer 0.1.0; weasyprint 69.0; pypdfium2 5.10.1"
  exit 0
fi
if [ "$1" != "render" ]; then
  echo "unexpected command $1" >&2
  exit 2
fi
while [ "$#" -gt 0 ]; do
  case "$1" in
    --html) html="$2"; shift 2 ;;
    --pdf) pdf="$2"; shift 2 ;;
    --preview-dir) preview_dir="$2"; shift 2 ;;
    *) shift ;;
  esac
done
test -s "$html" || exit 3
mkdir -p "$preview_dir"
printf "%%PDF-1.7\nfake\n" > "$pdf"
printf "\211PNG\r\n\032\nfake\n" > "$preview_dir/page-1.png"
`)

	result, err := NewSidecar(binary, time.Second).RenderHTML(
		context.Background(),
		"<!doctype html><html><body>ok</body></html>",
	)
	if err != nil {
		t.Fatalf("render HTML: %v", err)
	}
	if !strings.HasPrefix(string(result.PDF), "%PDF-") {
		t.Fatalf("expected PDF bytes, got %q", string(result.PDF))
	}
	if len(result.PreviewPages) != 1 {
		t.Fatalf("expected one preview page, got %d", len(result.PreviewPages))
	}
	if !strings.Contains(result.RendererVersion, "weasyprint 69.0") {
		t.Fatalf("unexpected renderer version %q", result.RendererVersion)
	}
}

func TestSidecarRenderHTMLRejectsInvalidPDF(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX shell fake sidecar")
	}
	binary := fakeSidecar(t, `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "autocv-pdf-renderer 0.1.0"
  exit 0
fi
while [ "$#" -gt 0 ]; do
  case "$1" in
    --pdf) pdf="$2"; shift 2 ;;
    --preview-dir) preview_dir="$2"; shift 2 ;;
    *) shift ;;
  esac
done
mkdir -p "$preview_dir"
printf "not pdf" > "$pdf"
printf "\211PNG\r\n\032\nfake\n" > "$preview_dir/page-1.png"
`)

	_, err := NewSidecar(binary, time.Second).RenderHTML(
		context.Background(),
		"<!doctype html><html><body>ok</body></html>",
	)
	if err == nil || !strings.Contains(err.Error(), "not a PDF") {
		t.Fatalf("expected invalid PDF error, got %v", err)
	}
}

func fakeSidecar(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "autocv-pdf-renderer")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake sidecar: %v", err)
	}
	return path
}
