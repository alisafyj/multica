import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  adjustDesignDocument,
  discardDesignDocumentDraft,
  getDesignDocument,
  getDesignDocumentRevision,
  getProject,
  listAgents,
  listDesignDocumentRevisions,
  listTaskMessages,
  deliverDesignDocument,
  listIssues,
  navigate,
  regenerateDesignDocument,
  restoreDesignDocumentRevision,
  saveDesignDocument,
  toastError,
  toastSuccess,
} = vi.hoisted(() => ({
  adjustDesignDocument: vi.fn(),
  discardDesignDocumentDraft: vi.fn(),
  getDesignDocument: vi.fn(),
  getDesignDocumentRevision: vi.fn(),
  getProject: vi.fn(),
  listAgents: vi.fn(),
  listDesignDocumentRevisions: vi.fn(),
  listTaskMessages: vi.fn(),
  deliverDesignDocument: vi.fn(),
  listIssues: vi.fn(),
  navigate: vi.fn(),
  regenerateDesignDocument: vi.fn(),
  restoreDesignDocumentRevision: vi.fn(),
  saveDesignDocument: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    adjustDesignDocument,
    discardDesignDocumentDraft,
    getDesignDocument,
    getDesignDocumentRevision,
    getDesignDocumentPreviewFileURL: (base: string, path: string) => `https://api.test${base}/${path}`,
    getProject,
    listAgents,
    listDesignDocumentRevisions,
    deliverDesignDocument,
    listIssues,
    listTaskMessages,
    regenerateDesignDocument,
    restoreDesignDocumentRevision,
    saveDesignDocument,
    cancelTaskById: vi.fn(),
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    designs: () => "/acme/designs",
    projectDetail: (id: string) => `/acme/projects/${id}`,
    designDocumentDetail: (id: string) => `/acme/designs/documents/${id}`,
  }),
}));

vi.mock("../navigation", () => ({
  AppLink: ({ children, href }: { children: ReactNode; href: string }) => <a href={href}>{children}</a>,
  useNavigation: () => ({ push: navigate }),
}));

vi.mock("sonner", () => ({
  toast: { error: toastError, success: toastSuccess },
}));

vi.mock("../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
}));

import { I18nProvider } from "@multica/core/i18n/react";
import zhCommon from "../locales/zh-Hans/common.json";
import { DesignDocumentPage, defaultRevisionId, documentErrorMessage, previewEntries } from "./design-document-page";

const AGENT = { id: "agent-1", workspace_id: "ws-1", name: "小设计", runtime_id: "runtime-1", runtime_bound: true, archived_at: null };

function document(overrides: Record<string, unknown> = {}) {
  return {
    id: "document-1",
    workspace_id: "ws-1",
    project_id: "project-1",
    project_resource_id: "",
    issue_id: "",
    title: "订单总览",
    platform: "web",
    recipe: "ui-mockup",
    status: "draft",
    draft_revision_id: "revision-2",
    saved_revision_id: "",
    active_task: null,
    input_snapshot: { brief: "做一个订单总览页，支持筛选。" },
    last_error: null,
    repository_grounded: false,
    created_at: "2026-08-19T00:00:00Z",
    updated_at: "2026-08-19T00:00:00Z",
    saved_at: "",
    ...overrides,
  };
}

function summary(overrides: Record<string, unknown> = {}) {
  return {
    id: "revision-2",
    revision_number: 2,
    content_digest: `sha256:${"b".repeat(64)}`,
    base_revision_id: "revision-1",
    source_task_id: "task-2",
    agent_id: "agent-1",
    instruction: "把顶部导航收紧",
    scope: { kind: "page", id: "orders" },
    is_draft: true,
    is_saved: false,
    page_count: 2,
    flow_count: 0,
    created_at: "2026-08-19T00:10:00Z",
    ...overrides,
  };
}

const FIRST = summary({ id: "revision-1", revision_number: 1, base_revision_id: "", instruction: "", scope: null, is_draft: false, source_task_id: "task-1", created_at: "2026-08-19T00:00:00Z" });

