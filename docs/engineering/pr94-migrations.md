# PR 94 migration compatibility

PR 94 merges two independently developed migration series that both use prefixes
911 through 919. Migration identity is the complete filename stem, so neither
series may be renamed or removed. The feature stems from
`911_task_run_evidence` through `922_task_run_evidence_model_usage` are already
recorded in the private benchmark database. The duplicate-prefix allowlist
therefore freezes the exact 911-919 pairs and rejects any later addition or
rename.

## Supported upgrade paths

- Feature database: the runner skips the 12 already-recorded feature stems and
  applies the pending main stems in their original dependency order.
- Main database: the runner skips the already-recorded main stems and applies
  feature stems 911 through 922 in their original dependency order.
- Fresh database: lexical ordering interleaves same-prefix stems, while each
  series retains its internal dependency order. The two series create or alter
  independent schema objects, so the cross-series interleaving is intentional.

The migration lint regression checks that both complete dependency chains stay
present and ordered. Existing concurrent-index tests verify that every index
build from both series retains its invalid-index cleanup hook, including the
feature rollback at 919 and main's independent rollback at 450.

## Semantic overlap review

`913_agent_task_execution_metrics` adds the operational `execution_metrics`
JSONB snapshot to `agent_task_queue`. Feature migrations separately create the
normalized `task_run_evidence` record and add `queue_started_at` to
`agent_task_queue` at 921 for queue-duration anchoring. These fields describe
related execution telemetry but do not replace one another: the JSON snapshot
supports task execution status, while evidence rows and the queue timestamp
support durable benchmark measurements and their provenance.

Integration validation on 2026-09-08 covered three isolated databases: a fresh
installation, main's 601 migrations followed by the merged set, and the feature
branch's 603 migrations followed by the merged set. Each reached all 613 complete
stems, produced identical column schemas, had no invalid indexes, and passed a
second migration run without adding ledger entries. No existing application
database was migrated by these checks.

Migration lint and runner tests with an explicit isolated `DATABASE_URL` passed
66 assertions; two optional pg_bigm cases were skipped because that extension
was unavailable. Handler tests must also set `DATABASE_URL` explicitly because
they otherwise default to the local `multica` database.
