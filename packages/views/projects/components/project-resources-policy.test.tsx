// @vitest-environment jsdom

import { fireEvent, screen, waitFor } from "@testing-library/react";
import type { GithubRepoResourceRef } from "@multica/core/types";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";

const updateMock = vi.fn().mockResolvedValue({});
const defaultResourceRef = {
  url: "https://github.com/acme/app.git",
  ref: "release",
  default_branch_hint: "main",
  configuration_policy: "restricted" as const,
  mcp_servers: ["docs"],
};
let resourceRef: GithubRepoResourceRef = defaultResourceRef;
const resource = {
  id: "repo-1",
  project_id: "p1",
  workspace_id: "workspace-1",
  resource_type: "github_repo",
  label: null,
  position: 0,
  created_at: "2026-09-06T00:00:00Z",
  created_by: "u1",
};

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { queryKey?: unknown[] }) =>
    options.queryKey?.[0] === "project-resources"
      ? { data: [{ ...resource, resource_ref: resourceRef }] }
      : { data: [] },
  queryOptions: (options: unknown) => options,
}));
vi.mock("@multica/core/projects", () => ({
  projectResourcesOptions: () => ({ queryKey: ["project-resources"] }),
  useCreateProjectResource: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateProjectResource: () => ({ mutateAsync: updateMock }),
  useDeleteProjectResource: () => ({ mutateAsync: vi.fn() }),
}));
vi.mock("@multica/core/config", () => ({
  useConfigStore: (selector: (state: { localWorktreeSupported: boolean }) => unknown) =>
    selector({ localWorktreeSupported: true }),
}));
vi.mock("@multica/core/runtimes", () => ({
  runtimeListOptions: () => ({ queryKey: ["runtimes"] }),
  runtimeAdvertisesLocalWorktree: () => true,
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@multica/core/paths", () => ({
  useCurrentWorkspace: () => ({ id: "workspace-1", slug: "ws", repos: [] }),
  useWorkspacePaths: () => ({ designDocument: vi.fn() }),
}));
vi.mock("../../platform", () => ({
  isDesktopShell: () => false,
  pickDirectory: vi.fn(),
  useLocalDaemonStatus: () => ({ daemonId: null, running: false }),
  validateLocalDirectory: vi.fn(),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { ProjectResourcesSection } from "./project-resources-section";

describe("GitHub project resource configuration", () => {
  beforeEach(() => {
    resourceRef = defaultResourceRef;
    updateMock.mockReset();
    updateMock.mockImplementation(async ({ data }: { data: { resource_ref: GithubRepoResourceRef } }) => {
      resourceRef = data.resource_ref;
      return {};
    });
  });

  it("preserves repository identity while saving trust and selected MCP names", async () => {
    renderWithI18n(<ProjectResourcesSection projectId="p1" />);

    fireEvent.click(screen.getByRole("button", { name: /repository settings/i }));
    fireEvent.click(screen.getByRole("checkbox", { name: /trust repository configuration/i }));
    fireEvent.change(screen.getByLabelText(/mcp server names/i), {
      target: { value: "docs\nbuild" },
    });
    fireEvent.click(screen.getByRole("checkbox", { name: /download go modules/i }));
    fireEvent.click(screen.getByRole("checkbox", { name: /install pnpm dependencies/i }));
    fireEvent.change(screen.getByLabelText(/setup timeout/i), { target: { value: "300" } });
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => expect(updateMock).toHaveBeenCalledTimes(1));
    expect(updateMock).toHaveBeenCalledWith({
      resourceId: "repo-1",
      data: {
        resource_ref: {
          url: "https://github.com/acme/app.git",
          ref: "release",
          default_branch_hint: "main",
          configuration_policy: "trusted",
          mcp_servers: ["docs", "build"],
          setup: {
            steps: ["go_mod_download", "pnpm_install"],
            timeout_seconds: 300,
          },
        },
      },
    });
  });

  it("prepopulates and preserves a saved pnpm setup directory after reopening", async () => {
    resourceRef = {
      ...defaultResourceRef,
      configuration_policy: "trusted",
      setup: {
        steps: ["pnpm_install"],
        timeout_seconds: 300,
        step_directories: { pnpm_install: "apps/web" },
      },
    };
    renderWithI18n(<ProjectResourcesSection projectId="p1" />);

    fireEvent.click(screen.getByRole("button", { name: /repository settings/i }));
    expect(screen.getByLabelText(/pnpm directory/i)).toHaveValue("apps/web");
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));
    await waitFor(() => expect(updateMock).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByRole("button", { name: /repository settings/i }));
    expect(screen.getByLabelText(/pnpm directory/i)).toHaveValue("apps/web");
  });

  it("keeps legacy setup steps at the repository root when the directory is blank", async () => {
    resourceRef = {
      ...defaultResourceRef,
      configuration_policy: "trusted",
      setup: { steps: ["pnpm_install"], timeout_seconds: 300 },
    };
    renderWithI18n(<ProjectResourcesSection projectId="p1" />);

    fireEvent.click(screen.getByRole("button", { name: /repository settings/i }));
    expect(screen.getByLabelText(/pnpm directory/i)).toHaveValue("");
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => expect(updateMock).toHaveBeenCalledTimes(1));
    expect(updateMock.mock.calls[0]?.[0].data.resource_ref.setup).toEqual({
      steps: ["pnpm_install"],
      timeout_seconds: 300,
    });
  });

  it("removes a disabled step directory from the saved setup", async () => {
    resourceRef = {
      ...defaultResourceRef,
      configuration_policy: "trusted",
      setup: {
        steps: ["go_mod_download", "pnpm_install"],
        timeout_seconds: 300,
        step_directories: {
          go_mod_download: "server",
          pnpm_install: "apps/web",
        },
      },
    };
    renderWithI18n(<ProjectResourcesSection projectId="p1" />);

    fireEvent.click(screen.getByRole("button", { name: /repository settings/i }));
    fireEvent.click(screen.getByRole("checkbox", { name: /install pnpm dependencies/i }));
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => expect(updateMock).toHaveBeenCalledTimes(1));
    expect(updateMock.mock.calls[0]?.[0].data.resource_ref.setup).toEqual({
      steps: ["go_mod_download"],
      timeout_seconds: 300,
      step_directories: { go_mod_download: "server" },
    });
  });

  it("does not save an invalid setup directory", async () => {
    resourceRef = {
      ...defaultResourceRef,
      configuration_policy: "trusted",
      setup: { steps: ["pnpm_install"], timeout_seconds: 300 },
    };
    renderWithI18n(<ProjectResourcesSection projectId="p1" />);

    fireEvent.click(screen.getByRole("button", { name: /repository settings/i }));
    fireEvent.change(screen.getByLabelText(/pnpm directory/i), {
      target: { value: "../web" },
    });
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => expect(screen.getByText(/valid repository-relative directory/i)).toBeInTheDocument());
    expect(updateMock).not.toHaveBeenCalled();
  });

  it("keeps the dialog open after a failed save and allows retry", async () => {
    updateMock.mockRejectedValueOnce(new Error("sensitive API failure"));
    renderWithI18n(<ProjectResourcesSection projectId="p1" />);

    fireEvent.click(screen.getByRole("button", { name: /repository settings/i }));
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

    expect(await screen.findByText("Failed to save repository settings.")).toBeInTheDocument();
    expect(screen.queryByText(/sensitive API failure/i)).not.toBeInTheDocument();
    expect(screen.getByRole("dialog")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));
    await waitFor(() => expect(updateMock).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });
});
