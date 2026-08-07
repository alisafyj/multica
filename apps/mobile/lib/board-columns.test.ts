/**
 * Column model for the My Issues board. The board's whole parity claim is
 * "same issues, same counts as the list — just laid out differently", so
 * these tests pin the grouping rules the swipe pager depends on:
 * fixed page order, empty columns kept, `cancelled` dropped.
 */
import { describe, expect, it } from "vitest";
import type { Issue, IssueStatus } from "@multica/core/types";
import { BOARD_STATUSES } from "./issue-status";
import { buildBoardColumns } from "./board-columns";

function issue(id: string, status: IssueStatus): Issue {
  return { id, status } as Issue;
}

describe("buildBoardColumns", () => {
  it("returns every board status in BOARD_STATUSES order, empty ones included", () => {
    // The swipe pager needs a fixed page order — dropping empty columns
    // would make "swipe right twice" land on a different status day to day.
    const columns = buildBoardColumns([], []);
    expect(columns.map((c) => c.status)).toEqual(BOARD_STATUSES);
    expect(columns.every((c) => c.issues.length === 0)).toBe(true);
  });

  it("files each issue under its own status", () => {
    const columns = buildBoardColumns(
      [issue("a", "todo"), issue("b", "in_progress"), issue("c", "todo")],
      [],
    );
    const byStatus = Object.fromEntries(
      columns.map((c) => [c.status, c.issues.map((i) => i.id)]),
    );
    expect(byStatus.todo).toEqual(["a", "c"]);
    expect(byStatus.in_progress).toEqual(["b"]);
    expect(byStatus.done).toEqual([]);
  });

  it("preserves the incoming order within a column", () => {
    // The query returns server-sorted issues; regrouping must not reshuffle.
    const columns = buildBoardColumns(
      [issue("z", "todo"), issue("y", "todo"), issue("x", "todo")],
      [],
    );
    const todo = columns.find((c) => c.status === "todo");
    expect(todo?.issues.map((i) => i.id)).toEqual(["z", "y", "x"]);
  });

  it("drops cancelled issues, matching the list and web's board", () => {
    // `cancelled` is deliberately absent from BOARD_STATUSES. An issue in
    // that status has no column, so it must not leak into another one.
    const columns = buildBoardColumns(
      [issue("a", "cancelled"), issue("b", "done")],
      [],
    );
    expect(columns.map((c) => c.status)).toEqual(BOARD_STATUSES);
    expect(columns.flatMap((c) => c.issues.map((i) => i.id))).toEqual(["b"]);
  });

  it("narrows to the selected statuses when a status filter is active", () => {
    const columns = buildBoardColumns(
      [issue("a", "todo"), issue("b", "done")],
      ["done", "todo"],
    );
    // BOARD_STATUSES order wins over the order the user tapped the filters.
    expect(columns.map((c) => c.status)).toEqual(["todo", "done"]);
  });

  it("keeps a selected-but-empty status as a column", () => {
    // Filtering to a status with nothing in it should show an empty column,
    // not silently fall back to every column.
    const columns = buildBoardColumns([issue("a", "todo")], ["blocked"]);
    expect(columns).toEqual([{ status: "blocked", issues: [] }]);
  });
});
