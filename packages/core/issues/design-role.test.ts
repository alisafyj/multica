import { describe, expect, it } from "vitest";
import { ISSUE_DESIGN_ROLE_FRONTEND, ISSUE_DESIGN_ROLE_UI, inferIssueDesignRoleFromTitle, issueDesignRole } from "./design-role";

describe("issueDesignRole", () => {
  it("uses explicit metadata before title fallback", () => {
    expect(issueDesignRole({ title: "前端开发", metadata: { design_role: ISSUE_DESIGN_ROLE_UI } })).toBe(ISSUE_DESIGN_ROLE_UI);
    expect(issueDesignRole({ title: "UI设计", metadata: { design_role: ISSUE_DESIGN_ROLE_FRONTEND } })).toBe(ISSUE_DESIGN_ROLE_FRONTEND);
  });

  it("falls back to title heuristics for legacy issues", () => {
    expect(issueDesignRole({ title: "UI设计", metadata: {} })).toBe(ISSUE_DESIGN_ROLE_UI);
    expect(issueDesignRole({ title: "frontend work", metadata: {} })).toBe(ISSUE_DESIGN_ROLE_FRONTEND);
  });

  it("returns null for unmarked non-design titles", () => {
    expect(issueDesignRole({ title: "服务记录开发", metadata: {} })).toBeNull();
  });
});

describe("inferIssueDesignRoleFromTitle", () => {
  it("infers common UI and frontend child issue titles", () => {
    expect(inferIssueDesignRoleFromTitle("UI设计")).toBe(ISSUE_DESIGN_ROLE_UI);
    expect(inferIssueDesignRoleFromTitle("前端开发")).toBe(ISSUE_DESIGN_ROLE_FRONTEND);
  });
});
