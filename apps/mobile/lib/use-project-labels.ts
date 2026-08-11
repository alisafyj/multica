/**
 * Project status / priority labels bound to the active locale.
 *
 * Mirrors lib/use-issue-labels.ts; kept separate from lib/project-status.ts
 * so that module stays React-free and usable from the Node-only vitest lane.
 */
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { projectPriorityLabel, projectStatusLabel } from "./project-status";

export interface ProjectLabels {
  statusLabel: (value: string) => string;
  priorityLabel: (value: string) => string;
}

export function useProjectLabels(): ProjectLabels {
  const { t } = useTranslation("projects");
  return useMemo(
    () => ({
      statusLabel: (value: string) => projectStatusLabel(t, value),
      priorityLabel: (value: string) => projectPriorityLabel(t, value),
    }),
    [t],
  );
}
