// @vitest-environment node
import { describe, expect, it } from "vitest";
import { AutopilotRunSchema, ListAutopilotsResponseSchema } from "./schemas";

const listed = {
  id: "ap-1",
  workspace_id: "ws-1",
  title: "Nightly regression",
  assignee_id: "agent-1",
  status: "active",
  execution_mode: "test_run",
  created_by_type: "member",
  created_by_id: "u-1",
  created_at: "t",
  updated_at: "t",
};

// The test_run mode's fields are additive: a server that does not know the
// mode omits them, and a client must not invent a plan where there is none.
describe("autopilot test_run fields", () => {
  it("parses the plan and cap of a test_run autopilot", () => {
    const parsed = ListAutopilotsResponseSchema.parse({ autopilots: [{ ...listed, test_plan_id: "plan-1", test_run_parallelism: 2 }] });
    expect(parsed.autopilots[0]?.test_plan_id).toBe("plan-1");
    expect(parsed.autopilots[0]?.test_run_parallelism).toBe(2);
  });

  it("accepts an autopilot without them and rejects a non-numeric cap", () => {
    const parsed = ListAutopilotsResponseSchema.parse({ autopilots: [listed] });
    expect(parsed.autopilots[0]?.test_plan_id).toBeUndefined();
    expect(ListAutopilotsResponseSchema.safeParse({ autopilots: [{ ...listed, test_run_parallelism: "two" }] }).success).toBe(false);
  });

  it("carries the round a run launched, when there is one", () => {
    expect(AutopilotRunSchema.parse({ id: "r1", test_run_id: "run-9" }).test_run_id).toBe("run-9");
    expect(AutopilotRunSchema.parse({ id: "r1" }).test_run_id).toBeUndefined();
  });
});
