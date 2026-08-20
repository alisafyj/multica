import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const navigate = vi.hoisted(() => vi.fn());
const {
  createDesignDocument,
  listAgents,
  listDesignDocuments,
  listDesignScenarioRecipes,
  listIssues,
  listProjectResources,
  listProjects,
  listProjectDesignSystemCatalogue,
  listBuiltinDesignSystems,
  toastError,
  toastSuccess,
  uploadFile,
  createChatSession,
  sendChatMessage,
} = vi.hoisted(() => ({
  createDesignDocument: vi.fn(),
  listAgents: vi.fn(),
  listDesignDocuments: vi.fn(),
  listDesignScenarioRecipes: vi.fn(),
  listIssues: vi.fn(),
  listProjectResources: vi.fn(),
  listProjects: vi.fn(),
  listProjectDesignSystemCatalogue: vi.fn(),
  listBuiltinDesignSystems: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
  uploadFile: vi.fn(),
  createChatSession: vi.fn(),
  sendChatMessage: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    createDesignDocument,
    createChatSession,
    sendChatMessage,
    listAgents,
    listDesignDocuments,
    listDesignScenarioRecipes,
    listIssues,
    listProjectResources,
    listProjects,
    listProjectDesignSystemCatalogue,
    listBuiltinDesignSystems,
    uploadFile,
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("sonner", () => ({
  toast: { error: toastError, success: toastSuccess },
}));

vi.mock("../navigation", () => ({
  useNavigation: () => ({ push: navigate }),
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    designs: () => "/acme/designs",
    chatSession: (id: string) => `/acme/chat/${id}`,
  }),
}));

// Avatars resolve names through workspace providers this composer test does
// not mount; the composer's own behaviour is what is under test.
vi.mock("../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
}));

