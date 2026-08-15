/**
 * Key segment for the project-level design system: the one shared across
 * repositories and used when no repository is picked (DC-052). Repository
 * scopes use their `project_resource_id`, which is a UUID and can never
 * collide with this sentinel.
 */
export const PROJECT_LEVEL_DESIGN_SCOPE = "project-level";

export const designKeys = {
  all: (wsId: string) => ["designs", wsId] as const,
  folders: (wsId: string) => ["designs", wsId, "folders"] as const,
  files: (wsId: string) => ["designs", wsId, "files"] as const,
  file: (wsId: string, id: string) => ["designs", wsId, "files", id] as const,
  fileContext: (wsId: string, id: string, revisionId?: string) => ["designs", wsId, "files", id, "context", revisionId ?? "current"] as const,
  frameContext: (wsId: string, fileId: string, frameId: string, revisionId?: string) => ["designs", wsId, "files", fileId, "frames", frameId, "context", revisionId ?? "current"] as const,
  selectionContext: (wsId: string, fileId: string, frameId: string, input: unknown, revisionId?: string) => ["designs", wsId, "files", fileId, "frames", frameId, "selection-context", revisionId ?? "current", input] as const,
  revisions: (wsId: string, fileId: string) => ["designs", wsId, "files", fileId, "revisions"] as const,
  revision: (wsId: string, revisionId: string) => ["designs", wsId, "revisions", revisionId] as const,
  templates: (wsId: string, params?: unknown) => ["designs", wsId, "templates", params ?? {}] as const,
  template: (wsId: string, id: string) => ["designs", wsId, "templates", id] as const,
  designSystems: (wsId: string, projectId?: string) => ["designs", wsId, "design-systems", projectId ?? "all"] as const,
  designSystem: (wsId: string, id: string) => ["designs", wsId, "design-systems", id] as const,
  projectDesignSystems: (wsId: string) => ["designs", wsId, "project-design-systems"] as const,
  projectDesignSystemProjectScopes: (wsId: string, projectId: string) => ["designs", wsId, "project-design-systems", "project", projectId] as const,
  projectDesignSystemByProject: (wsId: string, projectId: string, projectResourceId?: string | null) => ["designs", wsId, "project-design-systems", "project", projectId, projectResourceId ? projectResourceId : PROJECT_LEVEL_DESIGN_SCOPE] as const,
  projectDesignSystem: (wsId: string, id: string) => ["designs", wsId, "project-design-systems", "system", id] as const,
  projectDesignSystemPackagePreview: (wsId: string, id: string) => ["designs", wsId, "project-design-systems", "system", id, "package-preview"] as const,
  drafts: (wsId: string) => ["designs", wsId, "drafts"] as const,
  draft: (wsId: string, id: string) => ["designs", wsId, "drafts", id] as const,
  repoAnalyses: (wsId: string, projectId: string) => ["designs", wsId, "repo-analyses", projectId] as const,
  repoAnalysis: (wsId: string, id: string) => ["designs", wsId, "repo-analysis", id] as const,
  deliveriesByIssue: (wsId: string, issueId: string) => ["designs", wsId, "deliveries", "issue", issueId] as const,
  restoreTasks: (wsId: string) => ["designs", wsId, "restore-tasks"] as const,
  restoreTask: (wsId: string, id: string) => ["designs", wsId, "restore-tasks", id] as const,
  restoreMappings: (wsId: string, taskId: string) => ["designs", wsId, "restore-tasks", taskId, "mappings"] as const,
  restorePlan: (wsId: string, taskId: string) => ["designs", wsId, "restore-tasks", taskId, "plan"] as const,
  restoreTaskItemContext: (wsId: string, taskId: string, itemId: string) => ["designs", wsId, "restore-tasks", taskId, "items", itemId, "context"] as const,
};
