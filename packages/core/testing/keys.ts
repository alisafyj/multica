export interface TestCaseListFilters {
  projectId?: string;
  status?: string;
  module?: string;
  priority?: string;
  caseType?: string;
  origin?: string;
}

/**
 * Every key is workspace-scoped at index 1, and derived keys spread the parent
 * so `invalidateQueries({ queryKey: testCaseKeys.all(wsId) })` reaches lists,
 * details, modules and revisions in one call regardless of active filters.
 */
export const testCaseKeys = {
  all: (wsId: string) => ["test-cases", wsId] as const,
  list: (wsId: string, filters: TestCaseListFilters = {}) =>
    [...testCaseKeys.all(wsId), "list", filters] as const,
  detail: (wsId: string, ref: string) =>
    [...testCaseKeys.all(wsId), "detail", ref] as const,
  modules: (wsId: string, projectId: string) =>
    [...testCaseKeys.all(wsId), "modules", projectId] as const,
  revisions: (wsId: string, ref: string) =>
    [...testCaseKeys.all(wsId), "revisions", ref] as const,
  proposals: (wsId: string, ref: string) =>
    [...testCaseKeys.all(wsId), "proposals", ref] as const,
};

export interface TestGenerationJobListFilters {
  projectId?: string;
  status?: string;
}

/**
 * Generation jobs are workspace-scoped at index 1. The plan and proposals sit
 * under the job key so a single invalidation clears both when the job updates.
 */
export const testGenerationJobKeys = {
  all: (wsId: string) => ["test-generation-jobs", wsId] as const,
  list: (wsId: string, filters: TestGenerationJobListFilters = {}) =>
    [...testGenerationJobKeys.all(wsId), "list", filters] as const,
  detail: (wsId: string, id: string) =>
    [...testGenerationJobKeys.all(wsId), "detail", id] as const,
  plan: (wsId: string, jobId: string) =>
    [...testGenerationJobKeys.all(wsId), "plan", jobId] as const,
};
