export type TestCasePriority = "p0" | "p1" | "p2" | "p3";
export type TestCaseStatus = "draft" | "active" | "deprecated";
export type TestCaseOrigin = "ai" | "human";
export type TestCaseScope = "single_repo" | "cross_repo" | "no_repo";
export type TestCaseExecutionMode = "manual" | "agent" | "both";
export type TestCaseRepoRole = "under_test" | "driver" | "verifier" | "fixture";
export type TestCaseChangeKind =
  | "human_edit"
  | "proposal_accepted"
  | "status_change"
  | "restore";

export type TestCaseType =
  | "functional"
  | "business_flow"
  | "api"
  | "ui"
  | "e2e"
  | "regression"
  | "boundary"
  | "exception"
  | "permission"
  | "data_consistency"
  | "compatibility"
  | "performance"
  | "security";

/**
 * One row of a case's procedure. Steps are a typed array rather than a markdown
 * blob so an agent can execute them; `repo` names a {@link TestCaseRepo} alias
 * when the step runs against a specific repository of a multi-repo project.
 */
export interface TestCaseStep {
  index: number;
  action: string;
  expected: string;
  repo?: string;
}

/**
 * Binds a case to one repository of its project. The binding is by
 * `project_resource_id`, not a repo URL: URLs change, resource ids are stable
 * within the workspace and are already shipped to agents in the task claim.
 */
export interface TestCaseRepo {
  project_resource_id: string;
  alias: string;
  role: TestCaseRepoRole;
  path_globs: string[];
}

export interface TestCase {
  id: string;
  workspace_id: string;
  project_id: string;
  case_number: number;
  /** Human-readable key, `TC-42`. Accepted anywhere an id is. */
  key: string;
  title: string;
  module: string;
  preconditions: string;
  steps: TestCaseStep[];
  expected_result: string;
  test_data: Record<string, unknown>;
  priority: TestCasePriority;
  case_type: TestCaseType;
  scope: TestCaseScope;
  execution_mode: TestCaseExecutionMode;
  required_capabilities: Record<string, unknown>[];
  business_rules_ref: string[];
  status: TestCaseStatus;
  origin: TestCaseOrigin;
  source_refs: Record<string, unknown>;
  generation_job_id: string | null;
  version: number;
  repos: TestCaseRepo[];
  created_by: string | null;
  updated_by: string | null;
  reviewed_by: string | null;
  reviewed_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface TestCaseRevision {
  id: string;
  test_case_id: string;
  version: number;
  /** The case as it was BEFORE the change this revision records. */
  snapshot: Record<string, unknown>;
  change_kind: TestCaseChangeKind;
  changed_by: string | null;
  changed_by_type: "member" | "agent";
  note: string;
  created_at: string;
}

export interface TestCaseModule {
  module: string;
  case_count: number;
}

export interface TestCaseRepoInput {
  project_resource_id: string;
  alias: string;
  role?: TestCaseRepoRole;
  path_globs?: string[];
}

export interface CreateTestCaseRequest {
  project_id: string;
  title: string;
  module?: string;
  preconditions?: string;
  steps?: TestCaseStep[];
  expected_result?: string;
  test_data?: Record<string, unknown>;
  priority?: TestCasePriority;
  case_type?: TestCaseType;
  scope?: TestCaseScope;
  execution_mode?: TestCaseExecutionMode;
  required_capabilities?: Record<string, unknown>[];
  business_rules_ref?: string[];
  status?: TestCaseStatus;
  repos?: TestCaseRepoInput[];
}

export interface UpdateTestCaseRequest {
  title?: string;
  module?: string;
  preconditions?: string;
  steps?: TestCaseStep[];
  expected_result?: string;
  test_data?: Record<string, unknown>;
  priority?: TestCasePriority;
  case_type?: TestCaseType;
  scope?: TestCaseScope;
  execution_mode?: TestCaseExecutionMode;
  required_capabilities?: Record<string, unknown>[];
  business_rules_ref?: string[];
  status?: TestCaseStatus;
  repos?: TestCaseRepoInput[];
  note?: string;
}

export interface ListTestCasesResponse {
  test_cases: TestCase[];
  total: number;
}

export interface ListTestCaseModulesResponse {
  modules: TestCaseModule[];
}

export interface ListTestCaseRevisionsResponse {
  revisions: TestCaseRevision[];
}