function revision(overrides: Record<string, unknown> = {}) {
  return {
    ...summary(),
    brief: { title: "订单总览" },
    coverage: {},
    audit: { passed: true },
    preview_receipt: {},
    prototype_entry: "prototype/index.html",
    pages: [
      { id: "home", title: "首页", parent_id: "", entry: "prototype/index.html", state_ids: [] },
      { id: "orders", title: "订单列表", parent_id: "", entry: "prototype/orders.html", state_ids: ["empty"] },
    ],
    flows: [],
    preview_targets: [
      { id: "prototype-index", kind: "prototype_entry", path: "prototype/index.html" },
      { id: "prototype-orders", kind: "prototype_page", path: "prototype/orders.html" },
    ],
    files: [
      { path: "prototype/index.html", role: "prototype_entry", media_type: "text/html", size_bytes: 2048 },
      { path: "assets/logo.png", role: "asset", media_type: "image/png", size_bytes: 4096 },
    ],
    resource_base_path: "/api/design-document-previews/ws-1/revision-2/bb/token/files",
    resource_access_token: "token",
    resource_access_expires_at: "2026-08-19T00:40:00Z",
    ...overrides,
  };
}

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(
    <I18nProvider locale="zh-Hans" resources={{ "zh-Hans": { common: zhCommon } }}>
      <QueryClientProvider client={queryClient}>
        <DesignDocumentPage documentId="document-1" />
      </QueryClientProvider>
    </I18nProvider>,
  );
  return queryClient;
}

beforeEach(() => {
  vi.clearAllMocks();
  getDesignDocument.mockResolvedValue(document());
  listDesignDocumentRevisions.mockResolvedValue({ revisions: [summary(), FIRST] });
  getDesignDocumentRevision.mockImplementation(async (_documentId: string, revisionId: string) =>
    revisionId === "revision-1"
      ? revision({ ...FIRST, resource_base_path: "/api/design-document-previews/ws-1/revision-1/aa/token/files" })
      : revision(),
  );
  getProject.mockResolvedValue({ id: "project-1", title: "CRM", workspace_id: "ws-1" });
  listAgents.mockResolvedValue([AGENT]);
  listTaskMessages.mockResolvedValue([]);
  listIssues.mockResolvedValue({ issues: [{ id: "issue-1", identifier: "MUL-7", title: "实现订单总览", status: "todo" }], total: 1 });
  deliverDesignDocument.mockImplementation(async (_id: string, body: { issue_id: string }) => document({ issue_id: body.issue_id }));
  adjustDesignDocument.mockResolvedValue(document({ status: "running", active_task: { id: "task-3", agent_id: "agent-1", status: "queued", operation: "adjust", error: null, created_at: "2026-08-19T00:20:00Z", started_at: null, completed_at: null } }));
  saveDesignDocument.mockResolvedValue(document({ status: "saved", saved_revision_id: "revision-2" }));
  discardDesignDocumentDraft.mockResolvedValue(document({ status: "empty", draft_revision_id: "" }));
  restoreDesignDocumentRevision.mockResolvedValue(document({ draft_revision_id: "revision-1" }));
});

