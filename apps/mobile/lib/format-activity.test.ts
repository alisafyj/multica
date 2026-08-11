import { describe, expect, it } from "vitest";
import type { TimelineEntry } from "@multica/core/types";
import { formatActivity } from "./format-activity";
import type { IssueTranslate } from "./issue-status";

/**
 * The module is pure and takes `t`, so it is fully testable in the Node-only
 * vitest lane. The fake `t` echoes the key plus its interpolation params so
 * assertions read as "which key, with which values" — the two things that
 * can actually regress here. The rendered copy itself is guarded by
 * lib/i18n/parity.test.ts and the locale JSON.
 */
const t: IssueTranslate = (key, opts) => {
  const params = Object.entries(opts ?? {}).filter(
    ([k]) => k !== "defaultValue",
  );
  if (params.length === 0) return key;
  const rendered = params.map(([k, v]) => `${k}=${String(v)}`).join(",");
  return `${key}(${rendered})`;
};

const resolveName = (
  type: string | null | undefined,
  id: string | null | undefined,
): string => (id ? `${type}:${id}` : "");

function entry(overrides: Partial<TimelineEntry> = {}): TimelineEntry {
  return {
    type: "activity",
    id: "a1",
    actor_type: "member",
    actor_id: "u1",
    created_at: "2026-08-07T00:00:00Z",
    ...overrides,
  };
}

const format = (overrides: Partial<TimelineEntry>) =>
  formatActivity(entry(overrides), resolveName, t);

describe("formatActivity", () => {
  it("maps a plain action to its verb key", () => {
    expect(format({ action: "created" })).toBe("activity.verb.created");
    expect(format({ action: "description_updated" })).toBe(
      "activity.verb.description_updated",
    );
  });

  it("resolves status names through the shared status labels", () => {
    expect(
      format({ action: "status_changed", details: { from: "todo", to: "done" } }),
    ).toBe("activity.verb.status_changed(from=status.todo,to=status.done)");
  });

  it("resolves priority names through the shared priority labels", () => {
    expect(
      format({
        action: "priority_changed",
        details: { from: "low", to: "urgent" },
      }),
    ).toBe(
      "activity.verb.priority_changed(from=priority.low,to=priority.urgent)",
    );
  });

  it("keeps the '?' placeholder when a side of the change is missing", () => {
    expect(format({ action: "status_changed", details: { to: "done" } })).toBe(
      "activity.verb.status_changed(from=?,to=status.done)",
    );
  });

  describe("assignee_changed", () => {
    it("detects a self-assignment from actor identity", () => {
      expect(
        format({
          action: "assignee_changed",
          actor_type: "member",
          actor_id: "u1",
          details: { to_type: "member", to_id: "u1" },
        }),
      ).toBe("activity.verb.self_assigned");
    });

    it("reports removal when an assignee existed and none remains", () => {
      expect(
        format({
          action: "assignee_changed",
          details: { from_id: "u9" },
        }),
      ).toBe("activity.verb.removed_assignee");
    });

    it("names the new assignee", () => {
      expect(
        format({
          action: "assignee_changed",
          details: { to_type: "agent", to_id: "a7" },
        }),
      ).toBe("activity.verb.assigned_to(name=agent:a7)");
    });

    it("falls back to the generic verb when the target cannot be resolved", () => {
      expect(format({ action: "assignee_changed", details: {} })).toBe(
        "activity.verb.changed_assignee",
      );
    });
  });

  describe("dates", () => {
    it("distinguishes set from removed for start dates", () => {
      expect(
        format({ action: "start_date_changed", details: { to: "2026-08-06" } }),
      ).toBe("activity.verb.start_date_set(date=Aug 6)");
      expect(format({ action: "start_date_changed", details: {} })).toBe(
        "activity.verb.start_date_removed",
      );
    });

    it("distinguishes set from removed for due dates", () => {
      expect(
        format({ action: "due_date_changed", details: { to: "2026-08-06" } }),
      ).toBe("activity.verb.due_date_set(date=Aug 6)");
      expect(format({ action: "due_date_changed", details: {} })).toBe(
        "activity.verb.due_date_removed",
      );
    });
  });

  it("passes both title sides through interpolation", () => {
    expect(
      format({ action: "title_changed", details: { from: "Old", to: "New" } }),
    ).toBe("activity.verb.title_renamed(from=Old,to=New)");
  });

  it("hands the coalesced count to i18next for pluralisation", () => {
    expect(format({ action: "task_completed" })).toBe(
      "activity.verb.task_completed(count=1)",
    );
    expect(format({ action: "task_completed", coalesced_count: 3 })).toBe(
      "activity.verb.task_completed(count=3)",
    );
    expect(format({ action: "task_failed", coalesced_count: 2 })).toBe(
      "activity.verb.task_failed(count=2)",
    );
  });

  describe("squad_leader_evaluated", () => {
    it("picks the reason variant only when a reason is present", () => {
      expect(
        format({
          action: "squad_leader_evaluated",
          details: { outcome: "action", reason: "ready to merge" },
        }),
      ).toBe("activity.verb.squad_leader_action_reason(reason=ready to merge)");
      expect(
        format({
          action: "squad_leader_evaluated",
          details: { outcome: "action" },
        }),
      ).toBe("activity.verb.squad_leader_action");
    });

    it("treats a whitespace-only reason as absent", () => {
      expect(
        format({
          action: "squad_leader_evaluated",
          details: { outcome: "failed", reason: "   " },
        }),
      ).toBe("activity.verb.squad_leader_failed");
    });

    it("falls back to the generic verb for an unknown outcome", () => {
      expect(
        format({
          action: "squad_leader_evaluated",
          details: { outcome: "something_new" },
        }),
      ).toBe("activity.verb.squad_leader_evaluated");
    });
  });

  // API Response Compatibility (root CLAUDE.md): a server-side action this
  // build has never seen must render as itself, never throw, never vanish.
  it("passes an unknown action through untranslated", () => {
    expect(format({ action: "teleported_the_issue" })).toBe(
      "teleported_the_issue",
    );
  });

  it("returns an empty string when the entry carries no action", () => {
    expect(format({})).toBe("");
  });
});
