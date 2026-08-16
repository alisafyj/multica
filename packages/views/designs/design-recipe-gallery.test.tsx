import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  createDesignDocument,
  listAgents,
  listDesignScenarioRecipes,
  listProjects,
  toastError,
  toastSuccess,
} = vi.hoisted(() => ({
  createDesignDocument: vi.fn(),
  listAgents: vi.fn(),
  listDesignScenarioRecipes: vi.fn(),
  listProjects: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: { createDesignDocument, listAgents, listDesignScenarioRecipes, listProjects },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("sonner", () => ({
  toast: { error: toastError, success: toastSuccess },
}));

// Avatars resolve names through workspace providers this test does not mount.
vi.mock("../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
}));

import { I18nProvider } from "@multica/core/i18n/react";
import type { DesignDocument, DesignScenarioRecipe } from "@multica/core/types";
import zhIssues from "../locales/zh-Hans/issues.json";
import zhProjects from "../locales/zh-Hans/projects.json";
import { DesignRecipeGallery } from "./design-recipe-gallery";

const AGENT = {
  id: "agent-1",
  workspace_id: "ws-1",
  name: "小设计",
  runtime_id: "runtime-1",
  runtime_bound: true,
  archived_at: null,
};

function recipe(overrides: Record<string, unknown> = {}) {
  return {
    slug: "crm-console",
    title: "CRM 控制台",
    summary: "带筛选与批量操作的客户列表",
    category: "业务系统",
    subcategory: "后台",
    mode: "prototype",
    platform: "web",
    prompt: "做一个 CRM 客户列表页，支持筛选和批量操作。",
    preview_path: "",
    origin: "builtin",
    published_at: "2026-08-16T00:00:00Z",
    ...overrides,
  };
}

function renderGallery() {
  const onUseInComposer = vi.fn<(recipe: DesignScenarioRecipe) => void>();
  const onStarted = vi.fn<(document: DesignDocument) => void>();
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const ui: ReactNode = (
    <I18nProvider locale="zh-Hans" resources={{ "zh-Hans": { issues: zhIssues, projects: zhProjects } }}>
      <QueryClientProvider client={queryClient}>
        <DesignRecipeGallery onUseInComposer={onUseInComposer} onStarted={onStarted} />
      </QueryClientProvider>
    </I18nProvider>
  );
  render(ui);
  return { onUseInComposer, onStarted };
}

describe("DesignRecipeGallery", () => {
  beforeEach(() => {
    createDesignDocument.mockReset();
    listAgents.mockReset();
    listDesignScenarioRecipes.mockReset();
    listProjects.mockReset();
    toastError.mockReset();
    toastSuccess.mockReset();
    listAgents.mockResolvedValue([AGENT]);
    listProjects.mockResolvedValue({
      projects: [{ id: "project-1", title: "CRM", description: "CRM 项目" }],
      total: 1,
    });
    listDesignScenarioRecipes.mockResolvedValue({ recipes: [recipe()] });
    createDesignDocument.mockResolvedValue({
      id: "document-1",
      workspace_id: "ws-1",
      project_id: "project-1",
      project_resource_id: "",
      issue_id: "",
      title: "CRM",
      platform: "web",
      recipe: "crm-console",
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
    });
  });

  it("says the catalogue is empty instead of leaving the panel blank", async () => {
    listDesignScenarioRecipes.mockResolvedValue({ recipes: [] });
    renderGallery();

    expect(await screen.findByText("社区还没有可用的配方")).toBeInTheDocument();
    expect(screen.getByText(/官方配方上线/)).toBeInTheDocument();
    // No filters to offer and nothing to search through.
    expect(screen.queryByLabelText("搜索社区配方")).not.toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "配方分类" })).not.toBeInTheDocument();
  });

  it("narrows by category and then by subcategory inside it", async () => {
    const user = userEvent.setup();
    listDesignScenarioRecipes.mockResolvedValue({
      recipes: [
        recipe({ slug: "crm-console", title: "CRM 控制台", category: "业务系统", subcategory: "后台" }),
        recipe({ slug: "crm-portal", title: "客户门户", category: "业务系统", subcategory: "门户" }),
        recipe({ slug: "landing", title: "产品官网", category: "营销", subcategory: "" }),
      ],
    });
    renderGallery();

    expect(await screen.findByText("CRM 控制台")).toBeInTheDocument();
    expect(screen.getByText("产品官网")).toBeInTheDocument();
    // The second level belongs to a chosen category, so it is absent up front.
    expect(screen.queryByRole("group", { name: /子分类/ })).not.toBeInTheDocument();

    const categories = screen.getByRole("group", { name: "配方分类" });
    await user.click(within(categories).getByRole("button", { name: /业务系统/ }));

    expect(screen.queryByText("产品官网")).not.toBeInTheDocument();
    expect(screen.getByText("客户门户")).toBeInTheDocument();

    const subcategories = screen.getByRole("group", { name: "业务系统 子分类" });
    await user.click(within(subcategories).getByRole("button", { name: /门户/ }));

    expect(screen.getByText("客户门户")).toBeInTheDocument();
    expect(screen.queryByText("CRM 控制台")).not.toBeInTheDocument();
    // The active facet keeps a state hover cannot take away.
    expect(within(subcategories).getByRole("button", { name: /门户/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("offers a way back when a search matches nothing", async () => {
    const user = userEvent.setup();
    renderGallery();

    await user.type(await screen.findByLabelText("搜索社区配方"), "不存在的东西");

    expect(await screen.findByText("没有匹配的配方")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "清除筛选" }));
    expect(await screen.findByText("CRM 控制台")).toBeInTheDocument();
  });

  it("hands the whole recipe to the composer rather than starting anything", async () => {
    const user = userEvent.setup();
    const { onUseInComposer } = renderGallery();

    await user.click(await screen.findByRole("button", { name: "填入首页" }));

    expect(onUseInComposer).toHaveBeenCalledWith(
      expect.objectContaining({ slug: "crm-console", prompt: recipe().prompt }),
    );
    expect(createDesignDocument).not.toHaveBeenCalled();
  });

  it("starts a task from the card with the recipe's slug and seeded brief", async () => {
    const user = userEvent.setup();
    const { onStarted } = renderGallery();

    await user.click(await screen.findByRole("button", { name: "直接创建" }));

    const brief = await screen.findByLabelText("页面需求描述");
    expect(brief).toHaveValue(recipe().prompt);
    expect(screen.getByText("请选择项目")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "项目" }));
    await user.click(await screen.findByRole("button", { name: "CRM" }));
    await user.click(screen.getByRole("button", { name: "设计智能体" }));
    await user.click(await screen.findByRole("button", { name: AGENT.name }));

    await user.click(screen.getByRole("button", { name: "生成页面设计" }));

    await waitFor(() => expect(createDesignDocument).toHaveBeenCalledTimes(1));
    expect(createDesignDocument).toHaveBeenCalledWith({
      project_id: "project-1",
      agent_id: "agent-1",
      platform: "web",
      recipe: "crm-console",
      brief: recipe().prompt,
    });
    // DC-053: the gallery attaches no repository, so the copy must not imply
    // the agent read any code.
    await waitFor(() =>
      expect(toastSuccess).toHaveBeenCalledWith("已创建页面设计任务，本次未做仓库取证"),
    );
    expect(onStarted).toHaveBeenCalledWith(expect.objectContaining({ project_id: "project-1" }));
  });

  it("closes both actions for an artifact kind that has no producer yet", async () => {
    listDesignScenarioRecipes.mockResolvedValue({
      recipes: [recipe({ slug: "pitch", title: "路演材料", mode: "deck" })],
    });
    renderGallery();

    expect(await screen.findByText("路演材料")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "填入首页" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "直接创建" })).toBeDisabled();
    expect(screen.getByText(/暂时还不能创建/)).toBeInTheDocument();
  });

  it("shows a composed tile instead of an image frame when a recipe has no preview", async () => {
    listDesignScenarioRecipes.mockResolvedValue({
      recipes: [
        recipe({ slug: "no-preview", title: "无预览", preview_path: "" }),
        recipe({ slug: "with-preview", title: "有预览", preview_path: "/media/recipes/a.png" }),
      ],
    });
    renderGallery();

    expect(await screen.findByText("无预览")).toBeInTheDocument();
    const images = document.body.querySelectorAll("img");
    expect(images).toHaveLength(1);
    expect(images[0]).toHaveAttribute("src", "/media/recipes/a.png");
    // The card without a preview still states what it is.
    expect(screen.getAllByText("业务系统 · 后台").length).toBeGreaterThan(0);
  });

  it("keeps the brief in the dialog when the server rejects the task", async () => {
    const user = userEvent.setup();
    createDesignDocument.mockRejectedValue(new Error("agent unavailable"));
    const { onStarted } = renderGallery();

    await user.click(await screen.findByRole("button", { name: "直接创建" }));
    await user.click(screen.getByRole("button", { name: "项目" }));
    await user.click(await screen.findByRole("button", { name: "CRM" }));
    await user.click(screen.getByRole("button", { name: "设计智能体" }));
    await user.click(await screen.findByRole("button", { name: AGENT.name }));
    await user.click(screen.getByRole("button", { name: "生成页面设计" }));

    await waitFor(() => expect(toastError).toHaveBeenCalledWith("agent unavailable"));
    expect(onStarted).not.toHaveBeenCalled();
    expect(screen.getByLabelText("页面需求描述")).toHaveValue(recipe().prompt);
  });
});
