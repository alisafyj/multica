import { describe, expect, it } from "vitest";
import type { TestCase } from "@multica/core/types";
import {
  crossRepoWarning,
  formatRepoSummary,
  groupByModule,
  knownEnumKey,
  normalizeStepIndexes,
  repoAliases,
} from "./case-summary";

function makeCase(overrides: Partial<TestCase> = {}): TestCase {
  return {
    id: "case-1",
    workspace_id: "ws-1",
    project_id: "p1",
    case_number: 1,
    key: "TC-1",
    title: "用例",
    module: "",
    preconditions: "",
    steps: [],
    expected_result: "",
    test_data: {},
    priority: "p2",
    case_type: "functional",
    scope: "single_repo",
    execution_mode: "manual",
    required_capabilities: [],
    business_rules_ref: [],
    status: "active",
    origin: "human",
    source_refs: {},
    generation_job_id: null,
    version: 1,
    repos: [],
    created_by: null,
    updated_by: null,
    reviewed_by: null,
    reviewed_at: null,
    created_at: "",
    updated_at: "",
    ...overrides,
  };
}

describe("groupByModule", () => {
  it("puts cases with no module in one explicit bucket that sorts first", () => {
    const groups = groupByModule([
      makeCase({ id: "a", module: "订单" }),
      makeCase({ id: "b", module: "" }),
      makeCase({ id: "c", module: "计费" }),
    ]);
    expect(groups.map((group) => group.module)).toEqual(["", "计费", "订单"]);
    expect(groups[0]?.cases.map((c) => c.id)).toEqual(["b"]);
  });

  it("keeps every case in exactly one bucket", () => {
    const groups = groupByModule([
      makeCase({ id: "a", module: "订单" }),
      makeCase({ id: "b", module: "订单" }),
    ]);
    expect(groups).toHaveLength(1);
    expect(groups[0]?.cases).toHaveLength(2);
  });

  it("returns an empty array for no cases", () => {
    expect(groupByModule([])).toEqual([]);
  });
});

describe("normalizeStepIndexes", () => {
  it("renumbers to 1..n after a middle step is removed", () => {
    const steps = [
      { index: 1, action: "a", expected: "x" },
      { index: 3, action: "c", expected: "z" },
    ];
    expect(normalizeStepIndexes(steps).map((step) => step.index)).toEqual([1, 2]);
  });

  it("preserves the other step fields", () => {
    const normalized = normalizeStepIndexes([
      { index: 7, action: "点击", expected: "跳转", repo: "admin-web" },
    ]);
    expect(normalized[0]).toEqual({ index: 1, action: "点击", expected: "跳转", repo: "admin-web" });
  });

  it("handles an empty list", () => {
    expect(normalizeStepIndexes([])).toEqual([]);
  });
});

describe("formatRepoSummary", () => {
  it("renders alias(role) pairs", () => {
    const summary = formatRepoSummary(
      makeCase({
        repos: [
          { project_resource_id: "r1", alias: "admin-web", role: "driver", path_globs: [] },
          { project_resource_id: "r2", alias: "mobile-app", role: "verifier", path_globs: [] },
        ],
      }),
    );
    expect(summary).toBe("admin-web(driver), mobile-app(verifier)");
  });

  it("returns an empty string when no repositories are linked", () => {
    expect(formatRepoSummary(makeCase())).toBe("");
  });
});

describe("crossRepoWarning", () => {
  it("returns null for a single-repo case regardless of bindings", () => {
    expect(crossRepoWarning(makeCase({ scope: "single_repo" }))).toBeNull();
  });

  it("flags a cross-repo case with fewer than two repositories", () => {
    expect(
      crossRepoWarning(
        makeCase({
          scope: "cross_repo",
          repos: [{ project_resource_id: "r1", alias: "a", role: "driver", path_globs: [] }],
        }),
      ),
    ).toBe("missing_repos");
  });

  it("flags a cross-repo case whose repositories all share one role", () => {
    expect(
      crossRepoWarning(
        makeCase({
          scope: "cross_repo",
          repos: [
            { project_resource_id: "r1", alias: "a", role: "under_test", path_globs: [] },
            { project_resource_id: "r2", alias: "b", role: "under_test", path_globs: [] },
          ],
        }),
      ),
    ).toBe("single_role");
  });

  it("accepts a coherent cross-repo case", () => {
    expect(
      crossRepoWarning(
        makeCase({
          scope: "cross_repo",
          repos: [
            { project_resource_id: "r1", alias: "a", role: "driver", path_globs: [] },
            { project_resource_id: "r2", alias: "b", role: "verifier", path_globs: [] },
          ],
        }),
      ),
    ).toBeNull();
  });
});

describe("repoAliases", () => {
  it("drops empty aliases so the step selector never offers a blank option", () => {
    expect(
      repoAliases({
        repos: [
          { project_resource_id: "r1", alias: "admin-web", role: "driver", path_globs: [] },
          { project_resource_id: "r2", alias: "", role: "verifier", path_globs: [] },
        ],
      }),
    ).toEqual(["admin-web"]);
  });
});

describe("knownEnumKey", () => {
  it("returns the value when it is a known key", () => {
    expect(knownEnumKey("draft", ["draft", "active"] as const)).toBe("draft");
  });

  it("returns null for a value the frontend does not know", () => {
    expect(knownEnumKey("quarantined", ["draft", "active"] as const)).toBeNull();
  });
});
