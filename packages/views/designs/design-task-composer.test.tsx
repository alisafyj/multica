import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  createDesignDocument,
  listAgents,
  listIssues,
  listProjectResources,
  listProjects,
  toastError,
  toastSuccess,
} = vi.hoisted(() => ({
  createDesignDocument: vi.fn(),
  listAgents: vi.fn(),
  listIssues: vi.fn(),
  listProjectResources: vi.fn(),
  listProjects: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: { createDesignDocument, listAgents, listIssues, listProjectResources, listProjects },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("sonner", () => ({
  toast: { error: toastError, success: toastSuccess },
}));

// Avatars resolve names through workspace providers this composer test does
// not mount; the composer's own behaviour is what is under test.
vi.mock("../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
}));

import { I18nProvider } from "@multica/core/i18n/react";
import zhIssues from "../locales/zh-Hans/issues.json";
import zhProjects from "../locales/zh-Hans/projects.json";
import { DesignTaskComposer } from "./design-task-composer";

const AGENT = {
  id: "agent-1",
  workspace_id: "ws-1",
  name: "小设计",
  runtime_id: "runtime-1",
  runtime_bound: true,
  archived_at: null,
};

const REPOSITORY = {
  id: "resource-h5",
  project_id: "project-1",
  workspace_id: "ws-1",
  resource_type: "github_repo",
  resource_ref: { url: "https://github.com/acme/crm-h5" },
  label: null,
  position: 0,
  created_at: "2026-08-16T00:00:00Z",
  created_by: null,
};

function createdDocument(overrides: Record<string, unknown> = {}) {
  return {
    id: "document-1",
    workspace_id: "ws-1",
    project_id: "project-1",
    project_resource_id: "",
    issue_id: "",
    title: "CRM",
    platform: "web",
    recipe: "default",
    status: "running",
    draft_revision_id: "",
    saved_revision_id: "",
    active_task: null,
    input_snapshot: null,
    last_error: null,
    repository_grounded: false,
    created_at: "2026-08-16T00:00:00Z",
    updated_at: "2026-08-16T00:00:00Z",
    saved_at: "",
    ...overrides,
  };
}

const RECIPE = {
  slug: "crm-console",
  title: "CRM 控制台",
  summary: "带筛选与批量操作的客户列表",
  category: "业务系统",
  subcategory: "后台",
  mode: "prototype",
  platform: "web" as const,
  prompt: "做一个 CRM 客户列表页，支持筛选和批量操作。",
  preview_path: "",
  origin: "builtin",
  published_at: "2026-08-16T00:00:00Z",
};

function renderComposer(
  onCreated = vi.fn(),
  props: Partial<React.ComponentProps<typeof DesignTaskComposer>> = {},
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const ui: ReactNode = (
    <I18nProvider locale="zh-Hans" resources={{ "zh-Hans": { issues: zhIssues, projects: zhProjects } }}>
      <QueryClientProvider client={queryClient}>
        <DesignTaskComposer onCreated={onCreated} {...props} />
      </QueryClientProvider>
    </I18nProvider>
  );
  render(ui);
  return onCreated;
}

async function pickProject(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole("button", { name: "项目" }));
  await user.click(await screen.findByRole("button", { name: "CRM" }));
}

async function pickAgent(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "设计智能体" }));
  await user.click(await screen.findByRole("button", { name: AGENT.name }));
}

