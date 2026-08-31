import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { DesignMvpViewSwitcher } from "./design-mvp-view-switcher";

describe("DesignMvpViewSwitcher", () => {
  it("exposes two keyboard-activatable icon controls with explicit selected state", async () => {
    const user = userEvent.setup();
    const onModeChange = vi.fn();
    render(<DesignMvpViewSwitcher mode="project" onModeChange={onModeChange} />);

    const group = screen.getByRole("group", { name: "设计中心视角" });
    expect(within(group).getByRole("button", { name: "项目视角" })).toHaveAttribute("aria-pressed", "true");
    expect(within(group).getByRole("button", { name: "仓库视角" })).toHaveAttribute("aria-pressed", "false");

    await user.click(screen.getByRole("button", { name: "仓库视角" }));
    expect(onModeChange).toHaveBeenCalledWith("repository");
  });
});
