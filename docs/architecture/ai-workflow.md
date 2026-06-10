# AutoCV AI 工作流职责

This document records the current responsibility model for AutoCV's AI-assisted workflow. It is not a final agent architecture, database design, or implementation specification.

The workflow should make resume tailoring repeatable and reviewable. Agent names are useful for reasoning about responsibilities, but a responsibility does not need to become an independent runtime agent, process, or service.

## Product Boundary

The first MVP starts from a reusable local profile and a target JD:

```text
Profile Intake
  -> JD Analysis
  -> Profile Matching
  -> Clarification
  -> Resume Drafting
  -> Resume Review
  -> PDF Rendering
```

Automatic job discovery is a second-phase workflow. Application packaging, cover letters, email drafts, and interview preparation are outside the MVP.

The current product rules are defined in the [MVP product specification](../product/autocv-mvp-product-spec.md). The broader [product definition plan](../plans/autocv-product-definition-plan.md) remains the validation backlog.

## Workflow Principles

- Keep user resumes and supporting material local.
- Reuse the local profile across multiple JDs.
- Send only task-relevant content to an AI provider.
- Prefer structured inputs and outputs between workflow stages.
- Preserve enough source context to review important generated highlights.
- Ask the user when evidence is insufficient.
- Use AI to extract, infer, rank, and write; do not hide uncertainty behind a match score.
- Let the user lock content that must survive later rewrites.
- Persist intermediate results so interrupted work can resume.
- Keep final approval with the user.

## MVP Responsibilities

### Profile Intake And Indexing

Purpose:

- Build and maintain the reusable local source of truth for the user's background.

Candidate inputs:

- PDF, Word, and Markdown resumes
- Project documentation
- Work notes
- Portfolio material
- Skills, education, certifications, and domain vocabulary

Responsibilities:

- Extract structured career information from local files.
- Separate reusable experience, projects, skills, achievements, and supporting evidence.
- Preserve references back to source material.
- Detect incomplete or conflicting information.
- Keep the profile editable, exportable, and deletable.

The MVP memory boundary is the user's profile material. Learning from long-term application history, accepted wording, rejected wording, or behavioral preferences is not required yet.

### JD Analysis

Purpose:

- Turn one target JD into a structured description of what the role appears to require.

Candidate output:

```json
{
  "role": "",
  "level": "",
  "responsibilities": [],
  "required_skills": [],
  "preferred_skills": [],
  "domain_signals": [],
  "screening_signals": [],
  "ambiguities": []
}
```

Responsibilities:

- Extract explicit requirements and responsibilities.
- Identify seniority, domain, language, and screening signals.
- Separate required and preferred qualifications.
- Record ambiguous or conflicting JD content instead of silently resolving it.

The final schema remains a product-spec decision.

### Profile Matching

Purpose:

- Select the strongest relevant evidence from the user's profile and expose real gaps.

Candidate output:

```json
{
  "match_score": 0,
  "matched_requirements": [],
  "weak_matches": [],
  "missing_requirements": [],
  "recommended_experience": [],
  "important_sources": []
}
```

Responsibilities:

- Match JD requirements against profile evidence.
- Rank relevant experience and projects.
- Produce an overall, explainable match score.
- Distinguish strong evidence, weak evidence, missing evidence, and genuine capability gaps.
- Recommend how to emphasize strengths and reduce attention on weaker areas without hiding material facts.

The score formula is not final. A high score must not conceal unsupported claims or weak evidence.

### Clarification

Purpose:

- Obtain information that materially improves the resume instead of guessing silently.

Responsibilities:

- Detect missing context, outcomes, ownership, technical details, or business impact.
- Ask focused questions before drafting when the answer could change the resume.
- Avoid asking for information that already exists in the local profile.
- Stop asking when further detail would not materially improve the result.

The triggers, question limit, and stopping rules must be validated with real examples.

### Resume Drafting

Purpose:

- Produce a JD-specific Markdown resume from the selected profile evidence.

Responsibilities:

- Support Chinese and English resumes.
- Reorder and select experience according to the target role.
- Apply one of several explicit packaging levels.
- Adjust language strength, keyword coverage, experience selection, inference depth, and career positioning according to that level.
- Infer defensible capabilities and explain implicit business value.
- Respect locked content.
- Keep the resume readable instead of maximizing keyword density.
- Normally target two pages or fewer, while allowing longer output when justified by the user's experience.

