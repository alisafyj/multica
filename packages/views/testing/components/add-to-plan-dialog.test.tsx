import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithI18n } from "../../test/i18n";
import { AddToPlanDialog } from "./add-to-plan-dialog";

const mocks = vi.hoisted(() => ({
  plans: [] as unknown[],
  planCases: [] as unknown[],
  addCases: vi.fn(),
  createPlan: vi.fn(),
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { queryKey?: readonly unknown[] }) => {
    const kind = options.queryKey?.[2];
    if (kind === "cases") return { data: mocks.planCases, isLoading: false };
    return { data: mocks.plans, isLoading: false };
  },
}));

vi.mock("@multica/core/testing", () => ({
  testPlanListOptions: () => ({ queryKey: ["test-plans", "ws-1", "list"] }),
  testPlanCasesOptions: () => ({ queryKey: ["test-plans", "ws-1", "cases"] }),
  useAddTestPlanCases: () => ({ mutateAsync: mocks.addCases, isPending: false }),
  useCreateTestPlan: () => ({ mutateAsync: mocks.createPlan, isPending: false }),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function renderDialog(caseIds = ["case-a", "case-b"]) {
  renderWithI18n(
    <AddToPlanDialog
      open
      onOpenChange={vi.fn()}
      wsId="ws-1"
      projectId="p-1"
      caseIds={caseIds}
    />,
  );
}

async function clickConfirm() {
  const buttons = await screen.findAllByRole("button");
  const confirm = buttons.find((b) => b.textContent?.match(/^(加入|Add|追加|추가)$/));
  expect(confirm, "the dialog must offer a confirm button").toBeTruthy();
  await userEvent.click(confirm!);
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.plans = [];
  mocks.planCases = [];
});

// This dialog is the only caller of the add-cases endpoint. Before it existed,
// `useAddTestPlanCases` was dead code and a hand-made plan could never be
// filled, which left its "create run" button permanently disabled.
describe("AddToPlanDialog", () => {
  it("appends the selected cases after the plan's existing ones", async () => {
    mocks.plans = [{ id: "plan-1", title: "Sprint 1" }];
    mocks.planCases = [{ test_case_id: "case-existing", position: 0 }];
    mocks.addCases.mockResolvedValue({});

    renderDialog();
    await clickConfirm();

    expect(mocks.addCases).toHaveBeenCalledWith({
      planId: "plan-1",
      data: {
        cases: [
          { test_case_id: "case-a", position: 1 },
          { test_case_id: "case-b", position: 2 },
        ],
      },
    });
  });

  // The endpoint upserts on (plan, case), so re-sending a case already in the
  // plan would silently move it to the end of the run order.
  it("skips cases the plan already contains", async () => {
    mocks.plans = [{ id: "plan-1", title: "Sprint 1" }];
    mocks.planCases = [{ test_case_id: "case-a", position: 0 }];
    mocks.addCases.mockResolvedValue({});

    renderDialog();
    await clickConfirm();

    expect(mocks.addCases).toHaveBeenCalledWith(
      expect.objectContaining({
        data: { cases: [{ test_case_id: "case-b", position: 1 }] },
      }),
    );
  });

  it("sends nothing when every selected case is already in the plan", async () => {
    mocks.plans = [{ id: "plan-1", title: "Sprint 1" }];
    mocks.planCases = [
      { test_case_id: "case-a", position: 0 },
      { test_case_id: "case-b", position: 1 },
    ];

    renderDialog();
    await clickConfirm();

    expect(mocks.addCases).not.toHaveBeenCalled();
  });

  it("creates the plan first when the project has none", async () => {
    mocks.createPlan.mockResolvedValue({ id: "plan-new", title: "Regression" });
    mocks.addCases.mockResolvedValue({});

    renderDialog();
    // With no plans to pick, the dialog is already in create mode and only
    // needs a title.
    const titleInput = await screen.findByRole("textbox");
    await userEvent.type(titleInput, "Regression");
    await clickConfirm();

    expect(mocks.createPlan).toHaveBeenCalledWith({
      project_id: "p-1",
      title: "Regression",
    });
    // A brand-new plan starts empty, so positions start at zero regardless of
    // whatever `testPlanCasesOptions` last returned.
    expect(mocks.addCases).toHaveBeenCalledWith({
      planId: "plan-new",
      data: {
        cases: [
          { test_case_id: "case-a", position: 0 },
          { test_case_id: "case-b", position: 1 },
        ],
      },
    });
  });

  it("does not submit a new plan without a title", async () => {
    renderDialog();
    await clickConfirm();
    expect(mocks.createPlan).not.toHaveBeenCalled();
    expect(mocks.addCases).not.toHaveBeenCalled();
  });
});
