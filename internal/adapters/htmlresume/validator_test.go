package htmlresume

import (
	"strings"
	"testing"

	"github.com/ch1lam/autocv/internal/domain"
)

func TestValidatorAcceptsTemplateBodyFilledByComposer(t *testing.T) {
	resume := validResume()
	template, err := TemplateFor(domain.ResumeLanguageChinese)
	if err != nil {
		t.Fatalf("template: %v", err)
	}

	validated, err := (Validator{}).Validate(
		resume,
		template,
		fillTemplate(t, template.HTML, validBody()),
	)
	if err != nil {
		t.Fatalf("validate HTML: %v", err)
	}
	if validated.HTMLHash == "" || validated.StyleHash != template.StyleHash {
		t.Fatalf("unexpected validation hashes: %#v", validated)
	}
}

func TestValidatorRejectsUnsafeOrInconsistentHTML(t *testing.T) {
	resume := validResume()
	template, err := TemplateFor(domain.ResumeLanguageChinese)
	if err != nil {
		t.Fatalf("template: %v", err)
	}
	cases := []struct {
		name string
		html string
		want string
	}{
		{
			name: "style changed",
			html: strings.Replace(
				fillTemplate(t, template.HTML, validBody()),
				"--paper: #f7f1e6",
				"--paper: #ffffff",
				1,
			),
			want: "style does not match",
		},
		{
			name: "script injected",
			html: fillTemplate(t, template.HTML, validBody()+`<script>alert(1)</script>`),
			want: "not allowed",
		},
		{
			name: "inline style injected",
			html: fillTemplate(
				t,
				template.HTML,
				strings.Replace(validBody(), `class="item-content"`, `class="item-content" style="color:red"`, 1),
			),
			want: "inline style",
		},
		{
			name: "field text changed",
			html: fillTemplate(
				t,
				template.HTML,
				strings.Replace(validBody(), "负责 Go 服务开发。", "负责 Java 服务开发。", 1),
			),
			want: "changed text",
		},
		{
			name: "item id missing",
			html: fillTemplate(
				t,
				template.HTML,
				strings.Replace(validBody(), ` data-autocv-id="item-1"`, "", 1),
			),
			want: "appears 0 times",
		},
		{
			name: "unknown class",
			html: fillTemplate(
				t,
				template.HTML,
				strings.Replace(validBody(), `class="item"`, `class="item card"`, 1),
			),
			want: "not in template",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (Validator{}).Validate(resume, template, tc.html)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
		})
	}
}

func validResume() domain.Resume {
	return domain.NormalizeResume(domain.Resume{
		ID:            "resume-1",
		RunID:         "run-1",
		InputHash:     "input-1",
		Version:       1,
		SchemaVersion: domain.ResumeSchemaV2,
		Language:      domain.ResumeLanguageChinese,
		TargetRole:    "后端工程师",
		Header: domain.ResumeHeader{
			Name:       "张三",
			TargetRole: "后端工程师",
		},
		Sections: []domain.ResumeSection{{
			ID:    "section-work",
			Title: "代表作品",
			Items: []domain.ResumeItem{{
				ID:                "item-1",
				Kind:              domain.ResumeBlockProject,
				Content:           "负责 Go 服务开发。",
				SourceEvidenceIDs: []string{"evidence-1"},
				GroundingLevel:    domain.GroundingSource,
				Optimization:      "突出交付产物。",
			}},
		}},
		OptimizationNotes: []string{},
	})
}

func validBody() string {
	return `<main class="sheet">
  <header class="resume-header">
    <h1 class="name" data-autocv-field="header.name">张三</h1>
    <p class="target" data-autocv-field="header.target_role">后端工程师</p>
  </header>
  <section class="section" data-autocv-id="section-work">
    <h2 class="section-title" data-autocv-field="section.title">代表作品</h2>
    <article class="item" data-autocv-id="item-1">
      <p class="item-content" data-autocv-field="item.content">负责 Go 服务开发。</p>
    </article>
  </section>
</main>`
}

func fillTemplate(t *testing.T, templateHTML string, body string) string {
	t.Helper()
	start := strings.Index(templateHTML, "<body>")
	end := strings.Index(templateHTML, "</body>")
	if start < 0 || end < 0 || end < start {
		t.Fatal("template body is missing")
	}
	return templateHTML[:start+len("<body>")] + "\n" + body + "\n" + templateHTML[end:]
}
