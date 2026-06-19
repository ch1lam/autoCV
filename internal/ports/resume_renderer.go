package ports

import (
	"context"

	"github.com/ch1lam/autocv/internal/domain"
)

type RenderedResume struct {
	PDF          []byte
	PreviewPages [][]byte
	Metadata     RenderMetadata
}

type RenderMetadata struct {
	Renderer                string
	RendererVersion         string
	ExpectedRendererVersion string
	TemplateVersion         string
}

type ResumeRenderer interface {
	Render(context.Context, domain.Resume) (RenderedResume, error)
}
