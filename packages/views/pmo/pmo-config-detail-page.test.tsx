/**
 * PMOConfigDetailPage (/pmo/:configId) state coverage.
 *
 * The page is tested through real DOM assertions with only the data and
 * primitive layers mocked (hooks + UI components). The config id is read
 * from the real NavigationProvider adapter's pathname; row/back navigation
 * is asserted against the adapter's push. i18n resolves through the real
 * RESOURCES bundle. The Tabs mock is CONTROLLED (only the active panel
 * mounts, mirroring the real Base UI tabs), so tab-switching is exercised.
 */
import React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithI18n } from "../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import type { PMOConfig, PMORun } from "@multica/core/types";
import { PMOConfigDetailPage } from "./pmo-config-detail-page";

// ---------------------------------------------------------------------------
// Fixtures — fictional only (no company names, domains, real identifiers).
// ---------------------------------------------------------------------------

const CONFIG: PMOConfig = {
  id: "cfg-1",
  workspace_id: "ws-1",
  name: "Platform requirements",
  agent_id: "agent-1",
  root_external_key: "EXT-P-001",
  workload_property_id: null,
  schedule_enabled: false,
  next_run_at: null,
  last_run_at: null,
  last_applied_at: null,
  created_by: "user-1",
  created_at: "2026-08-01T00:00:00Z",
  updated_at: "2026-08-01T00:00:00Z",
};

const PREVIEW_DIFF = {
  entities: [
    {
      external_type: "requirement",
      external_key: "EXT-P-001",
      local_type: "project",
      local_id: "project-1",
      action: "update",
      fields: {
        title: {
          baseline_external: "Old external title",
          baseline_local: "Old local title",
          external: "New external title",
          local: "New local title",
          decision: "conflict",
        },
        status: {
          baseline_external: "todo",
          baseline_local: "todo",
          external: "in_progress",
          local: "todo",
          decision: "incoming",
        },
      },
    },
    {
      external_type: "task",
      external_key: "TASK-001",
      local_type: "issue",
      local_id: "issue-1",
      action: "create",
      fields: {
        title: { external: "New task title", decision: "incoming" },
      },
    },
  ],
  warnings: [
    {
      code: "unresolved_assignee",
      external_id: "EXT-U-001",
      display_name: "Example User",
      external_type: "requirement",
      external_key: "EXT-P-001",
      field: "assignee_id",
    },
  ],
  summary: {
    creates: 1,
    incoming_fields: 2,
    local_only_fields: 0,
    converged_fields: 0,
    conflicts: 1,
    external_removed: 0,
    unresolved_assignees: 1,
  },
};

function makeRun(overrides: Partial<PMORun> = {}): PMORun {
  return {
    id: "run-1",
    workspace_id: "ws-1",
    config_id: "cfg-1",
    agent_task_id: null,
    trigger: "manual",
    status: "preview_ready",
    source_snapshot: null,
    diff: PREVIEW_DIFF,
    summary: null,
    error_code: null,
    error_message: null,
    requested_by: "user-1",
    created_at: "2026-08-03T00:00:00Z",
    started_at: "2026-08-03T00:00:05Z",
    completed_at: "2026-08-03T00:01:00Z",
    applied_at: null,
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const startRunMutate = vi.fn();
const applyRunMutate = vi.fn();
const setMappingMutate = vi.fn();
const updateConfigMutate = vi.fn();
const push = vi.fn();
const transcriptButtonProps = vi.fn();

vi.mock("@multica/core/pmo/mutations", () => ({
  useStartPMORun: () => ({ mutate: startRunMutate, isPending: false }),
  useApplyPMORun: () => ({ mutate: applyRunMutate, isPending: false }),
  useSetPMOAssigneeMapping: () => ({ mutate: setMappingMutate, isPending: false }),
  useUpdatePMOConfig: () => ({ mutate: updateConfigMutate, isPending: false }),
  useCreatePMOConfig: () => ({ mutate: vi.fn(), isPending: false }),
  useDeletePMOConfig: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock("@multica/core/pmo/queries", () => ({
  pmoConfigsOptions: () => ({ queryKey: ["pmo", "configs"] }),
  pmoRunsOptions: (_wsId: string, configId: string) => ({
    queryKey: ["pmo", "runs", configId],
    enabled: Boolean(configId),
  }),
}));

vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"] }),
  agentListOptions: () => ({ queryKey: ["agents"] }),
}));

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/agents", () => ({
  isAgentRuntimeBound: (agent: { runtime_bound?: boolean }) => agent.runtime_bound !== false,
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    pmo: () => "/ws-1/pmo",
    pmoConfigDetail: (id: string) => `/ws-1/pmo/${id}`,
  }),
}));

