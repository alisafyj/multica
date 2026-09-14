import type {
  AgentTask,
  TaskExecutionMetrics,
} from "@multica/core/types/agent";
import { estimateCost, isModelPriced } from "../../../runtimes/utils";

const COST_USD_TICKS_PER_USD = 10_000_000_000;

export const EXECUTION_PHASES = ["prepare", "execute", "finalize"] as const;
export type ExecutionMode = "normal" | "concise" | "unknown";
export type ExecutionPhase = (typeof EXECUTION_PHASES)[number];

export interface ObservedMetric {
  total: number | null;
  mean: number | null;
  count: number;
}

export interface ExecutionSummary {
  sampleCount: number;
  completed: number;
  failed: number;
  cancelled: number;
  nonterminal: number;
  successRate: number | null;
  retries: number | null;
  attemptsKnown: number;
  metadataKnown: number;
  duration: ObservedMetric;
  queue: ObservedMetric;
  dispatch: ObservedMetric;
  phases: Record<ExecutionPhase, ObservedMetric>;
  toolCalls: ObservedMetric;
  tokens: {
    input: number;
    output: number;
    cacheRead: number;
    cacheWrite: number;
    total: number;
  } | null;
  usageKnown: number;
  estimatedCost: ObservedMetric;
  reportedCost: ObservedMetric;
  usageRows: number;
  unpricedRows: number;
}

export interface ExecutionFilters {
  from: string;
  to: string;
  taskIds: string;
}

export function executionMode(task: AgentTask): ExecutionMode {
  const concise = task.execution_metrics?.concise_mode ?? task.concise_mode;
  return concise === true
    ? "concise"
    : concise === false
      ? "normal"
      : "unknown";
}

export function knownSnapshotText(value: string | undefined): string | null {
  const text = value?.trim();
  return text && text.toLowerCase() !== "unknown" ? text : null;
}

export function elapsedMs(
  start: string | null | undefined,
  end: string | null | undefined,
): number | null {
  if (!start || !end) return null;
  const duration = Date.parse(end) - Date.parse(start);
  return Number.isFinite(duration) && duration >= 0 ? duration : null;
}

function isTerminal(task: AgentTask): boolean {
  return (
    task.status === "completed" ||
    task.status === "failed" ||
    task.status === "cancelled"
  );
}

export function executionDuration(task: AgentTask): number | null {
  return isTerminal(task)
    ? elapsedMs(task.created_at, task.completed_at)
    : null;
}

export function phaseDuration(
  task: AgentTask,
  name: ExecutionPhase,
): number | null {
  const phases = task.execution_metrics?.phases.filter(
    (phase) => phase.name === name,
  );
  if (!phases?.length || phases.some((phase) => phase.status === "running"))
    return null;
  return phases.reduce((total, phase) => total + phase.duration_ms, 0);
}

function emptyMetric(): ObservedMetric {
  return { total: null, mean: null, count: 0 };
}

function observe(
  metric: ObservedMetric,
  value: number | null | undefined,
): void {
  if (value == null || !Number.isFinite(value) || value < 0) return;
  metric.total = (metric.total ?? 0) + value;
  metric.count += 1;
  metric.mean = metric.total / metric.count;
}

function validAttempt(task: AgentTask): boolean {
  return (
    typeof task.attempt === "number" &&
    Number.isInteger(task.attempt) &&
    task.attempt >= 1
  );
}

