# AutoCV

AutoCV is a local-first, AI-native desktop application for turning a user's existing career material into a resume tailored to a specific job description.

The long-term product has two core capabilities:

- Tailor resumes to a JD with domain-specific analysis and review.
- Discover suitable jobs automatically from public recruitment sources.

The first MVP focuses only on the resume workflow. Job monitoring comes in the second phase.

## Why AutoCV

General-purpose AI can rewrite a resume, but the result usually depends on repeatedly collecting context, explaining constraints, adjusting prompts, checking unsupported claims, and repairing formatting.

AutoCV aims to make that process repeatable:

- Keep resumes, project notes, work history, and supporting material in a reusable local profile.
- Analyze each JD against that profile.
- Ask targeted questions when evidence is missing.
- Surface strengths that are implied by real experience, not just restate the source text.
- Apply a selectable level of resume packaging.
- Let the user review the result instead of rebuilding the workflow for every application.

## Current Product Direction

- The first user is the project author; broader job seekers come later.
- Chinese and English resumes are both in scope.
- The initial focus is domestic job applications without designing out overseas JDs.
- The product is a local-first desktop application.
- Files and records stay on the user's machine.
- AI requests should send only the content required for the current task.
- Users should be able to export and delete their data.
- No AutoCV cloud account is required.
- The project is open source under the MIT license.

These are product constraints. The current development baseline is captured in
the [MVP product specification](./docs/product/autocv-mvp-product-spec.md),
[MVP architecture](./docs/architecture/mvp-architecture.md), and
[MVP implementation plan](./docs/plans/mvp-implementation-plan.md).

## MVP Workflow

The working MVP loop is:

```text
Local profile library
  -> add or paste a target JD
  -> analyze requirements
  -> match profile evidence
  -> ask for missing information
  -> choose a packaging level
  -> generate a tailored Markdown resume
  -> review and lock important content
  -> explain the main optimization choices
  -> render and preview a PDF
  -> export Markdown and PDF
```

The user's profile exists before a JD is added. Resumes and supporting material are reusable inputs; each new JD creates another tailored output.

## MVP Inputs And Outputs

Planned inputs:

- PDF resumes
- Word resumes
- Markdown resumes
- Project documentation
- Work notes
- Portfolio material
- Pasted or imported JD text

Planned outputs:

- Tailored Markdown resume
- Previewable PDF resume
- Overall JD match score
- Missing-requirement summary
- Short explanation of the main changes
- Source references for important generated highlights

The first version does not need a full rich-text editor. A constrained Markdown editing experience is sufficient.

## Resume Generation Principles

AutoCV should do more than keyword replacement:

- Infer relevant capabilities from the user's real work.
- Explain business value that may be implicit in source material.
- Select and reorder experience according to the JD.
- Adjust language strength, keyword coverage, experience selection, inference depth, and career positioning through explicit packaging levels.
- Ask the user when the available material is insufficient.
- Show genuine gaps instead of pretending every requirement is covered.
- Allow the user to lock content that AI must not rewrite.

The exact boundaries for inference, estimated metrics, packaging strength, and source display are not final. They must be validated with real resumes and JDs before becoming product rules.

The working quality bar is that the user should need only minor changes before submitting the result. A resume should normally stay within two pages, but longer resumes are allowed when the experience genuinely requires it; the product should not force every resume into one page.

## MVP Scope

The MVP includes:

- A reusable local profile library
- PDF, Word, and Markdown resume intake
- Optional supporting-material intake
- JD import and structured analysis
- Profile-to-JD matching with an overall score
- Missing-information questions
- Tailored resume generation in Chinese and English
- Multiple packaging levels
- Content locking
- Markdown output
- PDF rendering and preview
- Local export and deletion

The MVP excludes:

- Automatic recruitment-site monitoring
- Cover letters
- Application emails
- Interview preparation
- A full rich-text resume editor
- Long-term behavioral memory beyond the user's profile material
- External-user validation as a release requirement

