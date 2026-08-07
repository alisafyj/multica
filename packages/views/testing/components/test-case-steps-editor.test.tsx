import { describe, expect, it, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import type { TestCaseStep } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { TestCaseStepsEditor } from "./test-case-steps-editor";

const STEPS: TestCaseStep[] = [
  { index: 1, action: "打开订单页", expected: "列表可见" },
  { index: 2, action: "点击下单", expected: "跳转支付页" },
  { index: 3, action: "完成支付", expected: "订单状态为已支付" },
];

describe("TestCaseStepsEditor", () => {
  it("appends a step numbered one past the last", () => {
    const onChange = vi.fn();
    renderWithI18n(
      <TestCaseStepsEditor value={STEPS} onChange={onChange} repoAliases={[]} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Add step" }));

    const next = onChange.mock.calls[0]?.[0] as TestCaseStep[];
    expect(next).toHaveLength(4);
    expect(next[3]?.index).toBe(4);
    expect(next[3]?.action).toBe("");
  });

  it("renumbers the remaining steps after deleting a middle step", () => {
    const onChange = vi.fn();
    renderWithI18n(
      <TestCaseStepsEditor value={STEPS} onChange={onChange} repoAliases={[]} />,
    );

    fireEvent.click(screen.getAllByRole("button", { name: "Remove this step" })[1]!);

    const next = onChange.mock.calls[0]?.[0] as TestCaseStep[];
    expect(next.map((step) => step.index)).toEqual([1, 2]);
    expect(next.map((step) => step.action)).toEqual(["打开订单页", "完成支付"]);
  });

  it("hides the repository selector when the case has no linked repositories", () => {
    renderWithI18n(
      <TestCaseStepsEditor value={STEPS} onChange={vi.fn()} repoAliases={[]} />,
    );
    expect(screen.queryAllByRole("combobox", { name: "Repository" })).toHaveLength(0);
  });

  it("limits the repository selector to the case's own aliases", () => {
    renderWithI18n(
      <TestCaseStepsEditor
        value={[STEPS[0]!]}
        onChange={vi.fn()}
        repoAliases={["admin-web", "mobile-app"]}
      />,
    );
    const select = screen.getByRole("combobox", { name: "Repository" });
    const options = Array.from(select.querySelectorAll("option")).map((o) => o.value);
    expect(options).toEqual(["", "admin-web", "mobile-app"]);
  });

  it("edits the action of the targeted step only", () => {
    const onChange = vi.fn();
    renderWithI18n(
      <TestCaseStepsEditor value={STEPS} onChange={onChange} repoAliases={[]} />,
    );

    fireEvent.change(screen.getAllByRole("textbox", { name: "Action" })[1]!, {
      target: { value: "点击立即下单" },
    });

    const next = onChange.mock.calls[0]?.[0] as TestCaseStep[];
    expect(next[1]?.action).toBe("点击立即下单");
    expect(next[0]?.action).toBe("打开订单页");
    expect(next[2]?.action).toBe("完成支付");
  });

  it("renders the empty state when there are no steps", () => {
    renderWithI18n(<TestCaseStepsEditor value={[]} onChange={vi.fn()} repoAliases={[]} />);
    expect(screen.getByText("No steps yet")).toBeTruthy();
  });

  it("disables every control while a save is in flight", () => {
    renderWithI18n(
      <TestCaseStepsEditor value={STEPS} onChange={vi.fn()} repoAliases={[]} disabled />,
    );
    expect(screen.getByRole("button", { name: "Add step" })).toHaveProperty("disabled", true);
    for (const input of screen.getAllByRole("textbox")) {
      expect(input).toHaveProperty("disabled", true);
    }
  });
});
