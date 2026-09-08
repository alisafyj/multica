import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { TaskRunEvidenceListResponseSchema } from "./schemas";

const evidencePayload = {
  schema_version: "task_run_evidence/v1",
  task_id: "task/1",
  attempts: [
    {
      schema_version: "task_run_evidence/v1",
      task_id: "task/1",
      attempt: 1,
      evidence_id: `sha256:${"b".repeat(64)}`,
      claim_identity_source: "claim_generation",
      revision: 2,
      timings: {
        queue: { known: true, duration_ms: 0 },
        preparation: { known: false, duration_ms: null },
        first_tool: { known: true, duration_ms: 1250 },
        execution: { known: true, duration_ms: 3200 },
        finalization: { known: false, duration_ms: null },
      },
      requested: { model: "requested-model", effort: "high" },
      client_effective: { model: "effective-model", effort: "medium" },
      provider_reported: { model: "provider-model", source: "provider_summary" },
      runtime: {
        version: "0.2.21",
        content_sha256: `sha256:${"a".repeat(64)}`,
      },
      usage: {
        input_uncached_tokens: 0,
        input_cache_read_tokens: 12,
        input_cache_write_tokens: null,
        output_tokens: 4,
        complete: false,
        source: "provider_summary",
      },
      provider_cost: {
        amount_usd_ticks: 125_000_000,
        complete: true,
        authority: "provider_reported",
        basis: "provider_reported",
        source: "provider_summary",
      },
      created_at: "2026-09-06T10:00:00Z",
      updated_at: "2026-09-06T10:00:01Z",
    },
  ],
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("TaskRunEvidenceListResponseSchema", () => {
  it("preserves unknown nulls separately from measured zeroes", () => {
    const parsed = TaskRunEvidenceListResponseSchema.parse(evidencePayload);

    expect(parsed.attempts[0]?.timings.queue.duration_ms).toBe(0);
    expect(parsed.attempts[0]?.timings.preparation.duration_ms).toBeNull();
    expect(parsed.attempts[0]?.usage.input_uncached_tokens).toBe(0);
    expect(parsed.attempts[0]?.usage.input_cache_write_tokens).toBeNull();
  });

  it("preserves the opaque claim evidence identity", () => {
    const parsed = TaskRunEvidenceListResponseSchema.parse(evidencePayload);

    expect(parsed.attempts[0]?.evidence_id).toBe(`sha256:${"b".repeat(64)}`);
    expect(parsed.attempts[0]?.claim_identity_source).toBe("claim_generation");
  });

  it("accepts old-server evidence without a claim identity", () => {
    const { evidence_id: _evidenceId, claim_identity_source: _source, ...legacyAttempt } =
      evidencePayload.attempts[0]!;
    const legacy = { ...evidencePayload, attempts: [legacyAttempt] };

    const parsed = TaskRunEvidenceListResponseSchema.parse(legacy);

    expect(parsed.attempts[0]?.evidence_id).toBeUndefined();
    expect(parsed.attempts[0]?.claim_identity_source).toBeUndefined();
  });

  it("rejects an incomplete evidence envelope", () => {
    expect(() =>
      TaskRunEvidenceListResponseSchema.parse({
        schema_version: "task_run_evidence/v1",
        task_id: "task-1",
        attempts: [{ attempt: 1 }],
      }),
    ).toThrow();
  });
});

describe("ApiClient.listTaskRunEvidence", () => {
  it("parses the task evidence endpoint and encodes the task id", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(evidencePayload), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const result = await new ApiClient("https://api.example.test").listTaskRunEvidence("task/1");

    expect(result?.attempts[0]?.usage.input_cache_write_tokens).toBeNull();
    expect(fetchMock).toHaveBeenCalledWith(
      "https://api.example.test/api/tasks/task%2F1/run-evidence",
      expect.any(Object),
    );
  });

  it("treats an old server 404 as unsupported evidence", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: "not found" }), {
          status: 404,
          statusText: "Not Found",
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    await expect(
      new ApiClient("https://api.example.test").listTaskRunEvidence("task-1"),
    ).resolves.toBeNull();
  });
});

describe("per-model execution evidence", () => {
  const attempt = evidencePayload.attempts[0]!;
  const entry = { model: "model-a", usage: attempt.usage, provider_cost: attempt.provider_cost };
  const inventory = { entries: [entry], complete: false, truncated: false, source: "model_usage" };

  it("preserves optional model inventory without inventing a main model", () => {
    const parsed = TaskRunEvidenceListResponseSchema.parse({ ...evidencePayload,
      attempts: [{ ...attempt, model_usage: inventory, provider_reported: { model: null, source: "missing" } }],
    });
    expect(parsed.attempts[0]?.model_usage).toEqual(inventory);
    expect(parsed.attempts[0]?.provider_reported.model).toBeNull();
    expect(TaskRunEvidenceListResponseSchema.parse(evidencePayload).attempts[0]?.model_usage).toBeUndefined();
  });

  it.each([null, "unknown", { ...inventory, entries: null },
    { ...inventory, entries: [{ ...entry, usage: { ...entry.usage, output_tokens: -1 } }] },
    { ...inventory, complete: "true" }, { ...inventory, entries: Array(13).fill(entry) },
  ])("omits malformed optional inventory without discarding valid evidence %#", (modelUsage) => {
    const parsed = TaskRunEvidenceListResponseSchema.parse({ ...evidencePayload,
      attempts: [{ ...attempt, model_usage: modelUsage }],
    });
    expect(parsed.attempts[0]?.model_usage).toBeUndefined();
    expect(parsed.attempts[0]?.usage).toEqual(attempt.usage);
    expect(parsed.attempts[0]?.timings).toEqual(attempt.timings);
  });

  it("retains other attempts through the API when one optional inventory is malformed", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({
      ...evidencePayload,
      attempts: [{ ...attempt, model_usage: { ...inventory, entries: null } },
        { ...attempt, attempt: 2, model_usage: inventory }],
    }), { status: 200, headers: { "Content-Type": "application/json" } })));
    const result = await new ApiClient("https://api.example.test").listTaskRunEvidence("task/1");
    expect(result?.attempts).toHaveLength(2);
    expect(result?.attempts[0]?.model_usage).toBeUndefined();
    expect(result?.attempts[0]?.usage).toEqual(attempt.usage);
    expect(result?.attempts[0]?.timings).toEqual(attempt.timings);
    expect(result?.attempts[1]?.model_usage).toEqual(inventory);
  });
});
