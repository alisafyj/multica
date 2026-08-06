import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import { testCaseKeys, testGenerationJobKeys, type TestCaseListFilters, type TestGenerationJobListFilters } from "./keys";

export function testCaseListOptions(wsId: string, filters: TestCaseListFilters = {}) {
  return queryOptions({
    queryKey: testCaseKeys.list(wsId, filters),
    queryFn: () => api.listTestCases(filters),
    select: (data) => data.test_cases,
  });
}

export function testCaseDetailOptions(wsId: string, ref: string) {
  return queryOptions({
    queryKey: testCaseKeys.detail(wsId, ref),
    queryFn: () => api.getTestCase(ref),
    enabled: ref.length > 0,
  });
}

export function testCaseModulesOptions(wsId: string, projectId: string) {
  return queryOptions({
    queryKey: testCaseKeys.modules(wsId, projectId),
    queryFn: () => api.listTestCaseModules(projectId),
    select: (data) => data.modules,
    enabled: projectId.length > 0,
  });
}

export function testCaseRevisionsOptions(wsId: string, ref: string) {
  return queryOptions({
    queryKey: testCaseKeys.revisions(wsId, ref),
    queryFn: () => api.listTestCaseRevisions(ref),
    select: (data) => data.revisions,
    enabled: ref.length > 0,
  });
}

export function testCaseProposalsOptions(wsId: string, ref: string, status?: string) {
  return queryOptions({
    queryKey: testCaseKeys.proposals(wsId, ref),
    queryFn: () => api.listTestCaseProposals(ref, status),
    select: (data) => data.proposals,
    enabled: ref.length > 0,
  });
}

// ---------------------------------------------------------------------------
// Test generation job queries — Phase 2
// ---------------------------------------------------------------------------

export function testGenerationJobListOptions(
  wsId: string,
  filters: TestGenerationJobListFilters = {},
) {
  return queryOptions({
    queryKey: testGenerationJobKeys.list(wsId, filters),
    queryFn: () => api.listTestGenerationJobs(filters),
    select: (data) => data.jobs,
  });
}

export function testGenerationJobDetailOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: testGenerationJobKeys.detail(wsId, id),
    queryFn: () => api.getTestGenerationJob(id),
    enabled: id.length > 0,
  });
}

export function testGenerationPlanOptions(wsId: string, jobId: string) {
  return queryOptions({
    queryKey: testGenerationJobKeys.plan(wsId, jobId),
    queryFn: () => api.getTestGenerationPlan(jobId),
    enabled: jobId.length > 0,
  });
}
