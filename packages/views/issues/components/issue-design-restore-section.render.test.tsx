// @vitest-environment jsdom
import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { api } from "@multica/core/api";
import { useCommentDraftStore } from "@multica/core/issues/stores";
import type { Agent, AgentTask, DesignDocument, DesignDraft, DesignFile, DesignRestoreTask, Issue, TimelineEntry } from "@multica/core/types";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { IssueDesignRestoreSection } from "./issue-design-restore-section";

const mockDesignQueries = vi.hoisted(() => ({
  restoreTasks: [] as DesignRestoreTask[],
  designDrafts: [] as DesignDraft[],
  designFiles: [{ id: "file-1", title: "服务记录设计稿", project_id: "project-1" }] as Array<Partial<DesignFile> & Pick<DesignFile, "id" | "title">>,
  designDocuments: [] as DesignDocument[],
  implementationFrames: [] as Array<{ frame_ref: string; selection_key: string; title: string }>,
  projectResources: [] as Array<{ id: string; resource_type: string; label: string; resource_ref: Record<string, unknown> }>,
  agentTasks: [] as AgentTask[],
}));

afterEach(cleanup);
vi.mock("@multica/core/api", () => ({
  api: {
    buildDesignImplementationPrompt: vi.fn(),
    createDesignDraftAgentTask: vi.fn(),
    listTasksByIssue: vi.fn(() => Promise.resolve(mockDesignQueries.agentTasks)),
  },
}));


vi.mock("@multica/core/projects", () => ({
  projectResourcesOptions: () => ({
    queryKey: ["project-resources"],
    queryFn: () => Promise.resolve(mockDesignQueries.projectResources),
  }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    designDetail: (id: string) => `/designs/${id}`,
    designDraftDetail: (id: string) => `/design-drafts/${id}`,
    designFrameDetail: (id: string, frameId: string) => `/designs/${id}/frames/${frameId}`,
    designRestoreTaskDetail: (id: string) => `/design-restore-tasks/${id}`,
  }),
}));

vi.mock("../../navigation", () => ({
  useNavigation: () => ({
    push: vi.fn(),
  }),
}));

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    info: vi.fn(),
    success: vi.fn(),
  },
}));


vi.mock("@multica/core/designs/queries", () => ({
  designDeliveriesByIssueOptions: () => ({
    queryKey: ["design-deliveries"],
    queryFn: () => Promise.resolve([]),
  }),
  designAssetFramesOptions: (_wsId: string, designRef: string) => ({
    queryKey: ["design-asset-frames", designRef],
    queryFn: () => Promise.resolve({
      design_ref: designRef,
      revision_id: "revision-1",
      content_digest: "sha256:digest-1",
      frames: mockDesignQueries.implementationFrames,
    }),
  }),
  designDocumentListOptions: () => ({
    queryKey: ["design-documents"],
    queryFn: () => Promise.resolve(mockDesignQueries.designDocuments),
  }),
  designDraftListOptions: () => ({
    queryKey: ["design-drafts"],
    queryFn: () => Promise.resolve(mockDesignQueries.designDrafts),
  }),
  designFileDetailOptions: () => ({
    queryKey: ["design-file-detail"],
    queryFn: () => Promise.resolve({
      current_revision: {
        id: "revision-1",
        native_json: {
          frames: [{ id: "frame-1", name: "服务记录" }],
        },
      },
    }),
  }),
  designFileListOptions: () => ({
    queryKey: ["design-files"],
    queryFn: () => Promise.resolve(mockDesignQueries.designFiles),
  }),
  designRestoreMappingsOptions: () => ({
    queryKey: ["design-restore-mappings"],
    queryFn: () => Promise.resolve([]),
  }),
  designRestorePlanOptions: () => ({
    queryKey: ["design-restore-plan"],
    queryFn: () => Promise.resolve(null),
  }),
  designRestoreTaskDetailOptions: () => ({
    queryKey: ["design-restore-task-detail"],
    queryFn: () => Promise.resolve(null),
  }),
  designRestoreTaskListOptions: () => ({
    queryKey: ["design-restore-tasks"],
    queryFn: () => Promise.resolve(mockDesignQueries.restoreTasks),
  }),
}));

vi.mock("@multica/core/issues/queries", () => ({
  childIssuesOptions: () => ({
    queryKey: ["child-issues"],
    queryFn: () => Promise.resolve([]),
  }),
  issueKeys: {
    children: (wsId: string, id: string) => ["issues", wsId, "children", id],
    detail: (wsId: string, id: string) => ["issues", wsId, "detail", id],
    list: (wsId: string) => ["issues", wsId, "list"],
    myAll: (wsId: string) => ["issues", wsId, "my-all"],
    tasks: (id: string) => ["issues", "tasks", id],
  },
}));

vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({
    queryKey: ["members"],
    queryFn: () => Promise.resolve([]),
  }),
}));

function renderSection(issue: Issue, timeline: TimelineEntry[] = []) {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  const agents: Agent[] = [{
    id: "agent-1",
    workspace_id: "ws-1",
    name: "UI Agent",
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
    archived_at: null,
    archived_by: null,
    owner_id: null,
    skills: [],
    runtime_id: "runtime-1",
    created_at: "2026-07-02T00:00:00Z",
    updated_at: "2026-07-02T00:00:00Z",
  }];

  return render(
    <QueryClientProvider client={client}>
      <IssueDesignRestoreSection issue={issue} agents={agents} timeline={timeline} />
    </QueryClientProvider>,
  );
}

function issue(overrides: Partial<Issue>): Issue {
  return {
    id: "issue-1",
    workspace_id: "ws-1",
    number: 1,
    identifier: "MUL-1",
    title: "UI设计",
    description: null,
    status: "todo",
    priority: "medium",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "user-1",
    parent_issue_id: "parent-1",
    project_id: "project-1",
    position: 0,
    start_date: null,
    due_date: null,
    metadata: {},
    created_at: "2026-07-02T00:00:00Z",
    updated_at: "2026-07-02T00:00:00Z",
    ...overrides,
    stage: overrides.stage ?? null,
    properties: overrides.properties ?? {},
  };
}

function restoreTask(overrides: Partial<DesignRestoreTask> = {}): DesignRestoreTask {
  return {
    id: "restore-task-1",
    workspace_id: "ws-1",
    file_id: "file-1",
    revision_id: "revision-1",
    issue_id: "issue-1",
    delivery_id: null,
    agent_task_id: null,
    status: "queued",
    input: {},
    result: {},
    error: null,
    created_by: null,
    created_at: "2026-07-02T00:00:00Z",
    updated_at: "2026-07-02T00:00:00Z",
    execution_status: overrides.execution_status ?? null,
    ...overrides,
  };
}

function implementationReceipt(result: Record<string, unknown>) {
  return {
    design_implementation: {
      schema_version: "multica.design-implementation-receipt/v1",
      collected_at: "2026-07-03T00:01:00Z",
      result_digest: "sha256:1234567890abcdef",
      identity: { design_ref: "design-ref-1", revision_id: "revision-1" },
      result,
      target_files: ["src/customer-list.tsx"],
      preview_paths: ["artifacts/customer-list.png"],
    },
  };
}

beforeEach(() => {
  mockDesignQueries.restoreTasks = [];
  mockDesignQueries.designDrafts = [];
  mockDesignQueries.designFiles = [{ id: "file-1", title: "服务记录设计稿", project_id: "project-1" }];
  mockDesignQueries.designDocuments = [];
  mockDesignQueries.implementationFrames = [];
  mockDesignQueries.projectResources = [];
  mockDesignQueries.agentTasks = [];
  useCommentDraftStore.setState({ drafts: {} });
  vi.mocked(api.buildDesignImplementationPrompt).mockReset();
  vi.mocked(api.buildDesignImplementationPrompt).mockResolvedValue({
    prompt: "实现选中的 Design Center 资产",
    context: { revision_id: "revision-1" },
  } as never);
  vi.mocked(api.createDesignDraftAgentTask).mockReset();
  vi.mocked(api.createDesignDraftAgentTask).mockResolvedValue({ task_id: "task-12345678", status: "queued" });
});