// Query results controlled per test.
const queryState = {
  configs: { data: [] as PMOConfig[] | undefined, isPending: false, isError: false, isSuccess: false },
  runs: { data: [] as PMORun[] | undefined, isPending: false, isError: false, isSuccess: false },
};

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { queryKey?: readonly unknown[] }) => {
    const key = options.queryKey?.[0];
    const second = options.queryKey?.[1];
    if (key === "pmo" && second === "configs") return queryState.configs;
    if (key === "pmo" && second === "runs") return { ...queryState.runs, data: options.queryKey?.[2] ? queryState.runs.data : undefined };
    if (key === "members") return { data: [
      { id: "member-1", name: "Example Member", email: "example@example.test", user_id: "user-1" },
      { id: "member-2", name: "Feng Member", email: "fengyujie@example.test", user_id: "user-feng-uuid" },
      { id: "member-3", name: "Other Member", email: "other@example.test", user_id: "user-other-uuid" },
    ] };
    if (key === "agents") return { data: [
      { id: "agent-1", name: "Example Agent", owner_id: "user-1", archived_at: null, runtime_bound: true },
      { id: "agent-feng", name: "Frontend Agent", owner_id: "user-feng-uuid", archived_at: null, runtime_bound: true },
      { id: "agent-other", name: "Other Agent", owner_id: "user-other-uuid", archived_at: null, runtime_bound: true },
      { id: "agent-unbound", name: "Unbound Agent", owner_id: "user-feng-uuid", archived_at: null, runtime_bound: false },
    ] };
    return { data: [] };
  },
}));

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

vi.mock("../common/task-transcript", () => ({
  TranscriptButton: (props: { task: { id: string }; agentName: string; isLive?: boolean; title?: string }) => {
    transcriptButtonProps(props);
    return <button aria-label={props.title}>{props.title}</button>;
  },
}));

// Keep the ui primitives as light DOM so the state logic is what is under test.
// Button preserves its `render` prop (a real AppLink) so link-style buttons
// (e.g. the not-found back link) stay clickable and navigable.
vi.mock("@multica/ui/components/ui/button", async () => {
  const React = await import("react");
  return {
    Button: ({ children, render, ...props }: {
      children?: React.ReactNode;
      render?: React.ReactElement;
      [key: string]: unknown;
    }) => (render ? React.cloneElement(render, undefined, children) : <button {...props}>{children}</button>),
  };
});
vi.mock("@multica/ui/components/ui/badge", () => ({
  Badge: ({ children }: { children?: React.ReactNode }) => <span>{children}</span>,
}));
vi.mock("@multica/ui/components/ui/input", () => ({
  Input: (props: React.InputHTMLAttributes<HTMLInputElement>) => <input {...props} />,
}));
vi.mock("@multica/ui/components/ui/switch", () => ({
  Switch: ({ checked, disabled, onCheckedChange, ...rest }: {
    checked?: boolean;
    disabled?: boolean;
    onCheckedChange?: (value: boolean) => void;
    [key: string]: unknown;
  }) => (
    <button
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onCheckedChange?.(!checked)}
      {...rest}
    />
  ),
}));
vi.mock("@multica/ui/components/ui/skeleton", () => ({
  Skeleton: ({ className }: { className?: string }) => <div data-testid="skeleton" className={className} />,
}));
vi.mock("@multica/ui/components/ui/spinner", () => ({
  Spinner: () => <span data-testid="spinner" />,
}));
vi.mock("@multica/ui/components/ui/native-select", () => ({
  NativeSelect: ({ children, className, size: _size, ...props }: React.SelectHTMLAttributes<HTMLSelectElement> & { size?: "sm" | "default" | number }) => (
    <select className={className} {...props}>{children}</select>
  ),
  NativeSelectOption: ({ children, ...props }: React.OptionHTMLAttributes<HTMLOptionElement>) => (
    <option {...props}>{children}</option>
  ),
}));
vi.mock("@multica/ui/components/ui/dialog", () => ({
  Dialog: ({ open, children }: { open?: boolean; children?: React.ReactNode }) => (open ? <>{children}</> : null),
  DialogContent: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  DialogHeader: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  DialogFooter: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  DialogDescription: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));
vi.mock("@multica/ui/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ render }: { render?: React.ReactElement }) => (render ?? null),
  TooltipContent: () => null,
}));

