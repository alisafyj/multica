import type { TestCase, TestCaseStep } from "@multica/core/types";

/** A module bucket for the list page's left rail. */
export interface TestCaseModuleGroup {
  module: string;
  cases: TestCase[];
}

/**
 * Group cases by module. Cases with no module land in one explicit `""` bucket
 * that sorts first, so "ungrouped" is a visible state rather than a silent
 * omission.
 */
export function groupByModule(cases: TestCase[]): TestCaseModuleGroup[] {
  const buckets = new Map<string, TestCase[]>();
  for (const testCase of cases) {
    const module = testCase.module ?? "";
    const bucket = buckets.get(module);
    if (bucket) {
      bucket.push(testCase);
    } else {
      buckets.set(module, [testCase]);
    }
  }
  return [...buckets.entries()]
    .map(([module, grouped]) => ({ module, cases: grouped }))
    .sort((a, b) => {
      if (a.module === b.module) return 0;
      if (a.module === "") return -1;
      if (b.module === "") return 1;
      return a.module.localeCompare(b.module);
    });
}

/**
 * Renumber steps to 1..n. Deleting a step in the middle otherwise leaves a gap,
 * and an agent executing the case reads the index as the running order.
 */
export function normalizeStepIndexes(steps: TestCaseStep[]): TestCaseStep[] {
  return steps.map((step, position) => ({ ...step, index: position + 1 }));
}

/** `"admin-web(driver), mobile-app(verifier)"` for the list column. */
export function formatRepoSummary(testCase: TestCase): string {
  const repos = testCase.repos ?? [];
  return repos.map((repo) => `${repo.alias}(${repo.role})`).join(", ");
}

export type CrossRepoWarning = "missing_repos" | "single_role";

/**
 * A cross-repo case has to name at least two repositories AND give them more
 * than one role — "change data in the backend, verify in the app" is only
 * expressible when the roles differ. Returns null when the case is coherent, or
 * when it is not cross-repo at all.
 */
export function crossRepoWarning(testCase: TestCase): CrossRepoWarning | null {
  if (testCase.scope !== "cross_repo") return null;
  const repos = testCase.repos ?? [];
  if (repos.length < 2) return "missing_repos";
  const roles = new Set(repos.map((repo) => repo.role));
  if (roles.size < 2) return "single_role";
  return null;
}

/** Aliases a step's repo selector may point at. */
export function repoAliases(testCase: Pick<TestCase, "repos">): string[] {
  return (testCase.repos ?? []).map((repo) => repo.alias).filter((alias) => alias.length > 0);
}

/**
 * Narrow a server-provided enum value to a known key before using it as an i18n
 * lookup. Schemas are lenient by design, so an unknown value must render as
 * itself rather than blow up the row.
 */
export function knownEnumKey<T extends string>(value: string, allowed: readonly T[]): T | null {
  return (allowed as readonly string[]).includes(value) ? (value as T) : null;
}
