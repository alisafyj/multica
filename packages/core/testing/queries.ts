import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import { testCaseKeys, type TestCaseListFilters } from "./keys";

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