// Controlled Tabs mock: only the active panel mounts, mirroring the real
// Base UI tabs. The trigger click calls onValueChange so the page's own tab
// state drives which panel renders.
vi.mock("@multica/ui/components/ui/tabs", async () => {
  const React = await import("react");
  const TabContext = React.createContext<{
    value: string;
    onValueChange?: (value: string) => void;
  }>({ value: "" });
  return {
    Tabs: ({ value, onValueChange, children }: {
      value?: string;
      onValueChange?: (value: string) => void;
      children?: React.ReactNode;
    }) => (
      <TabContext.Provider value={{ value: value ?? "", onValueChange }}>
        {children}
      </TabContext.Provider>
    ),
    TabsList: ({ children }: { children?: React.ReactNode }) => (
      <div role="tablist">{children}</div>
    ),
    TabsTrigger: ({ value, children }: { value: string; children?: React.ReactNode }) => {
      const ctx = React.useContext(TabContext);
      return (
        <button
          type="button"
          role="tab"
          aria-selected={ctx.value === value}
          onClick={() => ctx.onValueChange?.(value)}
        >
          {children}
        </button>
      );
    },
    TabsContent: ({ value, children }: { value: string; children?: React.ReactNode }) => {
      const ctx = React.useContext(TabContext);
      if (ctx.value !== value) return null;
      return (
        <div role="tabpanel" data-value={value}>{children}</div>
      );
    },
  };
});
vi.mock("../layout/collection-page", () => ({
  CollectionPageHeader: ({ children, actions }: { children?: React.ReactNode; actions?: React.ReactNode }) => (
    <header>{children}{actions}</header>
  ),
  CollectionPageHeaderAction: ({ label, onClick }: { label?: React.ReactNode; onClick?: () => void }) => (
    <button type="button" onClick={onClick}>{label}</button>
  ),
  CollectionPageState: ({ title, description, actions }: {
    title?: React.ReactNode;
    description?: React.ReactNode;
    actions?: React.ReactNode;
  }) => (
    <div data-testid="page-state">
      <div>{title}</div>
      <div>{description}</div>
      <div>{actions}</div>
    </div>
  ),
}));

function pageElement() {
  const adapter: NavigationAdapter = {
    pathname: "/ws-1/pmo/cfg-1",
    push,
    replace: vi.fn(),
    back: vi.fn(),
    searchParams: new URLSearchParams(),
    getShareableUrl: (path) => path,
  };
  return (
    <NavigationProvider value={adapter}>
      <PMOConfigDetailPage />
    </NavigationProvider>
  );
}

function renderPage() {
  return renderWithI18n(pageElement());
}

function previewConfig(overrides: Partial<PMOConfig> = {}) {
  queryState.configs = { data: [{ ...CONFIG, ...overrides }], isPending: false, isError: false, isSuccess: true };
}

function loadingConfigs() {
  queryState.configs = { data: undefined, isPending: true, isError: false, isSuccess: false };
}

function errorConfigs() {
  queryState.configs = { data: undefined, isPending: false, isError: true, isSuccess: false };
}

function noConfig() {
  queryState.configs = { data: [], isPending: false, isError: false, isSuccess: true };
}

function setRuns(runs: PMORun[]) {
  queryState.runs = { data: runs, isPending: false, isError: false, isSuccess: true };
}

const previewPanel = () =>
  document.querySelector<HTMLElement>('[role="tabpanel"][data-value="preview"]') as HTMLElement;

beforeEach(() => {
  queryState.configs = { data: [], isPending: false, isError: false, isSuccess: false };
  queryState.runs = { data: [], isPending: false, isError: false, isSuccess: false };
  startRunMutate.mockClear();
  applyRunMutate.mockClear();
  setMappingMutate.mockClear();
  updateConfigMutate.mockClear();
  push.mockClear();
  transcriptButtonProps.mockClear();
});

describe("PMOConfigDetailPage routing and states", () => {
  it("reads the config id from the pathname and renders the config name", () => {
    previewConfig();
    setRuns([]);
    renderPage();
    // Breadcrumb leaf shows the matched config name.
    expect(screen.getByText("Platform requirements")).toBeInTheDocument();
  });

  it("renders skeletons while configs load", () => {
    loadingConfigs();
    const { container } = renderPage();
    expect(container.querySelectorAll('[data-testid="skeleton"]').length).toBeGreaterThan(0);
  });

  it("shows the error state when the config list fails", () => {
    errorConfigs();
    renderPage();
    expect(screen.getByText("Failed to load sync configs.")).toBeInTheDocument();
  });

  it("shows the not-found state with a back link when the config id is unknown", () => {
    noConfig();
    renderPage();
    expect(screen.getByText("Sync config not found")).toBeInTheDocument();
    expect(screen.getByText("This config may have been deleted.")).toBeInTheDocument();
    const backLink = screen.getByRole("link", { name: "Back to requirements" });
    expect(backLink).toHaveAttribute("href", "/ws-1/pmo");
    fireEvent.click(backLink);
    expect(push).toHaveBeenCalledWith("/ws-1/pmo");
  });
});

