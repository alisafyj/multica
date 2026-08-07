/**
 * Column model for the My Issues board.
 *
 * Mirrors the status branch of web's `buildGroups`
 * (`packages/views/issues/components/board-view.tsx:81-87`): one column per
 * visible status, in `BOARD_STATUSES` order, `cancelled` excluded.
 *
 * Two rules the swipe pager depends on, both inherited from web's board
 * rather than from mobile's list:
 *
 *   - **Empty columns stay.** The list drops empty status sections so the
 *     screen isn't full of "(0)" headers, but a pager needs a stable page
 *     order — otherwise "swipe right twice" lands on a different status as
 *     issues move around. Web keeps empty columns too (they're drop targets
 *     there; here they're addressable pages).
 *   - **`BOARD_STATUSES` order wins** over the order the user tapped status
 *     filters, so the page order never depends on filter input history.
 */
import type { Issue, IssueStatus } from "@multica/core/types";
import { BOARD_STATUSES } from "./issue-status";

export interface BoardColumn {
  status: IssueStatus;
  issues: Issue[];
}

export function buildBoardColumns(
  issues: Issue[],
  statusFilters: IssueStatus[],
): BoardColumn[] {
  const byStatus = new Map<IssueStatus, Issue[]>();
  for (const issue of issues) {
    const bucket = byStatus.get(issue.status);
    if (bucket) bucket.push(issue);
    else byStatus.set(issue.status, [issue]);
  }

  const visibleStatuses =
    statusFilters.length > 0
      ? BOARD_STATUSES.filter((status) => statusFilters.includes(status))
      : BOARD_STATUSES;

  return visibleStatuses.map((status) => ({
    status,
    issues: byStatus.get(status) ?? [],
  }));
}
