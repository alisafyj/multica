import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithI18n } from "../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { TestRunDetail } from "./test-run-detail";

const mocks = vi.hoisted(() => ({
  run: null as unknown,
  cases: [] as unknown[],
  agents: [] as unknown[],
  startRun: vi.fn(),
  abortRun: vi.fn(),
  retryRun: vi.fn(),
  dispatchRun: vi.fn(),
  updateCaseResult: vi.fn(),
  openDefect: vi.fn(),
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { queryKey?: readonly unknown[] }) => {
    const key = options.queryKey?.[0];
    const kind = options.queryKey?.[2];
    if (key === "test-runs" && kind === "detail") return { data: mocks.run, isLoading: false };
    if (key === "test-runs" && kind === "cases") return { data: mocks.cases, isLoading: false };
    if (key === "agents") return { data: mocks.agents, isLoading: false };
    return { data: [], isLoading: false };
  },
}));

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    testPlans: () => "/acme/tests/plans",
    testPlanDetail: (id: string) => `/acme/tests/plans/${id}`,
    testRunDetail: (id: string) => `/acme/tests/runs/${id}`,
    issueDetail: (id: string) => `/acme/issues/${id}`,
  }),
}));

vi.mock("@multica/core/testing", () => ({
  testRunDetailOptions: () => ({ queryKey: ["test-runs", "ws-1", "detail"] }),
  testRunCasesOptions: () => ({ queryKey: ["test-runs", "ws-1", "cases"] }),
  useStartTestRun: () => ({ mutateAsync: mocks.startRun, isPending: false }),
  useAbortTestRun: () => ({ mutateAsync: mocks.abortRun, isPending: false }),
  useRetryTestRun: () => ({ mutateAsync: mocks.retryRun, isPending: false }),
  useDispatchTestRun: () => ({ mutateAsync: mocks.dispatchRun, isPending: false }),
  useUpdateTestRunCaseResult: () => ({ mutate: mocks.updateCaseResult, isPending: false }),
  useOpenTestRunCaseDefect: () => ({ mutateAsync: mocks.openDefect, isPending: false }),
}));

