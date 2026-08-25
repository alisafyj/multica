import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { DesignNextSteps, NEXT_STEPS } from "./design-next-steps";

describe("DesignNextSteps", () => {
  // A suggestion is a prompt seed, not an action: it hands back editable text
  // so the user always owns what reaches the agent.
  it("hands the picked instruction back instead of dispatching it", async () => {
    const user = userEvent.setup();
    const onPick = vi.fn();
    render(<DesignNextSteps onPick={onPick} disabled={false} />);

    await user.click(screen.getByRole("button", { name: "设计润色" }));

    expect(onPick).toHaveBeenCalledTimes(1);
    expect(onPick).toHaveBeenCalledWith(NEXT_STEPS[0]?.instruction);
  });

  it("carries the full instruction as the chip's title, so it is readable before it is sent", () => {
    render(<DesignNextSteps onPick={vi.fn()} disabled={false} />);

    for (const step of NEXT_STEPS) {
      expect(screen.getByRole("button", { name: step.label })).toHaveAttribute("title", step.instruction);
    }
  });

  it("stops offering follow-ups while a run is in flight", async () => {
    const user = userEvent.setup();
    const onPick = vi.fn();
    render(<DesignNextSteps onPick={onPick} disabled />);

    const chip = screen.getByRole("button", { name: "响应式检查" });
    expect(chip).toBeDisabled();
    await user.click(chip);
    expect(onPick).not.toHaveBeenCalled();
  });

  // The panel used to hold a permanent chip above the composer. It belongs with
  // the other follow-ups: it is a request you make of a design that exists, and
  // it is the only one that adds tooling rather than refining the design, so it
  // goes last.
  it("carries the tweaks panel as the last follow-up", async () => {
    const onPick = vi.fn();
    render(<DesignNextSteps onPick={onPick} disabled={false} />);

    expect(NEXT_STEPS[NEXT_STEPS.length - 1]?.id).toBe("tweaks");
    await userEvent.click(screen.getByRole("button", { name: "调整面板" }));

    const instruction = onPick.mock.calls[0]?.[0] as string;
    // Every design is token-driven now, so the request is the control surface —
    // it must not ask the agent to rethread a stylesheet that already reads the
    // variables, and --mode is the one it still has to add.
    expect(instruction).toContain("--accent / --scale / --density / --motion");
    expect(instruction).toContain("--mode");
    expect(instruction).not.toContain("通过 CSS 自定义属性");
  });
});
