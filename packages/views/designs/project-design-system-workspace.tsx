"use client";

import { GitBranch, LoaderCircle, Package } from "lucide-react";
import type {
  Agent,
  DesignFile,
  DesignSystemProfile,
  Project,
  ProjectDesignSystem,
  ProjectResource,
} from "@multica/core/types";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@multica/ui/components/ui/toggle-group";
import { ProjectDesignSystemCanvas } from "./project-design-system-canvas";
import { ProjectDesignSystemCreate } from "./project-design-system-create";
import { ProjectDesignSystemTaskActivity } from "./project-design-system-task-activity";
import { repositoryLabel, repositoryUrl } from "./project-repository";

// Base UI toggle values must be non-empty strings, while the project-level
// scope is an empty `project_resource_id` on the wire (DC-052).
const PROJECT_SCOPE_VALUE = "__project__";

/**
 * Repository scope switcher (DC-052). A project can hold several repositories
 * — a consumer H5 site, a mobile app, an admin console — and each keeps its
 * own design system, with the project-level one shared across them.
 *
 * This is a scope switch, not a list page and not a secondary entry: the
 * content main view below keeps rendering directly (DC-031).
 */
function ProjectDesignSystemScopeSwitcher({
  repositories,
  selectedRepositoryId,
  onSelectRepository,
  fallbackNotice,
}: {
  repositories: ProjectResource[];
  selectedRepositoryId: string;
  onSelectRepository: (projectResourceId: string) => void;
  fallbackNotice: boolean;
}) {
  if (repositories.length === 0) return null;
  const activeValue = selectedRepositoryId || PROJECT_SCOPE_VALUE;
  // Repository names can repeat across hosts and get truncated, so the title
  // carries the full URL.
  const scopes = [
    { value: PROJECT_SCOPE_VALUE, label: "项目通用", title: "跨仓库通用的项目级设计体系" },
    ...repositories.map((repository) => {
      const label = repositoryLabel(repository);
      return { value: repository.id, label, title: repositoryUrl(repository) || label };
    }),
  ];
  return (
    <div className="shrink-0 border-b px-4 py-2 lg:px-6">
      <div className="mx-auto flex w-full max-w-[1600px] flex-wrap items-center gap-x-3 gap-y-1">
        <ToggleGroup
          aria-label="设计体系范围"
          value={[activeValue]}
          // Single-select toggle groups still report an array, and clicking the
          // pressed item clears it — a scope switcher has no "nothing selected"
          // state, so keep the current scope in that case.
          onValueChange={(next) => {
            const picked = next[0] ?? activeValue;
            onSelectRepository(picked === PROJECT_SCOPE_VALUE ? "" : picked);
          }}
          spacing={1}
          className="max-w-full flex-nowrap overflow-x-auto rounded-lg bg-muted p-[3px]"
        >
          {scopes.map((scope) => (
            <ToggleGroupItem
              key={scope.value}
              value={scope.value}
              title={scope.title}
              // The selected scope has to stay readable while hovered, so it
              // keeps a surface, weight and shadow that hover never touches.
              className="max-w-[14rem] gap-1.5 rounded-md px-2.5 font-normal text-muted-foreground hover:bg-background/60 hover:text-foreground aria-pressed:bg-background aria-pressed:font-medium aria-pressed:text-foreground aria-pressed:shadow-sm aria-pressed:hover:bg-background data-[state=on]:bg-background data-[state=on]:font-medium data-[state=on]:text-foreground data-[state=on]:shadow-sm data-[state=on]:hover:bg-background"
            >
              {scope.value === PROJECT_SCOPE_VALUE
                ? <Package className="h-3.5 w-3.5" />
                : <GitBranch className="h-3.5 w-3.5" />}
              <span className="truncate">{scope.label}</span>
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
        {fallbackNotice ? (
          <p className="text-caption text-muted-foreground">
            该仓库还没有自己的设计体系，当前显示项目通用体系。
          </p>
        ) : null}
      </div>
    </div>
  );
}

function ProjectDesignSystemSkeleton() {
  return (
    <div className="h-full overflow-auto p-4">
      <div className="mx-auto w-full max-w-5xl space-y-4 py-2">
        <Skeleton className="h-7 w-48" />
        <Skeleton className="h-20 w-full" />
        <Skeleton className="h-36 w-full" />
      </div>
    </div>
  );
}

function ProjectDesignSystemTaskStatus({
  project,
  agents,
  system,
}: {
  project: Project;
  agents: Agent[];
  system: ProjectDesignSystem;
}) {
  const isRepositoryAnalysis = system.active_task?.operation === "repository_analysis";
  return (
    <div className="h-full overflow-auto p-4">
      <div className="mx-auto w-full max-w-5xl py-2">
        <div className="flex items-start gap-3 border-b pb-5">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
            <LoaderCircle className="h-4 w-4 animate-spin" />
          </span>
          <div className="min-w-0 flex-1">
            <h2 className="text-title-sm font-semibold">
              {isRepositoryAnalysis ? "正在分析项目仓库" : "正在生成设计体系"}
            </h2>
            <p className="mt-1 text-body text-muted-foreground">{project.title}</p>
          </div>
        </div>
        <ProjectDesignSystemTaskActivity system={system} agents={agents} />
      </div>
    </div>
  );
}

function ProjectDesignSystemContent({
  project,
  agents,
  designFiles,
  legacyProfiles,
  system,
  isLoading,
  repositories,
  selectedRepositoryId,
}: {
  project: Project;
  agents: Agent[];
  designFiles: DesignFile[];
  legacyProfiles: DesignSystemProfile[];
  system: ProjectDesignSystem | undefined;
  isLoading: boolean;
  repositories: ProjectResource[];
  selectedRepositoryId: string;
}) {
  if (isLoading || !system) return <ProjectDesignSystemSkeleton />;

  const hasContent = Boolean(
    system.content.preview_html
      || system.content.sections.length
      || system.content.token_groups.length,
  );
  if (system.id && (hasContent || system.status === "draft" || system.status === "saved")) {
    return <ProjectDesignSystemCanvas system={system} project={project} agents={agents} />;
  }

  if (system.status === "generating" || system.active_task) {
    return <ProjectDesignSystemTaskStatus project={project} agents={agents} system={system} />;
  }

  return (
    <div className="h-full overflow-auto p-4">
      <ProjectDesignSystemCreate
        project={project}
        agents={agents}
        designFiles={designFiles}
        legacyProfiles={legacyProfiles}
        system={system}
        repositories={repositories}
        projectResourceId={selectedRepositoryId}
      />
    </div>
  );
}

export function ProjectDesignSystemWorkspace({
  project,
  agents,
  designFiles,
  legacyProfiles,
  system,
  isLoading,
  repositories,
  selectedRepositoryId,
  onSelectRepository,
}: {
  project: Project;
  agents: Agent[];
  designFiles: DesignFile[];
  legacyProfiles: DesignSystemProfile[];
  system: ProjectDesignSystem | undefined;
  isLoading: boolean;
  repositories: ProjectResource[];
  selectedRepositoryId: string;
  onSelectRepository: (projectResourceId: string) => void;
}) {
  // A repository without its own system resolves to the project-level one, so
  // say so instead of letting it read as this repository's design system.
  const showsProjectFallback = Boolean(
    selectedRepositoryId && system?.id && !system.project_resource_id,
  );
  return (
    <div className="flex h-full min-h-0 flex-col">
      <ProjectDesignSystemScopeSwitcher
        repositories={repositories}
        selectedRepositoryId={selectedRepositoryId}
        onSelectRepository={onSelectRepository}
        fallbackNotice={showsProjectFallback}
      />
      <div className="flex min-h-0 flex-1 flex-col">
        <ProjectDesignSystemContent
          // Drafts and canvas selection belong to one scope; remount so a
          // repository switch never carries the previous scope's local state.
          key={selectedRepositoryId || PROJECT_SCOPE_VALUE}
          project={project}
          agents={agents}
          designFiles={designFiles}
          legacyProfiles={legacyProfiles}
          system={system}
          isLoading={isLoading}
          repositories={repositories}
          selectedRepositoryId={selectedRepositoryId}
        />
      </div>
    </div>
  );
}
