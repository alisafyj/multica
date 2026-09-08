// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Agent, AgentRuntime } from "@multica/core/types";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../../locales/en/common.json";
import enAgents from "../../../locales/en/agents.json";

const TEST_RESOURCES = { en: { common: enCommon, agents: enAgents } };

const mockListSkills = vi.hoisted(() => vi.fn());
const mockGetSkill = vi.hoisted(() => vi.fn());
const mockSetAgentSkillEnabled = vi.hoisted(() => vi.fn());
const mockSetAgentRuntimeSkillEnabled = vi.hoisted(() => vi.fn());
const mockRemoveAgentSkill = vi.hoisted(() => vi.fn());
const mockRuntimeCapabilities = vi.hoisted(() => vi.fn());
const mockToastError = vi.hoisted(() => vi.fn());
const mockToastSuccess = vi.hoisted(() => vi.fn());

// ApiError mirrors the production export. The tab branches on
// `instanceof ApiError` for the 403 permission notice, so the class identity
// must match the one the mocked query rejects with.
const { ApiError } = vi.hoisted(() => {
  class ApiError extends Error {
    status: number;
    statusText: string;
    constructor(message: string, status: number, statusText: string) {
      super(message);
      this.status = status;
      this.statusText = statusText;
    }
  }
  return { ApiError };
});

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/api", () => ({
  api: {
    listSkills: (...args: unknown[]) => mockListSkills(...args),
    getSkill: (...args: unknown[]) => mockGetSkill(...args),
    setAgentSkills: vi.fn(),
    setAgentSkillEnabled: (...args: unknown[]) => mockSetAgentSkillEnabled(...args),
    setAgentRuntimeSkillEnabled: (...args: unknown[]) =>
      mockSetAgentRuntimeSkillEnabled(...args),
    removeAgentSkill: (...args: unknown[]) => mockRemoveAgentSkill(...args),
  },
  ApiError,
}));

vi.mock("@multica/core/runtimes", async () => {
  const actual =
    await vi.importActual<typeof import("@multica/core/runtimes")>(
      "@multica/core/runtimes",
    );
  return {
    ...actual,
    runtimeCapabilitiesOptions: (runtimeId: string | null) => ({
      queryKey: ["runtime-capabilities", runtimeId],
      queryFn: () => mockRuntimeCapabilities(runtimeId),
      enabled: Boolean(runtimeId),
      retry: false,
    }),
  };
});

vi.mock("sonner", () => ({
  toast: {
    error: mockToastError,
    success: mockToastSuccess,
  },
}));

import { SkillsTab } from "./skills-tab";

const agent: Agent = {
  id: "agent-1",
  workspace_id: "ws-1",
  runtime_id: "runtime-1",
  name: "Agent",
  description: "",
  instructions: "",
  avatar_url: null,
  runtime_mode: "local",
  runtime_config: {},
  custom_args: [],
  visibility: "workspace",
  permission_mode: "public_to",
  invocation_targets: [{ target_type: "workspace", target_id: null }],
  status: "idle",
  max_concurrent_tasks: 1,
  model: "",
  owner_id: "user-1",
  skills: [],
  created_at: "2026-04-16T00:00:00Z",
  updated_at: "2026-04-16T00:00:00Z",
  archived_at: null,
  archived_by: null,
};

const onlineRuntime: AgentRuntime = {
  id: "runtime-1",
  workspace_id: "ws-1",
  daemon_id: "daemon-1",
  name: "Codex (Mac)",
  runtime_mode: "local",
  provider: "codex",
  launch_header: "",
  status: "online",
  device_info: "Mac",
  metadata: {},
  owner_id: "user-1",
  visibility: "private",
  last_seen_at: null,
  created_at: "2026-07-11T00:00:00Z",
  updated_at: "2026-07-11T00:00:00Z",
};