import { I18nProvider } from "@multica/core/i18n/react";
import zhCommon from "../locales/zh-Hans/common.json";
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
  preview_kind: "",
  preview_url: "",
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
    <I18nProvider
      locale="zh-Hans"
      resources={{ "zh-Hans": { common: zhCommon, issues: zhIssues, projects: zhProjects } }}
    >
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
    listDesignDocuments.mockReset();
    listDesignScenarioRecipes.mockReset();
    listIssues.mockReset();
    listProjectResources.mockReset();
    listProjects.mockReset();
    toastError.mockReset();
    toastSuccess.mockReset();
    createChatSession.mockReset();
    sendChatMessage.mockReset();
    navigate.mockReset();
    listProjectDesignSystemCatalogue.mockReset();
    listBuiltinDesignSystems.mockReset();
    listProjectDesignSystemCatalogue.mockResolvedValue({
      design_systems: [{
        id: "system-1",
        project_id: "",
        project_title: "",
        project_resource_id: "",
        name: "品牌视觉基线",
        platform: "web",
        summary: "克制的工具品牌",
        has_draft_package: false,
        saved_at: "2026-08-20T00:00:00Z",
      }],
    });
    listBuiltinDesignSystems.mockResolvedValue({
      design_systems: [{ slug: "agentic", name: "Agentic", category: "工具", description: "", swatches: ["#ffffff", "#f6f6f1", "#111827", "#ff5701"] }],
    });
    listAgents.mockResolvedValue([AGENT]);
    listDesignDocuments.mockResolvedValue({ documents: [] });
    listDesignScenarioRecipes.mockResolvedValue({ recipes: [] });
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

  it("stages reference files by attachment id and sends them with the request", async () => {
    const user = userEvent.setup();
    uploadFile.mockResolvedValue({ id: "attachment-1", filename: "home.png", url: "https://cdn.test/home.png", content_type: "image/png", size_bytes: 3 });
    renderComposer();

    await pickProject(user);
    await pickAgent(user);
    await user.type(screen.getByLabelText("页面需求描述"), "客户列表页");
    await user.upload(screen.getByLabelText("上传参考文件"), new File(["png"], "home.png", { type: "image/png" }));
    expect(await screen.findByText("home.png")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "生成页面设计" }));
    await waitFor(() => expect(createDesignDocument).toHaveBeenCalledTimes(1));
    expect(createDesignDocument.mock.calls[0]?.[0]).toMatchObject({
      attachments: [{ attachment_id: "attachment-1" }],
    });

    // Removing a chip drops it from the next request.
    uploadFile.mockResolvedValue({ id: "attachment-2", filename: "flow.pdf", url: "https://cdn.test/flow.pdf", content_type: "application/pdf", size_bytes: 3 });
    await user.upload(screen.getByLabelText("上传参考文件"), new File(["pdf"], "flow.pdf", { type: "application/pdf" }));
    expect(await screen.findByText("flow.pdf")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "移除 flow.pdf" }));
    expect(screen.queryByText("flow.pdf")).not.toBeInTheDocument();
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

  it("keeps every unbuilt creation scenario in the rail without letting it run", async () => {
    const user = userEvent.setup();
    renderComposer();

    // The rail lays out the whole surface, so a position with no producer
    // still holds its place — while saying in its name that it cannot run.
    const slides = await screen.findByRole("button", { name: "幻灯片（即将支持）" });
    expect(slides).toBeDisabled();
    expect(slides).not.toHaveAttribute("aria-pressed");
    // A design system belongs to a project's own scope (DC-052), so this
    // position is never live from the composer.
    expect(screen.getByRole("button", { name: "创建设计体系（即将支持）" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "实时看板（即将支持）" })).toBeDisabled();

    await user.click(slides);
    expect(screen.getByRole("button", { name: "原型" })).toHaveAttribute("aria-pressed", "false");
  });

  it("sends the community entry to the gallery instead of arming a recipe", async () => {
    const user = userEvent.setup();
    const onBrowseRecipes = vi.fn();
    listDesignScenarioRecipes.mockResolvedValue({ recipes: [RECIPE] });
    renderComposer(vi.fn(), { onBrowseRecipes });

    const entry = await screen.findByRole("button", { name: "从社区模板开始" });
    // It navigates rather than arming a recipe, so it never claims a pressed
    // state it cannot enter.
    expect(entry).not.toHaveAttribute("aria-pressed");

    await user.click(entry);
    expect(onBrowseRecipes).toHaveBeenCalledTimes(1);
  });

  it("hides the community entry when it has nowhere to go", async () => {
    listDesignScenarioRecipes.mockResolvedValue({ recipes: [RECIPE] });
    renderComposer();

    expect(await screen.findByText("示例提示词")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "从社区模板开始" })).not.toBeInTheDocument();
  });

  // DC-060: design systems are workspace platform material, so the home
  // composer can pin any saved system — or a bundled catalogue preset — to a
  // page design, independent of which project the document belongs to.
  it("pins a chosen workspace design system to the run", async () => {
    const user = userEvent.setup();
    renderComposer();

    await pickProject(user);
    await pickAgent(user);
    await user.type(screen.getByLabelText("页面需求描述"), "客户列表页");

    await user.click(screen.getByRole("button", { name: "设计体系" }));
    await user.click(await screen.findByRole("button", { name: "品牌视觉基线" }));
    expect(screen.getByText(/设计体系已指定/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "生成页面设计" }));
    await waitFor(() => expect(createDesignDocument).toHaveBeenCalledWith(
      expect.objectContaining({ design_system_id: "system-1" }),
    ));
    expect(createDesignDocument.mock.calls[0]?.[0]).not.toHaveProperty("builtin_design_system");
  });

  it("pins a bundled catalogue preset instead, never both", async () => {
    const user = userEvent.setup();
    renderComposer();

    await pickProject(user);
    await pickAgent(user);
    await user.type(screen.getByLabelText("页面需求描述"), "客户列表页");

    // Picking the preset after a workspace system replaces it: the server
    // refuses a request carrying both.
    await user.click(screen.getByRole("button", { name: "设计体系" }));
    await user.click(await screen.findByRole("button", { name: "品牌视觉基线" }));
    await user.click(screen.getByRole("button", { name: "设计体系" }));
    await user.click(await screen.findByRole("button", { name: "Agentic" }));

    await user.click(screen.getByRole("button", { name: "生成页面设计" }));
    await waitFor(() => expect(createDesignDocument).toHaveBeenCalledWith(
      expect.objectContaining({ builtin_design_system: "agentic" }),
    ));
    expect(createDesignDocument.mock.calls[0]?.[0]).not.toHaveProperty("design_system_id");
  });

  // Open Design's composer modes: 提问 and 规划 hand the prompt to an agent
  // chat instead of creating a design document.
  it("提问 mode starts an agent chat with the prompt and never creates a document", async () => {
    const user = userEvent.setup();
    createChatSession.mockResolvedValue({ id: "chat-1" });
    sendChatMessage.mockResolvedValue({});
    renderComposer();

    await pickAgent(user);
    await user.type(screen.getByLabelText("页面需求描述"), "这个布局有什么可改进的？");

    await user.click(screen.getByRole("button", { name: "创作模式" }));
    await user.click(await screen.findByRole("menuitem", { name: /提问/ }));
    // Design-only settings leave the row; no project is required.
    expect(screen.queryByRole("button", { name: "项目" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "发送提问" }));
    await waitFor(() => expect(createChatSession).toHaveBeenCalledWith(expect.objectContaining({ agent_id: "agent-1" })));
    expect(sendChatMessage).toHaveBeenCalledWith("chat-1", "这个布局有什么可改进的？", []);
    expect(createDesignDocument).not.toHaveBeenCalled();
    expect(navigate).toHaveBeenCalledWith("/acme/chat/chat-1");
  });

  it("规划 mode wraps the prompt in the planning instruction", async () => {
    const user = userEvent.setup();
    createChatSession.mockResolvedValue({ id: "chat-2" });
    sendChatMessage.mockResolvedValue({});
    renderComposer();

    await pickAgent(user);
    await user.type(screen.getByLabelText("页面需求描述"), "会员中心改版");
    await user.click(screen.getByRole("button", { name: "创作模式" }));
    await user.click(await screen.findByRole("menuitem", { name: /规划/ }));
    await user.click(screen.getByRole("button", { name: "生成规划" }));

    await waitFor(() => expect(sendChatMessage).toHaveBeenCalledTimes(1));
    const content = sendChatMessage.mock.calls[0]?.[1] as string;
    expect(content).toContain("设计规划");
    expect(content).toContain("会员中心改版");
    expect(navigate).toHaveBeenCalledWith("/acme/chat/chat-2");
  });

  it("seeds the brief from an example prompt and sends that recipe", async () => {
    const user = userEvent.setup();
    listDesignScenarioRecipes.mockResolvedValue({ recipes: [RECIPE] });
    renderComposer();

    await user.click(await screen.findByRole("button", { name: /CRM 控制台/ }));
    expect(screen.getByLabelText("页面需求描述")).toHaveValue(RECIPE.prompt);

    await pickProject(user);
    await pickAgent(user);
    await user.click(screen.getByRole("button", { name: "生成页面设计" }));

    await waitFor(() => expect(createDesignDocument).toHaveBeenCalledTimes(1));
    expect(createDesignDocument).toHaveBeenCalledWith(
      expect.objectContaining({ recipe: "crm-console", brief: RECIPE.prompt }),
    );
  });

  it("says the recent list is empty instead of inventing a run", async () => {
    renderComposer();

    expect(await screen.findByText("最近生成")).toBeInTheDocument();
    expect(screen.getByText(/还没有生成过页面设计/)).toBeInTheDocument();
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
