import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithI18n } from "../../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { IssueTestCoverage } from "./issue-test-coverage";

const mocks = vi.hoisted(() => ({
  cases: [] as unknown[],
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: () => ({ data: mocks.cases, isLoading: false }),
}));

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    testCaseDetail: (ref: string) => `/acme/tests/${ref}`,
  }),
}));

vi.mock("@multica/core/testing", () => ({
  issueTestCasesOptions: () => ({ queryKey: ["issue-test-cases", "ws-1", "i-1"] }),
  TEST_RUN_RESULTS: ["pending", "running", "passed", "failed", "blocked", "skipped"],
  TEST_RUN_RESULT_TONE: {
    pending: "",
    running: "",
    passed: "",
    failed: "",
    blocked: "",
    skipped: "",
  },
}));

function makeAdapter(): NavigationAdapter {
  return {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/issues/i-1",
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: (p) => p,
  };
}

function renderCoverage() {
  return renderWithI18n(
    <NavigationProvider value={makeAdapter()}>
      <IssueTestCoverage issueId="i-1" />
    </NavigationProvider>,
  );
}

function makeLink(overrides = {}) {
  return {
    test_case_id: "c-1",
    issue_id: "i-1",
    case_number: 1,
    case_key: "TC-1",
    case_title: "Checkout succeeds",
    case_status: "active",
    case_priority: "p1",
    case_type: "functional",
    latest_result: "passed",
    latest_executed_at: "2024-05-06T00:00:00Z",
    origin: "human",
    created_at: "2024-05-01T00:00:00Z",
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.cases = [];
});

describe("IssueTestCoverage", () => {
  // Every issue in a workspace that does not use the testing surface would
  // otherwise carry an empty block.
  it("renders nothing when the issue has no linked cases", () => {
    const { container } = renderCoverage();
    expect(container).toBeEmptyDOMElement();
  });

  it("lists each covering case with a link to it", async () => {
    mocks.cases = [makeLink()];
    renderCoverage();

    expect(await screen.findByText("Checkout succeeds")).toBeTruthy();
    const key = screen.getByText("TC-1");
    expect(key.closest("a")).toHaveAttribute("href", "/acme/tests/TC-1");
  });

  // A case linked but never run is a coverage claim, not evidence — saying
  // "pending" would assert it is queued in a round it was never added to.
  it("shows a never-executed case as not run rather than pending", async () => {
    mocks.cases = [makeLink({ latest_result: null, latest_executed_at: null })];
    renderCoverage();

    expect(
      await screen.findByText(/Not run|未执行|未実行|미실행/),
    ).toBeTruthy();
    expect(screen.queryByText(/^(Pending|待执行)$/)).toBeNull();
  });

  it("counts failing and never-run cases next to the total", async () => {
    mocks.cases = [
      makeLink({ test_case_id: "c-1", case_key: "TC-1", latest_result: "failed" }),
      makeLink({ test_case_id: "c-2", case_key: "TC-2", latest_result: "passed" }),
      makeLink({ test_case_id: "c-3", case_key: "TC-3", latest_result: null }),
    ];
    renderCoverage();

    expect(await screen.findByText("3")).toBeTruthy();
    expect(screen.getByText(/1 failing|1 条失败|失敗 1 件|실패 1건/)).toBeTruthy();
    expect(screen.getByText(/1 not run|1 条未执行|未実行 1 件|미실행 1건/)).toBeTruthy();
  });

  // An AI-asserted coverage claim is what a reviewer needs flagged; a
  // hand-drawn link needs no badge.
  it("flags an AI-asserted link and leaves a human one unmarked", async () => {
    mocks.cases = [makeLink({ origin: "ai" })];
    const { rerender } = renderCoverage();
    expect(await screen.findByText(/AI/)).toBeTruthy();

    mocks.cases = [makeLink({ origin: "human" })];
    rerender(
      <NavigationProvider value={makeAdapter()}>
        <IssueTestCoverage issueId="i-1" />
      </NavigationProvider>,
    );
    expect(screen.queryByText(/^AI( 生成)?$/)).toBeNull();
  });
});
