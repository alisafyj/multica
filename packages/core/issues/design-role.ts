import type { Issue, IssueMetadata } from "../types";

export const ISSUE_DESIGN_ROLE_KEY = "design_role";
export const ISSUE_DESIGN_ROLE_UI = "ui_design";
export const ISSUE_DESIGN_ROLE_FRONTEND = "frontend_dev";

export type IssueDesignRole = typeof ISSUE_DESIGN_ROLE_UI | typeof ISSUE_DESIGN_ROLE_FRONTEND;

function titleLooksLikeUiDesignIssue(title: string) {
  return /ui/i.test(title) || title.includes("设计");
}

function titleLooksLikeFrontendDevIssue(title: string) {
  const normalized = title.toLowerCase().trim();
  return normalized.includes("前端") || normalized.includes("frontend");
}

export function explicitIssueDesignRole(metadata: IssueMetadata | undefined): IssueDesignRole | null {
  const explicitRole = metadata?.[ISSUE_DESIGN_ROLE_KEY];
  if (explicitRole === ISSUE_DESIGN_ROLE_UI || explicitRole === ISSUE_DESIGN_ROLE_FRONTEND) return explicitRole;
  return null;
}

export function inferIssueDesignRoleFromTitle(title: string): IssueDesignRole | null {
  if (titleLooksLikeUiDesignIssue(title)) return ISSUE_DESIGN_ROLE_UI;
  if (titleLooksLikeFrontendDevIssue(title)) return ISSUE_DESIGN_ROLE_FRONTEND;
  return null;
}

export function issueDesignRole(issue: Pick<Issue, "title" | "metadata">): IssueDesignRole | null {
  return explicitIssueDesignRole(issue.metadata) ?? inferIssueDesignRoleFromTitle(issue.title);
}
