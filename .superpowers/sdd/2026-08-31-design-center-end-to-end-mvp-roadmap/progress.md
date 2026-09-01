/Users/a1234/multica_workspaces_desktop-iworker.soyoung.com/7a08c097-ae66-4a3d-973f-4a5059056f89/e1b965826379/workdir/design-center-end-to-end-mvp-tasks-10-12/docs/superpowers/plans/2026-08-31-design-center-end-to-end-mvp-roadmap.md

# Tasks 10-12 SDD Progress

## Conflict Table

| Task | Produces | Consumed by | Shared surface / ruling |
| --- | --- | --- | --- |
| 10 | Multica source materialization and restore execution/result | 11, 12 | Put source-neutral restore flow in the existing implementation-context core; source adapter only resolves evidence. |
| 11 | Figma resolver on the same context/result contract | 12 | Reuse Task 10 flow and result validation; do not duplicate execution or outer workflow. |
| 12 | Issue selector, prompt prefill, execution status/result projection | User | Consume unified APIs; no source branch outside metadata/badge, no auto-send or Issue status change. |

## Boundaries

- Task 13 and Task 14 are out of scope.
- No push, PR, main merge, or integration-branch merge.
- No automatic target-repository commit.
- Real API/UI/repository validation is required; HTTP 200 alone is insufficient.
- Existing root DESIGN.md and Design Center product decisions are authoritative.
- Prefer existing handlers, context/result types, repository execution helpers, Issue sidebar and comment editor.

## Status

- [x] Update authoritative handoff (commit 7eeeb9950)
- [x] Task 10 implement
- [ ] Task 10 review
- [ ] Task 10 real validation
- [ ] Task 11 implement
- [ ] Task 11 review
- [ ] Task 11 real validation
- [ ] Task 12 implement
- [ ] Task 12 review
- [ ] Task 12 real UI validation
- [ ] Broad review and final verification
