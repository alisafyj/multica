import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const navigate = vi.hoisted(() => vi.fn());
const toastError = vi.hoisted(() => vi.fn());
const { createProjectDesignSystem, listBuiltinDesignSystems, listAgents } = vi.hoisted(() => ({
  createProjectDesignSystem: vi.fn(),
  listBuiltinDesignSystems: vi.fn(),
  listAgents: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: { createProjectDesignSystem, listBuiltinDesignSystems },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/designs/queries", () => ({
  builtinDesignSystemListOptions: () => ({ queryKey: ["builtin-design-systems"], queryFn: listBuiltinDesignSystems }),
}));

vi.mock("@multica/core/workspace/queries", () => ({
  agentListOptions: () => ({
    queryKey: ["agents"],
    queryFn: listAgents,
  }),
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    designs: () => "/acme/designs",
    projectDesignSystemDetail: (id: string) => `/acme/designs/systems/${id}`,
    projectDesignSystemNew: () => "/acme/designs/systems/new",
  }),
}));

vi.mock("../navigation", () => ({
  useNavigation: () => ({ push: navigate }),
}));

vi.mock("sonner", () => ({
  toast: { error: toastError },
}));

import { WorkspaceDesignSystemCreate } from "./workspace-design-system-create";

const AGENT = { id: "agent-1", workspace_id: "ws-1", name: "小设计", runtime_id: "runtime-1", runtime_bound: true, archived_at: null, status: "online" };

function renderCreate() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <WorkspaceDesignSystemCreate />
    </QueryClientProvider>,
  );
}

describe("WorkspaceDesignSystemCreate", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listAgents.mockResolvedValue([AGENT]);
    // The options object the module builds normally selects `.design_systems`
    // out of the API response; this mock replaces the whole options, so it
    // resolves the already-selected array.
    listBuiltinDesignSystems.mockResolvedValue([
      { slug: "apple", name: "Apple", category: "媒体与消费", description: "", swatches: ["#0071e3"] },
    ]);
  });

  it("sends a standalone create with no project and the picked inputs, then opens the system", async () => {
    const user = userEvent.setup();
    createProjectDesignSystem.mockResolvedValue({ id: "system-9", project_id: "", name: "品牌视觉基线" });
    renderCreate();

    // Nothing is filled yet: the footer says what is missing instead of a
    // dead button.
    expect(screen.getByRole("button", { name: /生成设计体系/ })).toBeDisabled();
    expect(screen.getByRole("status").textContent).toContain("名字");

    await user.type(screen.getByLabelText("设计体系名称"), "品牌视觉基线");
    await user.type(screen.getByLabelText("GitHub 或网站"), "https://acme.example");
    await user.click(screen.getByRole("button", { name: "添加" }));
    await user.type(screen.getByLabelText("品牌描述"), "克制的工具品牌，蓝灰基调。");
    await user.click(screen.getByRole("radio", { name: "移动端" }));

    // agents query resolves on the next tick; pick once options appear.
    await screen.findByRole("option", { name: /小设计/ });
    await user.selectOptions(screen.getByLabelText("智能体"), "agent-1");

    // A source link becomes a link reference; a picked official system a
    // builtin reference.
    await user.click(await screen.findByRole("checkbox", { name: "Apple" }));

    await user.click(screen.getByRole("button", { name: /生成设计体系/ }));

    await vi.waitFor(() =>
      expect(createProjectDesignSystem).toHaveBeenCalledWith(
        expect.objectContaining({
          project_id: "",
          name: "品牌视觉基线",
          brief: "克制的工具品牌，蓝灰基调。",
          platform: "mobile",
          references: [
            { kind: "link", value: "https://acme.example", label: "来源链接" },
            { kind: "builtin_design_system", value: "apple", label: "Apple" },
          ],
        }),
      ),
    );
    expect(navigate).toHaveBeenCalledWith("/acme/designs/systems/system-9");
  });

  it("rejects a source that is not an https link", async () => {
    const user = userEvent.setup();
    renderCreate();

    await user.type(screen.getByLabelText("GitHub 或网站"), "http://insecure.example");
    expect(screen.getByRole("button", { name: "添加" })).toBeDisabled();
    expect(screen.getByText("请输入 https:// 开头的完整链接。")).toBeInTheDocument();
  });
});
