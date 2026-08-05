import type {
  TestCaseExecutionMode,
  TestCaseOrigin,
  TestCasePriority,
  TestCaseRepoRole,
  TestCaseScope,
  TestCaseStatus,
  TestCaseType,
} from "../types";

/**
 * Display metadata for the test case enums. Labels are i18n keys under the
 * `testing` namespace, never literal copy — `packages/core` has no i18n runtime
 * and views resolve them with `useT("testing")`.
 */
export const TEST_CASE_STATUSES: TestCaseStatus[] = ["draft", "active", "deprecated"];
export const TEST_CASE_PRIORITIES: TestCasePriority[] = ["p0", "p1", "p2", "p3"];
export const TEST_CASE_ORIGINS: TestCaseOrigin[] = ["ai", "human"];
export const TEST_CASE_SCOPES: TestCaseScope[] = ["single_repo", "cross_repo", "no_repo"];
export const TEST_CASE_EXECUTION_MODES: TestCaseExecutionMode[] = ["manual", "agent", "both"];
export const TEST_CASE_REPO_ROLES: TestCaseRepoRole[] = [
  "under_test",
  "driver",
  "verifier",
  "fixture",
];
export const TEST_CASE_TYPES: TestCaseType[] = [
  "functional",
  "business_flow",
  "api",
  "ui",
  "e2e",
  "regression",
  "boundary",
  "exception",
  "permission",
  "data_consistency",
  "compatibility",
  "performance",
  "security",
];

/** Semantic token classes, never hardcoded colors. */
export const TEST_CASE_STATUS_TONE: Record<TestCaseStatus, string> = {
  draft: "text-warning",
  active: "text-success",
  deprecated: "text-muted-foreground",
};

export const TEST_CASE_PRIORITY_TONE: Record<TestCasePriority, string> = {
  p0: "text-destructive",
  p1: "text-warning",
  p2: "text-muted-foreground",
  p3: "text-muted-foreground",
};
