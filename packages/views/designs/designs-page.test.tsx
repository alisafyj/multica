import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  getProjectDesignSystemForProject,
  listAgents,
  listDesignDrafts,
  listDesignFiles,
  listDesignFolders,
  listDesignSystemProfiles,
  listDesignTemplates,
  listProjects,
} = vi.hoisted(() => ({
  getProjectDesignSystemForProject: vi.fn(),
  listAgents: vi.fn(),
  listDesignDrafts: vi.fn(),
  listDesignFiles: vi.fn(),
  listDesignFolders: vi.fn(),
  listDesignSystemProfiles: vi.fn(),
  listDesignTemplates: vi.fn(),
  listProjects: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    createFigmaImportConnection: vi.fn(),
    createProjectDesignSystem: vi.fn(),
    getProjectDesignSystemForProject,
    listAgents,
    listDesignDrafts,
    listDesignFiles,
    listDesignFolders,
    listDesignSystemProfiles,
    listDesignTemplates,
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
  useNavigation: () => ({ push: vi.fn() }),
}));

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

import { DesignsPage } from "./designs-page";

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <DesignsPage />
    </QueryClientProvider>,
  );
}

describe("DesignsPage project design system entry", () => {
  beforeEach(() => {
    getProjectDesignSystemForProject.mockReset();
    listAgents.mockReset();
    listDesignDrafts.mockReset();
    listDesignFiles.mockReset();
    listDesignFolders.mockReset();
    listDesignSystemProfiles.mockReset();
    listDesignTemplates.mockReset();
    listProjects.mockReset();

    listAgents.mockResolvedValue([]);
    listDesignDrafts.mockResolvedValue({ drafts: [], total: 0 });
    listDesignFiles.mockResolvedValue({ design_files: [], total: 0 });
    listDesignFolders.mockResolvedValue({ folders: [], total: 0 });
    listDesignTemplates.mockResolvedValue({ templates: [], total: 0 });
    listProjects.mockResolvedValue({
      projects: [{ id: "project-1", title: "CRM", description: "CRM 项目设计目标" }],
      total: 1,
    });
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
    getProjectDesignSystemForProject.mockResolvedValue({
      id: "",
      workspace_id: "ws-1",
      project_id: "project-1",
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

  it("uses 设计稿, 模版, and 设计体系 as the exact top-level entries", async () => {
    const user = userEvent.setup();
    renderPage();

    expect(await screen.findByRole("button", { name: /设计稿/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /模版/ })).toBeInTheDocument();
    const systemEntry = screen.getByRole("button", { name: /设计体系/ });
    expect(systemEntry).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /UI 规范/ })).not.toBeInTheDocument();

    await user.click(systemEntry);
    expect(screen.getByPlaceholderText("搜索设计体系…")).toBeInTheDocument();
    expect(screen.getByText("尚未建立设计体系")).toBeInTheDocument();
    expect(screen.queryByText("旧 Figma UI 规范")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "创建设计体系" }));
    expect(screen.getByLabelText("旧 Figma UI 规范")).toBeInTheDocument();
  });
});
