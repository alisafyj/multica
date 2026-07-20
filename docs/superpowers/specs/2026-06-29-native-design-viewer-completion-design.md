# Native Design Viewer Completion Design

## Context

The Native Design Viewer can display imported Figma frames as real layers, compare against the frame preview, show a layer tree, inspect layer metadata, and perform safe lightweight edits. The most recent work improved legacy import fidelity by supporting local/exportable fallbacks, preview crop fallbacks, text rendering fixes, and render-quality scoring.

The remaining work should finish the productized viewer path rather than expand into a full online Figma editor.

## Goals

1. Make the viewer useful for day-to-day handoff: inspect, compare, and safely edit simple layer attributes.
2. Keep improving render fidelity for legacy and new imports without hiding real gaps.
3. Add file-level quality visibility so users can find low-fidelity frames and layers quickly.
4. Preserve the current scope boundary: no arbitrary geometry edits, no full vector editor, no multiplayer editing, and no heavy new persistence model unless needed.

## Non-Goals

- Building a complete design tool.
- Editing layer geometry, layout hierarchy, masks, or children.
- Using full-frame preview as the primary restore or render result.
- Adding large database migrations before proving the data model with `native_json.source` summaries.

## Approach

Use a staged sequence: finish lightweight editing, tighten render quality, then expose quality at file level. This produces useful improvements after each phase and avoids platform UI ahead of core quality.

## Phase 1: Lightweight Editing Completion

Add safe edits for:

- Image replacement for layers with image fills or image assets.
- Stroke color and stroke width.
- Undo/revert of recent lightweight edits.
- Clearer edit summaries in the inspector.

Rules:

- Validate all edit inputs on the server.
- Store edit history under `native_json.source.lightweightEdits`.
- Update `native_json.source.importFidelityReport` after edits.
- Keep geometry, layout, child order, and mask edits blocked.

## Phase 2: Render Fidelity Improvements

Improve fidelity by focusing on visible differences:

- Make fallback classification match actual renderer behavior.
- Treat local fallback, legacy exportable slice, and preview crop as high-quality fallback, not native.
- Keep placeholders and unsupported layers clearly marked.
- Refine preview-crop rules so they help missing image fills and small complex icons without duplicating whole-frame content.
- Continue improving text overflow, icon fallback, image fill, mask, and vector handling where data supports it.

Render quality should use `renderQualityPercent`, not only `nativePercent`:

- Native: full credit.
- Local/exportable/cropped fallback: high partial credit.
- Generic placeholder: low partial credit.
- Unsupported: zero credit.

## Phase 3: File-Level Quality Visibility

Expose quality above the individual frame page:

- Show file-level average render quality.
- Show frame-level quality in frame lists or cards.
- Provide a low-quality layer/frame summary.
- Allow sorting or filtering by low fidelity.
- Surface top reasons for quality loss: missing image fills, unsupported vector/mask, placeholder fallback, missing assets.

Prefer deriving this from existing `native_json` and `importFidelityReport`. Add persistent schema only if client-side computation becomes too slow or needs querying across many files.

## UI/UX Design

The inspector remains the primary place for single-layer editing and diagnostics. The design file detail page becomes the place to answer: “Which frame needs attention first?”

Suggested UI labels:

- `还原度` for render quality.
- `原生` for truly native rendering.
- `兜底` for local/exportable/crop fallback.
- `缺失` for unsupported or missing render data.

Quality explanations must stay honest: a frame can look good because of high-quality fallback, but it should not be reported as fully native.

## Data Flow

1. Import produces `native_json` with frames, layers, assets, and source metadata.
2. Renderer computes frame/layer fidelity for display.
3. Lightweight edits create a new revision or update the revision through existing design APIs.
4. Server validates and records changed fields under `source.lightweightEdits`.
5. Server refreshes persisted import fidelity summary when the native JSON changes.
6. Client recomputes live `renderQualityPercent` for the active frame.

## Error Handling

- Reject invalid colors, stroke widths, image replacement payloads, stale revision IDs, and unsupported layer types with 400 errors.
- Show failed edits as toasts with server messages.
- Keep missing assets visible in fidelity reasons; do not silently mark them as native.
- If image replacement upload is not available, block the edit with a clear message instead of creating placeholder data.

## Testing

Backend:

- Focused handler tests for image replacement, stroke edits, undo/revert, stale revision rejection, and invalid payloads.
- Fidelity summary tests for native/fallback/unsupported counts and render quality percent.

Frontend:

- Typecheck `@multica/views` and `@multica/web`.
- Browser checks against existing sample frames to confirm quality values and no console errors.

Regression:

- Existing legacy sample should keep these quality targets unless data changes:
  - `frame-1`: about 85% or better.
  - `frame-0-468`: about 93% or better.
  - `frame-0-651`: about 92% or better.

## Implementation Order

1. Add stroke edit support.
2. Add image replacement support where upload plumbing already exists.
3. Add undo/revert for lightweight edits.
4. Tighten render quality classification and diagnostics.
5. Add file/frame quality summaries.
6. Run focused tests, typechecks, browser regression, `git diff --check`, and GitNexus detect changes.
