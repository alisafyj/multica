// @vitest-environment jsdom

import { act, cleanup, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AgentTask, TaskUsage } from "@multica/core/types";
import { useCustomPricingStore } from "@multica/core/runtimes/custom-pricing-store";
import { renderWithI18n } from "../../test/i18n";

vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
}));

import { IssueUsageDialog } from "./issue-usage-dialog";

function makeTask(overrides: Partial<AgentTask> = {}): AgentTask {
  return {
    id: "task-1",
    agent_id: "agent-1",
    runtime_id: "runtime-1",
    issue_id: "issue-1",
    status: "completed",
    priority: 0,
    dispatched_at: null,
    started_at: "2026-08-05T08:00:00Z",
    completed_at: "2026-08-05T08:11:00Z",
    result: null,
    error: null,
    created_at: "2026-08-05T08:00:00Z",
    trigger_summary: "Initial run",
    ...overrides,
  };
}

function usage(overrides: Partial<TaskUsage> = {}): TaskUsage {
  return {
    provider: "anthropic",
    model: "claude-opus-5",
    input_tokens: 1_000,
    output_tokens: 1_000,
    cache_read_tokens: 1_000,
    cache_write_tokens: 0,
    ...overrides,
  };
}

function open(tasks: AgentTask[]) {
  renderWithI18n(
    <IssueUsageDialog open onOpenChange={() => {}} identifier="ACM-1" tasks={tasks} />,
  );
}

beforeEach(() => {
  cleanup();
  vi.clearAllMocks();
});

afterEach(cleanup);

describe("IssueUsageDialog", () => {
  it("floors the cache hit rate instead of rounding it up to 100%", () => {
    // 99.55% — rounding would print "100% hit rate" and claim every token came
    // from cache on an issue that plainly read some fresh input.
    open([
      makeTask({
        usage: [usage({ input_tokens: 573_500, cache_read_tokens: 126_000_000 })],
      }),
    ]);

    expect(screen.getByText(/99% hit rate/)).toBeInTheDocument();
    expect(screen.queryByText(/100% hit rate/)).not.toBeInTheDocument();
  });

  it("still says 100% when the hit rate really is 100%", () => {
    open([
      makeTask({ usage: [usage({ input_tokens: 0, cache_read_tokens: 1_000 })] }),
    ]);

    expect(screen.getByText(/100% hit rate/)).toBeInTheDocument();
  });

  it("keeps the full model list reachable when the cell truncates", () => {
    // A run that spilled across models carries ids long enough to set the
    // table's width on their own; the cell is capped, so the untruncated list
    // has to survive somewhere.
    const models = "claude-haiku-4-5-20251001, claude-opus-5[1m]";
    open([
      makeTask({
        usage: [
          usage({ model: "claude-haiku-4-5-20251001" }),
          usage({ model: "claude-opus-5[1m]" }),
        ],
      }),
    ]);

    expect(screen.getByTitle(models)).toBeInTheDocument();
  });

  it("gives every terminal run a status a screen reader can read", () => {
    // The status glyph is aria-hidden, so without the paired label a failed
    // run and a completed one are indistinguishable to assistive tech.
    open([
      makeTask({ id: "t-ok", usage: [usage()] }),
      makeTask({ id: "t-bad", status: "failed", usage: [usage()] }),
    ]);

    expect(screen.getByText("Completed")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();
  });
});

describe("IssueUsageDialog amount completeness", () => {
  afterEach(() => act(() => useCustomPricingStore.setState({ pricings: {} })));
  const missing = usage({ provider: "r35-test", model: "unpriced-model" });

  it("shows unknown for unpriced cost and cache savings instead of zero", () => {
    open([makeTask({ usage: [missing] })]);
    expect(screen.getAllByText("Unknown")).toHaveLength(4);
    expect(screen.queryByText("$0.00")).not.toBeInTheDocument();
    expect(screen.getByText(/r35-test\/unpriced-model/)).toBeInTheDocument();
  });

  it.each([
    ["en", "Unknown"],
    ["zh-Hans", "未知"],
    ["ja", "不明"],
    ["ko", "알 수 없음"],
  ] as const)("localizes unknown amounts in %s", (locale, label) => {
    renderWithI18n(
      <IssueUsageDialog open onOpenChange={() => {}} identifier="ACM-1" tasks={[makeTask({ usage: [missing] })]} />,
      { locale },
    );
    expect(screen.getAllByText(label)).toHaveLength(4);
    expect(screen.queryByText("$0.00")).not.toBeInTheDocument();
  });

  it("keeps narrow KPI values stacked and the dialog vertically scrollable", () => {
    open([makeTask({ usage: [usage(), missing] })]);
    const dialog = screen.getByRole("dialog");
    expect(dialog.className).toContain("max-h-[calc(100dvh-2rem)]");
    expect(dialog.className).toContain("overflow-y-auto");
    const grid = screen.getAllByText("$0.03 (partial)")[0]?.parentElement?.parentElement;
    expect(grid?.className).toContain("grid-cols-1");
    expect(grid?.className).toContain("@min-[40rem]/usage-detail:grid-cols-3");
    expect(grid?.className).toContain("[&_.text-display]:text-title-lg");
  });

  it("preserves the known subtotal and suppresses incomplete cost comparisons", () => {
    open([
      makeTask({ id: "known", agent_id: "agent-known", status: "failed", usage: [usage()] }),
      makeTask({ id: "missing", agent_id: "agent-missing", usage: [missing] }),
    ]);
    expect(screen.getAllByText("$0.03 (partial)")).toHaveLength(2);
    expect(screen.queryByText(/failed run.*account/i)).not.toBeInTheDocument();
    expect(screen.queryByText("Cost by agent")).not.toBeInTheDocument();
    const rows = within(screen.getByRole("table")).getAllByRole("row");
    expect(within(rows[1]!).getByText("$0.03")).toBeInTheDocument();
    expect(within(rows[2]!).getByText("Unknown")).toBeInTheDocument();
  });

  it("keeps a recorded cost while cache savings remains unknown", () => {
    open([makeTask({ usage: [{ ...missing, cost_usd_ticks: 30_000_000_000 }] })]);
    expect(screen.getAllByText("$3.00")).toHaveLength(3);
    expect(screen.getAllByText("Unknown")).toHaveLength(1);
  });

  it("does not report excluded costs when a task consumed no tokens", () => {
    open([makeTask({ usage: [{ ...missing, input_tokens: 0, output_tokens: 0,
      cache_read_tokens: 0, cache_write_tokens: 0 }] })]);
    expect(screen.getAllByText("$0.00")).toHaveLength(4);
    expect(screen.queryByText(/No price on file/)).not.toBeInTheDocument();
  });

  it("refreshes unknown amounts when a zero custom rate is saved", () => {
    open([makeTask({ usage: [missing] })]);
    expect(screen.getAllByText("Unknown")).toHaveLength(4);
    act(() => useCustomPricingStore.getState().setCustomPricing("r35-test/unpriced-model", {
      input: 0, output: 0, cacheRead: 0, cacheWrite: 0,
    }));
    expect(screen.queryByText("Unknown")).not.toBeInTheDocument();
    expect(screen.getAllByText("$0.00")).toHaveLength(4);
    act(() => useCustomPricingStore.setState({ pricings: {} }));
    expect(screen.getAllByText("Unknown")).toHaveLength(4);
  });
});