export function summarizeExecutions(
  tasks: readonly AgentTask[],
): ExecutionSummary {
  const summary: ExecutionSummary = {
    sampleCount: 0,
    completed: 0,
    failed: 0,
    cancelled: 0,
    nonterminal: 0,
    successRate: null,
    retries: null,
    attemptsKnown: 0,
    metadataKnown: 0,
    duration: emptyMetric(),
    queue: emptyMetric(),
    dispatch: emptyMetric(),
    phases: {
      prepare: emptyMetric(),
      execute: emptyMetric(),
      finalize: emptyMetric(),
    },
    toolCalls: emptyMetric(),
    tokens: null,
    usageKnown: 0,
    estimatedCost: emptyMetric(),
    reportedCost: emptyMetric(),
    usageRows: 0,
    unpricedRows: 0,
  };
  const seen = new Set<string>();
  for (const task of tasks) {
    if (seen.has(task.id)) continue;
    seen.add(task.id);
    summary.sampleCount += 1;
    if (task.status === "completed") summary.completed += 1;
    else if (task.status === "failed") summary.failed += 1;
    else if (task.status === "cancelled") summary.cancelled += 1;
    else summary.nonterminal += 1;
    if (task.execution_metrics) summary.metadataKnown += 1;
    if (validAttempt(task)) {
      summary.attemptsKnown += 1;
      // Count selected retry executions, not cumulative attempt ordinals. A
      // 1 → 2 → 3 chain has two retries, not (2 - 1) + (3 - 1) = three.
      summary.retries = (summary.retries ?? 0) + (task.attempt! > 1 ? 1 : 0);
    }
    observe(summary.duration, executionDuration(task));
    observe(summary.queue, elapsedMs(task.created_at, task.dispatched_at));
    observe(summary.dispatch, elapsedMs(task.dispatched_at, task.started_at));
    for (const phase of EXECUTION_PHASES)
      observe(summary.phases[phase], phaseDuration(task, phase));
    observe(summary.toolCalls, task.execution_metrics?.tool_calls);
    if (!task.usage?.length) continue;
    summary.usageKnown += 1;
    summary.tokens ??= {
      input: 0,
      output: 0,
      cacheRead: 0,
      cacheWrite: 0,
      total: 0,
    };
    for (const row of task.usage) {
      summary.usageRows += 1;
      summary.tokens.input += row.input_tokens;
      summary.tokens.output += row.output_tokens;
      summary.tokens.cacheRead += row.cache_read_tokens;
      summary.tokens.cacheWrite += row.cache_write_tokens;
      if (
        row.cost_usd_ticks !== undefined &&
        Number.isFinite(row.cost_usd_ticks) &&
        row.cost_usd_ticks >= 0
      ) {
        observe(
          summary.reportedCost,
          row.cost_usd_ticks / COST_USD_TICKS_PER_USD,
        );
      } else if (isModelPriced(row.model, row.provider)) {
        observe(
          summary.estimatedCost,
          estimateCost({ ...row, cost_usd_ticks: undefined }),
        );
      } else {
        summary.unpricedRows += 1;
      }
    }
  }
  const terminal = summary.completed + summary.failed + summary.cancelled;
  summary.successRate = terminal ? summary.completed / terminal : null;
  if (summary.tokens) {
    summary.tokens.total =
      summary.tokens.input +
      summary.tokens.output +
      summary.tokens.cacheRead +
      summary.tokens.cacheWrite;
  }
  return summary;
}

const SNAPSHOT_DIMENSIONS = [
  "provider",
  "requested_model",
  "daemon_version",
  "daemon_commit",
  "community_base_version",
  "direct_agent_mode",
] as const satisfies readonly (keyof TaskExecutionMetrics)[];

export function compareExecutions(
  tasks: readonly AgentTask[],
  filters: ExecutionFilters,
) {
  const requestedIds = new Set(
    filters.taskIds.split(/[\s,，]+/).filter(Boolean),
  );
  const from = filters.from ? Date.parse(`${filters.from}T00:00:00Z`) : null;
  const to = filters.to
    ? Date.parse(`${filters.to}T00:00:00Z`) + 86_400_000
    : null;
  const invalidRange =
    (from !== null && !Number.isFinite(from)) ||
    (to !== null && !Number.isFinite(to)) ||
    (from !== null && to !== null && from >= to);
  const seen = new Set<string>();
  let missingDates = 0;
  const selected = tasks
    .filter((task) => {
      if (task.chat_session_id || task.kind === "chat" || seen.has(task.id))
        return false;
      seen.add(task.id);
      if (requestedIds.size && !requestedIds.has(task.id)) return false;
      const createdAt = Date.parse(task.created_at);
      if (!Number.isFinite(createdAt)) missingDates += 1;
      if (invalidRange) return false;
      if ((from !== null || to !== null) && !Number.isFinite(createdAt))
        return false;
      return (
        (from === null || createdAt >= from) && (to === null || createdAt < to)
      );
    })
    .sort(
      (a, b) =>
        (Date.parse(b.created_at) || 0) - (Date.parse(a.created_at) || 0),
    );
  const cohorts: Record<ExecutionMode, AgentTask[]> = {
    normal: [],
    concise: [],
    unknown: [],
  };
  const values = Object.fromEntries(
    SNAPSHOT_DIMENSIONS.map((dimension) => [
      dimension,
      new Set<string | boolean>(),
    ]),
  ) as Record<(typeof SNAPSHOT_DIMENSIONS)[number], Set<string | boolean>>;
  let unknownConfiguration = 0;
  for (const task of selected) {
    cohorts[executionMode(task)].push(task);
    let missing = false;
    for (const dimension of SNAPSHOT_DIMENSIONS) {
      const raw = task.execution_metrics?.[dimension];
      const value = typeof raw === "boolean" ? raw : knownSnapshotText(raw);
      if (value === null) missing = true;
      else values[dimension].add(value);
    }
    if (missing) unknownConfiguration += 1;
  }
  return {
    tasks: selected,
    summaries: {
      normal: summarizeExecutions(cohorts.normal),
      concise: summarizeExecutions(cohorts.concise),
      unknown: summarizeExecutions(cohorts.unknown),
    },
    mixedDimensions: SNAPSHOT_DIMENSIONS.filter(
      (dimension) => values[dimension].size > 1,
    ),
    unknownConfiguration,
    missingDates,
    invalidRange,
    unmatchedIds: requestedIds.size ? requestedIds.size - selected.length : 0,
  };
}
