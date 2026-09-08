"use client";

import { Loader2 } from "lucide-react";
import { useTaskRunEvidence } from "@multica/core/issues";
import type {
  TaskRunEvidenceAttempt,
  TaskRunEvidenceModelConfig,
  TaskRunEvidenceTiming,
} from "@multica/core/types";
import { useT } from "../../i18n";

const USD_TICKS_PER_DOLLAR = 10_000_000_000;

function formatDuration(timing: TaskRunEvidenceTiming, unknown: string): string {
  if (!timing.known || timing.duration_ms === null) return unknown;
  if (timing.duration_ms < 1_000) return `${timing.duration_ms}ms`;
  if (timing.duration_ms < 60_000) return `${(timing.duration_ms / 1_000).toFixed(1)}s`;
  const minutes = Math.floor(timing.duration_ms / 60_000);
  const seconds = Math.floor((timing.duration_ms % 60_000) / 1_000);
  return `${minutes}m ${seconds}s`;
}

function formatCount(value: number | null, unknown: string): string {
  return value === null ? unknown : new Intl.NumberFormat().format(value);
}

function formatModel(config: TaskRunEvidenceModelConfig, unknown: string): string {
  const model = config.model ?? unknown;
  return config.effort ? `${model} · ${config.effort}` : model;
}

function formatProviderCost(amountTicks: number | null, unknown: string): string {
  if (amountTicks === null) return unknown;
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 0,
    maximumFractionDigits: 6,
  }).format(amountTicks / USD_TICKS_PER_DOLLAR);
}

function shortHash(value: string): string {
  if (value.length <= 28) return value;
  return `${value.slice(0, 19)}…${value.slice(-8)}`;
}

export function taskRunEvidenceKey(evidence: TaskRunEvidenceAttempt): string {
  return evidence.evidence_id ?? `${evidence.attempt}:${evidence.revision}`;
}

function EvidenceValue({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <dt className="truncate text-[10px] leading-4 text-faint-foreground">{label}</dt>
      <dd className="truncate text-caption tabular-nums text-foreground" title={value}>
        {value}
      </dd>
    </div>
  );
}

