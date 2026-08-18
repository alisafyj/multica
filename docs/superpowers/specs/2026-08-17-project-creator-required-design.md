# Required Project Creator

## Goal

Keep historical projects usable when their creator was not recorded, while ensuring every newly created project records the authenticated member responsible for it.

## Behavior

- The project detail sidebar always shows a read-only **Created by** property.
- Projects with `created_by` show the existing member avatar and display name.
- Historical projects with `created_by = NULL` show the localized value **Unknown**.
- New project rows with `created_by = NULL` are rejected at the database boundary.

## Data Integrity

Add a fork-range migration with a named PostgreSQL check constraint:

```sql
CHECK (created_by IS NOT NULL) NOT VALID
```

`NOT VALID` intentionally preserves historical rows that predate creator tracking. PostgreSQL still enforces the constraint for new inserts and updates, so future code paths cannot create another unattributed project. No foreign key or backfill is added.

The existing supported creation paths remain unchanged:

- The authenticated project API writes the SSO user's ID.
- PMO synchronization writes the requesting member's ID.

## Compatibility

The API field remains nullable because older rows can still be null. No client request shape changes, and the creator remains immutable because update requests do not expose `created_by`.

## Verification

- Component test: a historical null creator renders **Unknown**.
- Existing component test: a recorded creator renders the member name.
- Migration test: a legacy null row remains readable after adding the constraint, while a new null insert fails and a non-null insert succeeds.
- Existing handler and PMO tests: authenticated and synchronized project creation still succeed and return/store the expected creator.
- Typecheck and migration lint checks pass.

