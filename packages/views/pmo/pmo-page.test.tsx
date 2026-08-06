/**
 * PMOPage state coverage.
 *
 * The page is tested through real DOM assertions with only the data and
 * primitive layers mocked (hooks + UI components). No framework routing
 * mocks — navigation goes through the @multica/core modals store, and i18n
 * resolves through the real RESOURCES bundle.
 */
import React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithI18n } from "../test/i18n";
import type { PMOConfig, PMORun } from "@multica/core/types";
import { PMOPage } from "./pmo-page";

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

const APPLIED_CONFIG: PMOConfig = {
  ...CONFIG,
  last_applied_at: "2026-08-02T00:00:00Z",
};
void APPLIED_CONFIG;

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
      external_type: "assignee",
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
const createConfigMutate = vi.fn();
const deleteConfigMutate = vi.fn();

vi.mock("@multica/core/pmo/mutations", () => ({
  useStartPMORun: () => ({ mutate: startRunMutate, isPending: false }),
  useApplyPMORun: () => ({ mutate: applyRunMutate, isPending: false }),
  useSetPMOAssigneeMapping: () => ({ mutate: setMappingMutate, isPending: false }),
  useUpdatePMOConfig: () => ({ mutate: updateConfigMutate, isPending: false }),
  useCreatePMOConfig: () => ({ mutate: createConfigMutate, isPending: false }),
  useDeletePMOConfig: () => ({ mutate: deleteConfigMutate, isPending: false }),
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

// Callable-store pattern: tests drive getState().modal / open() through the
// hoisted store; the page only calls getState().open(...) as the plan snippet
// requires.
const modalState = { modal: null as string | null, open: vi.fn((name: string) => { modalState.modal = name; }) };
vi.mock("@multica/core/modals", () => ({
  useModalStore: { getState: () => modalState },
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
    if (key === "members") return { data: [{ id: "member-1", name: "Example Member", user_id: "user-1" }] };
    if (key === "agents") return { data: [{ id: "agent-1", name: "Example Agent", archived_at: null, runtime_bound: true }] };
    return { data: [] };
  },
}));

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

// Keep the ui primitives as light DOM so the state logic is what is under test.
vi.mock("@multica/ui/components/ui/button", () => ({
  Button: ({ children, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement> & { children?: React.ReactNode }) => (
    <button {...props}>{children}</button>
  ),
}));
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
      // eslint-disable-next-line react/jsx-no-undef
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
vi.mock("@multica/ui/components/ui/select", () => ({
  Select: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  SelectTrigger: ({ children, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button type="button" {...props}>{children}</button>
  ),
  SelectValue: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  SelectContent: () => null,
  SelectItem: () => null,
}));
vi.mock("@multica/ui/components/ui/dialog", () => ({
  Dialog: ({ open, children }: { open?: boolean; children?: React.ReactNode }) => (open ? <>{children}</> : null),
  DialogContent: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  DialogHeader: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  DialogFooter: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  DialogDescription: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));
vi.mock("@multica/ui/components/ui/tabs", () => ({
  Tabs: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  TabsList: ({ children }: { children?: React.ReactNode }) => <div role="tablist">{children}</div>,
  TabsTrigger: ({ value, children, onClick }: { value: string; children?: React.ReactNode; onClick?: () => void }) => (
    <button type="button" role="tab" aria-selected={value === "preview"} onClick={onClick}>{children}</button>
  ),
  TabsContent: ({ value, children }: { value: string; children?: React.ReactNode }) => (
    <div role="tabpanel" data-value={value}>{children}</div>
  ),
}));
vi.mock("@multica/ui/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ render }: { render?: React.ReactElement }) => (render ?? null),
  TooltipContent: () => null,
}));
vi.mock("../layout/collection-page", () => ({
  CollectionPageHeader: ({ children, actions }: { children?: React.ReactNode; actions?: React.ReactNode }) => (
    <header>{children}{actions}</header>
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

const renderPage = () => renderWithI18n(<PMOPage />);

// The Tabs mock keeps every panel mounted (settings-page test pattern), so
// statuses and counts can appear in both Preview and History — scope the
// preview-specific assertions to the Preview panel.
const previewPanel = () =>
  document.querySelector<HTMLElement>('[role="tabpanel"][data-value="preview"]') as HTMLElement;

function previewConfig(overrides: Partial<PMOConfig> = {}) {
  queryState.configs = { data: [{ ...CONFIG, ...overrides }], isPending: false, isError: false, isSuccess: true };
}

function loadingConfigs() {
  queryState.configs = { data: undefined, isPending: true, isError: false, isSuccess: false };
}

function errorConfigs() {
  queryState.configs = { data: undefined, isPending: false, isError: true, isSuccess: false };
}

function setRuns(runs: PMORun[]) {
  queryState.runs = { data: runs, isPending: false, isError: false, isSuccess: true };
}

beforeEach(() => {
  queryState.configs = { data: [], isPending: false, isError: false, isSuccess: false };
  queryState.runs = { data: [], isPending: false, isError: false, isSuccess: false };
  modalState.modal = null;
  modalState.open.mockClear();
  startRunMutate.mockClear();
  applyRunMutate.mockClear();
  setMappingMutate.mockClear();
  updateConfigMutate.mockClear();
  createConfigMutate.mockClear();
  deleteConfigMutate.mockClear();
});

describe("PMOPage loading and empty states", () => {
  it("renders skeletons while configs load", () => {
    loadingConfigs();
    const { container } = renderPage();
    expect(container.querySelectorAll('[data-testid="skeleton"]').length).toBeGreaterThan(0);
  });

  it("shows the empty config state when the workspace has no configs", () => {
    queryState.configs = { data: [], isPending: false, isError: false, isSuccess: true };
    renderPage();
    expect(screen.getByText("No sync config yet")).toBeInTheDocument();
  });

  it("shows the error state when the config list fails", () => {
    errorConfigs();
    renderPage();
    expect(screen.getByText("Failed to load sync configs.")).toBeInTheDocument();
  });
});

describe("PMOPage preview tab", () => {
  it("renders a preview_ready manual run's field-level diff", () => {
    previewConfig();
    setRuns([makeRun()]);
    renderPage();
    // EXT-P-001 shows both in the diff rows and the assignee references.
    expect(screen.getAllByText("EXT-P-001").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("New external title")).toBeInTheDocument();
    expect(screen.getByText("New local title")).toBeInTheDocument();
    expect(screen.getByText("TASK-001")).toBeInTheDocument();
  });

  it("shows an empty preview when there are no runs", () => {
    previewConfig();
    setRuns([]);
    renderPage();
    expect(screen.getByText("No preview yet")).toBeInTheDocument();
  });

  it("renders a failed run with a retry action", () => {
    previewConfig();
    setRuns([makeRun({ status: "failed", error_code: "agent_unavailable", error_message: "agent unreachable" })]);
    renderPage();
    expect(screen.getByText("The last run failed")).toBeInTheDocument();
    const failures = screen.getAllByText((content) => content.includes("agent_unavailable"));
    expect(failures.length).toBeGreaterThan(0);
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
    // "Applied" appears both in the preview status line and the history tab
    // badge — the tabs mock keeps every panel mounted.
    expect(screen.getAllByText("Applied").length).toBeGreaterThanOrEqual(1);
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
    expect(screen.getByText("New external title")).toBeInTheDocument();
    // The incoming-only status row is filtered out under "Conflicts".
    expect(screen.queryByText("Incoming")).toBeNull();
  });

  it("opens existing create workflows", async () => {
    previewConfig();
    setRuns([makeRun()]);
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "New project" }));
    expect(modalState.modal).toBe("create-project");
    fireEvent.click(screen.getByRole("button", { name: /New issue/ }));
    expect(modalState.modal).toBe("create-issue");
  });
});

describe("PMOPage assignee tab", () => {
  it("lists unresolved external owners with member selection", () => {
    previewConfig();
    setRuns([makeRun()]);
    renderPage();
    fireEvent.click(screen.getByRole("tab", { name: /Assignee mappings/ }));
    expect(screen.getByText("Example User")).toBeInTheDocument();
    expect(screen.getByText(/EXT-U-001/)).toBeInTheDocument();
    const memberSelect = screen.getByLabelText(/Workspace member EXT-U-001/) as HTMLSelectElement;
    expect(memberSelect).toBeInTheDocument();
    fireEvent.change(memberSelect, { target: { value: "member-1" } });
    expect(setMappingMutate).toHaveBeenCalledWith(
      { configId: CONFIG.id, externalKey: "EXT-U-001", memberId: "member-1" },
      expect.anything(),
    );
  });
});

describe("PMOPage header controls", () => {
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

  it("triggers a manual sync", () => {
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
    const rootKeyInput = screen.getByLabelText("External root key");
    fireEvent.change(rootKeyInput, { target: { value: "EXT-P-002" } });
    fireEvent.blur(rootKeyInput);
    expect(updateConfigMutate).toHaveBeenCalledWith(
      expect.objectContaining({ root_external_key: "EXT-P-002" }),
      expect.anything(),
    );
  });
});
