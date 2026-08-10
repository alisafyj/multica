# Task 7 Regression Fix Report

Date: 2026-08-10 (Asia/Shanghai)

## Status

Completed the bounded Task 7 regression fix on `codex/multica-native-design-engine`.

- Historical Open Design lifecycle/evidence tests now seed their own historical task context and `open_design_run` fixture after native task creation.
- The enabled-feature-flag router test now requires `multica.project-design-system/v2` and zero `open_design_run` rows.
- `package_schema`, `preview_targets`, and `selection_enabled` remain schema-defaulted while the TypeScript interface accepts older responses that omit them.

## Impact

GitNexus found zero affected execution flows. The shared historical fixture helper has medium test-only reach through 14 dependent tests/helpers; the other edited symbols are low risk.

## Verification

- Focused handler Go tests: passed.
- Focused router Go test: passed.
- Core schema tests: 38/38 passed.
- Canvas, preview, and page tests: 36/36 passed.
- `pnpm typecheck --force`: remained nonzero on documented pre-existing contract diagnostics; the three new missing-field diagnostics are gone and the output contains no references to the three fields.
- Changed Go files were formatted with `gofmt` only.

## Concerns

The repository typecheck baseline remains nonzero and was not broadened or repaired, as required.