describe("PMOConfigDetailPage preview tab", () => {
  it("renders a preview_ready manual run's field-level diff", () => {
    previewConfig();
    setRuns([makeRun()]);
    renderPage();
    // Entity name is the primary text; the stable external key is secondary.
    expect(screen.getAllByTestId("pmo-entity-name").map((node) => node.textContent)).toEqual(
      expect.arrayContaining(["New external title", "New task title"]),
    );
    expect(screen.getAllByTestId("pmo-entity-key").map((node) => node.textContent)).toEqual(
      expect.arrayContaining(["EXT-P-001", "TASK-001"]),
    );
    expect(screen.getByText("New local title")).toBeInTheDocument();
  });

  it("renders each entity identity once while keeping every field row", () => {
    previewConfig();
    setRuns([makeRun()]);
    renderPage();

    expect(screen.getAllByTestId("pmo-entity-name").map((node) => node.textContent)).toEqual([
      "New external title",
      "New task title",
    ]);
    expect(screen.getAllByTestId("pmo-entity-key").map((node) => node.textContent)).toEqual([
      "EXT-P-001",
      "TASK-001",
    ]);
    expect(screen.getByText("status")).toBeInTheDocument();
  });

  it("keeps the long detail content in a vertical scroll container", () => {
    previewConfig();
    setRuns([makeRun()]);
    renderPage();

    const content = screen.getByTestId("pmo-detail-content");
    expect(content.className).toContain("min-h-0");
    expect(content.className).toContain("flex-1");
    expect(content.className).toContain("overflow-y-auto");
  });

  it("renders the source requirement summary and milestone schedule", () => {
    previewConfig();
    setRuns([makeRun({
      source_snapshot: {
        schema_version: "1",
        snapshot_complete: true,
        parent_requirement: {
          key: "SY-P-20260452",
          display_number: "PM-21503",
          numeric_id: 136076,
          title: "院务系统-开单-增加美团订单券码校验-1.0",
          description: "https://soyoung.feishu.cn/wiki/Ifl9wASw2iWHL4kEbN1cpF3Ynje",
          source_status: "已上线",
          status: "completed",
          priority: "P2-3",
          prd_url: "https://soyoung.feishu.cn/wiki/Ifl9wASw2iWHL4kEbN1cpF3Ynje",
          owner: { external_id: "fengyujie@example.test", display_name: "Feng External" },
          start_date: "2026-07-21",
          due_date: "2026-08-11",
          workload: 15,
          tasks: [],
        },
        child_requirements: [],
        tasks: [
          {
            task_id: "TASK-FE-1",
            scheme_id: "scheme-fe",
            scheme_name: "M4-开发-前端",
            title: "院务系统-开单处理",
            description: "",
            source_status: "未开始",
            status: "todo",
            owner: { external_id: "fengyujie", display_name: "风尘（冯钰杰）" },
            start_date: "2026-07-24",
            due_date: "2026-07-24",
            workload: 1,
            updated_at: null,
          },
          {
            task_id: "TASK-QA-1",
            scheme_id: "scheme-qa",
            scheme_name: "M5-测试",
            title: "订单券码校验测试",
            source_status: "进行中",
            status: "in_progress",
            owner: { external_id: "unmatched@example.test", display_name: "Unmatched User" },
            start_date: "2026-07-25",
            due_date: "2026-07-26",
            workload: 2,
          },
        ],
      },
      diff: {
        entities: [{
          external_type: "requirement",
          external_key: "SY-P-20260452",
          action: "update",
          fields: {
            title: { external: "Incoming title", local: "Local title", decision: "conflict" },
          },
        }, {
          external_type: "task",
          external_key: "TASK-FE-1",
          action: "update",
          fields: {
            title: { external: "院务系统-开单处理", local: "本地标题", decision: "conflict" },
            status: { external: "todo", local: "in_progress", decision: "conflict" },
          },
        }],
      },
    })]);
    renderPage();

    expect(screen.getByRole("heading", { name: "院务系统-开单-增加美团订单券码校验-1.0" })).toBeInTheDocument();
    expect(screen.getByText("PM-21503")).toBeInTheDocument();
    expect(screen.getByText("已上线")).toBeInTheDocument();
    expect(screen.getByText("P2-3")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /PRD/ })).toHaveAttribute(
      "href",
      "https://soyoung.feishu.cn/wiki/Ifl9wASw2iWHL4kEbN1cpF3Ynje",
    );
    const requirementTable = screen.getByTestId("pmo-requirement-table");
    expect(within(requirementTable).getAllByRole("columnheader").map((cell) => cell.textContent)).toEqual([
      "requirement ID",
      "Title",
      "External owner",
      "Start date",
      "Due date",
      "Workload",
      "Status",
      "PRD",
    ]);
    const requirementRow = within(requirementTable).getByRole("row", { name: /院务系统-开单-增加美团订单券码校验-1.0/ });
    expect(within(requirementRow).getByText("Feng Member")).toBeInTheDocument();
    expect(within(requirementRow).getByRole("button", { name: "Use external SY-P-20260452 title" })).toBeInTheDocument();
    const schedule = screen.getByTestId("pmo-schedule-scroll");
    expect(screen.getAllByTestId("pmo-schedule-scroll")).toHaveLength(1);
    expect(within(schedule).getAllByRole("row")).toHaveLength(3);
    const taskRow = screen.getByRole("row", { name: /院务系统-开单处理/ });
    expect(within(schedule).getAllByRole("columnheader").map((cell) => cell.textContent)).toEqual([
      "task ID",
      "task",
      "External owner",
      "Start date",
      "Due date",
      "Workload",
      "Milestone",
      "Status",
    ]);
    const taskCells = within(taskRow).getAllByRole("cell");
    expect(taskCells).toHaveLength(8);
    expect(taskCells[0]).toHaveTextContent("TASK-FE-1");
    expect(taskCells[1]).toHaveTextContent("院务系统-开单处理");
    expect(taskCells[2]).toHaveTextContent("Feng Member");
    expect(taskCells[3]).toHaveTextContent("2026-07-24");
    expect(taskCells[4]).toHaveTextContent("2026-07-24");
    expect(taskCells[5]).toHaveTextContent("1");
    expect(taskCells[6]).toHaveTextContent("M4-开发-前端");
    expect(taskCells[7]).toHaveTextContent("未开始");
    expect(screen.getByRole("row", { name: /订单券码校验测试/ })).toHaveTextContent("unmatched");
    expect(schedule).toHaveClass("overflow-x-auto");
    expect(screen.getByRole("button", { name: "Use external SY-P-20260452 title" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Use external TASK-FE-1 title" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Use external TASK-FE-1 status" })).toBeInTheDocument();
    expect(within(taskRow).getByText("Local: 本地标题")).toBeInTheDocument();
  });

  it("renders each task once and falls back to scheme id and a safe description URL", () => {
    previewConfig();
    setRuns([makeRun({
      source_snapshot: {
        parent_requirement: {
          key: "LEGACY-1",
          title: "Legacy requirement",
          description: "https://example.test/prd",
          tasks: [],
        },
        child_requirements: [],
        tasks: [
          { task_id: "LEGACY-T1", scheme_id: "legacy-scheme", title: "Legacy task 1", source_status: "todo", status: "todo", owner: null, start_date: null, due_date: null, workload: null },
          { task_id: "LEGACY-T2", scheme_id: "legacy-scheme", title: "Legacy task 2", source_status: "done", status: "done", owner: null, start_date: null, due_date: null, workload: null },
        ],
      },
    })]);
    renderPage();

    expect(screen.getAllByText("legacy-scheme")).toHaveLength(2);
    expect(screen.getAllByRole("row", { name: /Legacy task/ })).toHaveLength(2);
    expect(screen.getByRole("link", { name: /PRD/ })).toHaveAttribute("href", "https://example.test/prd");
  });

  it("shows only source requirements referenced by the active filter", () => {
    previewConfig();
    setRuns([makeRun({
      source_snapshot: {
        parent_requirement: { key: "ROOT", title: "Unrelated root", tasks: [] },
        child_requirements: [
          {
            key: "CHILD-MATCH",
            title: "Matching child",
            tasks: [{ task_id: "TASK-MATCH", scheme_id: "M1", title: "Matching task", status: "todo" }],
          },
          { key: "CHILD-OTHER", title: "Unrelated child", tasks: [] },
        ],
        tasks: [],
      },
      diff: {
        entities: [{
          external_type: "task",
          external_key: "TASK-MATCH",
          action: "update",
          fields: { title: { external: "Matching task", local: "Old task", decision: "conflict" } },
        }],
      },
    })]);
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Conflicts" }));

    expect(screen.getByRole("heading", { name: "Matching child" })).toBeInTheDocument();
    expect(screen.getByRole("row", { name: /Matching task/ })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Unrelated root" })).toBeNull();
    expect(screen.queryByRole("heading", { name: "Unrelated child" })).toBeNull();
  });

  it("shows the filter empty state for a source preview with no matching rows", () => {
    previewConfig();
    setRuns([makeRun({
      source_snapshot: {
        parent_requirement: { key: "ROOT", title: "Root requirement", tasks: [] },
        child_requirements: [],
        tasks: [],
      },
    })]);
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Conflicts" }));

    expect(screen.getByText("Nothing matches this filter.")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Root requirement" })).toBeNull();
  });

  it("falls back to the stable external key when no title is present", () => {
    previewConfig();
    setRuns([makeRun({
      diff: {
        entities: [
          {
            external_type: "task",
            external_key: "task-d46ba80ebcc030c3",
            local_type: "issue",
            local_id: "issue-2",
            action: "create",
            fields: {
              title: {
                baseline_external: null,
                baseline_local: null,
                external: null,
                local: null,
                decision: "incoming",
              },
            },
          },
        ],
        warnings: [],
        summary: { creates: 1, incoming_fields: 1, local_only_fields: 0, converged_fields: 0, conflicts: 0, external_removed: 0, unresolved_assignees: 0 },
      },
    })]);
    renderPage();
    expect(screen.getAllByTestId("pmo-entity-name").map((node) => node.textContent)).toEqual(
      expect.arrayContaining(["task-d46ba80ebcc030c3"]),
    );
    expect(screen.getAllByTestId("pmo-entity-key").map((node) => node.textContent)).toEqual(
      expect.arrayContaining(["task-d46ba80ebcc030c3"]),
    );
  });

  it("shows an empty preview when there are no runs", () => {
    previewConfig();
    setRuns([]);
    renderPage();
    expect(screen.getByText("No preview yet")).toBeInTheDocument();
  });

  it("shows a compact failure banner with retry while keeping the page visible", () => {
    previewConfig();
    setRuns([makeRun({ status: "failed", error_code: "agent_unavailable", error_message: "agent unreachable" })]);
    renderPage();
    // Banner present.
    expect(screen.getByText("The last run failed")).toBeInTheDocument();
    expect(screen.getByText(/agent_unavailable/)).toBeInTheDocument();
    // The rest of the page (config context + tabs + filters) stays visible.
    expect(screen.getByRole("switch")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /Preview/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "All" })).toBeInTheDocument();
    // Retry triggers a new run.
    fireEvent.click(screen.getByRole("button", { name: "Retry run" }));
    expect(startRunMutate).toHaveBeenCalledWith(CONFIG.id, expect.anything());
  });

  it("shows a loading indicator for queued/running runs", () => {
    previewConfig();
    setRuns([makeRun({ status: "running" })]);
    renderPage();
    expect(screen.getByTestId("spinner")).toBeInTheDocument();
    expect(screen.getByText("Loading the latest preview...")).toBeInTheDocument();
  });

  it("shows the applied state with summary counts", () => {
    previewConfig();
    setRuns([makeRun({ status: "applied", applied_at: "2026-08-03T00:02:00Z", summary: { created: 1, incoming_fields: 2, conflicts_resolved: 1 } })]);
    renderPage();
    expect(screen.getByText("Applied")).toBeInTheDocument();
    const preview = previewPanel();
    expect(preview.textContent).toContain("1 create");
    expect(preview.textContent).toContain("2 incoming fields");
  });

  it("applies a preview with conflict resolutions", async () => {
    previewConfig();
    setRuns([makeRun()]);
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Use external EXT-P-001 title" }));
    fireEvent.click(screen.getByRole("button", { name: "Apply preview" }));
    await waitFor(() => expect(screen.getByText("Apply this preview?")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(applyRunMutate).toHaveBeenCalledWith(
      {
        runId: "run-1",
        resolutions: [
          { external_type: "requirement", external_key: "EXT-P-001", field: "title", choice: "external" },
        ],
      },
      expect.anything(),
    );
  });

  it("keeps the apply button disabled when conflicts are unresolved", () => {
    previewConfig();
    setRuns([makeRun()]);
    renderPage();
    const applyButton = screen.getByRole("button", { name: "Apply preview" });
    expect(applyButton).toBeDisabled();
  });

  it("hides the filter rows that do not match", () => {
    previewConfig();
    setRuns([makeRun()]);
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Conflicts" }));
    expect(screen.getAllByText("New external title").length).toBeGreaterThan(0);
    // The incoming-only status/task rows are filtered out under "Conflicts".
    expect(screen.queryByText("Incoming")).toBeNull();
    expect(screen.queryByText("New task title")).toBeNull();
  });

  it("includes every entity referenced by an unresolved owner", () => {
    previewConfig();
    setRuns([makeRun({
      source_snapshot: {
        parent_requirement: {
          key: "EXT-P-001",
          owner: { external_id: "EXT-U-001", display_name: "Example User" },
        },
        child_requirements: [],
        tasks: [
          {
            task_id: "TASK-001",
            owner: { external_id: "EXT-U-001", display_name: "Example User" },
          },
        ],
      },
      diff: {
        ...PREVIEW_DIFF,
        entities: [
          {
            external_type: "requirement",
            external_key: "EXT-P-001",
            action: "update",
            fields: { title: { external: "Requirement title", decision: "incoming" } },
          },
          {
            external_type: "task",
            external_key: "TASK-001",
            action: "update",
            fields: { title: { external: "Task title", decision: "incoming" } },
          },
        ],
      },
    })]);
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Unresolved owners" }));

    expect(screen.getAllByTestId("pmo-entity-name").map((node) => node.textContent)).toEqual([
      "Requirement title",
      "Task title",
    ]);
  });
});

