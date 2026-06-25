package htmlresume

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	DefaultRendererBinary     = "autocv-pdf-renderer"
	ExpectedRendererVersion   = "autocv-pdf-renderer 0.1.0"
	defaultRendererTimeout    = 30 * time.Second
	rendererHTMLFilename      = "resume.html"
	rendererPDFFilename       = "resume.pdf"
	rendererPreviewDirName    = "previews"
	rendererPreviewGlob       = "page-*.png"
	rendererDiagnosticMaxSize = 1200
)

type Sidecar struct {
	binary  string
	timeout time.Duration
}

type SidecarResult struct {
	PDF             []byte
	PreviewPages    [][]byte
	RendererVersion string
}

func NewSidecar(binary string, timeout time.Duration) *Sidecar {
	if strings.TrimSpace(binary) == "" {
		binary = DefaultRendererBinary
	}
	if timeout <= 0 {
		timeout = defaultRendererTimeout
	}
	return &Sidecar{
		binary:  strings.TrimSpace(binary),
		timeout: timeout,
	}
}

func (sidecar *Sidecar) RenderHTML(
	ctx context.Context,
	html string,
) (SidecarResult, error) {
	directory, err := os.MkdirTemp("", "autocv-html-render-*")
	if err != nil {
		return SidecarResult{}, fmt.Errorf(
			"create HTML render temporary directory: %w",
			err,
		)
	}
	defer os.RemoveAll(directory)

	htmlPath := filepath.Join(directory, rendererHTMLFilename)
	pdfPath := filepath.Join(directory, rendererPDFFilename)
	previewDirectory := filepath.Join(directory, rendererPreviewDirName)
	if err := os.WriteFile(htmlPath, []byte(html), 0o600); err != nil {
		return SidecarResult{}, fmt.Errorf("write resume HTML: %w", err)
	}
	if err := os.MkdirAll(previewDirectory, 0o700); err != nil {
		return SidecarResult{}, fmt.Errorf("create preview directory: %w", err)
	}

	renderContext, cancel := context.WithTimeout(ctx, sidecar.timeout)
	defer cancel()
	if err := sidecar.run(
		renderContext,
		"render",
		"--html",
		htmlPath,
		"--pdf",
		pdfPath,
		"--preview-dir",
		previewDirectory,
	); err != nil {
		return SidecarResult{}, err
	}
	version, err := sidecar.version(renderContext)
	if err != nil {
		return SidecarResult{}, err
	}
	if renderContext.Err() != nil {
		return SidecarResult{}, fmt.Errorf(
			"HTML renderer timed out after %s",
			sidecar.timeout,
		)
	}

	pdf, err := os.ReadFile(pdfPath)
	if err != nil {
		return SidecarResult{}, fmt.Errorf("read rendered PDF: %w", err)
	}
	if len(pdf) < 5 || string(pdf[:5]) != "%PDF-" {
		return SidecarResult{}, errors.New("HTML renderer output is not a PDF")
	}
	previewPaths, err := filepath.Glob(filepath.Join(
		previewDirectory,
		rendererPreviewGlob,
	))
	if err != nil {
		return SidecarResult{}, fmt.Errorf("list rendered preview pages: %w", err)
	}
	sort.Strings(previewPaths)
	if len(previewPaths) == 0 {
		return SidecarResult{}, errors.New("HTML renderer did not create previews")
	}
	previews := make([][]byte, 0, len(previewPaths))
	for _, path := range previewPaths {
		page, err := os.ReadFile(path)
		if err != nil {
			return SidecarResult{}, fmt.Errorf("read preview page: %w", err)
		}
		previews = append(previews, page)
	}
	return SidecarResult{
		PDF:             pdf,
		PreviewPages:    previews,
		RendererVersion: version,
	}, nil
}

func (sidecar *Sidecar) version(ctx context.Context) (string, error) {
	output, err := exec.CommandContext(ctx, sidecar.binary, "--version").
		CombinedOutput()
	if ctx.Err() != nil {
		return "", fmt.Errorf(
			"HTML renderer version check timed out after %s",
			sidecar.timeout,
		)
	}
	if err != nil {
		return "", fmt.Errorf(
			"HTML renderer version check failed: %s",
			diagnosticSummary(output),
		)
	}
	version := strings.TrimSpace(string(output))
	if version == "" {
		return "", errors.New("HTML renderer version check returned empty output")
	}
	return version, nil
}

func (sidecar *Sidecar) run(ctx context.Context, arguments ...string) error {
	command := exec.CommandContext(ctx, sidecar.binary, arguments...)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf(
			"HTML rendering timed out after %s",
			sidecar.timeout,
		)
	}
	if err != nil {
		return fmt.Errorf("HTML rendering failed: %s", diagnosticSummary(output))
	}
	return nil
}

func diagnosticSummary(output []byte) string {
	message := strings.TrimSpace(string(output))
	if message == "" {
		return "no diagnostic output"
	}
	if len(message) > rendererDiagnosticMaxSize {
		return message[:rendererDiagnosticMaxSize] + "..."
	}
	return message
}
