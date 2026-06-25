package htmlresume

import (
	"context"
	"errors"
	"fmt"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

type Renderer struct {
	composer  ports.ResumeHTMLComposer
	sidecar   *Sidecar
	validator Validator
}

func NewRenderer(
	composer ports.ResumeHTMLComposer,
	sidecar *Sidecar,
) (*Renderer, error) {
	if composer == nil {
		return nil, errors.New("resume HTML composer is nil")
	}
	if sidecar == nil {
		sidecar = NewSidecar("", 0)
	}
	return &Renderer{
		composer: composer,
		sidecar:  sidecar,
	}, nil
}

func (renderer *Renderer) Render(
	ctx context.Context,
	resume domain.Resume,
) (ports.RenderedResume, error) {
	resume = domain.NormalizeResume(resume)
	template, err := TemplateFor(resume.Language)
	if err != nil {
		return ports.RenderedResume{}, err
	}
	composed, err := renderer.composer.ComposeResumeHTML(
		ctx,
		ports.ComposeResumeHTMLRequest{
			Resume:   resume,
			Template: template,
		},
	)
	if err != nil {
		return ports.RenderedResume{}, fmt.Errorf("compose resume HTML: %w", err)
	}
	if composed.TemplateID != "" && composed.TemplateID != template.ID {
		return ports.RenderedResume{}, fmt.Errorf(
			"composed resume HTML template id %q does not match %q",
			composed.TemplateID,
			template.ID,
		)
	}
	if composed.TemplateVersion != "" &&
		composed.TemplateVersion != template.Version {
		return ports.RenderedResume{}, fmt.Errorf(
			"composed resume HTML template version %q does not match %q",
			composed.TemplateVersion,
			template.Version,
		)
	}
	validated, err := renderer.validator.Validate(
		resume,
		template,
		composed.HTML,
	)
	if err != nil {
		return ports.RenderedResume{}, fmt.Errorf("validate resume HTML: %w", err)
	}
	rendered, err := renderer.sidecar.RenderHTML(ctx, validated.HTML)
	if err != nil {
		return ports.RenderedResume{}, err
	}
	return ports.RenderedResume{
		PDF:          rendered.PDF,
		PreviewPages: rendered.PreviewPages,
		Metadata: ports.RenderMetadata{
			Renderer:                "weasyprint-html",
			RendererVersion:         rendered.RendererVersion,
			ExpectedRendererVersion: ExpectedRendererVersion,
			TemplateVersion:         template.Version,
			HTMLTemplateID:          template.ID,
			HTMLHash:                validated.HTMLHash,
			HTMLStyleHash:           validated.StyleHash,
			Composer:                composed.Composer,
			ComposerVersion:         composed.ComposerVersion,
			PromptVersion:           composed.PromptVersion,
		},
	}, nil
}

func (renderer *Renderer) CacheKey() string {
	return fmt.Sprintf(
		"renderer=%s|template=%s|composer=%s",
		ExpectedRendererVersion,
		TemplateVersion,
		renderer.composer.CacheKey(),
	)
}

var _ ports.ResumeRenderer = (*Renderer)(nil)
