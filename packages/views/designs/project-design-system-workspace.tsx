"use client";

import { GitBranch, LoaderCircle, Package, Palette, ScanSearch } from "lucide-react";
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
}: {
  repositories: ProjectResource[];
  selectedRepositoryId: string;
  onSelectRepository: (projectResourceId: string) => void;
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


function ProgrammaticGenerationPreview() {
  return (
    <section aria-label="快速草稿预览" className="min-h-[520px] overflow-hidden rounded-xl border bg-card shadow-sm">
      <div className="flex items-center justify-between border-b px-5 py-3">
        <div>
          <p className="text-caption text-muted-foreground">实时预览</p>
          <h3 className="mt-0.5 text-body font-medium">设计体系正在逐步生成</h3>
        </div>
        <span className="flex items-center gap-1.5 text-caption text-muted-foreground">
          <LoaderCircle className="size-3.5 animate-spin" />
          验证前草稿
        </span>
      </div>
      <div className="space-y-7 bg-gradient-to-b from-muted/50 to-background p-6">
        <div className="rounded-xl border bg-background p-5">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="space-y-2">
              <Skeleton className="h-3 w-28" />
              <Skeleton className="h-8 w-56" />
              <Skeleton className="h-3 w-72 max-w-full" />
            </div>
            <div className="flex gap-2">
              <Skeleton className="h-7 w-24 rounded-full" />
              <Skeleton className="h-7 w-20 rounded-full" />
            </div>
          </div>
        </div>
        <div>
          <div className="mb-3 flex items-center gap-2 text-caption font-medium text-muted-foreground">
            <Palette className="size-3.5" />
            色彩与视觉 Tokens
          </div>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {["bg-primary/80", "bg-foreground/80", "bg-muted", "bg-background"].map((tone) => (
              <div key={tone} className="overflow-hidden rounded-lg border bg-background">
                <div className={`h-20 ${tone}`} />
                <div className="space-y-1.5 p-2.5"><Skeleton className="h-2.5 w-16" /><Skeleton className="h-2 w-12" /></div>
              </div>
            ))}
          </div>
        </div>
        <div>
          <div className="mb-3 flex items-center gap-2 text-caption font-medium text-muted-foreground">
            <ScanSearch className="size-3.5" />
            组件状态与页面模式
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            {[0, 1, 2, 3].map((item) => (
              <div key={item} className="flex items-center gap-3 rounded-lg border bg-background p-3">
                <Skeleton className="size-9 rounded-md" />
                <div className="min-w-0 flex-1 space-y-2"><Skeleton className="h-3 w-2/5" /><Skeleton className="h-2.5 w-4/5" /></div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
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
  const isProgrammaticFirst = system.active_task?.execution_mode === "programmatic_first";
  if (isProgrammaticFirst) {
    return (
      <div className="h-full overflow-auto p-4 lg:p-6">
        <div className="mx-auto w-full max-w-[1600px] py-2">
          <div className="mb-5 flex items-start gap-3">
            <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
              <LoaderCircle className="h-4 w-4 animate-spin" />
            </span>
            <div className="min-w-0 flex-1">
              <h2 className="text-title-sm font-semibold">正在生成仓库设计体系</h2>
              <p className="mt-1 text-body text-muted-foreground">{project.title} · 一次点击完成仓库读取、快速草稿和真实验证</p>
            </div>
          </div>
          <div className="grid min-h-0 gap-6 lg:grid-cols-[minmax(300px,380px)_minmax(0,1fr)]">
            <aside className="self-start rounded-xl border bg-background px-4">
              <ProjectDesignSystemTaskActivity system={system} agents={agents} />
            </aside>
            <ProgrammaticGenerationPreview />
          </div>
        </div>
      </div>
    );
  }
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

export function ProjectDesignSystemContent({
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

const emptyProjectDesignSystemContent = {
  sections: [],
  token_groups: [],
  locators: [],
  preview_html: "",
  integrity_sha256: "",
} satisfies ProjectDesignSystem["content"];

const emptyProjectDesignSystemPreviewValidation = {
  status: "none",
  integrity_sha256: "",
  report: {},
  verified_at: null,
} satisfies ProjectDesignSystem["preview_validation"];

function unestablishedRepositorySystem(): Pick<
  ProjectDesignSystem,
  | "id"
  | "project_resource_id"
  | "status"
  | "active_task"
  | "input_snapshot"
  | "content"
  | "preview_validation"
  | "has_unsaved_changes"
  | "last_error"
  | "activity"
  | "saved_at"
> {
  return {
    id: "",
    project_resource_id: "",
    status: "unestablished",
    active_task: null,
    input_snapshot: {},
    content: emptyProjectDesignSystemContent,
    preview_validation: emptyProjectDesignSystemPreviewValidation,
    has_unsaved_changes: false,
    last_error: null,
    activity: [],
    saved_at: null,
  };
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
  // A repository scope must render only its own system. The API returns an
  // unestablished response when that repository has no system, and a cached
  // project-level response must never masquerade as the repository's system.
  const repositoryHasNoSystem = Boolean(selectedRepositoryId && (!system?.id || !system.project_resource_id));
  const scopedSystem = repositoryHasNoSystem && system
    ? { ...system, ...unestablishedRepositorySystem() }
    : system;
  return (
    <div className="flex h-full min-h-0 flex-col">
      <ProjectDesignSystemScopeSwitcher
        repositories={repositories}
        selectedRepositoryId={selectedRepositoryId}
        onSelectRepository={onSelectRepository}
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
          system={scopedSystem}
          isLoading={isLoading}
          repositories={repositories}
          selectedRepositoryId={selectedRepositoryId}
        />
      </div>
    </div>
  );
}
