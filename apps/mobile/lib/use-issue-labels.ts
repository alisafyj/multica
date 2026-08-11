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

export interface IssueLabels {
  statusLabel: (value: string) => string;
  priorityLabel: (value: string) => string;
}

export function useIssueLabels(): IssueLabels {
  const { t } = useTranslation("issues");
  // Memoised so consumers can pass these into memoised children without
  // forcing a re-render on every parent render (root CLAUDE.md UI rules).
  return useMemo(
    () => ({
      statusLabel: (value: string) => issueStatusLabel(t, value),
      priorityLabel: (value: string) => issuePriorityLabel(t, value),
    }),
    [t],
  );
}