Reasonable inference is a core capability, not an exception. However, the exact rules for inferred capabilities and estimated metrics remain a high-risk product decision that must be validated with real samples.

### Resume Review

Purpose:

- Check whether the draft is useful, coherent, sufficiently matched, and ready for user review.

Responsibilities:

- Check JD coverage, ordering, clarity, repetition, and readability.
- Check whether important highlights can be connected to source material.
- Detect claims that exceed the approved inference or metric rules.
- Verify that locked content remains unchanged.
- Recalculate or explain the match score after drafting.
- Provide a short summary of the main optimization choices.
- Show capability gaps and help the user emphasize stronger areas.

The MVP does not require a full before-and-after diff. It should give the user enough explanation to review the important changes quickly.

### PDF Rendering

Purpose:

- Turn the reviewed Markdown resume into a professional, previewable PDF.

Responsibilities:

- Preserve the approved content.
- Produce an ATS-conscious, readable layout.
- Support Chinese and English output.
- Provide an in-app preview.
- Export both Markdown and PDF.

The visual quality produced by the current `kami` skill is the reference target. Whether `kami` is embedded, invoked as an external workflow, or replaced by an equivalent renderer remains an open feasibility question.

## Packaging Levels

Packaging is not a binary on/off setting. The product should offer a small number of explicit levels.

Each level may affect:

- Language strength
- JD keyword coverage
- Experience selection and ordering
- Depth of reasonable inference
- Career positioning

The number, names, and exact permissions of these levels are not final. They must be tested against real resumes, including examples where AI packaging becomes excessive.

## Grounding And Quantification

Important generated highlights should retain source references for review. The final resume does not need to label every reasonable inference, but the workflow must still apply the approved content rules.

Quantification requires special treatment. Future validation must distinguish:

- Numbers explicitly present in source material
- Values calculated from source material
- Values the user can confirm
- Ranges inferred from context
- Values suggested only by industry convention

The current product direction allows industry-informed estimates in principle. The acceptable boundary and confirmation behavior are not final and must not be hard-coded before sample-based validation.

## User Control

The workflow should allow the user to:

- Select local profile material for a run
- Add or paste a target JD
- Answer clarification questions
- Choose a packaging level
- Lock content against rewriting
- Review the match score and capability gaps
- Review important source-backed highlights
- Edit constrained Markdown if needed
- Preview and export the final PDF and Markdown
- Delete or export local data

The intended interaction is review-first: the system performs the repetitive analysis and drafting, while the user handles decisions and final approval.

## Run State

A resumable workflow remains the preferred direction. A candidate state model is:

```text
pending
  -> running
  -> requires_user_input
  -> succeeded
  -> failed
  -> skipped
```

This is a workflow requirement candidate, not a final persistence schema.

Each completed stage should retain enough information to explain:

- Which inputs were used
- Which provider or model was called
- Which structured output was produced
- Which user decisions were applied
- Which error or pause stopped the run

## Phase Two: Job Radar

Job Radar remains part of the long-term product, but it is not an MVP agent.

Future responsibilities may include:

- Monitor public official career sites and recruitment pages.
- Detect new and changed jobs.
- Normalize company, title, location, level, URL, and JD content.
- Deduplicate jobs by stable identifiers and content.
- Match promising jobs against the local profile.
- Start the resume workflow only after user approval.

Crawler tools, supported sources, scheduling, and storage are not settled. Any implementation must respect public access, website terms, robots.txt, rate limits, and anti-abuse boundaries.

## Explicitly Outside The MVP

- Application Packager agent
- Cover letter generation
- Application email generation
- Interview talking points
- Automatic recruitment-site monitoring
- Full rich-text resume editing
- Long-term preference learning beyond user profile material

## Current Technical Direction

The MVP implementation baseline is defined in the [MVP architecture](./mvp-architecture.md):

- Wails 3 desktop application
- Go-owned explicit workflow
- React and TypeScript frontend
- SQLite with FTS5
- Provider task interface with OpenAI as the first adapter
- Typst-based local PDF rendering
- No Agent framework, vector database, cloud account, or microservice split in the MVP

The delivery order and acceptance gates are defined in the [MVP implementation plan](../plans/mvp-implementation-plan.md).

Document parser libraries, the concrete OpenAI model, the SQLite driver, the Keychain wrapper, and the final font set remain implementation Spikes. They must not change the product contract or module boundaries without an architecture decision update.
