"use client";

import { LoaderCircle } from "lucide-react";
import type {
  Agent,
  DesignFile,
  DesignSystemProfile,
  Project,
  ProjectDesignSystem,
} from "@multica/core/types";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { ProjectDesignSystemCanvas } from "./project-design-system-canvas";
import { ProjectDesignSystemCreate } from "./project-design-system-create";
import { ProjectDesignSystemTaskActivity } from "./project-design-system-task-activity";

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
            <h2 className="text-base font-semibold">
              {isRepositoryAnalysis ? "正在分析项目仓库" : "正在生成设计体系"}
            </h2>
            <p className="mt-1 text-sm text-muted-foreground">{project.title}</p>
          </div>
        </div>
        <ProjectDesignSystemTaskActivity system={system} agents={agents} />
      </div>
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
}: {
  project: Project;
  agents: Agent[];
  designFiles: DesignFile[];
  legacyProfiles: DesignSystemProfile[];
  system: ProjectDesignSystem | undefined;
  isLoading: boolean;
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
      />
    </div>
  );
}
