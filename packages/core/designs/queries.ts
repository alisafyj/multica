import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { DesignSelectionInput } from "../types";
import { designKeys } from "./keys";

export function designFileListOptions(wsId: string) {
  return queryOptions({
    queryKey: designKeys.files(wsId),
    queryFn: () => api.listDesignFiles(),
    select: (data) => data.design_files,
  });
}

export function designFolderListOptions(wsId: string) {
  return queryOptions({
    queryKey: designKeys.folders(wsId),
    queryFn: () => api.listDesignFolders(),
    select: (data) => data.folders,
  });
}

export function designFileDetailOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: designKeys.file(wsId, id),
    queryFn: () => api.getDesignFile(id),
  });
}

export function designRevisionListOptions(wsId: string, fileId: string) {
  return queryOptions({
    queryKey: designKeys.revisions(wsId, fileId),
    queryFn: () => api.listDesignRevisions(fileId),
    select: (data) => data.revisions,
  });
}

export function designFileContextOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: designKeys.fileContext(wsId, id),
    queryFn: () => api.getDesignFileContext(id),
  });
}

export function designFrameContextOptions(wsId: string, fileId: string, frameId: string) {
  return queryOptions({
    queryKey: designKeys.frameContext(wsId, fileId, frameId),
    queryFn: () => api.getDesignFrameContext(fileId, frameId),
  });
}

export function designSelectionContextOptions(wsId: string, fileId: string, frameId: string, input: DesignSelectionInput) {
  return queryOptions({
    queryKey: designKeys.selectionContext(wsId, fileId, frameId, input),
    queryFn: () => api.getDesignSelectionContext(fileId, frameId, input),
  });
}

export function designRestoreTaskItemContextOptions(wsId: string, taskId: string, itemId: string) {
  return queryOptions({
    queryKey: designKeys.restoreTaskItemContext(wsId, taskId, itemId),
    queryFn: () => api.getDesignRestoreTaskItemContext(taskId, itemId),
  });
}

export function designRestoreTaskDetailOptions(wsId: string, taskId: string) {
  return queryOptions({
    queryKey: designKeys.restoreTask(wsId, taskId),
    queryFn: () => api.getDesignRestoreTask(taskId),
  });
}

export function designRestoreTaskListOptions(wsId: string) {
  return queryOptions({
    queryKey: designKeys.restoreTasks(wsId),
    queryFn: () => api.listDesignRestoreTasks(),
    select: (data) => data.tasks,
  });
}

export function designTemplateListOptions(wsId: string, params: { library_id?: string; category?: string } = {}) {
  return queryOptions({
    queryKey: designKeys.templates(wsId, params),
    queryFn: () => api.listDesignTemplates(params),
    select: (data) => data.templates,
  });
}

export function designTemplateDetailOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: designKeys.template(wsId, id),
    queryFn: () => api.getDesignTemplate(id),
  });
}

export function designDraftListOptions(wsId: string) {
  return queryOptions({
    queryKey: designKeys.drafts(wsId),
    queryFn: () => api.listDesignDrafts(),
    select: (data) => data.drafts,
  });
}

export function designDraftDetailOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: designKeys.draft(wsId, id),
    queryFn: () => api.getDesignDraft(id),
  });
}