function renderSkillsTab(
  agentOverrides: Partial<Agent> = {},
  runtime: AgentRuntime | null = null,
  currentUserId: string | null = "user-1",
  canEdit = true,
) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

  const view = (
    nextAgentOverrides: Partial<Agent>,
    nextRuntime: AgentRuntime | null,
    nextCurrentUserId: string | null,
    nextCanEdit: boolean,
  ) => (
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <QueryClientProvider client={queryClient}>
        <SkillsTab
          agent={{ ...agent, ...nextAgentOverrides }}
          runtime={nextRuntime}
          currentUserId={nextCurrentUserId}
          canEdit={nextCanEdit}
        />
      </QueryClientProvider>
    </I18nProvider>
  );
  const result = render(view(agentOverrides, runtime, currentUserId, canEdit));
  return {
    ...result,
    queryClient,
    rerenderSkillsTab: (
      nextAgentOverrides: Partial<Agent>,
      nextRuntime: AgentRuntime | null,
      nextCurrentUserId = currentUserId,
      nextCanEdit = canEdit,
    ) => result.rerender(
      view(nextAgentOverrides, nextRuntime, nextCurrentUserId, nextCanEdit),
    ),
  };
}

describe("SkillsTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListSkills.mockResolvedValue([]);
    mockSetAgentSkillEnabled.mockResolvedValue(undefined);
    mockSetAgentRuntimeSkillEnabled.mockResolvedValue(undefined);
    mockRemoveAgentSkill.mockResolvedValue(undefined);
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });
  });

  it("separates workspace assignments from inherited runtime skills", async () => {
    renderSkillsTab();

    expect(
      await screen.findByText("Assigned to agent"),
    ).toBeInTheDocument();
    expect(screen.getByText("Inherited from runtime")).toBeInTheDocument();
    expect(screen.getByText(/Assign a local runtime/i)).toBeInTheDocument();
  });

  it("disables an assigned skill without removing it", async () => {
    const user = userEvent.setup();
    renderSkillsTab({
      skills: [
        {
          id: "skill-1",
          name: "Review changes",
          description: "Review a patch",
          enabled: true,
        },
      ],
    });

    await user.click(screen.getByRole("switch", { name: /Toggle Review changes/i }));

    expect(mockSetAgentSkillEnabled).toHaveBeenCalledWith(
      "agent-1",
      "skill-1",
      false,
    );
    expect(mockRemoveAgentSkill).not.toHaveBeenCalled();
  });

  it("shows inherited skills discovered from the assigned runtime", async () => {
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [
        {
          key: "local-review",
          name: "Local review",
          description: "Host-level review workflow",
          source_path: "~/.codex/skills/local-review",
          provider: "codex",
          root: "provider",
          file_count: 2,
        },
      ],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });

    renderSkillsTab({}, onlineRuntime);

    expect(await screen.findByText("Local review")).toBeInTheDocument();
    expect(screen.getByText("Host-level review workflow")).toBeInTheDocument();
  });

  it("turns a controllable inherited skill off for this agent", async () => {
    const user = userEvent.setup();
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [
        {
          key: "local-review",
          name: "Local review",
          source_path: "~/.codex/skills/local-review",
          provider: "codex",
          root: "provider",
          can_disable: true,
          file_count: 1,
        },
      ],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });

    renderSkillsTab({}, onlineRuntime);
    await user.click(
      await screen.findByRole("switch", {
        name: /Toggle inherited Local review/i,
      }),
    );

    expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledWith("agent-1", {
      runtime_id: "runtime-1",
      root: "provider",
      key: "local-review",
      name: "Local review",
      plugin: undefined,
      enabled: false,
    });
  });

  it("shows an unavailable file count without disabling runtime control", async () => {
    const user = userEvent.setup();
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [
        runtimeSkill("local-review", "Local review", {
          can_import: false,
          file_count: 0,
        }),
      ],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });

    renderSkillsTab({}, onlineRuntime);

    const toggle = await screen.findByRole("switch", {
      name: /Toggle inherited Local review/i,
    });
    await user.click(screen.getByRole("button", { name: /Local review/i }));

    expect(screen.getByText("Unavailable")).toBeInTheDocument();
    await user.click(toggle);
    expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledWith(
      "agent-1",
      expect.objectContaining({ key: "local-review", enabled: false }),
    );
  });

  it("renders a persisted inherited-skill override as off", async () => {
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [
        {
          key: "local-review",
          name: "Local review",
          source_path: "~/.codex/skills/local-review",
          provider: "codex",
          root: "provider",
          can_disable: true,
          file_count: 1,
        },
      ],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });

    renderSkillsTab(
      {
        disabled_runtime_skills: [
          {
            runtime_id: "runtime-1",
            provider: "codex",
            root: "provider",
            key: "local-review",
          },
        ],
      },
      onlineRuntime,
    );

    expect(
      await screen.findByRole("switch", {
        name: /Toggle inherited Local review/i,
      }),
    ).not.toBeChecked();
  });

  it("shows and clears a saved runtime-skill override missing from a successful empty inventory", async () => {
    const user = userEvent.setup();
    renderSkillsTab(
      {
        disabled_runtime_skills: [
          {
            runtime_id: "runtime-1",
            provider: "codex",
            root: "plugin",
            key: "writer",
            name: "Writer",
            plugin: "content-tools",
          },
        ],
      },
      onlineRuntime,
    );

    expect(await screen.findByText("Writer")).toBeInTheDocument();
    expect(screen.getByText("Saved setting")).toBeInTheDocument();
    expect(screen.getByText("Not found in current runtime")).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", {
        name: "Clear saved setting for Writer",
      }),
    );

    await waitFor(() =>
      expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledWith("agent-1", {
        runtime_id: "runtime-1",
        root: "plugin",
        key: "writer",
        name: "Writer",
        plugin: "content-tools",
        enabled: true,
      }),
    );
  });

  it("offers recovery only for missing overrides while preserving listed skills", async () => {
    const user = userEvent.setup();
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [runtimeSkill("review", "Review")],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });
    renderSkillsTab(
      {
        disabled_runtime_skills: [
          disabledRuntimeSkill("review", "Review"),
          disabledRuntimeSkill("writer", "Writer", {
            root: "plugin",
            plugin: "content-tools",
          }),
        ],
      },
      onlineRuntime,
    );

    expect(
      await screen.findByRole("switch", { name: /Toggle inherited Review/i }),
    ).not.toBeChecked();
    expect(
      screen.queryByRole("button", {
        name: "Clear saved setting for Review",
      }),
    ).not.toBeInTheDocument();

    await user.click(
      screen.getByRole("button", {
        name: "Clear saved setting for Writer",
      }),
    );

    expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledTimes(1);
    expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledWith(
      "agent-1",
      expect.objectContaining({ key: "writer", enabled: true }),
    );
  });

  it("clears only an existing saved override for a listed noncontrollable skill", async () => {
    const user = userEvent.setup();
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [
        runtimeSkill("writer", "Writer", {
          root: "plugin",
          plugin: "content-tools",
          can_disable: false,
        }),
        runtimeSkill("review", "Review"),
      ],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });
    const writerOverride = disabledRuntimeSkill("writer", "Saved Writer", {
      root: "plugin",
      plugin: "content-tools",
    });
    const editable = renderSkillsTab(
      { disabled_runtime_skills: [writerOverride] },
      onlineRuntime,
    );

    expect(await screen.findByText("Writer")).toBeInTheDocument();
    expect(screen.queryByText("Not found in current runtime")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("switch", { name: /Toggle inherited Writer/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("switch", { name: /Toggle inherited Review/i }),
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", {
        name: "Clear saved setting for Saved Writer",
      }),
    );

    expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledTimes(1);
    expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledWith("agent-1", {
      runtime_id: "runtime-1",
      root: "plugin",
      key: "writer",
      name: "Saved Writer",
      plugin: "content-tools",
      enabled: true,
    });
    editable.unmount();

    vi.clearAllMocks();
    const withoutOverride = renderSkillsTab(
      {
        disabled_runtime_skills: [
          { ...writerOverride, runtime_id: "runtime-2" },
          { ...writerOverride, provider: "claude" },
        ],
      },
      onlineRuntime,
    );
    await screen.findByText("Writer");
    expect(
      screen.queryByRole("button", {
        name: /Clear saved setting for/i,
      }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("switch", { name: /Toggle inherited Writer/i }),
    ).not.toBeInTheDocument();
    withoutOverride.unmount();

    renderSkillsTab(
      { disabled_runtime_skills: [writerOverride] },
      onlineRuntime,
      "user-1",
      false,
    );
    expect(await screen.findByText("Writer")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: "Clear saved setting for Saved Writer",
      }),
    ).not.toBeInTheDocument();
  });

  it("excludes saved overrides for unknown runtimes and other providers", async () => {
    renderSkillsTab(
      {
        disabled_runtime_skills: [
          disabledRuntimeSkill("unknown-runtime", "Unknown runtime", {
            runtime_id: "runtime-2",
          }),
          disabledRuntimeSkill("other-provider", "Other provider", {
            provider: "claude",
          }),
        ],
      },
      onlineRuntime,
    );

    expect(
      await screen.findByText("No local skills were found for this runtime."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Unknown runtime")).not.toBeInTheDocument();
    expect(screen.queryByText("Other provider")).not.toBeInTheDocument();
  });

  it("does not label saved overrides as missing while inventory discovery loads or fails", async () => {
    let rejectDiscovery!: (error: Error) => void;
    mockRuntimeCapabilities.mockReturnValue(
      new Promise((_, reject) => {
        rejectDiscovery = reject;
      }),
    );
    renderSkillsTab(
      {
        disabled_runtime_skills: [disabledRuntimeSkill("writer", "Writer")],
      },
      onlineRuntime,
    );

    expect(
      await screen.findByText("Discovering skills from the local runtime…"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Not found in current runtime")).not.toBeInTheDocument();
    expect(screen.queryByText("Writer")).not.toBeInTheDocument();

    rejectDiscovery(new Error("discovery failed"));
    expect(
      await screen.findByText("Couldn't discover runtime skills. Try again."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Not found in current runtime")).not.toBeInTheDocument();
    expect(screen.queryByText("Writer")).not.toBeInTheDocument();
  });

  it("keeps failed recovery actions retryable and respects manage permissions", async () => {
    const user = userEvent.setup();
    mockSetAgentRuntimeSkillEnabled
      .mockRejectedValueOnce(new Error("save failed"))
      .mockResolvedValueOnce(undefined);
    const savedOverride = disabledRuntimeSkill("writer", "Writer");
    const editable = renderSkillsTab(
      { disabled_runtime_skills: [savedOverride] },
      onlineRuntime,
    );
    const clearButton = await screen.findByRole("button", {
      name: "Clear saved setting for Writer",
    });

    await user.click(clearButton);
    await waitFor(() => expect(clearButton).toBeEnabled());
    expect(screen.getByText("Writer")).toBeInTheDocument();

    await user.click(clearButton);
    await waitFor(() =>
      expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledTimes(2),
    );
    editable.unmount();

    renderSkillsTab(
      { disabled_runtime_skills: [savedOverride] },
      onlineRuntime,
      "user-1",
      false,
    );
    expect(await screen.findByText("Writer")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: "Clear saved setting for Writer",
      }),
    ).not.toBeInTheDocument();
  });

  it("disables all controllable runtime skills and enables only disabled ones from a partial selection", async () => {
    const user = userEvent.setup();
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [
        runtimeSkill("review", "Review"),
        runtimeSkill("release", "Release"),
        runtimeSkill("read-only", "Read only", { can_disable: false }),
      ],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });

    const allEnabled = renderSkillsTab({}, onlineRuntime);
    const bulk = await screen.findByRole("checkbox", {
      name: /Toggle all listed runtime skills/i,
    });
    expect(bulk).toBeChecked();
    await user.click(bulk);

    await waitFor(() => expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledTimes(2));
    expect(mockSetAgentRuntimeSkillEnabled.mock.calls.map(([, value]) => value)).toEqual([
      expect.objectContaining({ runtime_id: "runtime-1", key: "review", enabled: false }),
      expect.objectContaining({ runtime_id: "runtime-1", key: "release", enabled: false }),
    ]);
    allEnabled.unmount();

    vi.clearAllMocks();
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [runtimeSkill("review", "Review"), runtimeSkill("release", "Release")],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });
    renderSkillsTab({
      disabled_runtime_skills: [{
        runtime_id: "runtime-1",
        provider: "codex",
        root: "provider",
        key: "review",
      }],
    }, onlineRuntime);
    const partial = await screen.findByRole("checkbox", {
      name: /Toggle all listed runtime skills/i,
    });
    expect(partial).toBePartiallyChecked();
    await user.click(partial);

    await waitFor(() => expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledTimes(1));
    expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledWith("agent-1", expect.objectContaining({
      runtime_id: "runtime-1",
      key: "review",
      enabled: true,
    }));
  });

  it("continues a bulk update after a failure and exposes the partial failure", async () => {
    const user = userEvent.setup();
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [runtimeSkill("review", "Review"), runtimeSkill("release", "Release")],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });
    mockSetAgentRuntimeSkillEnabled
      .mockRejectedValueOnce(new Error("first failed"))
      .mockRejectedValueOnce(new Error("second failed"));
    const { queryClient, rerenderSkillsTab } = renderSkillsTab({}, onlineRuntime);
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const bulk = await screen.findByRole("checkbox", {
      name: /Toggle all listed runtime skills/i,
    });

    await user.click(bulk);

    await waitFor(() => expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(bulk).toHaveAttribute("aria-invalid", "true"));
    expect(invalidate).toHaveBeenCalledTimes(1);
    expect(mockToastError).toHaveBeenCalledWith("first failed");
    expect(mockToastSuccess).not.toHaveBeenCalled();
    expect(bulk).toBeChecked();

    rerenderSkillsTab({
      disabled_runtime_skills: [{
        runtime_id: "runtime-1",
        provider: "codex",
        root: "provider",
        key: "release",
      }],
    }, onlineRuntime);
    expect(bulk).toBePartiallyChecked();
    expect(bulk).toHaveAttribute("aria-invalid", "true");
  });

  it("shows the API plugin limit reason without reporting bulk success", async () => {
    const user = userEvent.setup();
    const limitMessage = "cannot disable more than 128 plugin skills";
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [runtimeSkill("review", "Review"), runtimeSkill("release", "Release")],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });
    mockSetAgentRuntimeSkillEnabled
      .mockRejectedValueOnce(new Error(limitMessage))
      .mockResolvedValueOnce(undefined);
    const { queryClient } = renderSkillsTab({}, onlineRuntime);
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const bulk = await screen.findByRole("checkbox", {
      name: /Toggle all listed runtime skills/i,
    });

    await user.click(bulk);

    await waitFor(() => expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(mockToastError).toHaveBeenCalledWith(limitMessage));
    expect(invalidate).toHaveBeenCalledTimes(1);
    expect(bulk).toHaveAttribute("aria-invalid", "true");
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("falls back to the localized bulk failure message for non-Error failures", async () => {
    const user = userEvent.setup();
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [runtimeSkill("review", "Review")],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });
    mockSetAgentRuntimeSkillEnabled.mockRejectedValueOnce("request failed");
    renderSkillsTab({}, onlineRuntime);
    const bulk = await screen.findByRole("checkbox", {
      name: /Toggle all listed runtime skills/i,
    });

    await user.click(bulk);

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        "Failed to change inherited skill status",
      ),
    );
    expect(bulk).toHaveAttribute("aria-invalid", "true");
  });

  it("hides bulk runtime controls when editing or runtime capability is unavailable", async () => {
    const readOnly = renderSkillsTab({}, onlineRuntime, "user-1", false);
    await screen.findByText("Inherited from runtime");
    expect(screen.queryByRole("checkbox", { name: /Toggle all listed runtime skills/i })).not.toBeInTheDocument();
    readOnly.unmount();

    mockRuntimeCapabilities.mockResolvedValue({
      skills: [runtimeSkill("review", "Review")],
      supported: false,
      mcpServers: [],
      mcpSupported: true,
    });
    renderSkillsTab({}, onlineRuntime);
    expect(await screen.findByText("This runtime does not expose local skills.")).toBeInTheDocument();
    expect(screen.queryByRole("checkbox", { name: /Toggle all listed runtime skills/i })).not.toBeInTheDocument();
  });

  it("binds bulk changes to the currently rendered runtime after a runtime rebound", async () => {
    const user = userEvent.setup();
    mockRuntimeCapabilities.mockImplementation(async (runtimeId: string) => ({
      skills: [runtimeSkill(runtimeId === "runtime-1" ? "old" : "new", runtimeId === "runtime-1" ? "Old" : "New")],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    }));
    const rendered = renderSkillsTab({}, onlineRuntime);
    await screen.findByText("Old");
    rendered.rerenderSkillsTab({}, { ...onlineRuntime, id: "runtime-2" });
    await screen.findByText("New");

    await user.click(screen.getByRole("checkbox", {
      name: /Toggle all listed runtime skills/i,
    }));

    await waitFor(() => expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledTimes(1));
    expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledWith("agent-1", expect.objectContaining({
      runtime_id: "runtime-2",
      key: "new",
      enabled: false,
    }));
  });

  it("shows a permission notice when capability discovery is forbidden", async () => {
    mockRuntimeCapabilities.mockRejectedValue(
      new ApiError("insufficient permissions", 403, "Forbidden"),
    );

    renderSkillsTab({}, onlineRuntime);

    expect(
      await screen.findByText(
        "You don't have permission to view this runtime's skills.",
      ),
    ).toBeInTheDocument();
  });

  it("does not discover skills for another member's private runtime", async () => {
    renderSkillsTab({}, { ...onlineRuntime, owner_id: "user-2" }, "admin-1");

    expect(
      await screen.findByText(
        "You don't have permission to view this runtime's skills.",
      ),
    ).toBeInTheDocument();
    expect(mockRuntimeCapabilities).not.toHaveBeenCalled();
  });

  it("shows a retry notice when capability discovery fails", async () => {
    mockRuntimeCapabilities.mockRejectedValue(
      new Error("daemon did not respond within 3 minutes"),
    );

    renderSkillsTab({}, onlineRuntime);

    expect(
      await screen.findByText("Couldn't discover runtime skills. Try again."),
    ).toBeInTheDocument();
  });
});

function runtimeSkill(
  key: string,
  name: string,
  overrides: Record<string, unknown> = {},
) {
  return {
    key,
    name,
    source_path: `~/.codex/skills/${key}`,
    provider: "codex",
    root: "provider",
    can_disable: true,
    file_count: 1,
    ...overrides,
  };
}

function disabledRuntimeSkill(
  key: string,
  name: string,
  overrides: Record<string, unknown> = {},
) {
  return {
    runtime_id: "runtime-1",
    provider: "codex",
    root: "provider" as const,
    key,
    name,
    ...overrides,
  };
}
