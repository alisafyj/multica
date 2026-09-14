// @vitest-environment node
import { describe, expect, it } from "vitest";
import type {
  AgentTask,
  TaskExecutionMetrics,
  TaskUsage,
} from "@multica/core/types/agent";
import {
  compareExecutions,
  summarizeExecutions,
} from "./execution-comparison-utils";

const allHistory = { from: "", to: "", taskIds: "" };
const snapshot: TaskExecutionMetrics = {
  schema_version: 1,
  provider: "hermes",
  requested_model: "gpt-5.4",
  daemon_version: "0.4.37-sso.11",
  daemon_commit: "abc123",
  community_base_version: "0.4.37",
  direct_agent_mode: false,
  concise_mode: false,
  started_at: "2026-09-05T12:00:00Z",
  phases: [],
};

function task(id: string, overrides: Partial<AgentTask> = {}): AgentTask {
  return {
    id,
    agent_id: "agent",
    runtime_id: "runtime",
    issue_id: "issue",
    status: "completed",
    priority: 0,
    created_at: "2026-09-05T12:00:00Z",
    dispatched_at: null,
    started_at: null,
    completed_at: "2026-09-05T12:00:10Z",
    result: null,
    error: null,
    ...overrides,
  };
}

function usage(model: string, overrides: Partial<TaskUsage> = {}): TaskUsage {
  return {
    model,
    input_tokens: 100,
    output_tokens: 20,
    cache_read_tokens: 300,
    cache_write_tokens: 40,
    ...overrides,
  };
}

