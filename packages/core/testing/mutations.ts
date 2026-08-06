import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import { testCaseKeys, testGenerationJobKeys } from "./keys";
import type {
  CreateTestCaseRequest,
  TestCase,
  UpdateTestCaseRequest,
  TestGenerationJob,
  TestGenerationPlan,
  TestCaseProposal,
  CreateTestGenerationJobRequest,
  UpdateTestGenerationPlanRequest,
  DispatchTestGenerationJobRequest,
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

// ---------------------------------------------------------------------------
// Test generation job mutations — Phase 2
// ---------------------------------------------------------------------------

/**
 * Create must await the server: the server allocates the job id we navigate to,
 * and idempotent re-create returns the existing in-flight job.
 */
export function useCreateTestGenerationJob() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: CreateTestGenerationJobRequest) =>
      api.createTestGenerationJob(data),
    onSuccess: (created) => {
      qc.setQueryData<TestGenerationJob>(
        testGenerationJobKeys.detail(wsId, created.id),
        created,
      );
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: testGenerationJobKeys.all(wsId) });
    },
  });
}

/**
 * Generate or re-generate the plan for a draft job. Optimistic: the user stays
 * on the same screen, and rollback is a cache restore. The server decides the
 * final plan content, so onSettled always invalidates.
 */
export function useGenerateTestGenerationPlan() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (jobId: string) => api.generateTestGenerationPlan(jobId),
    onSuccess: (plan, jobId) => {
      qc.setQueryData<TestGenerationPlan>(
        testGenerationJobKeys.plan(wsId, jobId),
        plan,
      );
    },
    onSettled: (_data, _error, jobId) => {
      qc.invalidateQueries({ queryKey: testGenerationJobKeys.plan(wsId, jobId) });
    },
  });
}

/**
 * Update the scope contract while the plan is still in `draft`. Optimistic:
 * stays on screen, rollback trivial, failure rare.
 */
export function useUpdateTestGenerationPlan() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ jobId, data }: { jobId: string; data: UpdateTestGenerationPlanRequest }) =>
      api.updateTestGenerationPlan(jobId, data),
    onMutate: async ({ jobId, data }) => {
      await qc.cancelQueries({ queryKey: testGenerationJobKeys.plan(wsId, jobId) });
      const previous = qc.getQueryData<TestGenerationPlan>(
        testGenerationJobKeys.plan(wsId, jobId),
      );
      if (previous && data.plan) {
        qc.setQueryData<TestGenerationPlan>(testGenerationJobKeys.plan(wsId, jobId), {
          ...previous,
          plan: data.plan as unknown as Record<string, unknown>,
          ...(data.review_notes !== undefined ? { review_notes: data.review_notes } : {}),
        });
      }
      return { previous, jobId };
    },
    onError: (_error, _vars, ctx) => {
      if (ctx?.previous) {
        qc.setQueryData(testGenerationJobKeys.plan(wsId, ctx.jobId), ctx.previous);
      }
    },
    onSettled: (_data, _error, { jobId }) => {
      qc.invalidateQueries({ queryKey: testGenerationJobKeys.plan(wsId, jobId) });
    },
  });
}

/**
 * Approve the plan. Optimistic: flips the plan status locally; stays on screen;
 * rollback is a cache restore; failure is rare because the server validates the
 * same repos the UI already validated.
 */
export function useApproveTestGenerationPlan() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (jobId: string) => api.approveTestGenerationPlan(jobId),
    onMutate: async (jobId) => {
      await qc.cancelQueries({ queryKey: testGenerationJobKeys.plan(wsId, jobId) });
      const previous = qc.getQueryData<TestGenerationPlan>(
        testGenerationJobKeys.plan(wsId, jobId),
      );
      if (previous) {
        qc.setQueryData<TestGenerationPlan>(testGenerationJobKeys.plan(wsId, jobId), {
          ...previous,
          status: "approved",
        });
      }
      return { previous, jobId };
    },
    onError: (_error, jobId, ctx) => {
      if (ctx?.previous) {
        qc.setQueryData(testGenerationJobKeys.plan(wsId, jobId), ctx.previous);
      }
    },
    onSettled: (_data, _error, jobId) => {
      qc.invalidateQueries({ queryKey: testGenerationJobKeys.plan(wsId, jobId) });
    },
  });
}

/**
 * Dispatch must await the server: it creates an agent task and we navigate to
 * the running job. No optimism — the server enforces plan-approved guard.
 */
export function useDispatchTestGenerationJob() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: DispatchTestGenerationJobRequest }) =>
      api.dispatchTestGenerationJob(id, data),
    onSuccess: ({ job }) => {
      qc.setQueryData<TestGenerationJob>(
        testGenerationJobKeys.detail(wsId, job.id),
        job,
      );
    },
    onSettled: (_data, _error, { id }) => {
      qc.invalidateQueries({ queryKey: testGenerationJobKeys.detail(wsId, id) });
      qc.invalidateQueries({ queryKey: testGenerationJobKeys.plan(wsId, id) });
    },
  });
}

/**
 * Accept a proposal. Optimistic: stays on the same detail screen, and
 * rolling back is a cache restore. Rare server failure cases (case deleted,
 * proposal already reviewed) invalidate on settle.
 */
export function useAcceptTestCaseProposal() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id }: { id: string; caseRef: string }) =>
      api.acceptTestCaseProposal(id),
    onMutate: async ({ id, caseRef }) => {
      await qc.cancelQueries({ queryKey: testCaseKeys.proposals(wsId, caseRef) });
      const previous = qc.getQueryData<TestCaseProposal[]>(
        testCaseKeys.proposals(wsId, caseRef),
      );
      if (previous) {
        qc.setQueryData<TestCaseProposal[]>(
          testCaseKeys.proposals(wsId, caseRef),
          previous.map((p) => (p.id === id ? { ...p, status: "accepted" } : p)),
        );
      }
      return { previous, caseRef };
    },
    onError: (_error, _vars, ctx) => {
      if (ctx?.previous) {
        qc.setQueryData(testCaseKeys.proposals(wsId, ctx.caseRef), ctx.previous);
      }
    },
    onSettled: (_data, _error, { caseRef }) => {
      qc.invalidateQueries({ queryKey: testCaseKeys.proposals(wsId, caseRef) });
      qc.invalidateQueries({ queryKey: testCaseKeys.detail(wsId, caseRef) });
    },
  });
}

/**
 * Reject a proposal. Same optimism rationale as accept.
 */
export function useRejectTestCaseProposal() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id }: { id: string; caseRef: string }) =>
      api.rejectTestCaseProposal(id),
    onMutate: async ({ id, caseRef }) => {
      await qc.cancelQueries({ queryKey: testCaseKeys.proposals(wsId, caseRef) });
      const previous = qc.getQueryData<TestCaseProposal[]>(
        testCaseKeys.proposals(wsId, caseRef),
      );
      if (previous) {
        qc.setQueryData<TestCaseProposal[]>(
          testCaseKeys.proposals(wsId, caseRef),
          previous.map((p) => (p.id === id ? { ...p, status: "rejected" } : p)),
        );
      }
      return { previous, caseRef };
    },
    onError: (_error, _vars, ctx) => {
      if (ctx?.previous) {
        qc.setQueryData(testCaseKeys.proposals(wsId, ctx.caseRef), ctx.previous);
      }
    },
    onSettled: (_data, _error, { caseRef }) => {
      qc.invalidateQueries({ queryKey: testCaseKeys.proposals(wsId, caseRef) });
      qc.invalidateQueries({ queryKey: testCaseKeys.detail(wsId, caseRef) });
    },
  });
}
