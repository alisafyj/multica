// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setApiInstance } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api";
import type { TaskRunEvidenceListResponse } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { taskRunEvidenceKey, TaskRunEvidenceDetails } from "./task-run-evidence-details";

const response: TaskRunEvidenceListResponse = {
  schema_version: "task_run_evidence/v1",
  task_id: "task-1",
  attempts: [
    {
      schema_version: "task_run_evidence/v1",
      task_id: "task-1",
      attempt: 2,
      evidence_id: `sha256:${"c".repeat(64)}`,
      claim_identity_source: "claim_generation",
      revision: 3,
      timings: {
        queue: { known: true, duration_ms: 0 },
        preparation: { known: false, duration_ms: null },
        first_tool: { known: true, duration_ms: 1200 },
        execution: { known: true, duration_ms: 65_000 },
        finalization: { known: false, duration_ms: null },
      },
      requested: { model: null, effort: "high" },
      client_effective: { model: "effective-model", effort: "medium" },
      provider_reported: { model: "provider-model", source: "provider_event" },
      runtime: { version: "0.2.21", content_sha256: `sha256:${"b".repeat(64)}` },
      usage: {
        input_uncached_tokens: 0,
        input_cache_read_tokens: 12,
        input_cache_write_tokens: null,
        output_tokens: 4,
        complete: false,
        source: "provider_event",
      },
      provider_cost: {
        amount_usd_ticks: 125_000_000,
        complete: false,
        authority: "provider_reported",
        basis: "unknown",
        source: "provider_event",
      },
      created_at: "2026-09-06T10:00:00Z",
      updated_at: "2026-09-06T10:00:01Z",
    },
  ],
};

afterEach(() => {
  vi.restoreAllMocks();
});

function renderDetails(result: TaskRunEvidenceListResponse | null) {
  setApiInstance({
    listTaskRunEvidence: vi.fn().mockResolvedValue(result),
  } as unknown as ApiClient);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderWithI18n(
    <QueryClientProvider client={client}>
      <TaskRunEvidenceDetails workspaceId="ws-1" taskId="task-1" />
    </QueryClientProvider>,
  );
}

describe("TaskRunEvidenceDetails", () => {
  it("keys same-attempt claims by opaque evidence identity", () => {
    const first = response.attempts[0]!;
    const second = { ...first, evidence_id: `sha256:${"d".repeat(64)}` };

    expect(taskRunEvidenceKey(first)).not.toBe(taskRunEvidenceKey(second));
  });

  it("labels a claim with its opaque evidence identity suffix", async () => {
    renderDetails(response);

    const identity = await screen.findByText("cccccccc");
    expect(identity).toHaveAttribute("title", `sha256:${"c".repeat(64)}`);
  });

  it("keeps provenance distinct and renders unknown values instead of zero", async () => {
    renderDetails(response);

    expect(await screen.findByText("Attempt 2")).toBeInTheDocument();
    expect(screen.getByText("Unknown · high")).toBeInTheDocument();
    expect(screen.getByText("effective-model · medium")).toBeInTheDocument();
    expect(screen.getByText("provider-model")).toBeInTheDocument();
    expect(screen.getAllByText("Unknown").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("0").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("$0.0125")).toBeInTheDocument();
    const costLabel = screen.getByText(/Provider-reported cost \(not an invoice\)/);
    expect(costLabel).toHaveTextContent("Partial");
    expect(costLabel).toHaveAttribute(
      "title",
      "Source: provider_event · Basis: unknown",
    );
    expect(screen.queryByText(/\/Users\//)).not.toBeInTheDocument();
  });

  it("shows old-server unavailability without fake zeroes", async () => {
    renderDetails(null);

    expect(await screen.findByText("Run evidence is unavailable on this server.")).toBeInTheDocument();
    expect(screen.queryByText("0")).not.toBeInTheDocument();
  });
});
