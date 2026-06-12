package ports

import (
	"context"

	"github.com/ch1lam/autocv/internal/domain"
)

type RenderedResume struct {
	PDF          []byte
	PreviewPages [][]byte
}

type ResumeRenderer interface {
	Render(context.Context, domain.Resume) (RenderedResume, error)
}
