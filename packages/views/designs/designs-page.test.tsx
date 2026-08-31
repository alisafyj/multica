import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrictMode, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  getProjectDesignSystem,
  getProjectDesignSystemForProject,
  listAgents,
  listDesignDocuments,
  listDesignDrafts,
  listDesignFiles,
  listDesignFolders,
  listDesignScenarioRecipes,
  listDesignSystemProfiles,
  listDesignRepositories,
  listDesignTemplates,
  listProjectDesignSystemCatalogue,
  listProjectResources,
  listProjects,
  navigate,
} = vi.hoisted(() => ({
  getProjectDesignSystem: vi.fn(),
  getProjectDesignSystemForProject: vi.fn(),
  listAgents: vi.fn(),
  listDesignDocuments: vi.fn(),
  listDesignDrafts: vi.fn(),
  listDesignFiles: vi.fn(),
  listDesignFolders: vi.fn(),
  listDesignScenarioRecipes: vi.fn(),
  listDesignSystemProfiles: vi.fn(),
  listDesignTemplates: vi.fn(),
  listDesignRepositories: vi.fn(),
  listProjectDesignSystemCatalogue: vi.fn(),
  listProjectResources: vi.fn(),
  listProjects: vi.fn(),
  navigate: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    analyzeProjectDesignSystemRepository: vi.fn(),
    createDesignDocument: vi.fn(),
    createFigmaImportConnection: vi.fn(),
    createProjectDesignSystem: vi.fn(),
    getProjectDesignSystem,
    getProjectDesignSystemForProject,
    listAgents,
    listDesignDocuments,
    listDesignDrafts,
    listDesignFiles,
    listDesignFolders,
    listDesignScenarioRecipes,
    listDesignSystemProfiles,
    listDesignTemplates,
    listProjectDesignSystemCatalogue,
    listProjectResources,
    listProjects,
    uploadFile: vi.fn(),
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    designDetail: (id: string) => `/acme/designs/${id}`,
    designDraftDetail: (id: string) => `/acme/designs/drafts/${id}`,
    projectDesignSystemDetail: (id: string) => `/acme/designs/systems/${id}`,
  }),
}));

vi.mock("../navigation", () => ({
  AppLink: ({ children, href }: { children: ReactNode; href: string }) => <a href={href}>{children}</a>,
  useNavigation: () => ({ push: navigate }),
}));

vi.mock("./project-design-system-canvas", () => ({
  ProjectDesignSystemCanvas: () => <h2>品牌原则</h2>,
}));

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock("@multica/ui/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: { children: ReactNode }) => <>{children}</>,
  DropdownMenuTrigger: ({ render }: { render: ReactNode }) => <>{render}</>,
  DropdownMenuContent: ({ children }: { children: ReactNode }) => <div role="menu">{children}</div>,
  DropdownMenuItem: ({ children, disabled, onClick }: { children: ReactNode; disabled?: boolean; onClick?: () => void }) => (
    <button type="button" role="menuitem" disabled={disabled} onClick={onClick}>{children}</button>
  ),
}));

import { I18nProvider } from "@multica/core/i18n/react";
import zhCommon from "../locales/zh-Hans/common.json";
import zhIssues from "../locales/zh-Hans/issues.json";
import zhProjects from "../locales/zh-Hans/projects.json";
import { DesignsPage } from "./designs-page";

const baseDraft = {
  id: "draft-1",
  workspace_id: "ws-1",
  template_id: null,
  catalog_template_id: null,
  template_revision_id: null,
  file_id: null,
  revision_id: null,
  generated_file_id: null,
  generated_revision_id: null,
  issue_id: "issue-1",
  title: "客户列表草稿",
  requirement_core: { title: "客户列表" },
  slot_values: {},
  patch: [],
  validation_errors: [],
  created_by: "user-1",
  created_at: "2026-07-23T00:00:00Z",
  updated_at: "2026-07-23T00:00:00Z",
  materialized_at: null,
};

function renderWithClient(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  // The home composer reuses the shared property pickers, which read their
  // filter placeholders from i18n — same provider the apps mount.
  return render(
    <I18nProvider
      locale="zh-Hans"
      resources={{ "zh-Hans": { common: zhCommon, issues: zhIssues, projects: zhProjects } }}
    >
      <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
    </I18nProvider>,
  );
}

