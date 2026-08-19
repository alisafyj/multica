import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DesignDocumentCritique, critiqueOutcomeLabel, openFindings, parseCritique } from "./design-document-critique";

const REPORT = {
  schema_version: "multica.design-document-critique/v1",
  threshold: 8,
  max_rounds: 3,
  outcome: "stopped_at_max_rounds",
  rounds: [
    { index: 1, scores: { designer: 6, critic: 7, brand: 8, a11y: 5, copy: 8 }, findings: [
      { lens: "a11y", severity: "must_fix", summary: "筛选芯片没有可见焦点环。", resolved: true },
    ] },
    { index: 2, scores: { designer: 8, critic: 8, brand: 9, a11y: 7, copy: 8 }, findings: [
      { lens: "a11y", severity: "should_fix", summary: "表格行高不足 44px。", resolved: false },
      { lens: "copy", severity: "note", summary: "空状态可以更具体。", resolved: false },
      { lens: "a11y", severity: "must_fix", summary: "对比度 3.2:1 低于 4.5:1。", resolved: false },
    ] },
  ],
};

// The parser's matrix lives here; the render test only checks the panel
// reaches the page with the agent's numbers.
describe("parseCritique", () => {
  it("reads a valid report and keeps its rounds, scores and findings", () => {
    const critique = parseCritique(REPORT);
    expect(critique?.outcome).toBe("stopped_at_max_rounds");
    expect(critique?.rounds).toHaveLength(2);
    expect(critique?.rounds[1]?.scores.a11y).toBe(7);
    expect(openFindings(critique!).map((finding) => finding.severity)).toEqual(["must_fix", "should_fix", "note"]);
    expect(critiqueOutcomeLabel("passed")).toBe("全部达标");
  });

  it("yields null for anything that is not a report", () => {
    expect(parseCritique(null)).toBeNull();
    expect(parseCritique({})).toBeNull();
    expect(parseCritique({ rounds: [] })).toBeNull();
    expect(parseCritique({ rounds: ["nope"] })).toBeNull();
    // Unknown outcomes read as not run rather than as a pass.
    expect(parseCritique({ ...REPORT, outcome: "shipped" })?.outcome).toBe("not_run");
  });
});

describe("DesignDocumentCritique", () => {
  it("shows the last round per lens, the outcome, and the findings still open", () => {
    render(<DesignDocumentCritique critique={parseCritique(REPORT)!} />);
    const panel = screen.getByRole("region", { name: "设计评审" });
    expect(within(panel).getByText(/达到轮数上限 · 2 轮/)).toBeInTheDocument();
    // Five lenses, last-round scores.
    for (const label of ["设计师", "评审", "品牌", "无障碍", "文案"]) {
      expect(within(panel).getByText(label)).toBeInTheDocument();
    }
    expect(within(panel).getByText("对比度 3.2:1 低于 4.5:1。")).toBeInTheDocument();
    expect(within(panel).getByText("表格行高不足 44px。")).toBeInTheDocument();
    // A resolved finding from an earlier round is history, not a to-do.
    expect(within(panel).queryByText("筛选芯片没有可见焦点环。")).not.toBeInTheDocument();
    expect(within(panel).getByText(/不决定草稿是否成立/)).toBeInTheDocument();
  });
});
