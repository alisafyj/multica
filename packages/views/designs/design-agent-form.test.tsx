import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { parseQuestionForm, type AgentQuestionForm } from "@multica/core/designs/agent-ui";
import { DesignAgentCard, DesignAgentForm } from "./design-agent-form";

function form(json: object): AgentQuestionForm {
  return parseQuestionForm('<question-form id="direction" title="先定个方向">', JSON.stringify(json))!;
}

describe("DesignAgentForm", () => {
  it("holds submission until every required question is answered, then hands back the agent's text", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(
      <DesignAgentForm
        onSubmit={onSubmit}
        form={form({
          questions: [
            { id: "tone", label: "气质", type: "radio", options: ["克制", "热烈"], required: true, allowCustom: false },
            { id: "audience", label: "受众", type: "text" },
          ],
        })}
      />,
    );

    const submit = screen.getByRole("button", { name: "填入调整" });
    expect(submit).toBeDisabled();
    // It says which question is holding it, rather than sitting dead.
    expect(screen.getByRole("status")).toHaveTextContent("气质");

    await user.click(screen.getByRole("radio", { name: "克制" }));
    await user.type(screen.getByLabelText("受众"), "SaaS 采购方");
    expect(submit).toBeEnabled();
    await user.click(submit);

    expect(onSubmit).toHaveBeenCalledWith(
      ["[form answers — direction]", "- 气质: 克制", "- 受众: SaaS 采购方"].join("\n"),
      { tone: "克制", audience: "SaaS 采购方" },
    );
  });

  it("keeps the escape hatch usable on a finite-choice question", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(
      <DesignAgentForm
        onSubmit={onSubmit}
        form={form({ questions: [{ id: "tone", label: "气质", type: "radio", options: ["克制"], customLabel: "自己写" }] })}
      />,
    );

    await user.click(screen.getByRole("radio", { name: "自己写" }));
    await user.type(screen.getByLabelText("气质（自定义）"), "冷冽");
    await user.click(screen.getByRole("button", { name: "填入调整" }));
    expect(onSubmit).toHaveBeenCalledWith(expect.stringContaining("- 气质: 冷冽"), { tone: "冷冽" });
  });

  it("collects a multi-select as a list", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(
      <DesignAgentForm
        onSubmit={onSubmit}
        form={form({ questions: [{ id: "surface", label: "平台", type: "checkbox", options: [
          { value: "web", label: "桌面网页" }, { value: "app", label: "移动应用" },
        ] }] })}
      />,
    );

    await user.click(screen.getByRole("checkbox", { name: "桌面网页" }));
    await user.click(screen.getByRole("checkbox", { name: "移动应用" }));
    await user.click(screen.getByRole("button", { name: "填入调整" }));
    expect(onSubmit).toHaveBeenCalledWith(expect.stringContaining("- 平台: 桌面网页、移动应用"), {
      surface: ["web", "app"],
    });
  });

  // The point of direction-cards: the choice is made by looking, so the
  // palette and the type sample have to actually render.
  it("renders a direction card's palette and type sample", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(
      <DesignAgentForm
        onSubmit={onSubmit}
        form={form({ questions: [{ id: "direction", label: "视觉方向", type: "direction-cards", cards: [
          { id: "editorial", label: "编辑风", mood: "克制的杂志感", references: ["Monocle"],
            palette: ["#111111", "#eeeeee"], displayFont: "Georgia", bodyFont: "Inter" },
        ] }] })}
      />,
    );

    const card = screen.getByRole("radio", { name: /编辑风/ });
    expect(card).toHaveTextContent("克制的杂志感");
    expect(card).toHaveTextContent("Monocle");
    expect(card).toHaveTextContent("Aa");

    await user.click(card);
    expect(card).toHaveAttribute("aria-checked", "true");
    await user.click(screen.getByRole("button", { name: "填入调整" }));
    expect(onSubmit).toHaveBeenCalledWith(expect.stringContaining("- 视觉方向: 编辑风"), { direction: "editorial" });
  });

  // A surface with nowhere to send a reply must not offer one.
  it("renders read-only without a submit handler, and locked once answered", () => {
    const one = form({ questions: [{ id: "tone", label: "气质", type: "radio", options: ["克制"] }] });
    const { rerender } = render(<DesignAgentForm form={one} />);
    expect(screen.queryByRole("button", { name: "填入调整" })).not.toBeInTheDocument();

    rerender(<DesignAgentForm form={one} onSubmit={vi.fn()} submittedAnswers={{ tone: "克制" }} />);
    expect(screen.getByText("已回答")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "克制" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "填入调整" })).not.toBeInTheDocument();
  });
});

describe("DesignAgentCard", () => {
  it("shows a scorecard's rows and flags the ones that did not pass", () => {
    render(
      <DesignAgentCard
        card={{
          kind: "verify-scorecard", title: "自检", body: "", items: [], status: "pass",
          rows: [{ label: "对比度", verdict: "pass", note: "AA" }, { label: "焦点态", verdict: "fail", note: "缺失" }],
        }}
      />,
    );
    const card = screen.getByLabelText("自检");
    expect(card).toHaveTextContent("对比度");
    expect(card).toHaveTextContent("AA");
    expect(card).toHaveTextContent("缺失");
    expect(screen.getByText("pass")).toBeInTheDocument();
  });
});
