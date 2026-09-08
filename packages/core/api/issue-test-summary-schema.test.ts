// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseWithFallback } from "./schema";
import {
  EMPTY_ISSUE_TEST_SUMMARY,
  EMPTY_TEST_PLAN_STATS,
  IssueTestSummarySchema,
  TestPlanStatsSchema,
} from "./schemas";

describe("IssueTestSummarySchema", () => {
  it("parses a full summary and keeps unknown fields", () => {
    const parsed = IssueTestSummarySchema.parse({
      cases: 2,
      verified: true,
      latest_run: { id: "r1", title: "Round 1", status: "completed", created_at: "t", completed_at: null, results: { passed: 2 } },
      defects: [{ issue_id: "d1", issue_number: 12, title: "Crash", status: "todo", run_id: "r1", run_title: "Round 1", run_case_id: "rc1", case_key: "TC-3", result: "failed", opened_at: null }],
      found_by: [],
      extra: true,
    });
    expect(parsed.verified).toBe(true);
    expect(parsed.latest_run?.results.passed).toBe(2);
    expect(parsed.defects[0]?.case_key).toBe("TC-3");
    expect((parsed as Record<string, unknown>).extra).toBe(true);
  });

  it("never invents verification from a malformed response", () => {
    const parsed = parseWithFallback({ cases: "two", verified: "yes" }, IssueTestSummarySchema, EMPTY_ISSUE_TEST_SUMMARY, {
      endpoint: "GET /api/issues/:id/test-summary",
    });
    expect(parsed).toEqual(EMPTY_ISSUE_TEST_SUMMARY);
    expect(parsed.verified).toBe(false);
  });
});

describe("TestPlanStatsSchema", () => {
  it("accepts a null pass rate and rejects one outside 0..1", () => {
    expect(TestPlanStatsSchema.parse({ runs: [{ id: "r1", pass_rate: null }] }).runs[0]?.pass_rate).toBeNull();
    expect(TestPlanStatsSchema.safeParse({ runs: [{ id: "r1", pass_rate: 1.5 }] }).success).toBe(false);
  });

  it("falls back to empty stats on garbage", () => {
    expect(parseWithFallback("nope", TestPlanStatsSchema, EMPTY_TEST_PLAN_STATS, { endpoint: "GET /api/test-plans/:id/stats" })).toEqual(EMPTY_TEST_PLAN_STATS);
  });
});