function AttemptEvidence({ evidence }: { evidence: TaskRunEvidenceAttempt }) {
  const { t } = useT("issues");
  const unknown = t(($) => $.execution_log.evidence.unknown);
  const stages = [
    [t(($) => $.execution_log.evidence.queue), evidence.timings.queue],
    [t(($) => $.execution_log.evidence.preparation), evidence.timings.preparation],
    [t(($) => $.execution_log.evidence.first_tool), evidence.timings.first_tool],
    [t(($) => $.execution_log.evidence.execution), evidence.timings.execution],
    [t(($) => $.execution_log.evidence.finalization), evidence.timings.finalization],
  ] as const;
  const tokens = [
    [
      t(($) => $.execution_log.evidence.input_uncached),
      evidence.usage.input_uncached_tokens,
    ],
    [
      t(($) => $.execution_log.evidence.cache_read),
      evidence.usage.input_cache_read_tokens,
    ],
    [
      t(($) => $.execution_log.evidence.cache_write),
      evidence.usage.input_cache_write_tokens,
    ],
    [t(($) => $.execution_log.evidence.output), evidence.usage.output_tokens],
  ] as const;

  return (
    <section className="space-y-2 border-t border-border/60 py-2 first:border-t-0 first:pt-0 last:pb-0">
      <div className="flex items-baseline justify-between gap-2">
        <div className="flex min-w-0 items-baseline gap-2">
          <h4 className="text-caption font-medium text-foreground">
            {t(($) => $.execution_log.evidence.attempt, { attempt: evidence.attempt })}
          </h4>
          {evidence.evidence_id && (
            <span
              className="truncate font-mono text-[10px] text-faint-foreground"
              title={evidence.evidence_id}
            >
              {evidence.evidence_id.slice(-8)}
            </span>
          )}
        </div>
        <span className="text-[10px] text-faint-foreground">
          {t(($) => $.execution_log.evidence.revision, { revision: evidence.revision })}
        </span>
      </div>

      <div>
        <p className="mb-1 text-[10px] font-medium uppercase text-faint-foreground">
          {t(($) => $.execution_log.evidence.stages)}
        </p>
        <dl className="grid grid-cols-2 gap-x-3 gap-y-1">
          {stages.map(([label, timing]) => (
            <EvidenceValue
              key={label}
              label={label}
              value={formatDuration(timing, unknown)}
            />
          ))}
        </dl>
      </div>

      <div>
        <div className="mb-1 flex items-center gap-2">
          <p className="text-[10px] font-medium uppercase text-faint-foreground">
            {t(($) => $.execution_log.evidence.tokens)}
          </p>
          {!evidence.usage.complete && (
            <span className="text-[10px] text-faint-foreground">
              {t(($) => $.execution_log.evidence.incomplete)}
            </span>
          )}
        </div>
        <dl className="grid grid-cols-2 gap-x-3 gap-y-1">
          {tokens.map(([label, value]) => (
            <EvidenceValue key={label} label={label} value={formatCount(value, unknown)} />
          ))}
        </dl>
      </div>

      <dl className="grid grid-cols-1 gap-y-1 border-t border-border/40 pt-2">
        <EvidenceValue
          label={t(($) => $.execution_log.evidence.requested_model)}
          value={formatModel(evidence.requested, unknown)}
        />
        <EvidenceValue
          label={t(($) => $.execution_log.evidence.client_effective_model)}
          value={formatModel(evidence.client_effective, unknown)}
        />
        <EvidenceValue
          label={t(($) => $.execution_log.evidence.provider_reported_model)}
          value={evidence.provider_reported.model ?? unknown}
        />
      </dl>

      <dl className="grid grid-cols-2 gap-x-3 gap-y-1 border-t border-border/40 pt-2">
        <EvidenceValue
          label={t(($) => $.execution_log.evidence.runtime_version)}
          value={evidence.runtime.version ?? unknown}
        />
        <div className="min-w-0">
          <dt className="truncate text-[10px] leading-4 text-faint-foreground">
            {t(($) => $.execution_log.evidence.runtime_hash)}
          </dt>
          <dd
            className="truncate font-mono text-[10px] leading-4 text-foreground"
            title={evidence.runtime.content_sha256 ?? undefined}
          >
            {evidence.runtime.content_sha256 ? shortHash(evidence.runtime.content_sha256) : unknown}
          </dd>
        </div>
        <div className="col-span-2 flex items-baseline justify-between gap-3 pt-1">
          <dt
            className="text-[10px] leading-4 text-faint-foreground"
            title={t(($) => $.execution_log.evidence.provider_cost_provenance, {
              source: evidence.provider_cost.source,
              basis: evidence.provider_cost.basis,
            })}
          >
            {t(($) => $.execution_log.evidence.provider_cost_not_invoice)}
            {!evidence.provider_cost.complete && (
              <> · {t(($) => $.execution_log.evidence.incomplete)}</>
            )}
          </dt>
          <dd className="shrink-0 text-caption tabular-nums text-foreground">
            {formatProviderCost(evidence.provider_cost.amount_usd_ticks, unknown)}
          </dd>
        </div>
      </dl>
    </section>
  );
}

export function TaskRunEvidenceDetails({
  workspaceId,
  taskId,
}: {
  workspaceId: string;
  taskId: string;
}) {
  const { t } = useT("issues");
  const query = useTaskRunEvidence(workspaceId, taskId, true);

  if (query.isPending) {
    return (
      <div className="flex items-center gap-1.5 border-t border-border/60 px-3 py-2 text-caption text-muted-foreground">
        <Loader2 className="h-3.5 w-3.5 animate-spin" />
        {t(($) => $.execution_log.evidence.loading)}
      </div>
    );
  }

  if (query.isError) {
    return (
      <p className="border-t border-border/60 px-3 py-2 text-caption text-destructive">
        {t(($) => $.execution_log.evidence.failed)}
      </p>
    );
  }

  if (query.data === null) {
    return (
      <p className="border-t border-border/60 px-3 py-2 text-caption text-muted-foreground">
        {t(($) => $.execution_log.evidence.unavailable)}
      </p>
    );
  }

  if (!query.data?.attempts.length) {
    return (
      <p className="border-t border-border/60 px-3 py-2 text-caption text-muted-foreground">
        {t(($) => $.execution_log.evidence.empty)}
      </p>
    );
  }

  return (
    <div className="border-t border-border/60 bg-muted/20 px-3 py-2">
      {query.data.attempts.map((evidence) => (
        <AttemptEvidence key={taskRunEvidenceKey(evidence)} evidence={evidence} />
      ))}
    </div>
  );
}
