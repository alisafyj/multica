import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithI18n } from "../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { TestPlansPage } from "./test-plans-page";

const mocks = vi.hoisted(() => ({
  plans: [] as unknown[],
  createPlan: vi.fn(),
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { queryKey?: readonly unknown[] }) => {
    const key = options.queryKey?.[0];
    if (key === "test-plans") return { data: mocks.plans, isLoading: false };
    return { data: [], isLoading: false };
  },
}));

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    testPlans: () => "/acme/tests/plans",
    testPlanDetail: (id: string) => `/acme/tests/plans/${id}`,
    testRunDetail: (id: string) => `/acme/tests/runs/${id}`,
  }),
}));

vi.mock("@multica/core/testing", () => ({
  testPlanListOptions: () => ({ queryKey: ["test-plans", "ws-1", "list"] }),
  useCreateTestPlan: () => ({ mutateAsync: mocks.createPlan, isPending: false }),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function makeAdapter(overrides: Partial<NavigationAdapter> = {}): NavigationAdapter {
  return {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/tests/plans",
    searchParams: new URLSearchParams(),
    getShareableUrl: (p) => p,
    ...overrides,
  };
}

function renderPage(adapter = makeAdapter()) {
  renderWithI18n(
    <NavigationProvider value={adapter}>
      <TestPlansPage />
    </NavigationProvider>,
  );
  return adapter;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.plans = [];
});

describe("TestPlansPage entry points", () => {
  it("renders the page with a create plan button", async () => {
    renderPage();
    const buttons = await screen.findAllByRole("button");
    const create = buttons.find((b) => b.textContent?.match(/New|新建|作成|새/));
    expect(create, "the page must offer a way to create a plan").toBeTruthy();
  });

  it("creates a plan and navigates to its detail page", async () => {
    mocks.createPlan.mockResolvedValue({ id: "plan-1", title: "Sprint 1" });
    const adapter = renderPage();

    const buttons = await screen.findAllByRole("button");
    const create = buttons.find((b) => b.textContent?.match(/New|新建|作成|새/));
    await userEvent.click(create!);

    expect(mocks.createPlan).toHaveBeenCalled();
    expect(adapter.push).toHaveBeenCalledWith("/acme/tests/plans/plan-1");
  });

  it("does not navigate when creation fails", async () => {
    mocks.createPlan.mockRejectedValue(new Error("boom"));
    const adapter = renderPage();

    const buttons = await screen.findAllByRole("button");
    const create = buttons.find((b) => b.textContent?.match(/New|新建|作成|새/));
    await userEvent.click(create!);

    expect(adapter.push).not.toHaveBeenCalled();
  });

  it("shows existing plans and navigates to plan detail on click", async () => {
    mocks.plans = [
      { id: "plan-1", title: "Sprint 1", status: "active", created_at: "2024-01-01T00:00:00Z" },
    ];
    const adapter = renderPage();
    const link = await screen.findByText("Sprint 1");
    await userEvent.click(link);
    expect(adapter.push).toHaveBeenCalledWith("/acme/tests/plans/plan-1");
  });
});