describe("IssueDesignRestoreSection role controls", () => {
  it("does not show manual role choice when the issue role is inferred from the title", () => {
    renderSection(issue({ title: "UI设计", metadata: {} }));

    expect(screen.getByText("UI 还原")).toBeInTheDocument();
    expect(screen.queryByText("标记 UI")).not.toBeInTheDocument();
    expect(screen.queryByText("标记前端")).not.toBeInTheDocument();
    expect(screen.queryByText("标题识别")).not.toBeInTheDocument();
    expect(screen.queryByText(/metadata\.design_role/)).not.toBeInTheDocument();
  });

  it("shows clear manual role choices only when the issue role is unknown", () => {
    renderSection(issue({ title: "服务记录开发", metadata: {} }));

    expect(screen.getByText("选择这个子 Issue 在设计流程中的阶段。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "设为 UI 设计" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "设为 UI 还原" })).toBeInTheDocument();
    expect(screen.queryByText(/metadata\.design_role/)).not.toBeInTheDocument();
  });
});

describe("IssueDesignRestoreSection frontend handoff visibility", () => {
  it.skip("can ask a UI Agent to generate a design draft from the issue requirement", async () => {
    renderSection(issue({ title: "服务记录 UI设计", metadata: {} }));

    const button = await screen.findByRole("button", { name: "让 UI Agent 生成设计稿" });
    fireEvent.click(button);

    await waitFor(() => {
      expect(api.createDesignDraftAgentTask).toHaveBeenCalledWith({
        agent_id: "agent-1",
        issue_id: "issue-1",
        title: "服务记录 UI设计 设计草稿",
        prompt: expect.stringContaining("模板候选"),
      });
    });
  });

  it.skip("shows the latest issue-linked design draft when one exists", async () => {
    mockDesignQueries.designDrafts = [{
      id: "draft-1",
      workspace_id: "ws-1",
      template_id: null,
      catalog_template_id: "template-1",
      template_revision_id: "template-revision-1",
      file_id: "file-1",
      revision_id: "revision-1",
      issue_id: "issue-1",
      title: "服务记录生成稿",
      requirement_core: {
        version: "1.0",
        title: "服务记录",
        pageType: "saas.filter-table-pagination",
        entity: { key: "service_record", label: "服务记录" },
      },
      slot_values: {},
      patch: [],
      status: "generated",
      validation_errors: [],
      generated_file_id: null,
      materialized_at: null,
      created_by: "user-1",
      created_at: "2026-07-02T00:00:00Z",
      updated_at: "2026-07-03T00:00:00Z",
    }];

    renderSection(issue({ title: "服务记录 UI设计", metadata: {} }));

    expect(await screen.findByText("服务记录生成稿")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "打开草稿" })).toBeInTheDocument();
  });

  it("keeps frontend handoff hidden before UI restore completes", () => {
    renderSection(issue({ title: "UI设计", metadata: {} }));

    expect(screen.queryByText("交付给前端开发")).not.toBeInTheDocument();
  });

  it.skip("shows frontend handoff after UI restore completes", async () => {
    mockDesignQueries.restoreTasks = [restoreTask({ status: "completed" })];

    renderSection(issue({ title: "UI设计", metadata: {} }));

    expect(await screen.findByText("交付给前端开发")).toBeInTheDocument();
  });

  it("shows runtime offline diagnostics for an active restore task", async () => {
    mockDesignQueries.restoreTasks = [restoreTask({
      agent_task_id: "agent-task-1",
      status: "running",
      execution_status: {
        agent_task_id: "agent-task-1",
        agent_task_status: "queued",
        agent_task_created_at: "2026-07-02T00:00:00Z",
        agent_task_dispatched_at: null,
        agent_task_started_at: null,
        agent_task_completed_at: null,
        agent_task_error: null,
        agent_task_wait_reason: null,
        runtime_id: "runtime-1",
        runtime_status: "offline",
        runtime_last_seen_at: "2026-07-02T00:00:00Z",
        last_message_seq: null,
        last_message_at: null,
        phase: "waiting_runtime",
        reason: "runtime_offline",
        severity: "warning",
      },
    })];

    renderSection(issue({ title: "UI设计", metadata: {} }));

    expect(await screen.findByText("运行时离线")).toBeInTheDocument();
    expect(screen.getByText("Agent 所在运行时当前离线，任务会继续等待守护进程恢复。")).toBeInTheDocument();
  });
});

