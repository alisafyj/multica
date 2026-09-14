export type TaskRunEvidenceSource =
  | "provider_event"
  | "provider_summary"
  | "model_usage"
  | "task_usage_api"
  | "missing";

export interface TaskRunEvidenceTiming {
  known: boolean;
  duration_ms: number | null;
}

export interface TaskRunEvidenceModelConfig {
  model: string | null;
  effort: string | null;
}

export interface TaskRunEvidenceAttempt {
  schema_version: string;
  task_id: string;
  attempt: number;
  evidence_id?: string | null;
  claim_identity_source?: "claim_generation" | "missing";
  revision: number;
  timings: {
    queue: TaskRunEvidenceTiming;
    preparation: TaskRunEvidenceTiming;
    first_tool: TaskRunEvidenceTiming;
    execution: TaskRunEvidenceTiming;
    finalization: TaskRunEvidenceTiming;
  };
  requested: TaskRunEvidenceModelConfig;
  client_effective: TaskRunEvidenceModelConfig;
  provider_reported: {
    model: string | null;
    source: Exclude<TaskRunEvidenceSource, "task_usage_api">;
  };
  runtime: {
    version: string | null;
    content_sha256: string | null;
  };
  usage: {
    input_uncached_tokens: number | null;
    input_cache_read_tokens: number | null;
    input_cache_write_tokens: number | null;
    output_tokens: number | null;
    complete: boolean;
    source: Exclude<TaskRunEvidenceSource, "task_usage_api">;
  };
  provider_cost: {
    amount_usd_ticks: number | null;
    complete: boolean;
    authority: "provider_reported" | "missing";
    basis: "provider_reported" | "unknown" | "missing";
    source: TaskRunEvidenceSource;
  };
  model_usage?: {
    entries: {
      model: string;
      usage: TaskRunEvidenceAttempt["usage"];
      provider_cost: TaskRunEvidenceAttempt["provider_cost"];
    }[];
    complete: boolean;
    truncated: boolean;
    source: "model_usage" | "missing";
  };
  created_at: string;
  updated_at: string;
}

export interface TaskRunEvidenceListResponse {
  schema_version: string;
  task_id: string;
  attempts: TaskRunEvidenceAttempt[];
}
