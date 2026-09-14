import { useQuery } from "@tanstack/react-query";
import { api } from "../api";

export const taskRunEvidenceKeys = {
  detail: (wsId: string, taskId: string) =>
    ["task-run-evidence", wsId, taskId] as const,
};

export function useTaskRunEvidence(wsId: string, taskId: string, enabled: boolean) {
  return useQuery({
    queryKey: taskRunEvidenceKeys.detail(wsId, taskId),
    queryFn: () => api.listTaskRunEvidence(taskId),
    enabled: enabled && taskId.length > 0,
    staleTime: 30_000,
    refetchOnWindowFocus: false,
  });
}
