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
});