describe("IssueDesignRestoreSection current project integration", () => {
  it("prefills an editable mention comment instead of sending it automatically", async () => {
    mockDesignQueries.designFiles = [{
      id: "file-figma",
      title: "Figma 客户列表",
      project_id: "project-1",
      project_resource_id: "repo-1",
      design_ref: "design-ref-1",
      current_revision_id: "revision-1",
      source: "figma",
      source_ref: {},
      updated_at: "2026-07-03T00:00:00Z",
    }];
    mockDesignQueries.implementationFrames = [{ frame_ref: "frame-ref-1", selection_key: "selection-1", title: "客户列表" }];
    mockDesignQueries.projectResources = [{
      id: "repo-1",
      resource_type: "github_repo",
      label: "multica/web",
      resource_ref: { url: "https://github.com/multica/web" },
    }];

    renderSection(issue({ title: "前端开发" }));

    const implementButton = await screen.findByRole("button", { name: "生成实现提示" });
    await waitFor(() => expect(implementButton).toBeEnabled());
    fireEvent.click(implementButton);

    await waitFor(() => {
      expect(api.buildDesignImplementationPrompt).toHaveBeenCalledWith("design-ref-1", {
        revision_id: "revision-1",
        frame_refs: ["frame-ref-1"],
        project_resource_id: "repo-1",
        issue_id: "issue-1",
      });
      const draft = useCommentDraftStore.getState().getDraft("new:issue-1");
      expect(draft).toContain("【Design Center 设计稿一键还原】\n<!-- multica-design-implementation:");
      expect(draft).toContain("\n[@UI Agent](mention://agent/agent-1)\n\n实现选中的 Design Center 资产");
    });
  });

  it("lets the user change the current project's target repository", async () => {
    mockDesignQueries.designFiles = [{
      id: "file-project",
      title: "Project-level design",
      project_id: "project-1",
      design_ref: "design-ref-project",
      current_revision_id: "revision-1",
      source: "figma",
      source_ref: {},
      updated_at: "2026-07-03T00:00:00Z",
    }];
    mockDesignQueries.implementationFrames = [{ frame_ref: "frame-ref-1", selection_key: "selection-1", title: "客户列表" }];
    mockDesignQueries.projectResources = [
      { id: "repo-1", resource_type: "github_repo", label: "multica/web", resource_ref: { url: "https://github.com/multica/web" } },
      { id: "repo-2", resource_type: "github_repo", label: "multica/desktop", resource_ref: { url: "https://github.com/multica/desktop" } },
    ];

    renderSection(issue({ title: "前端开发" }));

    const repoSelect = await screen.findByRole("combobox", { name: "实现目标仓库" });
    fireEvent.change(repoSelect, { target: { value: "repo-2" } });
    fireEvent.click(screen.getByRole("button", { name: "生成实现提示" }));

    await waitFor(() => expect(api.buildDesignImplementationPrompt).toHaveBeenCalledWith("design-ref-project", expect.objectContaining({
      project_resource_id: "repo-2",
    })));
  });

  it("shows every valid project design source and lets the user choose one", async () => {
    mockDesignQueries.designFiles = [
      {
        id: "file-figma",
        title: "Figma screen",
        project_id: "project-1",
        project_resource_id: "repo-1",
        design_ref: "design-ref-figma",
        current_revision_id: "revision-figma",
        source: "figma",
        source_ref: {},
        updated_at: "2026-07-03T00:00:00Z",
      },
      {
        id: "file-other-project",
        title: "Other project Figma screen",
        project_id: "project-2",
        project_resource_id: "repo-1",
        design_ref: "design-ref-other",
        current_revision_id: "revision-other",
        source: "figma",
        source_ref: {},
        updated_at: "2026-07-05T00:00:00Z",
      },
    ];
    mockDesignQueries.designDocuments = [{
      id: "design-document-1",
      design_ref: "design-ref-multica",
      workspace_id: "ws-1",
      project_id: "project-1",
      project_resource_id: "repo-1",
      issue_id: "issue-ui",
      title: "Multica Design screen",
      platform: "web",
      recipe: "saas-dashboard",
      status: "saved",
      draft_revision_id: "",
      saved_revision_id: "revision-multica",
      active_task: null,
      input_snapshot: {},
      last_error: null,
      repository_grounded: true,
      created_at: "2026-07-02T00:00:00Z",
      updated_at: "2026-07-04T00:00:00Z",
      saved_at: "2026-07-04T00:00:00Z",
    }];
    mockDesignQueries.implementationFrames = [{ frame_ref: "frame-ref-1", selection_key: "selection-1", title: "客户列表" }];
    mockDesignQueries.projectResources = [
      { id: "repo-1", resource_type: "github_repo", label: "repo one", resource_ref: { url: "https://github.com/multica/one" } },
    ];

    renderSection(issue({ title: "前端开发" }));

    const integrationCard = screen.getByText("当前项目集成").closest("div.rounded-md.border") as HTMLElement | null;
    expect(integrationCard).not.toBeNull();
    expect(await within(integrationCard!).findByRole("option", { name: /Multica Design screen/ })).toBeInTheDocument();
    expect(within(integrationCard!).getByRole("option", { name: /Figma screen/ })).toBeInTheDocument();
    expect(within(integrationCard!).queryByText("Other project Figma screen")).not.toBeInTheDocument();
    fireEvent.click(within(integrationCard!).getByRole("option", { name: /Figma screen/ }));
    const implementButton = within(integrationCard!).getByRole("button", { name: "生成实现提示" });
    await waitFor(() => expect(implementButton).toBeEnabled());
    fireEvent.click(implementButton);

    await waitFor(() => expect(api.buildDesignImplementationPrompt).toHaveBeenCalledWith("design-ref-figma", expect.objectContaining({
      revision_id: "revision-figma",
    })));
  });

  it("renders the structured implementation result returned by the ordinary Agent", async () => {
    mockDesignQueries.agentTasks = [{
      id: "implementation-task-1",
      agent_id: "agent-1",
      runtime_id: "runtime-1",
      issue_id: "issue-1",
      status: "completed",
      priority: 0,
      dispatched_at: "2026-07-03T00:00:00Z",
      started_at: "2026-07-03T00:00:01Z",
      completed_at: "2026-07-03T00:01:00Z",
      created_at: "2026-07-03T00:00:00Z",
      result: implementationReceipt({
        schema_version: "multica.design-implementation-result/v1",
        revision_id: "revision-1",
        status: "completed",
        mappings: [{ frame_ref: "frame-ref-1" }],
        commands: [{ command: "pnpm test", status: "passed" }],
        preview_evidence: [{ kind: "screenshot" }],
        blockers: [],
      }),
      error: null,
      trigger_summary: "【Design Center 设计稿一键还原】",
    } as AgentTask];

    renderSection(issue({ title: "前端开发" }));

    expect(await screen.findByText("验收通过")).toBeInTheDocument();
    expect(screen.getByText("结果：").parentElement).toHaveTextContent("completed");
    expect(screen.getByText("映射：").parentElement).toHaveTextContent("1");
    expect(screen.getByText("预览证据：").parentElement).toHaveTextContent("1");
  });

  it("renders a blocked implementation outcome instead of the completed Agent lifecycle", async () => {
    mockDesignQueries.agentTasks = [{
      id: "implementation-task-blocked",
      agent_id: "agent-1",
      runtime_id: "runtime-1",
      issue_id: "issue-1",
      status: "completed",
      priority: 0,
      dispatched_at: "2026-07-03T00:00:00Z",
      started_at: "2026-07-03T00:00:01Z",
      completed_at: "2026-07-03T00:01:00Z",
      created_at: "2026-07-03T00:00:00Z",
      result: implementationReceipt({
        schema_version: "multica.design-implementation-result/v1",
        revision_id: "revision-1",
        status: "blocked",
        mappings: [],
        commands: [],
        preview_evidence: [],
        blockers: ["design asset unavailable"],
      }),
      error: null,
      trigger_summary: "【Design Center 设计稿一键还原】",
    } as AgentTask];

    renderSection(issue({ title: "前端开发" }));

    expect(await screen.findByText("实现受阻")).toBeInTheDocument();
    expect(screen.queryByText("已完成")).not.toBeInTheDocument();
    expect(screen.getByText("结果：").parentElement).toHaveTextContent("blocked");
    expect(screen.getByText("阻塞：").parentElement).toHaveTextContent("1");
  });

  it("rejects an unvalidated result copied into the Agent's delivered comment", async () => {
    mockDesignQueries.agentTasks = [{
      id: "implementation-task-comment-result",
      agent_id: "agent-1",
      runtime_id: "runtime-1",
      issue_id: "issue-1",
      status: "completed",
      priority: 0,
      dispatched_at: "2026-07-03T00:00:00Z",
      started_at: "2026-07-03T00:00:01Z",
      completed_at: "2026-07-03T00:01:00Z",
      created_at: "2026-07-03T00:00:00Z",
      result: { output: "Issue remains blocked. Full result was delivered in the issue thread." },
      error: null,
      trigger_summary: "【Design Center 设计稿一键还原】",
    } as AgentTask];
    const timeline = [{
      id: "comment-result",
      actor_type: "agent",
      actor_id: "agent-1",
      issue_id: "issue-1",
      type: "comment",
      source_task_id: "implementation-task-comment-result",
      created_at: "2026-07-03T00:01:00Z",
      content: `Validated result:\n\n\`\`\`json\n${JSON.stringify({
        schema_version: "multica.design-implementation-result/v1",
        revision_id: "revision-1",
        status: "blocked",
        mappings: [],
        commands: [{ command: "pnpm test", status: "skipped" }],
        preview_evidence: [],
        blockers: ["frame mismatch", "assets unavailable"],
      })}\n\`\`\``,
    } as TimelineEntry];

    renderSection(issue({ title: "前端开发" }), timeline);

    expect(await screen.findByText("验收失败")).toBeInTheDocument();
    expect(screen.queryByText("实现受阻")).not.toBeInTheDocument();
    expect(screen.queryByText("结果：")).not.toBeInTheDocument();
  });
});
