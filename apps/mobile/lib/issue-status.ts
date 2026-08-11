/**
 * Mirror of the BOARD_STATUSES order from
 * packages/core/issues/config/status.ts, plus the locale-backed label
 * resolvers for issue status and priority.
 *
 * Mirrored, not imported: the source file co-exports `STATUS_CONFIG` with
 * web colour tokens (Tailwind v4 syntax) that mobile must not pull in.
 * Keeping this list owned by mobile keeps the import boundary clean.
 *
 * If web ever reorders BOARD_STATUSES or adds/removes a status, this file
 * must be updated to keep the "Counts and visibility must agree" rule
 * (apps/mobile/CLAUDE.md) intact.
 *
 * This module must stay React-free: lib/board-columns.ts imports
 * BOARD_STATUSES and lib/board-columns.test.ts runs in the Node-only vitest
 * lane (vitest.config.ts). Components read labels through useIssueLabels()
 * in lib/use-issue-labels.ts; pure modules such as lib/format-activity.ts
 * call the resolvers below with a `t` handed down by their caller.
 */
import type { IssueStatus } from "@multica/core/types";

/** Statuses surfaced in list/board views (matches web — `cancelled` excluded). */
export const BOARD_STATUSES: IssueStatus[] = [
  "backlog",
  "todo",
  "in_progress",
  "in_review",
  "done",
  "blocked",
];

/**
 * An i18next `t` already bound to the "issues" namespace — obtain it via
 * `useTranslation("issues")`. Passing a `t` bound to another namespace
 * resolves nothing and every label silently degrades to its raw value.
 */
export type IssueTranslate = (
  key: string,
  opts?: Record<string, unknown>,
) => string;

/**
 * Unknown values fall back to the raw server string instead of rendering
 * blank. Required by "State enums / transitions" in apps/mobile/CLAUDE.md
 * and "API Compatibility" in the repo-root CLAUDE.md: a status the server
 * introduces after this build shipped must still render a readable chip.
 */
export function issueStatusLabel(t: IssueTranslate, value: string): string {
  return t(`status.${value}`, { defaultValue: value });
}

export function issuePriorityLabel(t: IssueTranslate, value: string): string {
  return t(`priority.${value}`, { defaultValue: value });
}
