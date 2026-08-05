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
};