// The pure helpers behind the page have their matrix here; the DOM tests below
// keep to the happy path, the wiring and the named regressions.
describe("design document page helpers", () => {
  it("shows the draft first, then the saved revision, then the newest one", () => {
    expect(defaultRevisionId(document() as never, [summary() as never, FIRST as never])).toBe("revision-2");
    expect(defaultRevisionId(document({ draft_revision_id: "", saved_revision_id: "revision-1" }) as never, [summary() as never])).toBe("revision-1");
    expect(defaultRevisionId(document({ draft_revision_id: "", saved_revision_id: "" }) as never, [summary() as never, FIRST as never])).toBe("revision-2");
    expect(defaultRevisionId(undefined, [])).toBe("");
  });

  it("lists pages first and then any preview target the brief did not name", () => {
    const entries = previewEntries(revision({
      preview_targets: [
        { id: "prototype-index", kind: "prototype_entry", path: "prototype/index.html" },
        { id: "prototype-orders", kind: "prototype_page", path: "prototype/orders.html" },
        { id: "prototype-help", kind: "prototype_page", path: "prototype/help.html" },
      ],
    }) as never);
    expect(entries.map((entry) => [entry.title, entry.entry])).toEqual([
      ["首页", "prototype/index.html"],
      ["订单列表", "prototype/orders.html"],
      ["help.html", "prototype/help.html"],
    ]);
    expect(previewEntries(undefined)).toEqual([]);
  });

  it("reads a message out of whatever shape the server's last_error takes", () => {
    expect(documentErrorMessage(null)).toBeNull();
    expect(documentErrorMessage("runtime went offline")).toBe("runtime went offline");
    expect(documentErrorMessage({ code: "runtime_offline", message: "runtime went offline" })).toBe("runtime went offline");
    expect(documentErrorMessage({ code: "audit_failed" })).toBe("audit_failed");
    expect(documentErrorMessage({})).toBe("任务未能产出可用的设计稿。");
  });
});

