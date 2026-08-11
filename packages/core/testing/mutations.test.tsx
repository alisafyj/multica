/**
 * @vitest-environment jsdom
 */
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import { EMPTY_TEST_CASE } from "../api/schemas";
import type { TestCase } from "../types";
import { testCaseKeys } from "./keys";
import {
  useApproveTestCase,
  useCreateTestCase,
  useDeleteTestCase,
  useUpdateTestCase,
} from "./mutations";

vi.mock("../hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

function createWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

const CASE: TestCase = {
  ...EMPTY_TEST_CASE,
  id: "case-1",
  key: "TC-1",
  title: "原标题",
  priority: "p2",
  status: "draft",
};

describe("test case mutations", () => {
  let qc: QueryClient;

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  });

  it("patches the detail cache optimistically on update", async () => {
    let resolveUpdate: (value: TestCase) => void = () => {};
    const updateTestCase = vi.fn(
      () => new Promise<TestCase>((resolve) => { resolveUpdate = resolve; }),
    );
    setApiInstance({ updateTestCase } as unknown as ApiClient);
    qc.setQueryData(testCaseKeys.detail("ws-1", "TC-1"), CASE);

    const { result } = renderHook(() => useUpdateTestCase(), { wrapper: createWrapper(qc) });
    act(() => {
      result.current.mutate({ ref: "TC-1", title: "新标题" });
    });

    await waitFor(() => {
      expect(qc.getQueryData<TestCase>(testCaseKeys.detail("ws-1", "TC-1"))?.title).toBe("新标题");
    });

    await act(async () => {
      resolveUpdate({ ...CASE, title: "新标题", version: 2 });
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
  });

  it("rolls the title back when update fails", async () => {
    const updateTestCase = vi.fn().mockRejectedValue(new Error("boom"));
    setApiInstance({ updateTestCase } as unknown as ApiClient);
    qc.setQueryData(testCaseKeys.detail("ws-1", "TC-1"), CASE);

    const { result } = renderHook(() => useUpdateTestCase(), { wrapper: createWrapper(qc) });
    act(() => {
      result.current.mutate({ ref: "TC-1", title: "新标题" });
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(qc.getQueryData<TestCase>(testCaseKeys.detail("ws-1", "TC-1"))?.title).toBe("原标题");
  });

  it("does not send repos or note into the optimistic patch", async () => {
    let resolveUpdate: (value: TestCase) => void = () => {};
    const updateTestCase = vi.fn(
      () => new Promise<TestCase>((resolve) => { resolveUpdate = resolve; }),
    );
    setApiInstance({ updateTestCase } as unknown as ApiClient);
    qc.setQueryData(testCaseKeys.detail("ws-1", "TC-1"), CASE);

    const { result } = renderHook(() => useUpdateTestCase(), { wrapper: createWrapper(qc) });
    act(() => {
      result.current.mutate({
        ref: "TC-1",
        repos: [{ project_resource_id: "r1", alias: "admin-web" }],
        note: "changed the repos",
      });
    });

    await waitFor(() => expect(updateTestCase).toHaveBeenCalled());
    const patched = qc.getQueryData<TestCase>(testCaseKeys.detail("ws-1", "TC-1"));
    expect(patched?.repos).toEqual([]);
    expect(patched).not.toHaveProperty("note");

    await act(async () => {
      resolveUpdate({ ...CASE, repos: [{ project_resource_id: "r1", alias: "admin-web", role: "under_test", path_globs: [] }] });
    });
  });

  it("flips status to active optimistically on approve", async () => {
    let resolveApprove: (value: TestCase) => void = () => {};
    const approveTestCase = vi.fn(
      () => new Promise<TestCase>((resolve) => { resolveApprove = resolve; }),
    );
    setApiInstance({ approveTestCase } as unknown as ApiClient);
    qc.setQueryData(testCaseKeys.detail("ws-1", "TC-1"), CASE);

    const { result } = renderHook(() => useApproveTestCase(), { wrapper: createWrapper(qc) });
    act(() => {
      result.current.mutate("TC-1");
    });

    await waitFor(() => {
      expect(qc.getQueryData<TestCase>(testCaseKeys.detail("ws-1", "TC-1"))?.status).toBe("active");
    });

    await act(async () => {
      resolveApprove({ ...CASE, status: "active" });
    });
  });

  it("keeps the case in cache until the server confirms deletion", async () => {
    let resolveDelete: () => void = () => {};
    const deleteTestCase = vi.fn(
      () => new Promise<void>((resolve) => { resolveDelete = () => resolve(); }),
    );
    setApiInstance({ deleteTestCase } as unknown as ApiClient);
    qc.setQueryData(testCaseKeys.detail("ws-1", "TC-1"), CASE);

    const { result } = renderHook(() => useDeleteTestCase(), { wrapper: createWrapper(qc) });
    act(() => {
      result.current.mutate("TC-1");
    });

    await waitFor(() => expect(deleteTestCase).toHaveBeenCalledWith("TC-1"));
    expect(qc.getQueryData(testCaseKeys.detail("ws-1", "TC-1"))).toEqual(CASE);

    await act(async () => {
      resolveDelete();
    });
    await waitFor(() => {
      expect(qc.getQueryData(testCaseKeys.detail("ws-1", "TC-1"))).toBeUndefined();
    });
  });

  it("caches a created case under both its id and its TC key", async () => {
    const created: TestCase = { ...CASE, id: "case-9", key: "TC-9", status: "active" };
    const createTestCase = vi.fn().mockResolvedValue(created);
    setApiInstance({ createTestCase } as unknown as ApiClient);

    const { result } = renderHook(() => useCreateTestCase(), { wrapper: createWrapper(qc) });
    act(() => {
      result.current.mutate({ project_id: "p1", title: "新用例" });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(qc.getQueryData(testCaseKeys.detail("ws-1", "case-9"))).toEqual(created);
    expect(qc.getQueryData(testCaseKeys.detail("ws-1", "TC-9"))).toEqual(created);
  });
});
