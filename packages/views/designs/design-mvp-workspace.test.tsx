import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrictMode, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { listDesignFiles, listDesignDocuments, listDesignRepositories, listProjects, setDesignAssetRepositoryAssociation } = vi.hoisted(() => ({
  listDesignFiles: vi.fn(),
  listDesignDocuments: vi.fn(),
  listDesignRepositories: vi.fn(),
  listProjects: vi.fn(),
  setDesignAssetRepositoryAssociation: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: { listDesignFiles, listDesignDocuments, listDesignRepositories, listProjects, setDesignAssetRepositoryAssociation },
}));

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    designDetail: (id: string) => `/designs/${id}`,
    designDraftDetail: () => "/designs/drafts",
  }),
}));
vi.mock("./design-document-card", () => ({
  DesignDocumentCard: ({ document }: { document: { title: string } }) => <article>{document.title}</article>,
}));

import { DesignMvpWorkspace, type DesignMvpRepository } from "./design-mvp-workspace";

const repositories: DesignMvpRepository[] = [
  { id: "repo-1", projectId: "project-1", projectTitle: "CRM", label: "web", repositoryUrl: "https://github.com/example/web", defaultBranchHint: "main" },
  { id: "repo-2", projectId: "project-2", projectTitle: "App", label: "web", repositoryUrl: "https://github.com/example/web-app", defaultBranchHint: "develop" },
];

function renderWithClient(ui: ReactNode) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}

describe("DesignMvpWorkspace", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listProjects.mockResolvedValue({ projects: [{ id: "project-1", title: "CRM" }, { id: "project-2", title: "App" }], total: 2 });
    listDesignRepositories.mockResolvedValue({ repositories: repositories.map((repository) => ({
      id: repository.id,
      project_id: repository.projectId,
      project_title: repository.projectTitle,
      label: repository.label,
      repository_url: repository.repositoryUrl,
      default_branch_hint: repository.defaultBranchHint,
    })) });
    listDesignFiles.mockResolvedValue({ design_files: [], total: 0 });
    listDesignDocuments.mockResolvedValue({ documents: [] });
  });

  it("uses the exact project combined read and shows saved and draft panels", async () => {
    const user = userEvent.setup();
    listDesignFiles.mockResolvedValue({ design_files: [{ id: "file-1", workspace_id: "ws-1", title: "Figma saved", project_id: "project-1", project_resource_id: null, source_type: "upload", source_ref: {}, created_at: "", updated_at: "2026-08-20T00:00:00Z" }], total: 1 });
    listDesignDocuments.mockResolvedValue({ documents: [{ id: "doc-1", workspace_id: "ws-1", title: "Multica draft", project_id: "project-1", project_resource_id: null, status: "draft", saved_revision_id: "", draft_revision_id: "draft-1", repository_grounded: false, created_at: "", updated_at: "2026-08-21T00:00:00Z" }] });
    renderWithClient(<StrictMode><DesignMvpWorkspace /></StrictMode>);

    await user.click(await screen.findByRole("button", { name: "项目视角" }));
    await screen.findByLabelText("选择项目");
    await user.selectOptions(screen.getByLabelText("选择项目"), "project-1");

    await screen.findByText("Figma saved");
    expect(screen.getByText("Multica draft")).toBeInTheDocument();
    await waitFor(() => expect(listDesignFiles).toHaveBeenCalledWith({ projectId: "project-1" }));
    await waitFor(() => expect(listDesignDocuments).toHaveBeenCalledWith("project-1"));
    expect(screen.getByRole("heading", { name: "设计稿"})).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "设计草稿"})).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "设计体系"})).not.toBeInTheDocument();
  });

  it("uses one exact repository read and distinguishes duplicate labels", async () => {
    const user = userEvent.setup();
    listDesignFiles.mockResolvedValue({ design_files: [{ id: "file-repo", workspace_id: "ws-1", title: "Repository file", project_id: "project-1", project_resource_id: "repo-1", source_type: "upload", source_ref: {}, created_at: "", updated_at: "2026-08-20T00:00:00Z" }], total: 1 });
    listDesignDocuments.mockResolvedValue({ documents: [{ id: "doc-repo", workspace_id: "ws-1", title: "Repository draft", project_id: "project-1", project_resource_id: "repo-1", status: "draft", saved_revision_id: "", draft_revision_id: "draft-1", repository_grounded: true, created_at: "", updated_at: "2026-08-21T00:00:00Z" }] });
    renderWithClient(<StrictMode><DesignMvpWorkspace /></StrictMode>);

    await user.click(await screen.findByRole("button", { name: "仓库视角" }));
    await screen.findByLabelText("选择仓库");
    await user.selectOptions(screen.getByLabelText("选择仓库"), "repo-1");
    await screen.findByText("Repository file");
    expect(screen.getByText("Repository draft")).toBeInTheDocument();
    await waitFor(() => expect(listDesignFiles).toHaveBeenLastCalledWith({ projectId: "project-1", projectResourceId: "repo-1" }));
    await waitFor(() => expect(listDesignDocuments).toHaveBeenLastCalledWith("project-1", "repo-1"));
    expect(screen.getAllByText("CRM · web · https://github.com/example/web").length).toBeGreaterThan(0);
  });

  it("opens one-card association, confirms the typed mutation, and retains both choices on rejection", async () => {
    const user = userEvent.setup();
    listDesignFiles.mockResolvedValue({ design_files: [{ id: "file-1", workspace_id: "ws-1", title: "Associable file", project_id: "project-1", project_resource_id: null, source_type: "upload", source_ref: {}, created_at: "", updated_at: "2026-08-20T00:00:00Z" }], total: 1 });
    listDesignDocuments.mockResolvedValue({ documents: [] });
    setDesignAssetRepositoryAssociation.mockRejectedValueOnce(new Error("design_document_task_active"));
    renderWithClient(<StrictMode><DesignMvpWorkspace /></StrictMode>);
    await user.click(await screen.findByRole("button", { name: "项目视角" }));
    await screen.findByLabelText("选择项目");
    await user.selectOptions(screen.getByLabelText("选择项目"), "project-1");
    await screen.findByText("Associable file");
    await user.click(screen.getByRole("button", { name: "关联仓库：Associable file" }));
    const dialog = await screen.findByRole("dialog");
    await user.selectOptions(within(dialog).getByLabelText("选择目标仓库"), "repo-1");
    await user.click(within(dialog).getByRole("button", { name: "确认关联" }));
    expect(await within(dialog).findByText("当前设计文档任务运行中，请稍后重试。")).toBeInTheDocument();
    await waitFor(() => expect(setDesignAssetRepositoryAssociation).toHaveBeenCalledWith({
      project_id: "project-1", project_resource_id: "repo-1",
      items: [{ kind: "design_file", id: "file-1" }],
    }));
    expect(within(dialog).getByText("Figma · Associable file")).toBeInTheDocument();
    expect(within(dialog).getByLabelText("选择目标仓库")).toHaveValue("repo-1");
  });
});
