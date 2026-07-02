import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enIssues from "../../locales/en/issues.json";
import { toast } from "sonner";

const TEST_RESOURCES = { en: { common: enCommon, issues: enIssues } };
const mockSelectedIds = vi.hoisted(() => new Set<string>());
const mockBatchUpdate = vi.hoisted(() => vi.fn());
const mockBatchDelete = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector?: any) => {
      const state = { user: { id: "user-1", email: "test@test.com", name: "Test User" } };
      return selector ? selector(state) : state;
    },
    { getState: () => ({ user: { id: "user-1", email: "test@test.com", name: "Test User" } }) },
  ),
}));

vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({
    queryKey: ["workspaces", "ws-1", "members"],
    queryFn: () => Promise.resolve([]),
  }),
  agentListOptions: () => ({
    queryKey: ["workspaces", "ws-1", "agents"],
    queryFn: () => Promise.resolve([]),
  }),
  squadListOptions: () => ({
    queryKey: ["workspaces", "ws-1", "squads"],
    queryFn: () => Promise.resolve([]),
  }),
  assigneeFrequencyOptions: () => ({
    queryKey: ["workspaces", "ws-1", "assignee-frequency"],
    queryFn: () => Promise.resolve([]),
  }),
}));

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({
    getActorName: () => "Unknown",
  }),
}));

vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: ({ actorType, actorId }: any) => (
    <span data-testid="actor-avatar">
      {actorType}:{actorId}
    </span>
  ),
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

vi.mock("@multica/core/api", () => ({
  api: {
    batchUpdateIssues: (...args: unknown[]) => mockBatchUpdate(...args),
    batchDeleteIssues: (...args: unknown[]) => mockBatchDelete(...args),
  },
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

import { BatchActionToolbar } from "./batch-action-toolbar";

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
        <BatchActionToolbar placement="inline" />
      </QueryClientProvider>
    </I18nProvider>,
  );
}

describe("BatchActionToolbar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSelectedIds.clear();
    mockSelectedIds.add("issue-1");
    mockSelectedIds.add("issue-2");
    mockSelectedIds.add("issue-3");
    mockBatchUpdate.mockResolvedValue({ updated: 3 });
    mockBatchDelete.mockResolvedValue({ deleted: 3 });
  });

  it("shows a partial success toast when the batch update skips selected issues", async () => {
    mockBatchUpdate.mockResolvedValue({
      updated: 1,
      skipped: [{
        issue_id: "issue-2",
        identifier: "TES-2",
        title: "UI设计",
        reason: "UI design issue requires completed UI restore or raw design fallback handoff before completion",
      }],
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
