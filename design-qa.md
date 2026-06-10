# AutoCV Paper Trail Design QA

- Source visual truth: `docs/design/paper-trail-reference.png`
- Implementation screenshot: `docs/design/paper-trail-implementation.png`
- Full comparison: `docs/design/paper-trail-comparison.png`
- Focused comparison: `docs/design/paper-trail-focused-comparison.png`
- Viewport: `1487 x 1058`
- State: Match Review, first technical requirement selected, first evidence source expanded

## Comparison Result

The reference and the native Wails implementation were captured at the same
viewport and reviewed side by side. The full-view comparison covers layout,
navigation, score, requirement groups, and the evidence inspector. The focused
comparison covers requirement typography, status hierarchy, source locations,
source content, and the match explanation.

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

## Verification

- `npm run typecheck`
- `npm run build`
- `go test ./...`
- `wails3 build`
- `git diff --check`

final result: passed
