import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import type { Issue } from "@multica/core/types";
import { toast } from "sonner";
import enCommon from "../../locales/en/common.json";
import enIssues from "../../locales/en/issues.json";
import { BatchActionToolbar } from "./batch-action-toolbar";

const TEST_RESOURCES = { en: { common: enCommon, issues: enIssues } };
const mockSelectedIds = vi.hoisted(() => new Set<string>());
const mockBatchUpdate = vi.hoisted(() => vi.fn());
const mockBatchDelete = vi.hoisted(() => vi.fn());
const mockSurfaceActions = vi.hoisted(() => ({
  batchUpdate: vi.fn(),
  batchDelete: vi.fn(),
  isPending: false,
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/issues/stores/selection-store", () => ({
  useIssueSelectionStore: Object.assign(
    (selector?: any) => {
      const state = {
        selectedIds: mockSelectedIds,
        toggle: vi.fn(),
        select: vi.fn(),
        deselect: vi.fn(),
        clear: vi.fn(),
      };
      return selector ? selector(state) : state;
    },
    {
      getState: () => ({
        selectedIds: mockSelectedIds,
        toggle: vi.fn(),
        select: vi.fn(),
        deselect: vi.fn(),
        clear: vi.fn(),
      }),
    },
  ),
}));

vi.mock("../surface/actions-context", () => ({
  useIssueSurfaceActionsOptional: () => null,
}));

vi.mock("@multica/core/issues/mutations", () => ({
  useBatchUpdateIssues: () => ({ mutateAsync: mockBatchUpdate, isPending: false }),
  useBatchDeleteIssues: () => ({ mutateAsync: mockBatchDelete, isPending: false }),
}));

vi.mock("../../i18n", () => ({
  useT: () => ({ t: (fn: (f: any) => string) => fn({}) }),
}));

vi.mock("./pickers", () => ({
  StatusPicker: ({ status, onUpdate }: { status: string | null; onUpdate: (updates: any) => void }) => (
    <div>
      <button type="button" onClick={() => onUpdate({ status: "done" })}>
        {status ?? "__none__"}
      </button>
    </div>
  ),
  PriorityPicker: ({ priority, onUpdate }: { priority: string | null; onUpdate: (updates: any) => void }) => (
    <button type="button" onClick={() => onUpdate({ priority: "high" })}>{priority ?? "__none__"}</button>
  ),
  AssigneePicker: ({ assigneeType, assigneeId, mixed }: { assigneeType: string | null; assigneeId: string | null; mixed?: boolean }) => (
    <div data-testid="assignee-picker" data-assignee-type={assigneeType ?? "__null__"} data-assignee-id={assigneeId ?? "__null__"} data-mixed={String(Boolean(mixed))} />
  ),
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

function makeIssue(overrides: Partial<Issue> = {}): Issue {
  return {
    id: "issue-1",
    workspace_id: "ws-1",
    number: 1,
    identifier: "MUL-1",
    title: "Issue 1",
    description: null,
    status: "todo",
    priority: "none",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "user-1",
    parent_issue_id: null,
    project_id: null,
    position: 1,
    stage: null,
    start_date: null,
    due_date: null,
    metadata: {},
    properties: {},
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function renderToolbar() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });

  return render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <QueryClientProvider client={queryClient}>
        <BatchActionToolbar placement="inline" issues={[makeIssue(), makeIssue({ id: "issue-2" })]} />
      </QueryClientProvider>
    </I18nProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockSelectedIds.clear();
  mockSelectedIds.add("issue-1");
  mockSelectedIds.add("issue-2");
  mockSelectedIds.add("issue-3");
  mockBatchUpdate.mockResolvedValue({ updated: 3 });
  mockBatchDelete.mockResolvedValue({ deleted: 3 });
});

describe("BatchActionToolbar", () => {
  it("shows a partial success toast when the batch update skips selected issues", async () => {
    mockBatchUpdate.mockResolvedValue({
      updated: 1,
      skipped: [
        {
          issue_id: "issue-2",
          identifier: "TES-2",
          title: "UI设计",
          reason:
            "UI design issue requires completed UI restore or raw design fallback handoff before completion",
        },
      ],
    });

    renderToolbar();

    fireEvent.click(screen.getByRole("button", { name: "Status" }));
    fireEvent.click(await screen.findByRole("button", { name: /Done/i }));

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith("Updated 1 issue(s); skipped TES-2 UI设计");
    });
  });

  it("falls back to skipped count when partial batch update has no skipped details", async () => {
    mockBatchUpdate.mockResolvedValue({ updated: 1 });

    renderToolbar();

    fireEvent.click(screen.getByRole("button", { name: "Status" }));
    fireEvent.click(await screen.findByRole("button", { name: /Done/i }));

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith("Updated 1 issue(s); 2 skipped");
    });
  });
});
