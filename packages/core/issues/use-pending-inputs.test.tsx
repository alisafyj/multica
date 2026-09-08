/** @vitest-environment jsdom */
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import type { PendingInput } from "../types";
import { issueKeys } from "./queries";
import {
  pendingInputsRefetchInterval,
  useAnswerPendingInput,
  usePendingInputs,
} from "./use-pending-inputs";

const openInput = {
  id: "input-1",
  task_id: "task-1",
  issue_id: "issue-1",
  question_comment_id: "comment-1",
  state: "open",
  version: 1,
  questions: [],
  answers: null,
  created_at: "2026-09-06T00:00:00Z",
  answered_at: null,
  acked_at: null,
} satisfies PendingInput;

function wrapper(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

describe("pending input hooks", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
  });

  it("polls only while an input remains open", () => {
    expect(pendingInputsRefetchInterval([openInput])).toBe(5_000);
    expect(pendingInputsRefetchInterval([{ ...openInput, state: "cancelled" }])).toBe(false);
    expect(pendingInputsRefetchInterval(undefined)).toBe(false);
  });

  it("loads one issue-scoped list only when enabled", async () => {
    const listIssuePendingInputs = vi.fn().mockResolvedValue({ data: [openInput] });
    setApiInstance({ listIssuePendingInputs } as unknown as ApiClient);

    const { result, rerender } = renderHook(
      ({ enabled }) => usePendingInputs("ws-1", "issue-1", enabled),
      { initialProps: { enabled: false }, wrapper: wrapper(queryClient) },
    );
    expect(listIssuePendingInputs).not.toHaveBeenCalled();

    rerender({ enabled: true });
    await waitFor(() => expect(result.current.data).toEqual([openInput]));
    expect(listIssuePendingInputs).toHaveBeenCalledTimes(1);
  });

  it("replaces the answered item in the issue cache", async () => {
    const answered = { ...openInput, state: "answered" as const };
    const answerIssuePendingInput = vi.fn().mockResolvedValue(answered);
    setApiInstance({ answerIssuePendingInput } as unknown as ApiClient);
    queryClient.setQueryData(issueKeys.pendingInputs("ws-1", "issue-1"), [openInput]);

    const { result } = renderHook(() => useAnswerPendingInput("ws-1", "issue-1"), {
      wrapper: wrapper(queryClient),
    });
    await act(async () => {
      await result.current.mutateAsync({
        pendingInputId: "input-1",
        answers: {},
      });
    });

    expect(queryClient.getQueryData(issueKeys.pendingInputs("ws-1", "issue-1"))).toEqual([
      answered,
    ]);
    expect(answerIssuePendingInput).toHaveBeenCalledTimes(1);
  });

  it("reuses the idempotency key when a committed answer loses its response", async () => {
    const answered = { ...openInput, state: "answered" as const };
    const answerIssuePendingInput = vi.fn()
      .mockRejectedValueOnce(new TypeError("network response lost"))
      .mockResolvedValueOnce(answered);
    setApiInstance({ answerIssuePendingInput } as unknown as ApiClient);

    const { result } = renderHook(() => useAnswerPendingInput("ws-1", "issue-1"), {
      wrapper: wrapper(queryClient),
    });
    await expect(result.current.mutateAsync({
      pendingInputId: "input-1",
      answers: {
        second: { answers: ["B"] },
        first: { answers: ["A"] },
      },
    })).rejects.toThrow("network response lost");
    await result.current.mutateAsync({
      pendingInputId: "input-1",
      answers: {
        first: { answers: ["A"] },
        second: { answers: ["B"] },
      },
    });

    const firstRequest = answerIssuePendingInput.mock.calls[0]?.[2];
    const retryRequest = answerIssuePendingInput.mock.calls[1]?.[2];
    expect(retryRequest.idempotency_key).toBe(firstRequest.idempotency_key);
    expect(retryRequest.answers).toEqual(firstRequest.answers);
  });

  it("isolates pending-input caches by workspace", () => {
    expect(issueKeys.pendingInputs("ws-1", "issue-1")).not.toEqual(
      issueKeys.pendingInputs("ws-2", "issue-1"),
    );
  });
});
