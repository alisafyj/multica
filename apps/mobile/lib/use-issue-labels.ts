/**
 * Issue status / priority labels bound to the active locale.
 *
 * Separate from lib/issue-status.ts because that module must stay
 * React-free — lib/board-columns.ts imports it and runs in the Node-only
 * vitest lane (vitest.config.ts).
 */
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { issuePriorityLabel, issueStatusLabel } from "./issue-status";
import { useIssueStatuses } from "./use-issue-statuses";

export interface IssueLabels {
  statusLabel: (value: string) => string;
  priorityLabel: (value: string) => string;
}

export function useIssueLabels(): IssueLabels {
  const { t } = useTranslation("issues");
  // MUL-6243: a CUSTOM status has no translation key, so the workspace catalog
  // supplies its name. Resolving it here means every consumer of this hook
  // gets custom statuses without repeating the fallback at each call site.
  const { labelOf } = useIssueStatuses();
  // Memoised so consumers can pass these into memoised children without
  // forcing a re-render on every parent render (root CLAUDE.md UI rules).
  return useMemo(
    () => ({
      statusLabel: (value: string) => issueStatusLabel(t, value, labelOf(value)),
      priorityLabel: (value: string) => issuePriorityLabel(t, value),
    }),
    [t, labelOf],
  );
}