vi.mock("@multica/core/workspace/queries", () => ({
  agentListOptions: () => ({ queryKey: ["agents"] }),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function makeRun(overrides = {}) {
  return {
    id: "run-1",
    workspace_id: "ws-1",
    project_id: "proj-1",
    plan_id: null,
    title: "Sprint Run",
    executor_type: "member",
    executor_id: "user-1",
    agent_task_id: null,
    environment: "staging",
    build_ref: "abc123",
    capability_binding: {},
    status: "pending",
    source_run_id: null,
    retry_scope: null,
    error: null,
    started_at: null,
    completed_at: null,
    created_by: null,
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
    result_counts: {},
    ...overrides,
  };
}

function makeAdapter(overrides: Partial<NavigationAdapter> = {}): NavigationAdapter {
  return {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/tests/runs/run-1",
    searchParams: new URLSearchParams(),
    getShareableUrl: (p) => p,
    ...overrides,
  };
}

function renderPage(adapter = makeAdapter()) {
  renderWithI18n(
    <NavigationProvider value={adapter}>
      <TestRunDetail runId="run-1" />
    </NavigationProvider>,
  );
  return adapter;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.run = makeRun();
  mocks.cases = [];
  mocks.agents = [];
});

describe("TestRunDetail entry points", () => {
  it("renders the run title", async () => {
    renderPage();
    // The title appears in both the breadcrumb leaf and the meta section —
    // use findAllByText to accept multiple matches.
    const titles = await screen.findAllByText("Sprint Run");
    expect(titles.length).toBeGreaterThan(0);
  });

  it("shows a start button when run is pending and calls startRun on click", async () => {
    mocks.startRun.mockResolvedValue({});
    renderPage();

    const buttons = await screen.findAllByRole("button");
    const start = buttons.find((b) => b.textContent?.match(/Start|开始|開始|시작/));
    expect(start, "pending run must have a start button").toBeTruthy();
    await userEvent.click(start!);
    expect(mocks.startRun).toHaveBeenCalledWith("run-1");
  });

  it("shows retry-failed button when run is completed", async () => {
    mocks.run = makeRun({ status: "completed" });
    renderPage();

    const buttons = await screen.findAllByRole("button");
    const retry = buttons.find((b) =>
      b.textContent?.match(/Retry failed|仅重跑|失敗のみ|실패만/),
    );
    expect(retry, "completed run must have a retry-failed button").toBeTruthy();
  });

  it("shows dispatch section when run is pending (agent executor) and navigates to run after dispatch", async () => {
    mocks.run = makeRun({ status: "pending", executor_type: "agent" });
    mocks.agents = [{ id: "agent-1", name: "Bot", status: "active", runtime_id: "rt-1", archived_at: null }];
    mocks.dispatchRun.mockResolvedValue({ test_run: makeRun({ id: "run-1", status: "running" }) });
    const adapter = renderPage();

    const buttons = await screen.findAllByRole("button");
    const dispatch = buttons.find((b) =>
      b.textContent?.match(/Dispatch|派发|派遣|전달/),
    );
    expect(dispatch, "pending run must have a dispatch button").toBeTruthy();
    await userEvent.click(dispatch!);
    expect(mocks.dispatchRun).toHaveBeenCalled();
    expect(adapter.push).toHaveBeenCalledWith("/acme/tests/runs/run-1");
  });

  it("shows blocked-dispatch message when dispatch returns 409", async () => {
    mocks.run = makeRun({ status: "pending", executor_type: "agent" });
    mocks.agents = [{ id: "agent-1", name: "Bot", status: "active", runtime_id: "rt-1", archived_at: null }];
    const blockedError = Object.assign(new Error("blocked"), {
      status: 409,
      body: { missing_kind: "browser", message: "No browser capability" },
    });
    mocks.dispatchRun.mockRejectedValue(blockedError);
    const { toast } = await import("sonner");
    renderPage();

    const buttons = await screen.findAllByRole("button");
    const dispatch = buttons.find((b) => b.textContent?.match(/Dispatch|派发|派遣|전달/));
    await userEvent.click(dispatch!);

    expect(toast.error).toHaveBeenCalledWith(
      expect.stringMatching(/browser/),
    );
  });
});

describe("TestRunDetail case result", () => {
  beforeEach(() => {
    mocks.run = makeRun({ status: "running" });
    mocks.cases = [
      {
        id: "rc-1",
        run_id: "run-1",
        test_case_id: "tc-1",
        case_snapshot: { title: "Login flow", key: "TC-1" },
        position: 0,
        result: "pending",
        notes: "",
        evidence: [],
        step_results: [],
        duration_ms: null,
        executed_by_type: null,
        executed_by_id: null,
        executed_at: null,
        defect_issue_id: null,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    ];
  });

  it("shows result buttons for each run case", async () => {
    renderPage();
    // The row outer div has role="button" too (expand toggle) and its accessible
    // name includes all child text. Use an anchored pattern so we only match
    // <button> elements whose name IS exactly the result label, not the row.
    const passedBtns = await screen.findAllByRole("button", { name: /^(Passed|通过|합격|통과)$/ });
    expect(passedBtns.length).toBeGreaterThan(0);
  });

  it("marks a case result when a result button is clicked", async () => {
    renderPage();
    // Use anchored pattern — the outer row div (role="button") has a computed
    // accessible name that includes "Passed" as part of its full text content.
    // An exact match skips the row div and targets only the result <button>.
    const passedBtns = await screen.findAllByRole("button", { name: /^(Passed|通过|합격|통과)$/ });
    await userEvent.click(passedBtns[0]!);
    expect(mocks.updateCaseResult).toHaveBeenCalledWith(
      expect.objectContaining({ id: "rc-1", data: expect.objectContaining({ result: "passed" }) }),
    );
  });
});