describe("PMOConfigDetailPage assignee tab", () => {
  it("lists unresolved external owners with Agent selection", () => {
    previewConfig();
    setRuns([makeRun()]);
    renderPage();
    fireEvent.click(screen.getByRole("tab", { name: /Assignee mappings/ }));
    expect(screen.getByText("EXT-U-001")).toBeInTheDocument();
    const agentSelect = screen.getByLabelText(/Agent EXT-U-001/) as HTMLSelectElement;
    expect(agentSelect).toBeInTheDocument();
    fireEvent.change(agentSelect, { target: { value: "agent-1" } });
    expect(setMappingMutate).toHaveBeenCalledWith(
      { configId: CONFIG.id, externalKey: "EXT-U-001", agentId: "agent-1" },
      expect.anything(),
    );
  });

  it("lists recognized snapshot owners and selects the matching member by user id", () => {
    previewConfig();
    setRuns([makeRun({
      source_snapshot: {
        parent_requirement: {
          key: "EXT-P-001",
          owner: { external_id: "fengyujie", display_name: "Feng Yu Jie" },
        },
        child_requirements: [
          {
            key: "EXT-C-001",
            owner: { external_id: "fengyujie", display_name: "Feng Yu Jie" },
            tasks: [],
          },
        ],
        tasks: [],
      },
      diff: {
        ...PREVIEW_DIFF,
        entities: [
          {
            external_type: "requirement",
            external_key: "EXT-P-001",
            action: "update",
            fields: {
              assignee_id: {
                external: "agent-feng",
                local: null,
                decision: "incoming",
              },
            },
          },
          {
            external_type: "requirement",
            external_key: "EXT-C-001",
            action: "update",
            fields: {
              assignee_id: {
                external: "agent-feng",
                local: null,
                decision: "incoming",
              },
            },
          },
        ],
      },
    })]);
    renderPage();
    fireEvent.click(screen.getByRole("tab", { name: /Assignee mappings/ }));

    expect(screen.getByText("Feng Member")).toBeInTheDocument();
    const recognizedSelect = screen.getByLabelText(/Agent fengyujie/) as HTMLSelectElement;
    expect(recognizedSelect.closest("[data-testid='pmo-assignee-row']")).toHaveClass("grid");
    expect(recognizedSelect).toHaveValue("agent-feng");
    expect(screen.getByText("EXT-U-001")).toBeInTheDocument();
    expect(screen.getByLabelText(/Agent EXT-U-001/)).toHaveValue("");
    expect(screen.getAllByRole("option", { name: "Frontend Agent · Feng Member" })).toHaveLength(2);
    expect(screen.getAllByLabelText(/Agent (fengyujie|EXT-U-001)/).every((select) =>
      !select.querySelector('option[value="agent-unbound"]'))).toBe(true);
    expect(screen.getByText("EXT-P-001, EXT-C-001 · assignee_id")).toBeInTheDocument();
  });

  it("matches owners by external type and key", () => {
    previewConfig();
    setRuns([makeRun({
      source_snapshot: {
        parent_requirement: {
          key: "SHARED-001",
          owner: { external_id: "fengyujie", display_name: "Feng Yu Jie" },
        },
        child_requirements: [],
        tasks: [
          {
            task_id: "SHARED-001",
            owner: { external_id: "other-owner", display_name: "Other Owner" },
          },
        ],
      },
      diff: {
        ...PREVIEW_DIFF,
        warnings: [],
        entities: [
          {
            external_type: "requirement",
            external_key: "SHARED-001",
            action: "update",
            fields: { assignee_id: { external: "agent-feng", decision: "incoming" } },
          },
          {
            external_type: "task",
            external_key: "SHARED-001",
            action: "update",
            fields: { assignee_id: { external: "agent-other", decision: "incoming" } },
          },
        ],
      },
    })]);
    renderPage();
    fireEvent.click(screen.getByRole("tab", { name: /Assignee mappings/ }));

    expect(screen.getByLabelText(/Agent fengyujie/)).toHaveValue("agent-feng");
    expect(screen.getByLabelText(/Agent other-owner/)).toHaveValue("agent-other");
  });

  it("refreshes a resolved select when the latest run changes", () => {
    previewConfig();
    const snapshot = {
      parent_requirement: {
        key: "EXT-P-001",
        owner: { external_id: "fengyujie", display_name: "Feng Yu Jie" },
      },
      child_requirements: [],
      tasks: [],
    };
    const makeResolvedRun = (agentId: string) => makeRun({
      source_snapshot: snapshot,
      diff: {
        ...PREVIEW_DIFF,
        warnings: [],
        entities: [{
          external_type: "requirement",
          external_key: "EXT-P-001",
          action: "update",
          fields: { assignee_id: { external: agentId, decision: "incoming" } },
        }],
      },
    });
    setRuns([makeResolvedRun("agent-feng")]);
    const rendered = renderPage();
    fireEvent.click(screen.getByRole("tab", { name: /Assignee mappings/ }));
    expect(screen.getByLabelText(/Agent fengyujie/)).toHaveValue("agent-feng");

    setRuns([makeResolvedRun("agent-other")]);
    rendered.rerender(pageElement());
    expect(screen.getByLabelText(/Agent fengyujie/)).toHaveValue("agent-other");
  });
});

