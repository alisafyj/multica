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

// ---------------------------------------------------------------------------
// Test generation jobs — Phase 2
// ---------------------------------------------------------------------------

export type TestGenerationJobStatus = "queued" | "running" | "completed" | "failed" | "cancelled";
export type TestGenerationPlanStatus = "draft" | "approved" | "dispatched" | "archived";
export type TestCaseProposalKind = "update" | "obsolete";
export type TestCaseProposalStatus = "pending" | "accepted" | "rejected";

/**
 * One repository the generation run may read, scoped to specific path globs.
 * Bound by project_resource_id because repository URLs change but resource IDs
 * are stable within the workspace.
 */
export interface TestGenerationPlanRepo {
  project_resource_id: string;
  alias: string;
  url?: string;
  ref?: string;
  path_globs: string[];
}

/**
 * The human-reviewed scope contract. A human edits and approves this before any
 * tokens are spent on generation.
 */
export interface TestGenerationPlanPayload {
  version: string;
  repos: TestGenerationPlanRepo[];
  issues: string[];
  modules: string[];
  knowledge_refs: string[];
  attachment_ids: string[];
  expected_case_types: string[];
  existing_case_digest_count: number;
  instructions: string;
}

export interface TestGenerationJob {
  id: string;
  workspace_id: string;
  project_id: string;
  agent_id: string | null;
  agent_task_id: string | null;
  status: TestGenerationJobStatus;
  input: Record<string, unknown>;
  result: Record<string, unknown>;
  error: string | null;
  created_by: string | null;
  created_at: string;
  updated_at: string;
}

export interface TestGenerationPlan {
  id: string;
  workspace_id: string;
  job_id: string;
  status: TestGenerationPlanStatus;
  plan: Record<string, unknown>;
  review_notes: string;
  approved_by: string | null;
  approved_at: string | null;
  created_by: string | null;
  created_at: string;
  updated_at: string;
}

/**
 * A suggested change to an existing test case. `new` cases land directly as
 * drafts; only `update` and `obsolete` against an approved case come through
 * here so a human can decide.
 */
export interface TestCaseProposal {
  id: string;
  workspace_id: string;
  job_id: string;
  target_case_id: string;
  kind: TestCaseProposalKind;
  payload: Record<string, unknown>;
  rationale: string;
  status: TestCaseProposalStatus;
  reviewed_by: string | null;
  reviewed_at: string | null;
  created_at: string;
}

// Request types

export interface CreateTestGenerationJobRequest {
  project_id: string;
  issue_ids?: string[];
  modules?: string[];
  attachment_ids?: string[];
  instructions?: string;
}

export interface UpdateTestGenerationPlanRequest {
  plan?: TestGenerationPlanPayload;
  review_notes?: string;
}

export interface DispatchTestGenerationJobRequest {
  agent_id: string;
  prompt?: string;
}

// Response types

export interface ListTestGenerationJobsResponse {
  jobs: TestGenerationJob[];
  total: number;
}

export interface ListTestCaseProposalsResponse {
  proposals: TestCaseProposal[];
  total: number;
}

export interface DispatchTestGenerationJobResponse {
  job: TestGenerationJob;
  agent_task_id: string;
}
