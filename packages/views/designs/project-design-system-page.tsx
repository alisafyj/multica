"use client";

import { useQuery } from "@tanstack/react-query";
import { projectDesignSystemDetailOptions } from "@multica/core/designs/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { projectDetailOptions } from "@multica/core/projects/queries";
import { agentListOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { ProjectDesignSystemCanvas } from "./project-design-system-canvas";

function ProjectDesignSystemPageSkeleton() {
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex min-h-16 items-center justify-between gap-4 border-b px-6">
        <Skeleton className="h-7 w-64" />
        <Skeleton className="h-8 w-72" />
      </div>
      <div className="grid flex-1 gap-8 p-6 lg:grid-cols-[180px_minmax(0,1fr)]">
        <Skeleton className="h-64" />
        <Skeleton className="h-[680px]" />
      </div>
    </div>
  );
}

export function ProjectDesignSystemPage({ designSystemId }: { designSystemId: string }) {
  const wsId = useWorkspaceId();
  const systemQuery = useQuery(projectDesignSystemDetailOptions(wsId, designSystemId));
  const projectQuery = useQuery({
    ...projectDetailOptions(wsId, systemQuery.data?.project_id ?? ""),
    enabled: Boolean(systemQuery.data?.project_id),
  });
  const agentsQuery = useQuery(agentListOptions(wsId));

  if (systemQuery.isLoading || projectQuery.isLoading || agentsQuery.isLoading) {
    return <ProjectDesignSystemPageSkeleton />;
  }

  if (systemQuery.error || projectQuery.error || !systemQuery.data?.id || !projectQuery.data) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center px-6 text-center">
        <div className="space-y-3">
          <p className="text-sm font-medium">无法加载此设计体系</p>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              void systemQuery.refetch();
              void projectQuery.refetch();
              void agentsQuery.refetch();
            }}
          >
            重试
          </Button>
        </div>
      </div>
    );
  }

  return (
    <ProjectDesignSystemCanvas
      system={systemQuery.data}
      project={projectQuery.data}
      agents={agentsQuery.data ?? []}
    />
  );
}
