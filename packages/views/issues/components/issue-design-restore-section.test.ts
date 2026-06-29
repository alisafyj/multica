import { describe, expect, it } from "vitest";
import { selectIssueRestoreTask } from "./issue-design-restore-section";
import type { DesignRestoreTask } from "@multica/core/types";

function task(overrides: Partial<DesignRestoreTask>): DesignRestoreTask {
  return {
    id: overrides.id ?? "task",
    workspace_id: "workspace",
    file_id: "file",
    revision_id: overrides.revision_id ?? "rev-current",
    issue_id: overrides.issue_id ?? "issue-1",
    agent_task_id: overrides.agent_task_id ?? null,
    status: overrides.status ?? "queued",
    input: {},
    result: {},
    error: null,
    created_by: null,
    created_at: "2026-06-29T00:00:00Z",
    updated_at: "2026-06-29T00:00:00Z",
  };
}

describe("selectIssueRestoreTask", () => {
  it("ignores completed restore tasks from stale revisions", () => {
    const selected = selectIssueRestoreTask([
      task({ id: "old-completed", revision_id: "rev-old", status: "completed", agent_task_id: "agent-task" }),
      task({ id: "new-queued", revision_id: "rev-current", status: "queued" }),
    ], "issue-1", "rev-current");

    expect(selected?.id).toBe("new-queued");
  });

  it("does not fall back to queued tasks from stale revisions", () => {
    const selected = selectIssueRestoreTask([
      task({ id: "old-queued", revision_id: "rev-old", status: "queued" }),
    ], "issue-1", "rev-current");

    expect(selected).toBeNull();
  });
});