## Phase Two: Job Radar

Automatic job discovery remains a core part of the AutoCV vision, but it is not part of the first implementation loop.

The second phase may cover:

- Monitoring official company career sites and public recruitment pages
- Detecting new and changed jobs
- Normalizing JD content
- Deduplicating job records
- Matching new jobs against the local profile
- Triggering the resume workflow for promising roles

Any crawler must respect public-access boundaries, robots.txt, website terms, and rate limits. It must not bypass login, captcha, paywalls, or anti-abuse controls.

## AI Workflow

The current conceptual workflow is:

```text
Profile Intake
  -> JD Analysis
  -> Profile Matching
  -> Clarification
  -> Resume Drafting
  -> Resume Review
  -> PDF Rendering
```

These names describe responsibilities, not a final commitment to seven
independent agents or services. The Go workflow and Provider boundaries are
defined in the [MVP architecture](./docs/architecture/mvp-architecture.md).

## Technical Direction

Current MVP implementation choices:

- Desktop shell: Wails 3
- Core language: Go
- Frontend: React and TypeScript
- Local storage: SQLite
- First AI integration: a provider adapter backed by OpenAI
- PDF rendering: Kami-style HTML templates, a constrained HTML composer, and a local WeasyPrint/PDFium renderer
- Longer-term AI integration: multiple providers
- PDF quality benchmark: the resume output currently achievable through the `kami` skill

The workflow is explicitly owned by Go rather than an Agent framework. See the [MVP architecture](./docs/architecture/mvp-architecture.md).

## Development Status

AutoCV has completed the M0 engineering baseline and the fixture-backed M1
Markdown-to-PDF vertical flow:

- Wails 3 desktop shell with a React and TypeScript frontend.
- Paper Trail match-review interface with functional first-screen controls.
- Go health-check binding.
- Operating-system application data directories.
- SQLite startup migrations and persistent M1 workflow records.
- Versioned local configuration without API key fields.
- Structured JSON logs with default content and secret redaction.
- Strict JD Analysis Schema validation and a fixture-backed Fake Provider.
- Structured Resume versions with lock-preserving regeneration.
- Local PDF Artifact recovery, preview, and export.
- Fake Provider integration coverage from Markdown import through a persisted
  PDF Artifact.
- Go and frontend unit tests plus a unified verification command.

The current implementation documents are:

- [AutoCV MVP Product Specification](./docs/product/autocv-mvp-product-spec.md)
- [AutoCV MVP Architecture](./docs/architecture/mvp-architecture.md)
- [AutoCV MVP Implementation Plan](./docs/plans/mvp-implementation-plan.md)

The official OpenAI Go SDK adapter is wired through the Provider configuration
and macOS Keychain. The remaining M1 work is native desktop and real-sample
validation. Real local resumes and JDs are never committed to the repository.

## Local Development

Required Go and Node versions are pinned in `go.mod`, `.nvmrc`, and
`frontend/package.json`. PDF rendering uses the project HTML -> WeasyPrint ->
PDFium sidecar instead of a global Typst install.

```bash
npm --prefix frontend install
wails3 task verify
wails3 task dev
```

Build the local PDF renderer sidecar for macOS packaging with:

```bash
wails3 task darwin:renderer:build
```

For development, `AUTOCV_PDF_RENDERER_BIN` can point to a local
`autocv-pdf-renderer` binary.

Build the native application with:

```bash
wails3 build
```

On macOS, runtime data is stored under
`~/Library/Application Support/AutoCV`. Set `AUTOCV_DATA_DIR` to isolate a
development or test run:

```bash
AUTOCV_DATA_DIR=/tmp/autocv-dev wails3 task dev
```

The directory contains `autocv.db`, `config.json`, managed source files, run
artifacts, exports, logs, and backups. These private runtime files are excluded
from Git.

## License

MIT
