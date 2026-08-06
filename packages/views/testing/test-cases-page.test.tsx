import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithI18n } from "../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { TestCasesPage } from "./test-cases-page";

const mocks = vi.hoisted(() => ({
  projects: [{ id: "p-1", title: "Billing" }],
  cases: [] as unknown[],
  modules: [] as unknown[],
  createCase: vi.fn(),
  createGenerationJob: vi.fn(),
  approveCase: vi.fn(),
  viewState: {
    filters: {
      statuses: [] as string[],
      priorities: [] as string[],
      caseTypes: [] as string[],
      origins: [] as string[],
    },
    hiddenColumns: [] as string[],
    projectId: "p-1",
    module: null as string | null,
    setModule: vi.fn(),
    setFilter: vi.fn(),
    toggleFilter: vi.fn(),
    clearFilters: vi.fn(),
    setProjectId: vi.fn(),
    toggleColumn: vi.fn(),
  },
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { queryKey?: readonly unknown[] }) => {
    const key = options.queryKey?.[0];
    if (key === "projects") return { data: mocks.projects, isLoading: false };
    if (key === "test-cases") {
      const kind = options.queryKey?.[2];
      if (kind === "modules") return { data: mocks.modules, isLoading: false };
      return { data: mocks.cases, isLoading: false };
    }
    return { data: [], isLoading: false };
  },
}));

vi.mock("@multica/core/projects/queries", () => ({
  projectListOptions: () => ({ queryKey: ["projects"] }),
}));

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    testCaseDetail: (ref: string) => `/acme/tests/${ref}`,
    testGenerationJobDetail: (id: string) => `/acme/tests/jobs/${id}`,
    testPlans: () => "/acme/tests/plans",
  }),
}));

vi.mock("@multica/core/testing", () => {
  const store = (selector?: (s: typeof mocks.viewState) => unknown) =>
    selector ? selector(mocks.viewState) : mocks.viewState;
  store.getState = () => mocks.viewState;
  return {
    TEST_CASE_ORIGINS: ["ai", "human"],
    TEST_CASE_PRIORITIES: ["p0", "p1", "p2", "p3"],
    TEST_CASE_STATUSES: ["draft", "active", "deprecated"],
    TEST_CASE_PRIORITY_TONE: {},
    TEST_CASE_STATUS_TONE: {},
    testCaseListOptions: () => ({ queryKey: ["test-cases", "ws-1", "list"] }),
    testCaseModulesOptions: () => ({ queryKey: ["test-cases", "ws-1", "modules"] }),
    useApproveTestCase: () => ({ mutateAsync: mocks.approveCase, isPending: false }),
    useCreateTestCase: () => ({ mutateAsync: mocks.createCase, isPending: false }),
    useCreateTestGenerationJob: () => ({ mutateAsync: mocks.createGenerationJob, isPending: false }),
    useTestCaseViewStore: store,
  };
});

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function makeAdapter(overrides: Partial<NavigationAdapter> = {}): NavigationAdapter {
  return {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/tests",
    searchParams: new URLSearchParams(),
    getShareableUrl: (p) => p,
    ...overrides,
  };
}

function renderPage(adapter = makeAdapter()) {
  renderWithI18n(
    <NavigationProvider value={adapter}>
      <TestCasesPage />
    </NavigationProvider>,
  );
  return adapter;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.cases = [];
  mocks.modules = [];
});

// Both of these were decorative before: the "new case" button had no handler
// at all, and nothing in the app ever navigated to the generation job page or
// called the create-job mutation. The feature existed only from the API down.
describe("TestCasesPage entry points", () => {
  it("creates a generation job for the selected project and opens it", async () => {
    mocks.createGenerationJob.mockResolvedValue({ id: "job-1" });
    const adapter = renderPage();

    const buttons = await screen.findAllByRole("button");
    const generate = buttons.find((b) => b.textContent?.match(/AI|生成|Generate/));
    expect(generate, "the page must offer a way to start a generation run").toBeTruthy();
    await userEvent.click(generate!);

    expect(mocks.createGenerationJob).toHaveBeenCalledWith({ project_id: "p-1" });
    expect(adapter.push).toHaveBeenCalledWith("/acme/tests/jobs/job-1");
  });

  it("creates a case and opens its detail page", async () => {
    mocks.createCase.mockResolvedValue({ id: "c-1", key: "TC-1" });
    const adapter = renderPage();

    const buttons = await screen.findAllByRole("button");
    const create = buttons.find((b) => b.textContent?.match(/新建|New|作成|만들/));
    expect(create, "the page must offer a way to create a case").toBeTruthy();
    await userEvent.click(create!);

    expect(mocks.createCase).toHaveBeenCalledWith(
      expect.objectContaining({ project_id: "p-1" }),
    );
    expect(adapter.push).toHaveBeenCalledWith("/acme/tests/TC-1");
  });

  it("does not navigate when creating the generation job fails", async () => {
    mocks.createGenerationJob.mockRejectedValue(new Error("boom"));
    const adapter = renderPage();

    const buttons = await screen.findAllByRole("button");
    const generate = buttons.find((b) => b.textContent?.match(/AI|生成|Generate/));
    await userEvent.click(generate!);

    expect(adapter.push).not.toHaveBeenCalled();
  });
});

// The plans surface shipped with only its own breadcrumbs pointing at it, which
// helps nobody who is not already there. This asserts the way in.
it("links to the test plans surface", async () => {
  const adapter = makeAdapter();
  renderPage(adapter);

  const buttons = await screen.findAllByRole("button");
  const plans = buttons.find((b) => b.textContent?.match(/计划|Plans|プラン|계획/));
  expect(plans, "the cases page must offer a way to reach test plans").toBeTruthy();
  await userEvent.click(plans!);

  expect(adapter.push).toHaveBeenCalledWith("/acme/tests/plans");
});
