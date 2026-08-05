import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import { testCaseKeys } from "./keys";
import type {
  CreateTestCaseRequest,
  TestCase,
  UpdateTestCaseRequest,
} from "../types";

/**
 * Create is deliberately not optimistic: the server allocates the TC-<n> key
 * and the id the caller navigates to, so there is nothing correct to render
 * before it answers.
 */
export function useCreateTestCase() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: CreateTestCaseRequest) => api.createTestCase(data),
    onSuccess: (created) => {
      qc.setQueryData<TestCase>(testCaseKeys.detail(wsId, created.id), created);
      qc.setQueryData<TestCase>(testCaseKeys.detail(wsId, created.key), created);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: testCaseKeys.all(wsId) });
    },
  });
}

/**
 * Update is optimistic: the outcome is locally predictable, the user stays on
 * the same screen, and rollback is a cache restore. `version` and `updated_at`
 * are server-decided, so onSettled always re-reads.
 */
export function useUpdateTestCase() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ ref, ...data }: { ref: string } & UpdateTestCaseRequest) =>
      api.updateTestCase(ref, data),
    onMutate: async ({ ref, ...data }) => {
      await qc.cancelQueries({ queryKey: testCaseKeys.detail(wsId, ref) });
      const previous = qc.getQueryData<TestCase>(testCaseKeys.detail(wsId, ref));
      if (previous) {
        // `repos` and `note` are not patchable client-side: repos needs the
        // server-resolved binding rows and note is write-only.
        const { repos: _repos, note: _note, ...patch } = data;
        qc.setQueryData<TestCase>(testCaseKeys.detail(wsId, ref), {
          ...previous,
          ...patch,
        });
      }
      return { previous, ref };
    },
    onError: (_error, _vars, ctx) => {
      if (ctx?.previous) {
        qc.setQueryData(testCaseKeys.detail(wsId, ctx.ref), ctx.previous);
      }
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: testCaseKeys.all(wsId) });
    },
  });
}

/** Approve is a single status flip — same optimism rationale as update. */
export function useApproveTestCase() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (ref: string) => api.approveTestCase(ref),
    onMutate: async (ref) => {
      await qc.cancelQueries({ queryKey: testCaseKeys.detail(wsId, ref) });
      const previous = qc.getQueryData<TestCase>(testCaseKeys.detail(wsId, ref));
      if (previous) {
        qc.setQueryData<TestCase>(testCaseKeys.detail(wsId, ref), {
          ...previous,
          status: "active",
        });
      }
      return { previous, ref };
    },
    onError: (_error, ref, ctx) => {
      if (ctx?.previous) {
        qc.setQueryData(testCaseKeys.detail(wsId, ref), ctx.previous);
      }
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: testCaseKeys.all(wsId) });
    },
  });
}

/**
 * Delete navigates away on success, so it must await the server: optimistically
 * dropping the case would strand the user on a route whose entity may still
 * exist if the request failed.
 */
export function useDeleteTestCase() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (ref: string) => api.deleteTestCase(ref),
    onSuccess: (_data, ref) => {
      qc.removeQueries({ queryKey: testCaseKeys.detail(wsId, ref) });
      qc.removeQueries({ queryKey: testCaseKeys.revisions(wsId, ref) });
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: testCaseKeys.all(wsId) });
    },
  });
}