describe("DesignsPage", () => {
  beforeEach(() => {
    getProjectDesignSystem.mockReset();
    getProjectDesignSystemForProject.mockReset();
    listAgents.mockReset();
    listDesignDocuments.mockReset();
    listDesignDrafts.mockReset();
    listDesignFiles.mockReset();
    listDesignFolders.mockReset();
    listDesignScenarioRecipes.mockReset();
    listDesignSystemProfiles.mockReset();
    listDesignTemplates.mockReset();
    listDesignRepositories.mockReset();
    listProjectDesignSystemCatalogue.mockReset();
    listProjectResources.mockReset();
    listProjects.mockReset();
    navigate.mockReset();
    listAgents.mockResolvedValue([]);
    listDesignDocuments.mockResolvedValue({ documents: [] });
    listDesignDrafts.mockResolvedValue({ drafts: [], total: 0 });
    listDesignFiles.mockResolvedValue({ design_files: [], total: 0 });
    listDesignFolders.mockResolvedValue({ folders: [], total: 0 });
    listDesignScenarioRecipes.mockResolvedValue({ recipes: [] });
    listDesignSystemProfiles.mockResolvedValue({ design_systems: [] });
    listDesignTemplates.mockResolvedValue({ templates: [], total: 0 });
    listDesignRepositories.mockResolvedValue({ repositories: [] });
    listProjectDesignSystemCatalogue.mockResolvedValue({ design_systems: [] });
    listProjectResources.mockResolvedValue({ resources: [], total: 0 });
    listProjects.mockResolvedValue({ projects: [{ id: "project-1", title: "CRM", description: "CRM 项目设计目标" }], total: 1 });
    getProjectDesignSystemForProject.mockResolvedValue({
      id: "",
      workspace_id: "ws-1",
      project_id: "project-1",
      project_resource_id: "",
      name: "",
      platform: "",
      current_agent_id: null,
      status: "unestablished",
      active_task: null,
      input_snapshot: {},
      content: { sections: [], token_groups: [], locators: [], preview_html: "", integrity_sha256: "" },
      has_unsaved_changes: false,
      last_error: null,
      activity: [],
      created_at: "",
      updated_at: "",
      saved_at: null,
    });
  });

  it("keeps home fixed while every project tab can be closed", async () => {
    const user = userEvent.setup();
    listProjects.mockResolvedValue({
      projects: [
        { id: "project-1", title: "CRM", description: "CRM 项目设计目标" },
        { id: "project-2", title: "staffrnapp", description: "移动端项目" },
      ],
      total: 2,
    });
    renderWithClient(<StrictMode><DesignsPage /></StrictMode>);

    const homeTab = await screen.findByRole("tab", { name: "首页" });
    expect(homeTab).toHaveAttribute("aria-selected", "true");
    // Home carries the cross-project design task composer, not the
    // project-scoped asset views.
    expect(within(screen.getByRole("tabpanel", { name: "首页" })).getByLabelText("页面需求描述")).toBeInTheDocument();
    expect(screen.queryByText("工作区设计资产")).not.toBeInTheDocument();
    expect(screen.queryByText("UI 规范")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /关闭.*首页/ })).not.toBeInTheDocument();

    await screen.findByRole("menuitem", { name: "staffrnapp" });
    expect(screen.queryByRole("tab", { name: "CRM" })).not.toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "staffrnapp" })).not.toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "选择项目" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "打开项目" }));
    await user.click(screen.getByRole("menuitem", { name: "CRM" }));

    const crmTab = screen.getByRole("tab", { name: "CRM" });
    expect(crmTab).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("button", { name: "关闭项目 CRM" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "关闭项目 CRM" }));
    expect(screen.queryByRole("tab", { name: "CRM" })).not.toBeInTheDocument();
    expect(homeTab).toHaveAttribute("aria-selected", "true");

    await user.click(screen.getByRole("button", { name: "打开项目" }));
    await user.click(screen.getByRole("menuitem", { name: "staffrnapp" }));

    const staffTab = screen.getByRole("tab", { name: "staffrnapp" });
    expect(staffTab).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("button", { name: "关闭项目 staffrnapp" })).toBeInTheDocument();
  });

  it("nests 创作 / 社区 / 设计体系 under the one fixed 首页 tab", async () => {
    const user = userEvent.setup();
    renderWithClient(<DesignsPage />);

    const homeTab = await screen.findByRole("tab", { name: "首页" });
    expect(homeTab).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByRole("button", { name: /关闭.*首页/ })).not.toBeInTheDocument();
    expect(
      within(screen.getByRole("tabpanel", { name: "首页" })).getByLabelText("页面需求描述"),
    ).toBeInTheDocument();

    // 社区 is a sub-tab of 首页 now rather than a second workspace tab, so it
    // never gets a close affordance and never leaves the home tab.
    const communityTab = screen.getByRole("tab", { name: /社区/ });
    expect(communityTab).toHaveAttribute("aria-selected", "false");
    expect(screen.queryByRole("button", { name: /关闭.*社区/ })).not.toBeInTheDocument();

    await user.click(communityTab);
    expect(communityTab).toHaveAttribute("aria-selected", "true");
    expect(homeTab).toHaveAttribute("aria-selected", "true");
    // An empty catalogue still says something rather than spinning forever.
    expect(await screen.findByText("社区还没有可用的配方")).toBeInTheDocument();
    // The home tab carries no project, so the project-scoped search never
    // appears there.
    expect(screen.queryByPlaceholderText("搜索设计稿…")).not.toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: /设计体系/ }));
    expect(await screen.findByRole("group", { name: "设计体系归属" })).toBeInTheDocument();
    // The library is repository-scoped: no workspace default is ever offered.
    expect(screen.queryByText("设为默认")).not.toBeInTheDocument();
    expect(screen.queryByText("默认")).not.toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: /创作/ }));
    expect(
      within(screen.getByRole("tabpanel", { name: "首页" })).getByLabelText("页面需求描述"),
    ).toBeInTheDocument();
  });

  it("carries a community recipe back to the home composer", async () => {
    const user = userEvent.setup();
    listDesignScenarioRecipes.mockResolvedValue({
      recipes: [{
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
      }],
    });
    renderWithClient(<DesignsPage />);

    // The create panel's community entry is the way in, not a dead placeholder.
    const entry = await screen.findByRole("button", { name: "从社区模板开始" });
    await user.click(entry);

    expect(screen.getByRole("tab", { name: /社区/ })).toHaveAttribute("aria-selected", "true");
    await user.click(await screen.findByRole("button", { name: "填入首页" }));

    expect(screen.getByRole("tab", { name: /创作/ })).toHaveAttribute("aria-selected", "true");
    const homePanel = screen.getByRole("tabpanel", { name: "首页" });
    expect(within(homePanel).getByLabelText("页面需求描述")).toHaveValue(
      "做一个 CRM 客户列表页，支持筛选和批量操作。",
    );
    expect(within(homePanel).getByRole("button", { name: "不使用该社区配方" })).toBeInTheDocument();
  });

  it("keeps project Designs content on the unchanged default query path", async () => {
    const user = userEvent.setup();
    listDesignFiles.mockResolvedValue({
      design_files: [{
        id: "file-1",
        workspace_id: "ws-1",
        project_id: "project-1",
        project_resource_id: null,
        title: "CRM 首页设计稿",
        description: null,
        source_type: "figma",
        source_ref: {},
        thumbnail_url: null,
        current_revision_id: null,
        created_by: null,
        created_at: "2026-08-27T00:00:00Z",
        updated_at: "2026-08-27T00:00:00Z",
      }],
      total: 1,
    });
    renderWithClient(<DesignsPage />);
    await user.click(await screen.findByRole("button", { name: "打开项目" }));
    await user.click(screen.getByRole("menuitem", { name: "CRM" }));

    expect(await screen.findByRole("tab", { name: /设计稿.*1/ })).toHaveAttribute("aria-selected", "true");
    expect(await screen.findByText("CRM 首页设计稿")).toBeInTheDocument();
    expect(listDesignFiles).toHaveBeenCalledWith(undefined);
    // Slice 2A is read-model only: the current project panel does not add a
    // repository Finder or repository scope controls.
    expect(screen.queryByRole("searchbox", { name: /仓库/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: /仓库/ })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "仓库视角" })).toBeInTheDocument();
  });

  it("opens the composer from a project's 新建设计稿 and filters its artifacts", async () => {
    const user = userEvent.setup();
    renderWithClient(<DesignsPage />);
    await user.click(await screen.findByRole("button", { name: "打开项目" }));
    await user.click(screen.getByRole("menuitem", { name: "CRM" }));

    // Nothing produces a deck yet, so the position is laid out but closed
    // rather than a filter that quietly matches everything.
    const slides = await screen.findByRole("button", { name: /幻灯片/ });
    expect(slides).toBeDisabled();
    expect(await screen.findByText(/还没有生成过页面设计/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "新建设计稿" }));
    expect(screen.getByRole("tab", { name: "首页" })).toHaveAttribute("aria-selected", "true");
    expect(
      within(screen.getByRole("tabpanel", { name: "首页" })).getByLabelText("页面需求描述"),
    ).toBeInTheDocument();
  });

  it("keeps active design drafts in their own tab without review wording", async () => {
    const user = userEvent.setup();
    listDesignDrafts.mockResolvedValue({
      drafts: [
        {
          ...baseDraft,
          id: "semantic-draft",
          title: "客户列表草稿",
          status: "generated_with_warnings",
          generation_mode: "semantic_pagespec",
          page_spec: { version: "1.0", page: { type: "list", title: "客户列表" } },
          quality_report: { diagnostics: [{ severity: "warning", code: "minor_spacing" }] },
        },
        {
          ...baseDraft,
          id: "failed-draft",
          title: "失败草稿",
          status: "compile_failed",
          generation_mode: "semantic_pagespec",
          quality_report: { diagnostics: [{ severity: "error", code: "missing_table" }] },
        },
      ],
      total: 2,
    });

    renderWithClient(<DesignsPage />);
    await user.click(await screen.findByRole("button", { name: "打开项目" }));
    await user.click(screen.getByRole("menuitem", { name: "CRM" }));

    expect(within(screen.getByRole("tabpanel")).queryByText("客户列表草稿")).not.toBeInTheDocument();

    const draftsEntry = screen.getByRole("tab", { name: /设计草稿.*1/ });
    await user.click(draftsEntry);

    expect(screen.getByPlaceholderText("搜索设计草稿…")).toBeInTheDocument();
    expect(await screen.findByText("客户列表草稿")).toBeInTheDocument();
    expect(screen.getByText("PageSpec 语义稿")).toBeInTheDocument();
    expect(screen.queryByText("失败草稿")).not.toBeInTheDocument();
    expect(screen.queryByText(/审核|批准|驳回/)).not.toBeInTheDocument();
  });

  it("uses four compact asset tabs without duplicate panel titles", async () => {
    const user = userEvent.setup();
    listDesignSystemProfiles.mockResolvedValue({
      design_systems: [{
        id: "profile-1",
        project_id: "project-1",
        source_file_id: "file-1",
        name: "旧 Figma UI 规范",
        status: "ready",
        is_default: true,
        updated_at: "2026-07-29T00:00:00Z",
      }],
    });
    renderWithClient(<DesignsPage />);
    await user.click(await screen.findByRole("button", { name: "打开项目" }));
    await user.click(screen.getByRole("menuitem", { name: "CRM" }));

    const designsEntry = await screen.findByRole("tab", { name: /设计稿.*0/ });
    expect(designsEntry).toHaveAttribute("aria-selected", "true");
    expect(designsEntry.querySelector("[data-slot='badge']")).toHaveTextContent("0");
    const draftsEntry = screen.getByRole("tab", { name: /设计草稿.*0/ });
    const templatesEntry = screen.getByRole("tab", { name: /模版.*0/ });
    const systemEntry = screen.getByRole("tab", { name: /设计体系.*0/ });
    expect([designsEntry, draftsEntry, templatesEntry, systemEntry].map((entry) => entry.textContent)).toEqual([
      "设计稿0",
      "设计草稿0",
      "模版0",
      "设计体系0",
    ]);
    expect(systemEntry).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: /UI 规范/ })).not.toBeInTheDocument();
    expect(screen.queryByText("CRM / 设计稿")).not.toBeInTheDocument();

    await user.click(templatesEntry);
    expect(screen.getAllByText("模版")).toHaveLength(1);

    await user.click(systemEntry);
    expect(screen.getAllByText("设计体系")).toHaveLength(1);
    expect(screen.queryByPlaceholderText("搜索设计体系…")).not.toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "生成设计体系" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "创建设计体系" })).not.toBeInTheDocument();
    expect(screen.queryByText("尚未建立设计体系")).not.toBeInTheDocument();
    expect(screen.getByLabelText("旧 Figma UI 规范")).toBeInTheDocument();
  });

  it("renders saved design-system content directly without a detail link", async () => {
    const user = userEvent.setup();
    getProjectDesignSystemForProject.mockResolvedValue({
      id: "system-1",
      workspace_id: "ws-1",
      project_id: "project-1",
      name: "CRM 设计体系",
      platform: "web",
      current_agent_id: "agent-1",
      status: "saved",
      active_task: null,
      input_snapshot: {},
      content: {
        sections: [{ id: "brand-principles", title: "品牌原则", markdown: "克制、清晰。" }],
        token_groups: [],
        locators: [],
        preview_html: "<main>CRM UI Kit</main>",
        integrity_sha256: "digest-1",
      },
      has_unsaved_changes: false,
      last_error: null,
      activity: [],
      created_at: "2026-07-29T00:00:00Z",
      updated_at: "2026-07-29T08:00:00Z",
      saved_at: "2026-07-29T08:00:00Z",
    });

    renderWithClient(<DesignsPage />);
    await user.click(await screen.findByRole("button", { name: "打开项目" }));
    await user.click(screen.getByRole("menuitem", { name: "CRM" }));
    await user.click(await screen.findByRole("tab", { name: /设计体系.*1/ }));

    expect(await screen.findByRole("heading", { name: "品牌原则" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "打开设计体系" })).not.toBeInTheDocument();
    expect(screen.queryByPlaceholderText("搜索设计体系…")).not.toBeInTheDocument();
  });

  it("asks the API for the picked repository's design system", async () => {
    const user = userEvent.setup();
    listProjectResources.mockResolvedValue({
      resources: [
        {
          id: "resource-h5",
          project_id: "project-1",
          workspace_id: "ws-1",
          resource_type: "github_repo",
          resource_ref: { url: "https://github.com/acme/crm-h5" },
          label: null,
          position: 0,
          created_at: "2026-08-16T00:00:00Z",
          created_by: null,
        },
        // Only repositories carry their own design system (DC-052).
        {
          id: "resource-doc",
          project_id: "project-1",
          workspace_id: "ws-1",
          resource_type: "document",
          resource_ref: { url: "https://example.test/spec", title: "业务规则" },
          label: "业务规则",
          position: 1,
          created_at: "2026-08-16T00:00:00Z",
          created_by: null,
        },
      ],
      total: 2,
    });

    renderWithClient(<DesignsPage />);
    await user.click(await screen.findByRole("button", { name: "打开项目" }));
    await user.click(screen.getByRole("menuitem", { name: "CRM" }));
    await user.click(await screen.findByRole("tab", { name: /设计体系.*0/ }));

    expect(await screen.findByRole("button", { name: "crm-h5" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "业务规则" })).not.toBeInTheDocument();
    expect(getProjectDesignSystemForProject).toHaveBeenLastCalledWith("project-1", {
      project_resource_id: "",
    });

    await user.click(screen.getByRole("button", { name: "crm-h5" }));

    await waitFor(() => expect(getProjectDesignSystemForProject).toHaveBeenLastCalledWith("project-1", {
      project_resource_id: "resource-h5",
    }));
    expect(screen.getByRole("button", { name: "crm-h5" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "项目通用" })).toHaveAttribute("aria-pressed", "false");
  });
});

describe("DesignsPage MVP object workspace", () => {
  it("keeps Home available and opens the bounded project/repository workspace", async () => {
    renderWithClient(<StrictMode><DesignsPage /></StrictMode>);
    const home = await screen.findByRole("tab", { name: "首页" });
    const workspace = await screen.findByRole("group", { name: "设计中心视角" });
    expect(home).toHaveAttribute("aria-selected", "true");
    expect(within(workspace).getByRole("button", { name: "项目视角" })).toBeInTheDocument();
    expect(within(workspace).getByRole("button", { name: "仓库视角" })).toBeInTheDocument();
  });
});
