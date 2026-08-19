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
  navigate,
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
  navigate: vi.fn(),
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
    listTaskMessages,
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
    // The document now runs a task: the composer waits for it.
    expect(await screen.findByText("任务执行中，完成后可以继续调整")).toBeInTheDocument();
    expect(screen.getByLabelText("智能体任务活动")).toBeInTheDocument();
  });

  it("offers the tweaks panel as a ready-made adjustment (DC-050)", async () => {
    renderPage();
    await screen.findByTitle("订单总览 · 首页");
    await userEvent.click(screen.getByRole("button", { name: "添加调整面板" }));
    const textarea = screen.getByPlaceholderText(/描述你想怎么改/) as HTMLTextAreaElement;
    expect(textarea.value).toContain("--accent / --scale / --density / --mode / --motion");
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
    expect(screen.getByLabelText("智能体任务活动")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "保存为设计稿" })).not.toBeInTheDocument();
  });
});