describe("DesignTaskComposer", () => {
  beforeEach(() => {
    createDesignDocument.mockReset();
    listAgents.mockReset();
    listIssues.mockReset();
    listProjectResources.mockReset();
    listProjects.mockReset();
    toastError.mockReset();
    toastSuccess.mockReset();
    listAgents.mockResolvedValue([AGENT]);
    listIssues.mockResolvedValue({ issues: [], total: 0 });
    listProjectResources.mockResolvedValue({ resources: [REPOSITORY], total: 1 });
    listProjects.mockResolvedValue({
      projects: [{ id: "project-1", title: "CRM", description: "CRM 项目" }],
      total: 1,
    });
    createDesignDocument.mockResolvedValue(createdDocument());
  });

  it("keeps submit closed until project, agent and brief are all present", async () => {
    const user = userEvent.setup();
    renderComposer();

    const submit = await screen.findByRole("button", { name: "生成页面设计" });
    expect(submit).toBeDisabled();
    expect(screen.getByText("请选择项目")).toBeInTheDocument();

    await pickProject(user);
    expect(await screen.findByText("请选择智能体")).toBeInTheDocument();

    await pickAgent(user);
    expect(await screen.findByText("请描述你想要的页面")).toBeInTheDocument();

    await user.type(screen.getByLabelText("页面需求描述"), "客户列表页");
    await waitFor(() => expect(screen.getByRole("button", { name: "生成页面设计" })).toBeEnabled());
    expect(createDesignDocument).not.toHaveBeenCalled();
  });

  it("sends the picked scenario recipe and omits the optional links it has no value for", async () => {
    const user = userEvent.setup();
    const onCreated = renderComposer();

    await pickProject(user);
    await pickAgent(user);
    await user.type(screen.getByLabelText("页面需求描述"), "  客户列表页  ");
    await user.click(screen.getByRole("button", { name: /线框图/ }));

    await user.click(screen.getByRole("button", { name: "生成页面设计" }));

    await waitFor(() => expect(createDesignDocument).toHaveBeenCalledTimes(1));
    expect(createDesignDocument).toHaveBeenCalledWith({
      project_id: "project-1",
      agent_id: "agent-1",
      platform: "web",
      recipe: "wireframe",
      brief: "客户列表页",
    });
    // Optional links stay absent rather than being sent as empty strings the
    // server would have to parse as UUIDs.
    const payload = createDesignDocument.mock.calls[0]?.[0] as Record<string, unknown>;
    expect(payload).not.toHaveProperty("project_resource_id");
    expect(payload).not.toHaveProperty("issue_id");
    expect(onCreated).toHaveBeenCalledWith(expect.objectContaining({ project_id: "project-1" }));
  });

  it("clears the picked scenario when its chip is clicked again", async () => {
    const user = userEvent.setup();
    renderComposer();

    const wireframe = await screen.findByRole("button", { name: /线框图/ });
    expect(wireframe).toHaveAttribute("aria-pressed", "false");

    await user.click(wireframe);
    expect(wireframe).toHaveAttribute("aria-pressed", "true");

    await user.click(wireframe);
    expect(wireframe).toHaveAttribute("aria-pressed", "false");
  });

  it("offers the not-yet-built scenario as an inert placeholder", async () => {
    const user = userEvent.setup();
    renderComposer();

    // The design system catalogue slice has not landed, so this chip keeps its
    // spot without pretending to do anything.
    const brandKit = await screen.findByRole("button", { name: /创建品牌套件/ });
    expect(brandKit).toBeDisabled();
    expect(brandKit).not.toHaveAttribute("aria-pressed");
    expect(brandKit).toHaveTextContent("即将支持");

    await user.click(brandKit);
    expect(screen.getByRole("button", { name: /UI Mockup/ })).toHaveAttribute("aria-pressed", "false");
  });

  it("sends the template chip to the community gallery instead of toggling a recipe", async () => {
    const user = userEvent.setup();
    const onBrowseRecipes = vi.fn();
    renderComposer(vi.fn(), { onBrowseRecipes });

    const template = await screen.findByRole("button", { name: /来自模板/ });
    expect(template).toBeEnabled();
    expect(template).not.toHaveTextContent("即将支持");
    // It navigates rather than arming a recipe, so it never claims a pressed
    // state it cannot enter.
    expect(template).not.toHaveAttribute("aria-pressed");

    await user.click(template);
    expect(onBrowseRecipes).toHaveBeenCalledTimes(1);
  });

  it("stays inert when the template chip has nowhere to go", async () => {
    renderComposer();

    const template = await screen.findByRole("button", { name: /来自模板/ });
    expect(template).toBeDisabled();
    expect(template).toHaveTextContent("即将支持");
  });

  it("keeps the gallery's recipe after the user rewrites the brief", async () => {
    const user = userEvent.setup();
    renderComposer(vi.fn(), { recipeSelection: { token: 1, recipe: RECIPE } });

    const brief = await screen.findByLabelText("页面需求描述");
    expect(brief).toHaveValue(RECIPE.prompt);
    // A gallery recipe is not one of the five chips, so the composer says which
    // one is armed instead of leaving the rail blank.
    expect(screen.getByText("CRM 控制台")).toBeInTheDocument();

    await user.clear(brief);
    await user.type(brief, "改成客户详情页");
    await pickProject(user);
    await pickAgent(user);
    await user.click(screen.getByRole("button", { name: "生成页面设计" }));

    await waitFor(() => expect(createDesignDocument).toHaveBeenCalledTimes(1));
    expect(createDesignDocument).toHaveBeenCalledWith(
      expect.objectContaining({ recipe: "crm-console", brief: "改成客户详情页" }),
    );
  });

  it("lets a scenario chip and the clear affordance both drop the gallery recipe", async () => {
    const user = userEvent.setup();
    renderComposer(vi.fn(), { recipeSelection: { token: 1, recipe: RECIPE } });

    await user.click(await screen.findByRole("button", { name: /线框图/ }));
    expect(screen.queryByText("CRM 控制台")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /线框图/ })).toHaveAttribute("aria-pressed", "true");
  });

  it("drops the gallery recipe when its clear affordance is used", async () => {
    const user = userEvent.setup();
    renderComposer(vi.fn(), { recipeSelection: { token: 1, recipe: RECIPE } });

    await user.click(await screen.findByRole("button", { name: "不使用该社区配方" }));
    expect(screen.queryByText("CRM 控制台")).not.toBeInTheDocument();

    await pickProject(user);
    await pickAgent(user);
    await user.type(screen.getByLabelText("页面需求描述"), "自由发挥");
    await user.click(screen.getByRole("button", { name: "生成页面设计" }));

    await waitFor(() => expect(createDesignDocument).toHaveBeenCalledTimes(1));
    expect(createDesignDocument).toHaveBeenCalledWith(
      expect.objectContaining({ recipe: "default" }),
    );
  });

  it("states plainly that no repository was read, without framing it as an error", async () => {
    const user = userEvent.setup();
    renderComposer();

    // DC-053: skipping the repository is a legitimate choice, so the copy is a
    // statement of what will happen — but it must never leave the user
    // believing the agent inspected code.
    expect(
      await screen.findByText(/未选择仓库：本次不读取任何代码仓库/),
    ).toBeInTheDocument();

    await pickProject(user);
    await user.click(await screen.findByRole("button", { name: "代码仓库" }));
    await user.click(await screen.findByRole("button", { name: "crm-h5" }));

    expect(await screen.findByText(/已选择仓库：智能体会在任务内对该仓库做一次有界只读取证/)).toBeInTheDocument();
  });

  it("reports grounding from the server's own flag, not from what was submitted", async () => {
    const user = userEvent.setup();
    // A repository was attached, yet the run produced no repository evidence.
    // The UI has to follow the server, or it would claim the agent read code.
    createDesignDocument.mockResolvedValue(
      createdDocument({ project_resource_id: "resource-h5", repository_grounded: false }),
    );
    renderComposer();

    await pickProject(user);
    await pickAgent(user);
    await user.click(await screen.findByRole("button", { name: "代码仓库" }));
    await user.click(await screen.findByRole("button", { name: "crm-h5" }));
    await user.type(screen.getByLabelText("页面需求描述"), "客户列表页");
    await user.click(screen.getByRole("button", { name: "生成页面设计" }));

    await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith("已创建页面设计任务，本次未做仓库取证"));
    expect(createDesignDocument).toHaveBeenCalledWith(
      expect.objectContaining({ project_resource_id: "resource-h5" }),
    );
  });

  it("keeps the brief and does not navigate when the server rejects the task", async () => {
    const user = userEvent.setup();
    const onCreated = renderComposer();
    createDesignDocument.mockRejectedValue(new Error("agent unavailable"));

    await pickProject(user);
    await pickAgent(user);
    await user.type(screen.getByLabelText("页面需求描述"), "客户列表页");
    await user.click(screen.getByRole("button", { name: "生成页面设计" }));

    await waitFor(() => expect(toastError).toHaveBeenCalledWith("agent unavailable"));
    expect(onCreated).not.toHaveBeenCalled();
    expect(screen.getByLabelText("页面需求描述")).toHaveValue("客户列表页");
  });
});
