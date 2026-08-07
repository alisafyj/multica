import type { TestCaseProposal } from "@multica/core/types";

// ---------------------------------------------------------------------------
// Proposal statistics
// ---------------------------------------------------------------------------

export interface ProposalStats {
  total: number;
  pending: number;
  accepted: number;
  rejected: number;
}

/**
 * Aggregate proposal status counts. Unknown statuses (from a newer backend) are
 * treated as pending so the total is always consistent and the UI degrades
 * gracefully.
 */
export function aggregateProposalStats(proposals: TestCaseProposal[]): ProposalStats {
  let pending = 0;
  let accepted = 0;
  let rejected = 0;
  for (const p of proposals) {
    if (p.status === "accepted") {
      accepted++;
    } else if (p.status === "rejected") {
      rejected++;
    } else {
      // "pending" and any future status the backend may emit
      pending++;
    }
  }
  return { total: proposals.length, pending, accepted, rejected };
}

// ---------------------------------------------------------------------------
// Plan JSON validation
// ---------------------------------------------------------------------------

/**
 * Validate the user's plan JSON draft. Returns null when the draft is a valid
 * JSON object; returns a human-readable error string otherwise. The validation
 * is intentionally permissive: any JSON object is accepted so the user can add
 * keys that a future backend version will understand.
 */
export function validatePlanJson(draft: string): string | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(draft);
  } catch (err) {
    return err instanceof Error ? err.message : "Invalid JSON";
  }
  if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
    return "Plan must be a JSON object, not an array or primitive";
  }
  return null;
}

// ---------------------------------------------------------------------------
// Plan field accessors — defensive reads from Record<string, unknown>
// ---------------------------------------------------------------------------

/**
 * Extract repository aliases from the plan. The plan's `repos` array holds
 * objects with an `alias` field; anything that does not match is silently
 * skipped so a partially-formed plan never throws.
 */
export function planRepos(plan: Record<string, unknown>): string[] {
  const repos = plan.repos;
  if (!Array.isArray(repos)) return [];
  return repos
    .filter(
      (r): r is Record<string, unknown> =>
        r !== null && typeof r === "object" && !Array.isArray(r),
    )
    .map((r) => (typeof r.alias === "string" ? r.alias : ""))
    .filter((alias) => alias.length > 0);
}

/** Extract module names from the plan's `modules` array. */
export function planModules(plan: Record<string, unknown>): string[] {
  const modules = plan.modules;
  if (!Array.isArray(modules)) return [];
  return modules.filter((m): m is string => typeof m === "string");
}

/** Extract the free-text instructions from the plan, or empty string if absent. */
export function planInstructions(plan: Record<string, unknown>): string {
  const instructions = plan.instructions;
  return typeof instructions === "string" ? instructions : "";
}
