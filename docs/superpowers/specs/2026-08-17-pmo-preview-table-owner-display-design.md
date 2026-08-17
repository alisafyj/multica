# PMO Preview Table And Owner Display Design

## Goal

Make the PMO preview match the source PM system's readable structure and use the same owner naming rule in preview and assignee mapping.

## Layout

- Render each requirement as one one-row information table.
- Render all tasks under that requirement in one table, regardless of milestone.
- Use stable task columns: task ID, title, owner, start date, due date, workload, milestone, and status.
- Keep diff decisions inside the cell for the field they affect.
- Keep horizontal overflow on the table wrapper and use fixed table layout so columns stay aligned.
- Render assignee mapping rows as a stable content/select grid.

## Owner Display

Normalize external owner IDs by trimming and lowercasing. Match a full external email to `member.email`; match an ID without `@` to the email prefix. A matched member displays `member.name`. An unmatched owner displays the email prefix or original external ID. Preview and assignee mapping use the same resolver and never repeat equivalent owner strings.

## Scope

This is a frontend-only presentation change. It does not alter source snapshots, saved assignee mappings, automatic Agent resolution, or API contracts.
