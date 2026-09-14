import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithI18n } from "../../test/i18n";
import { RecommendCasesDialog, parseChangedPaths } from "./recommend-cases-dialog";

const mocks = vi.hoisted(() => ({
  recommend: vi.fn(),
}));

vi.mock("@multica/core/testing", () => ({
  useRecommendTestCases: () => ({ mutateAsync: mocks.recommend, isPending: false }),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function recommendation(id: string, key: string, title: string, pathCount: number) {
  return {
    test_case: { id, key, title, module: "orders", repos: [] },
    matches: [{ alias: "web", role: "under_test", glob: "src/order/**", paths: ["src/order/a.ts"] }],
    path_count: pathCount,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
});

// The path parser's matrix lives here; the component test keeps the wiring.
describe("parseChangedPaths", () => {
  it("trims, drops blanks and duplicates, and accepts CRLF", () => {
    expect(parseChangedPaths(" src/a.ts \r\n\nsrc/a.ts\nsrc/b.ts\n")).toEqual(["src/a.ts", "src/b.ts"]);
  });
});

describe("RecommendCasesDialog", () => {
  it("asks the server with the pasted paths and hands the ticked cases back", async () => {
    mocks.recommend.mockResolvedValue({
      cases: [recommendation("c1", "TC-1", "Checkout total", 2), recommendation("c2", "TC-2", "Cart badge", 1)],
      unmatched_paths: ["docs/readme.md"],
      total: 2,
    });
    const onSelect = vi.fn();
    renderWithI18n(
      <RecommendCasesDialog open onOpenChange={vi.fn()} projectId="p-1" onSelect={onSelect} onStartRun={vi.fn()} />,
    );

    await userEvent.type(screen.getByLabelText("Changed paths"), "src/order/a.ts\ndocs/readme.md");
    await userEvent.click(screen.getByRole("button", { name: "Find cases" }));

    expect(mocks.recommend).toHaveBeenCalledWith({ project_id: "p-1", paths: ["src/order/a.ts", "docs/readme.md"] });
    expect(await screen.findByText("Checkout total")).toBeTruthy();
    expect(screen.getByText("1 paths claimed by no case")).toBeTruthy();

    // Every recommended case starts ticked; unticking one drops it from the hand-off.
    await userEvent.click(screen.getByRole("checkbox", { name: "TC-2" }));
    await userEvent.click(screen.getByRole("button", { name: "Select in list (1)" }));
    expect(onSelect).toHaveBeenCalledWith(["c1"]);
  });

  it("starts a run with the ticked cases through the page", async () => {
    mocks.recommend.mockResolvedValue({ cases: [recommendation("c1", "TC-1", "Checkout total", 1)], unmatched_paths: [], total: 1 });
    const onStartRun = vi.fn().mockResolvedValue(undefined);
    const onOpenChange = vi.fn();
    renderWithI18n(
      <RecommendCasesDialog open onOpenChange={onOpenChange} projectId="p-1" onSelect={vi.fn()} onStartRun={onStartRun} />,
    );
    await userEvent.type(screen.getByLabelText("Changed paths"), "src/order/a.ts");
    await userEvent.click(screen.getByRole("button", { name: "Find cases" }));
    await screen.findByText("Checkout total");
    await userEvent.click(screen.getByRole("button", { name: "Start run" }));
    expect(onStartRun).toHaveBeenCalledWith(["c1"]);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("says so when nothing claims the paths", async () => {
    mocks.recommend.mockResolvedValue({ cases: [], unmatched_paths: ["x.ts"], total: 0 });
    renderWithI18n(
      <RecommendCasesDialog open onOpenChange={vi.fn()} projectId="p-1" onSelect={vi.fn()} onStartRun={vi.fn()} />,
    );
    await userEvent.type(screen.getByLabelText("Changed paths"), "x.ts");
    await userEvent.click(screen.getByRole("button", { name: "Find cases" }));
    expect(await screen.findByText(/No case claims these paths/)).toBeTruthy();
  });
});
