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
	HTMLTemplateID          string
	HTMLHash                string
	HTMLStyleHash           string
	Composer                string
	ComposerVersion         string
	PromptVersion           string
}

type ResumeRenderer interface {
	Render(context.Context, domain.Resume) (RenderedResume, error)
}

type VersionedResumeRenderer interface {
	ResumeRenderer
	CacheKey() string
}
