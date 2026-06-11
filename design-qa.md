# AutoCV Paper Trail Design QA

- Source visual truth: `docs/design/paper-trail-reference.png`
- Implementation screenshot: `docs/design/paper-trail-implementation.png`
- Full comparison: `docs/design/paper-trail-comparison.png`
- Focused comparison: `docs/design/paper-trail-focused-comparison.png`
- Profile Library screenshot: `docs/design/profile-library-implementation.png`
- Profile Library comparison: `docs/design/profile-library-comparison.png`
- Viewport: `1487 x 1058`
- Match Review state: first technical requirement selected, first evidence source expanded
- Profile Library state: synthetic Markdown imported, second Evidence selected,
  duplicate import feedback visible

## Comparison Result

The reference and the native Wails implementation were captured at the same
viewport and reviewed side by side. The full-view comparison covers layout,
navigation, score, requirement groups, and the evidence inspector. The focused
comparison covers requirement typography, status hierarchy, source locations,
source content, and the match explanation.

The Profile Library was then captured from the native Wails application and
reviewed beside the same Paper Trail source. The comparison covers the shared
navigation and top bar, document and Evidence density, selected-row treatment,
feedback hierarchy, and the source inspector.

No actionable P0, P1, or P2 differences remain.

## Accepted Differences

- The reference labels the technical group as containing nine requirements but
  only renders five rows. The implementation renders all nine requirements and
  keeps the center column internally scrollable so the product data remains
  complete.
- Native WebView font rasterisation differs slightly from the generated source
  image. Font size, weight, hierarchy, and density remain aligned.

## Verified Behaviour

- Requirement filters and sorting menu
- Requirement and source selection
- Group and source expansion
- Evidence copy feedback
- Reanalysis state
- Resume generation confirmation dialog
- Responsive evidence-inspector dismissal
- Empty Profile Library state
- Native Markdown file selection
- Successful Markdown import and persisted overview refresh
- Duplicate-content feedback without creating a second document
- Evidence selection and Markdown source locator updates

## Verification

- `npm run typecheck`
- `npm run build`
- `go test ./...`
- `wails3 build`
- `git diff --check`

final result: passed