describe("PMOConfigDetailPage header controls", () => {
  it("keeps the schedule switch disabled until last_applied_at exists", () => {
    previewConfig({ last_applied_at: null });
    setRuns([makeRun()]);
    renderPage();
    const scheduleSwitch = screen.getByRole("switch");
    expect(scheduleSwitch).toBeDisabled();
    expect(screen.getByText("Enable the schedule after applying your first preview.")).toBeInTheDocument();
  });

  it("enables the schedule once last_applied_at exists and calls update", () => {
    previewConfig({ last_applied_at: "2026-08-02T00:00:00Z" });
    setRuns([makeRun()]);
    renderPage();
    const scheduleSwitch = screen.getByRole("switch");
    expect(scheduleSwitch).not.toBeDisabled();
    fireEvent.click(scheduleSwitch);
    expect(updateConfigMutate).toHaveBeenCalledWith(
      expect.objectContaining({ schedule_enabled: true, id: CONFIG.id }),
      expect.anything(),
    );
  });

  it("triggers a manual sync from the header", () => {
    previewConfig();
    setRuns([]);
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Sync now" }));
    expect(startRunMutate).toHaveBeenCalledWith(CONFIG.id, expect.anything());
  });

  it("saves the external root key on blur", () => {
    previewConfig();
    setRuns([makeRun()]);
    renderPage();
    const rootKeyInput = screen.getByLabelText("External root key") as HTMLInputElement;
    fireEvent.change(rootKeyInput, { target: { value: "EXT-P-002" } });
    fireEvent.blur(rootKeyInput);
    expect(updateConfigMutate).toHaveBeenCalledWith(
      expect.objectContaining({ root_external_key: "EXT-P-002" }),
      expect.anything(),
    );
  });

  it("locks the root key editor once the config has been applied", () => {
    previewConfig({ last_applied_at: "2026-08-02T00:00:00Z" });
    setRuns([makeRun()]);
    renderPage();
    const rootKeyInput = screen.getByLabelText("External root key") as HTMLInputElement;
    expect(rootKeyInput).toBeDisabled();
  });
});

