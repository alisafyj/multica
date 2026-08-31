import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { DesignMvpAssociationDialog } from "./design-mvp-association-dialog";
import type { DesignMvpRepository } from "./design-mvp-workspace";

const repositories: DesignMvpRepository[] = [
  { id: "repo-1", projectId: "project-1", projectTitle: "CRM", label: "web", repositoryUrl: "https://github.com/example/web", defaultBranchHint: "main" },
];

describe("DesignMvpAssociationDialog", () => {
  it("requires an explicit target and confirms first association without an early mutation", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    render(
      <DesignMvpAssociationDialog
        open
        item={{ id: "file-1", kind: "design_file", projectId: "project-1", projectResourceId: null, title: "Login design", sourceLabel: "Figma" }}
        repositories={repositories}
        pending={false}
        error={null}
        onClose={vi.fn()}
        onConfirm={onConfirm}
      />,
    );
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("Figma · Login design")).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "确认关联" }));
    expect(onConfirm).not.toHaveBeenCalled();
    await user.selectOptions(within(dialog).getByLabelText("选择目标仓库"), "repo-1");
    await user.click(within(dialog).getByRole("button", { name: "确认关联" }));
    await waitFor(() => expect(onConfirm).toHaveBeenCalledWith("repo-1"));
  });

  it("offers unlink and preserves asset and target after a rejected change", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn().mockRejectedValue(new Error("network unavailable"));
    render(
      <DesignMvpAssociationDialog
        open
        item={{ id: "doc-1", kind: "design_document", projectId: "project-1", projectResourceId: "repo-1", title: "Settings page", sourceLabel: "Multica Design" }}
        repositories={repositories}
        pending={false}
        error={null}
        onClose={vi.fn()}
        onConfirm={onConfirm}
      />,
    );
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByRole("button", { name: "取消关联" })).toBeInTheDocument();
    expect(within(dialog).getByLabelText("选择目标仓库")).toHaveValue("repo-1");
    await user.click(within(dialog).getByRole("button", { name: "确认更换" }));
    expect(await within(dialog).findByText(/仓库关联失败/)).toBeInTheDocument();
    expect(within(dialog).getByText("Multica Design · Settings page")).toBeInTheDocument();
    expect(within(dialog).getByLabelText("选择目标仓库")).toHaveValue("repo-1");
  });
});