describe("DesignDocumentPage", () => {
  it("frames the draft's prototype with page tabs and lists the revision timeline", async () => {
    renderPage();
    expect(await screen.findByText("订单总览")).toBeInTheDocument();
    const frame = await screen.findByTitle("订单总览 · 首页");
    expect(frame).toHaveAttribute("src", "https://api.test/api/design-document-previews/ws-1/revision-2/bb/token/files/prototype/index.html");
    expect(frame).toHaveAttribute("sandbox", "allow-scripts");

    // Switching pages swaps the framed document without leaving the revision.
    await userEvent.click(screen.getByRole("tab", { name: "订单列表" }));
    expect(await screen.findByTitle("订单总览 · 订单列表")).toHaveAttribute(
      "src",
      "https://api.test/api/design-document-previews/ws-1/revision-2/bb/token/files/prototype/orders.html",
    );

    const timeline = screen.getByRole("region", { name: "版本" });
    expect(within(timeline).getByText("v2")).toBeInTheDocument();
    expect(within(timeline).getByText("v1")).toBeInTheDocument();
    expect(within(timeline).getByText("把顶部导航收紧")).toBeInTheDocument();
    expect(within(timeline).getByText("草稿")).toBeInTheDocument();
    // Only a revision that is not the draft offers to be brought back.
    expect(within(timeline).getAllByRole("button", { name: "回退到此版本" })).toHaveLength(1);
    expect(screen.getByText("未做仓库取证")).toBeInTheDocument();
  });

  // Open Design's 预览/代码 toggle: the same revision, rendered or read. The
  // source view lists the package's artifact index and fetches text files
  // over the capability route the preview frame already uses.
  it("switches to the 代码 view, lists the package files and shows a file's source", async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, text: async () => "<!doctype html><title>Orders</title>" });
    vi.stubGlobal("fetch", fetchMock);
    try {
      renderPage();
      await screen.findByRole("tab", { name: "首页" });

      await user.click(screen.getByRole("button", { name: "代码" }));
      const filesNav = screen.getByRole("navigation", { name: "包内文件" });
      expect(within(filesNav).getByText("prototype/index.html")).toBeInTheDocument();
      expect(within(filesNav).getByText("assets/logo.png")).toBeInTheDocument();

      // The prototype entry opens by default; its bytes render as source.
      expect(await screen.findByLabelText("prototype/index.html 的源码")).toHaveTextContent("<!doctype html>");
      expect(fetchMock).toHaveBeenCalledWith(expect.stringContaining("prototype/index.html"));

      // Back to 预览 restores the page tabs.
      await user.click(screen.getByRole("button", { name: "预览" }));
      expect(screen.getByRole("tab", { name: "首页" })).toBeInTheDocument();
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("sends an adjustment scoped to the current page against the revision on screen", async () => {
    renderPage();
    await screen.findByTitle("订单总览 · 首页");
    await userEvent.click(screen.getByRole("tab", { name: "订单列表" }));
    await userEvent.click(screen.getByRole("button", { name: "整份文档" }));
    await userEvent.type(screen.getByPlaceholderText(/描述你想怎么改/), "订单列表加一个状态筛选");
    // Once the adjustment is accepted the server reports the document as
    // running; the refetch after the mutation must see that too.
    const running = document({ status: "running", active_task: { id: "task-3", agent_id: "agent-1", status: "queued", operation: "adjust", error: null, created_at: "2026-08-19T00:20:00Z", started_at: null, completed_at: null } });
    adjustDesignDocument.mockResolvedValue(running);
    getDesignDocument.mockResolvedValue(running);
    await userEvent.click(screen.getByRole("button", { name: "发起调整" }));

    await waitFor(() => expect(adjustDesignDocument).toHaveBeenCalledTimes(1));
    expect(adjustDesignDocument).toHaveBeenCalledWith("document-1", {
      instruction: "订单列表加一个状态筛选",
      agent_id: "agent-1",
      scope: { kind: "page", id: "orders" },
      base_revision_id: "revision-2",
    });
    // The document now runs a task: the composer switches to queue mode
    // instead of closing.
    // Sending clears the box, so the composer has nothing staged and its one
    // control is the run's stop — Open Design's rule, not a second button.
    const queueBox = await screen.findByPlaceholderText(/任务执行中，现在提交会排队/);
    expect(queueBox).toBeEnabled();
    expect(await screen.findByRole("button", { name: "停止任务" })).toBeInTheDocument();
    // Typing again gives the same slot something to send, so it goes back to
    // queueing rather than stopping.
    await userEvent.type(queueBox, "再紧凑一点");
    expect(await screen.findByRole("button", { name: "排队调整" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "停止任务" })).not.toBeInTheDocument();
  });

  it("offers the tweaks panel among the finished run's follow-ups (DC-050)", async () => {
    renderPage();
    await screen.findByTitle("订单总览 · 首页");
    await userEvent.click(screen.getByRole("button", { name: "调整面板" }));
    const textarea = screen.getByPlaceholderText(/描述你想怎么改/) as HTMLTextAreaElement;
    expect(textarea.value).toContain("--accent / --scale / --density / --motion");
    expect(textarea.value).toContain("localStorage");
    expect(screen.getByRole("button", { name: "发起调整" })).toBeEnabled();
  });

  it("saves the draft the user is looking at and offers to discard it", async () => {
    renderPage();
    await screen.findByTitle("订单总览 · 首页");
    await userEvent.click(screen.getByRole("button", { name: "保存为设计稿" }));
    await waitFor(() => expect(saveDesignDocument).toHaveBeenCalledWith("document-1", { draft_revision_id: "revision-2" }));
    expect(toastSuccess).toHaveBeenCalled();
  });

  it("previews a historical revision without leaving the draft, and can bring it back", async () => {
    renderPage();
    await screen.findByTitle("订单总览 · 首页");
    const timeline = screen.getByRole("region", { name: "版本" });
    await userEvent.click(within(timeline).getByText("v1"));
    expect(await screen.findByText(/正在查看历史版本 v1/)).toBeInTheDocument();
    expect(screen.getByTitle("订单总览 · 首页")).toHaveAttribute(
      "src",
      "https://api.test/api/design-document-previews/ws-1/revision-1/aa/token/files/prototype/index.html",
    );

    await userEvent.click(within(timeline).getByRole("button", { name: "回退到此版本" }));
    await waitFor(() => expect(restoreDesignDocumentRevision).toHaveBeenCalledWith("document-1", "revision-1"));
  });

  it("shows the failure of the last run and keeps the previous version available", async () => {
    getDesignDocument.mockResolvedValue(document({
      status: "failed",
      last_error: { code: "runtime_offline", message: "runtime went offline" },
      active_task: { id: "task-9", agent_id: "agent-1", status: "failed", operation: "adjust", error: "runtime went offline", created_at: "2026-08-19T00:20:00Z", started_at: "2026-08-19T00:20:00Z", completed_at: "2026-08-19T00:25:00Z" },
    }));
    renderPage();
    expect(await screen.findByRole("alert")).toHaveTextContent("调整失败");
    expect(screen.getByRole("alert")).toHaveTextContent("runtime went offline");
    expect(screen.getByRole("alert")).toHaveTextContent("上一版仍然可用");
    // The prototype of the draft that failed run never replaced is still framed.
    expect(await screen.findByTitle("订单总览 · 首页")).toBeInTheDocument();
  });

  it("explains an empty document instead of framing nothing", async () => {
    getDesignDocument.mockResolvedValue(document({ status: "running", draft_revision_id: "", active_task: { id: "task-1", agent_id: "agent-1", status: "running", operation: "generate", error: null, created_at: "2026-08-19T00:00:00Z", started_at: "2026-08-19T00:00:01Z", completed_at: null } }));
    listDesignDocumentRevisions.mockResolvedValue({ revisions: [] });
    renderPage();
    expect(await screen.findByText("智能体正在生成，完成并通过校验后这里会显示原型。")).toBeInTheDocument();
    expect(screen.getByText("第一版正在生成。")).toBeInTheDocument();
    // The live run is visible through the composer now, not through a card
    // stacked above the thread: nothing is staged, so the send control is the
    // stop control.
    expect(screen.getByRole("button", { name: "停止任务" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "保存为设计稿" })).not.toBeInTheDocument();
    // A live run is not the dead end — no rerun offer while it could still land.
    expect(screen.queryByRole("button", { name: "重新生成" })).not.toBeInTheDocument();
  });

  // The dead end: the first run failed (or was stopped) before any revision
  // landed, so there is nothing to adjust — the page must offer the rerun.
  it("offers 重新生成 when the first run died before producing a revision", async () => {
    getDesignDocument.mockResolvedValue(document({
      status: "failed",
      draft_revision_id: "",
      saved_revision_id: "",
      last_error: { code: "design_document_cancelled", message: "design document task was cancelled" },
      active_task: null,
    }));
    listDesignDocumentRevisions.mockResolvedValue({ revisions: [] });
    regenerateDesignDocument.mockResolvedValue(document({
      status: "running",
      draft_revision_id: "",
      saved_revision_id: "",
      last_error: null,
      active_task: { id: "task-2", agent_id: "agent-1", status: "queued", operation: "generate", error: null, created_at: "2026-08-19T00:30:00Z", started_at: null, completed_at: null },
    }));
    renderPage();

    await userEvent.click(await screen.findByRole("button", { name: "重新生成" }));
    await waitFor(() => expect(regenerateDesignDocument).toHaveBeenCalledWith("document-1", {}));
  });

  // Export scope follows from what each format is for, so the menu states it
  // rather than asking the user to configure anything.
  it("offers every export format and says what each one covers", async () => {
    renderPage();
    await screen.findByTitle("订单总览 · 首页");

    await userEvent.click(screen.getByRole("button", { name: "导出" }));
    const menu = await screen.findByRole("menu");
    for (const label of ["图片 (PNG)", "单页 HTML（自包含）", "PDF", "演示文稿 (PPTX)"]) {
      expect(within(menu).getByText(label)).toBeInTheDocument();
    }
    // A picture and a self-contained page are of the page on screen; a
    // document and a deck are of the whole design.
    expect(within(menu).getAllByText("当前页")).toHaveLength(2);
    expect(within(menu).getAllByText("全部 2 页")).toHaveLength(2);
  });

  it("keeps export and screenshot out of the source view, which has nothing to rasterise", async () => {
    renderPage();
    await screen.findByTitle("订单总览 · 首页");
    expect(screen.getByRole("button", { name: "截图" })).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "代码" }));
    expect(screen.queryByRole("button", { name: "截图" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "导出" })).not.toBeInTheDocument();
  });

  // 编辑 is the second way to change a design: the designer edits it directly
  // instead of asking an agent. It still produces a revision, so it carries
  // the same preconditions an adjustment does.
  it("opens the properties panel in 编辑 mode and blocks an empty apply", async () => {
    renderPage();
    await screen.findByTitle("订单总览 · 首页");

    await userEvent.click(screen.getByRole("button", { name: "编辑" }));
    expect(await screen.findByText(/在画布上点选一个元素/)).toBeInTheDocument();
    // Nothing picked, nothing changed: applying would create a revision
    // identical to its base.
    expect(screen.getByRole("button", { name: "应用修改" })).toBeDisabled();
    expect(screen.getByText("在画布上选中元素后修改属性")).toBeInTheDocument();
    // The adjust composer steps aside: one way to change the design at a time.
    expect(screen.queryByPlaceholderText(/描述你想怎么改/)).not.toBeInTheDocument();
  });

  // taskMessagesOptions is gated on a UUID task id, so a plan fixture has to
  // carry a real one or the query never runs and the bar never renders.
  const PLAN_TASK_ID = "01a03707-5b0d-7728-87ba-daf0d4a5b315";

  // Two lists, two homes, and they are not interchangeable. The plan answers
  // "what is left" and belongs where it cannot scroll away — pinned above the
  // composer. The follow-ups are what the conversation arrives at, so they
  // land at the end of the thread, and only once there is a design to refine.
  it("pins the run's plan and holds the follow-ups until it lands", async () => {
    getDesignDocument.mockResolvedValue(document({
      status: "running",
      draft_revision_id: "",
      active_task: { id: PLAN_TASK_ID, agent_id: "agent-1", status: "running", operation: "generate" },
    }));
    listTaskMessages.mockResolvedValue([
      {
        task_id: PLAN_TASK_ID, seq: 1, type: "tool_use", tool: "todo_write",
        input: { todos: [
          { content: "锁定页面范围", status: "completed" },
          { content: "实现原型", status: "in_progress" },
        ] },
      },
    ]);
    renderPage();

    // Pinned, collapsed, and present while the run is still going.
    expect(await screen.findByText("待办")).toBeInTheDocument();
    expect(screen.getByText("1/2")).toBeInTheDocument();
    // Nothing to refine yet, so no follow-ups.
    expect(screen.queryByText("设计润色")).not.toBeInTheDocument();
  });

  it("offers the follow-ups once a version exists", async () => {
    getDesignDocument.mockResolvedValue(document({ status: "saved", saved_revision_id: "revision-2" }));
    renderPage();
    await screen.findByTitle("订单总览 · 首页");

    expect(screen.getByText("设计润色")).toBeInTheDocument();
    expect(screen.getByText("下一步")).toBeInTheDocument();
  });

  // The end of the designer's flow (DC-062). A draft must not be deliverable:
  // an agent building from something the designer never stood behind is the
  // failure this gate exists to prevent.
  it("only offers delivery once the design is saved", async () => {
    renderPage();
    await screen.findByTitle("订单总览 · 首页");
    expect(screen.getByLabelText("交付给实现任务")).toBeDisabled();
    expect(screen.getByText(/草稿不是承诺/)).toBeInTheDocument();
  });

  // The launcher's companion task writes issue_id when the document is created,
  // long before there is anything to hand over. Reading that column as the
  // delivery state announced 已交付 while the first version was still being
  // generated — a promise the document had not made.
  it("does not call a linked task a delivery before anything is saved", async () => {
    getDesignDocument.mockResolvedValue(document({
      status: "running",
      saved_revision_id: "",
      issue_id: "issue-1",
      active_task: { id: "task-1", agent_id: "agent-1", status: "running", operation: "generate" },
    }));
    renderPage();

    expect(await screen.findByText(/已关联任务，但还没有交付/)).toBeInTheDocument();
    expect(screen.queryByText(/会在工作区中收到这份已保存的设计包/)).not.toBeInTheDocument();
  });

  it("delivers the saved design to the issue that implements it", async () => {
    getDesignDocument.mockResolvedValue(document({ status: "saved", saved_revision_id: "revision-2" }));
    renderPage();
    await screen.findByTitle("订单总览 · 首页");

    await userEvent.click(screen.getByLabelText("交付给实现任务"));
    await userEvent.click(await screen.findByText("实现订单总览"));

    await waitFor(() => expect(deliverDesignDocument).toHaveBeenCalledWith("document-1", { issue_id: "issue-1" }));
    expect(toastSuccess).toHaveBeenCalledWith("已交付给实现任务");
  });

  it("keeps the rerun away from documents that have a revision to adjust", async () => {
    renderPage();
    await screen.findByTitle("订单总览 · 首页");
    expect(screen.queryByRole("button", { name: "重新生成" })).not.toBeInTheDocument();
  });

  // Open Design queues chat sends during a run; the composer does the same:
  // a submission while running is held and fired when the run lands.
  it("queues an adjustment during a run and flushes it when the run lands", async () => {
    getDesignDocument.mockResolvedValue(document({
      status: "running",
      active_task: { id: "task-9", agent_id: "agent-1", status: "running", operation: "adjust", error: null, created_at: "2026-08-19T00:20:00Z", started_at: "2026-08-19T00:20:01Z", completed_at: null },
    }));
    const queryClient = renderPage();

    const textarea = await screen.findByPlaceholderText(/任务执行中，现在提交会排队/);
    await userEvent.type(textarea, "顶栏加一个搜索框");
    await userEvent.click(screen.getByRole("button", { name: "排队调整" }));
    expect(adjustDesignDocument).not.toHaveBeenCalled();
    expect(screen.getByText(/已排队/)).toBeInTheDocument();
    expect(screen.getByText("顶栏加一个搜索框")).toBeInTheDocument();

    // The run lands: the document refetches as adjustable and the queued
    // instruction fires on its own.
    getDesignDocument.mockResolvedValue(document());
    await queryClient.invalidateQueries();
    await waitFor(() => expect(adjustDesignDocument).toHaveBeenCalledTimes(1));
    expect(adjustDesignDocument.mock.calls[0]?.[1]).toMatchObject({ instruction: "顶栏加一个搜索框" });
  });

  it("hands the queued text back when the run dies without a revision", async () => {
    getDesignDocument.mockResolvedValue(document({
      status: "running",
      draft_revision_id: "",
      saved_revision_id: "",
      active_task: { id: "task-1", agent_id: "agent-1", status: "running", operation: "generate", error: null, created_at: "2026-08-19T00:00:00Z", started_at: "2026-08-19T00:00:01Z", completed_at: null },
    }));
    listDesignDocumentRevisions.mockResolvedValue({ revisions: [] });
    const queryClient = renderPage();

    const textarea = await screen.findByPlaceholderText(/任务执行中，现在提交会排队/);
    await userEvent.type(textarea, "配色换成品牌蓝");
    await userEvent.click(screen.getByRole("button", { name: "排队调整" }));

    getDesignDocument.mockResolvedValue(document({
      status: "failed",
      draft_revision_id: "",
      saved_revision_id: "",
      last_error: { code: "design_document_cancelled", message: "design document task was cancelled" },
      active_task: null,
    }));
    await queryClient.invalidateQueries();

    await waitFor(() => expect(toastError).toHaveBeenCalledWith("这次运行没有产出可调整的版本，排队的调整未发送"));
    expect(adjustDesignDocument).not.toHaveBeenCalled();
    expect((screen.getByPlaceholderText(/生成完成后可以在这里继续调整|描述你想怎么改/) as HTMLTextAreaElement).value).toBe("配色换成品牌蓝");
  });
});