describe("execution comparison", () => {
  it("intersects exact IDs and inclusive UTC creation dates without chats or duplicate samples", () => {
    const rows = [
      task("first", { created_at: "2026-09-05T00:00:00Z" }),
      task("last", { created_at: "2026-09-05T23:59:59.999Z" }),
      task("next", { created_at: "2026-09-06T00:00:00Z" }),
      task("chat", { chat_session_id: "private" }),
      task("chat-kind", { kind: "chat" }),
      task("missing-date", { created_at: "" }),
      task("unselected"),
      task("first"),
    ];
    const filtered = compareExecutions(rows, {
      from: "2026-09-05",
      to: "2026-09-05",
      taskIds: "first, last, next, chat,chat-kind,missing-date,missing,first",
    });
    expect(filtered.tasks.map((row) => row.id)).toEqual(["last", "first"]);
    expect(filtered.unmatchedIds).toBe(5);
    expect(filtered.missingDates).toBe(1);
    expect(filtered.summaries.unknown.sampleCount).toBe(2);
    expect(
      compareExecutions(rows, {
        from: "2026-09-06",
        to: "2026-09-05",
        taskIds: "",
      }).invalidRange,
    ).toBe(true);
  });

  it("uses terminal outcomes for success and only valid observed durations for averages", () => {
    const summary = summarizeExecutions([
      task("success", {
        dispatched_at: "2026-09-05T12:00:02Z",
        started_at: "2026-09-05T12:00:03Z",
      }),
      task("failed", {
        status: "failed",
        completed_at: "2026-09-05T12:00:20Z",
      }),
      task("cancelled", { status: "cancelled", completed_at: null }),
      task("inverted", { completed_at: "2026-09-05T11:59:59Z" }),
      task("live", { status: "running" }),
    ]);
    expect(summary).toMatchObject({
      completed: 2,
      failed: 1,
      cancelled: 1,
      nonterminal: 1,
      successRate: 0.5,
    });
    expect(summary.duration).toEqual({ total: 30_000, mean: 15_000, count: 2 });
    expect(summary.queue).toEqual({ total: 2000, mean: 2000, count: 1 });
    expect(summary.dispatch).toEqual({ total: 1000, mean: 1000, count: 1 });
  });

  it("counts each selected automatic retry once, not cumulative attempt ordinals or reruns", () => {
    const first = task("first", { attempt: 1 });
    const second = task("second", { attempt: 2, parent_task_id: "first" });
    const third = task("third", { attempt: 3, parent_task_id: "second" });
    expect(
      summarizeExecutions([
        first,
        second,
        third,
        third,
        task("rerun", { attempt: 1 }),
      ]).retries,
    ).toBe(2);
    expect(
      compareExecutions([first, second, third], {
        ...allHistory,
        taskIds: "third",
      }).summaries.unknown.retries,
    ).toBe(1);
    expect(summarizeExecutions([task("unknown")])).toMatchObject({
      retries: null,
      attemptsKnown: 0,
    });
  });

  it("preserves unknown values and excludes running phases without losing genuine zeros", () => {
    const legacy = summarizeExecutions([
      task("legacy", { completed_at: null, usage: [] }),
    ]);
    expect(legacy).toMatchObject({
      tokens: null,
      metadataKnown: 0,
      usageKnown: 0,
      estimatedCost: { total: null },
      reportedCost: { total: null },
      toolCalls: { total: null },
      duration: { total: null },
    });
    const observed = summarizeExecutions([
      task("observed", {
        execution_metrics: {
          ...snapshot,
          tool_calls: 0,
          phases: [
            {
              name: "prepare",
              started_at: snapshot.started_at,
              duration_ms: 0,
              status: "completed",
            },
            {
              name: "execute",
              started_at: snapshot.started_at,
              duration_ms: 2000,
              status: "running",
            },
          ],
        },
      }),
    ]);
    expect(observed.phases.prepare).toEqual({ total: 0, mean: 0, count: 1 });
    expect(observed.phases.execute).toEqual({
      total: null,
      mean: null,
      count: 0,
    });
    expect(observed.toolCalls.total).toBe(0);
  });

  it("counts cache categories once and keeps rate estimates, reported cost and unknown prices separate", () => {
    const summary = summarizeExecutions([
      task("priced", { usage: [usage("gpt-5.4")] }),
      task("reported", {
        usage: [usage("unmapped-a", { cost_usd_ticks: 20_000_000_000 })],
      }),
      task("unpriced", { usage: [usage("unmapped-b")] }),
      task("missing"),
    ]);
    expect(summary.tokens).toEqual({
      input: 300,
      output: 60,
      cacheRead: 900,
      cacheWrite: 120,
      total: 1380,
    });
    expect(summary.estimatedCost.total).toBeCloseTo(0.000725);
    expect(summary.reportedCost.total).toBe(2);
    expect(summary).toMatchObject({
      usageKnown: 3,
      usageRows: 3,
      unpricedRows: 1,
    });
    expect(
      summarizeExecutions([
        task("only-unpriced", { usage: [usage("unmapped-b")] }),
      ]).estimatedCost.total,
    ).toBeNull();
  });

  it("preserves a reported zero without replacing it with a known model rate", () => {
    const summary = summarizeExecutions([
      task("reported-zero", {
        usage: [usage("gpt-5.4", { cost_usd_ticks: 0 })],
      }),
    ]);
    expect(summary.reportedCost).toEqual({ total: 0, mean: 0, count: 1 });
    expect(summary.estimatedCost).toEqual({
      total: null,
      mean: null,
      count: 0,
    });
    expect(summary.unpricedRows).toBe(0);
  });

  it("compares only historical dimensions and treats missing settings as unknown, not mixed false", () => {
    const normal = task("normal", {
      concise_mode: true,
      execution_metrics: snapshot,
    });
    const concise = task("concise", {
      execution_metrics: { ...snapshot, concise_mode: true },
    });
    const consistent = compareExecutions([normal, concise], allHistory);
    expect(consistent.mixedDimensions).toEqual([]);
    expect(consistent.unknownConfiguration).toBe(0);
    expect(consistent.summaries.normal.sampleCount).toBe(1);
    const mixed = compareExecutions(
      [
        normal,
        concise,
        task("direct", {
          execution_metrics: { ...snapshot, direct_agent_mode: true },
        }),
        task("legacy", { concise_mode: false }),
      ],
      allHistory,
    );
    expect(mixed.mixedDimensions).toEqual(["direct_agent_mode"]);
    expect(mixed.unknownConfiguration).toBe(1);
    const missingVersion = compareExecutions(
      [
        task("missing-version", {
          execution_metrics: { ...snapshot, daemon_version: "unknown" },
        }),
      ],
      allHistory,
    );
    expect(missingVersion.unknownConfiguration).toBe(1);
  });
});