describe("PMOConfigDetailPage history tab", () => {
  it("shows the empty history state when there are no runs", () => {
    previewConfig();
    setRuns([]);
    renderPage();
    fireEvent.click(screen.getByRole("tab", { name: /Run history/ }));
    expect(screen.getByText("No runs yet")).toBeInTheDocument();
  });

  it("renders run rows with status, trigger and timestamp", () => {
    previewConfig();
    setRuns([makeRun(), makeRun({ id: "run-2", status: "failed", error_code: "sync_error", error_message: "boom" })]);
    renderPage();
    fireEvent.click(screen.getByRole("tab", { name: /Run history/ }));
    expect(screen.getByText("Preview ready")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();
    expect(screen.getAllByText("Manual").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/sync_error/)).toBeInTheDocument();
  });

  it("shows a transcript button for runs with an agent task", () => {
    previewConfig();
    setRuns([makeRun({ agent_task_id: "task-1", status: "running" })]);
    renderPage();
    fireEvent.click(screen.getByRole("tab", { name: /Run history/ }));

    expect(screen.getByRole("button", { name: "View execution log" })).toBeInTheDocument();
    expect(transcriptButtonProps).toHaveBeenCalledWith(
      expect.objectContaining({
        task: expect.objectContaining({ id: "task-1", agent_id: "agent-1", status: "running" }),
        agentName: "Example Agent",
        isLive: true,
        title: "View execution log",
      }),
    );
  });

  it("does not show a transcript button without an agent task", () => {
    previewConfig();
    setRuns([makeRun({ agent_task_id: null })]);
    renderPage();
    fireEvent.click(screen.getByRole("tab", { name: /Run history/ }));

    expect(screen.queryByRole("button", { name: "View execution log" })).toBeNull();
  });
});
