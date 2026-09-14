// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api";
import { taskRunEvidenceKeys, useTaskRunEvidence } from "./use-task-run-evidence";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useTaskRunEvidence", () => {
  it("does not fetch until its row is expanded", async () => {
    const request = vi.fn().mockResolvedValue(null);
    setApiInstance({ listTaskRunEvidence: request } as unknown as ApiClient);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    const { rerender } = renderHook(
      ({ expanded }) => useTaskRunEvidence("ws-1", "task-1", expanded),
      { initialProps: { expanded: false }, wrapper },
    );

    expect(request).not.toHaveBeenCalled();

    rerender({ expanded: true });
    await waitFor(() => expect(request).toHaveBeenCalledTimes(1));
  });

  it("isolates evidence caches by workspace", () => {
    expect(taskRunEvidenceKeys.detail("ws-1", "task-1")).not.toEqual(
      taskRunEvidenceKeys.detail("ws-2", "task-1"),
    );
  });
});
