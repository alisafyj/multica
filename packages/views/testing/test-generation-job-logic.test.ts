import { describe, expect, it } from "vitest";
import type { TestCaseProposal } from "@multica/core/types";
import {
  aggregateProposalStats,
  validatePlanJson,
  planRepos,
  planModules,
  planInstructions,
} from "./test-generation-job-logic";

function makeProposal(overrides: Partial<TestCaseProposal> = {}): TestCaseProposal {
  return {
    id: "p1",
    workspace_id: "ws-1",
    job_id: "job-1",
    target_case_id: "case-1",
    kind: "update",
    payload: {},
    rationale: "",
    status: "pending",
    reviewed_by: null,
    reviewed_at: null,
    created_at: "",
    ...overrides,
  };
}

describe("aggregateProposalStats", () => {
  it("counts pending, accepted and rejected correctly", () => {
    const proposals = [
      makeProposal({ status: "pending" }),
      makeProposal({ status: "pending" }),
      makeProposal({ status: "accepted" }),
      makeProposal({ status: "rejected" }),
    ];
    expect(aggregateProposalStats(proposals)).toEqual({
      total: 4,
      pending: 2,
      accepted: 1,
      rejected: 1,
    });
  });

  it("returns all zeros for an empty list", () => {
    expect(aggregateProposalStats([])).toEqual({
      total: 0,
      pending: 0,
      accepted: 0,
      rejected: 0,
    });
  });

  it("treats unknown statuses as pending rather than crashing", () => {
    // A newer backend may emit a status the frontend does not recognise yet.
    // Cast through unknown to simulate the runtime value escaping the union type.
    const unknownStatus = { ...makeProposal(), status: "reviewing" } as unknown as TestCaseProposal;
    expect(aggregateProposalStats([unknownStatus])).toEqual({
      total: 1,
      pending: 1,
      accepted: 0,
      rejected: 0,
    });
  });

  it("handles an all-accepted list", () => {
    const proposals = [
      makeProposal({ status: "accepted" }),
      makeProposal({ status: "accepted" }),
    ];
    const stats = aggregateProposalStats(proposals);
    expect(stats).toEqual({ total: 2, pending: 0, accepted: 2, rejected: 0 });
  });
});

describe("validatePlanJson", () => {
  it("returns null for a valid JSON object", () => {
    expect(
      validatePlanJson('{"repos":[],"modules":[],"instructions":""}'),
    ).toBeNull();
  });

  it("returns an error string for malformed JSON", () => {
    const result = validatePlanJson("{bad json");
    expect(result).not.toBeNull();
    expect(typeof result).toBe("string");
  });

  it("returns an error when the JSON is not an object", () => {
    expect(validatePlanJson('"just a string"')).not.toBeNull();
    expect(validatePlanJson("[1, 2, 3]")).not.toBeNull();
    expect(validatePlanJson("42")).not.toBeNull();
    expect(validatePlanJson("null")).not.toBeNull();
  });

  it("accepts an object with unknown extra keys (lenient)", () => {
    expect(
      validatePlanJson('{"future_key":"value","repos":[]}'),
    ).toBeNull();
  });
});

describe("planRepos", () => {
  it("extracts the alias from each repo entry", () => {
    const plan = {
      repos: [
        { alias: "backend", project_resource_id: "r1", path_globs: [] },
        { alias: "frontend", project_resource_id: "r2", path_globs: [] },
      ],
    };
    expect(planRepos(plan)).toEqual(["backend", "frontend"]);
  });

  it("returns an empty array when repos is absent", () => {
    expect(planRepos({})).toEqual([]);
  });

  it("returns an empty array when repos is not an array", () => {
    expect(planRepos({ repos: "not-an-array" })).toEqual([]);
    expect(planRepos({ repos: 42 })).toEqual([]);
  });

  it("skips entries that have no alias field", () => {
    const plan = {
      repos: [
        { project_resource_id: "r1", path_globs: [] },
        { alias: "api", project_resource_id: "r2", path_globs: [] },
      ],
    };
    expect(planRepos(plan)).toEqual(["api"]);
  });
});

describe("planModules", () => {
  it("returns the modules list when present", () => {
    expect(planModules({ modules: ["auth", "billing"] })).toEqual([
      "auth",
      "billing",
    ]);
  });

  it("returns an empty array when modules is absent", () => {
    expect(planModules({})).toEqual([]);
  });

  it("returns an empty array when modules is not an array", () => {
    expect(planModules({ modules: 42 })).toEqual([]);
    expect(planModules({ modules: null })).toEqual([]);
  });

  it("filters out non-string items", () => {
    expect(planModules({ modules: ["auth", 42, null, "billing"] })).toEqual([
      "auth",
      "billing",
    ]);
  });
});

describe("planInstructions", () => {
  it("returns the instructions string when present", () => {
    expect(planInstructions({ instructions: "Focus on edge cases." })).toBe(
      "Focus on edge cases.",
    );
  });

  it("returns an empty string when instructions is absent", () => {
    expect(planInstructions({})).toBe("");
  });

  it("returns an empty string when instructions is not a string", () => {
    expect(planInstructions({ instructions: 123 })).toBe("");
    expect(planInstructions({ instructions: null })).toBe("");
  });
});
