import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import { projectKeys } from "../projects/queries";
import { issueKeys } from "../issues/queries";
import { pmoKeys } from "./queries";
import type {
  CreatePMOConfigRequest,
  UpdatePMOConfigRequest,
  PMOConflictResolution,
} from "../types";

/**
 * PMO mutations. Apply is deliberately conservative: it NEVER optimistically
 * updates the run or any project/issue list (apply can conflict and the
 * server is the only authority on the outcome), so every post-apply refresh
 * is an invalidate-and-refetch.
 */

export function useCreatePMOConfig() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: CreatePMOConfigRequest) => api.createPMOConfig(wsId, data),
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: pmoKeys.configs(wsId) });
    },
  });
}

export function useUpdatePMOConfig() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string } & UpdatePMOConfigRequest) =>
      api.updatePMOConfig(wsId, id, data),
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: pmoKeys.configs(wsId) });
    },
  });
}

export function useDeletePMOConfig() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.deletePMOConfig(wsId, id),
    onSettled: () => {
      // Deleting a config removes its runs/links server-side; drop the
      // whole PMO subtree for this workspace so no stale run list survives.
      void qc.invalidateQueries({ queryKey: pmoKeys.all(wsId) });
    },
  });
}

export function useStartPMORun() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (configId: string) => api.startPMORun(wsId, configId),
    onSettled: (_data, _err, configId) => {
      // The run list and the config's schedule columns both change.
      void qc.invalidateQueries({ queryKey: pmoKeys.runs(wsId, configId) });
      void qc.invalidateQueries({ queryKey: pmoKeys.configs(wsId) });
    },
  });
}

export function useApplyPMORun() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({
      runId,
      resolutions,
    }: {
      runId: string;
      resolutions?: PMOConflictResolution[];
    }) => api.applyPMORun(wsId, runId, resolutions),
    onSettled: () => {
      // Apply can create a project and issues and rewrite links. Refresh
      // the run views and every project/issue list for the workspace; the
      // server response is authoritative so nothing is set optimistically.
      void qc.invalidateQueries({ queryKey: pmoKeys.all(wsId) });
      void qc.invalidateQueries({ queryKey: projectKeys.all(wsId) });
      void qc.invalidateQueries({ queryKey: issueKeys.all(wsId) });
    },
  });
}

export function useSetPMOAssigneeMapping() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({
      configId,
      externalKey,
      memberId,
    }: {
      configId: string;
      externalKey: string;
      memberId: string;
    }) => api.setPMOAssigneeMapping(wsId, configId, externalKey, memberId),
    onSettled: (_data, _err, vars) => {
      // The mapping changes which assignees the next apply can resolve;
      // refresh the run views so the review surface re-renders.
      void qc.invalidateQueries({ queryKey: pmoKeys.runs(wsId, vars.configId) });
    },
  });
}
