"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CircleAlert, LoaderCircle, RefreshCcw } from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { designKeys } from "@multica/core/designs/keys";
import { projectDesignSystemDetailOptions } from "@multica/core/designs/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { projectDetailOptions } from "@multica/core/projects/queries";
import { agentListOptions } from "@multica/core/workspace/queries";
import type { Agent, ProjectDesignSystem } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import {
  ProjectDesignSystemCanvas,
  designSystemErrorMessage,
  isDesignSystemAgentAvailable,
} from "./project-design-system-canvas";
import { ProjectDesignSystemTaskActivity } from "./project-design-system-task-activity";

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

/**
 * First generation in flight and nothing to render yet. Mirrors the project
 * workbench's task-status branch — evidence only (live task activity plus the
 * agent transcript), no invented progress — where Open Design shows its
 * extraction chat beside the pending preview.
 */
function GeneratingView({ system, agents }: { system: ProjectDesignSystem; agents: Agent[] }) {
  return (
    <div className="min-h-0 flex-1 overflow-auto">
      <div className="mx-auto w-full max-w-3xl px-6 py-10">
        <div className="flex items-start gap-3 border-b pb-5">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
            <LoaderCircle className="h-4 w-4 animate-spin" />
          </span>
          <div className="min-w-0 flex-1">
            <h2 className="text-title-sm font-semibold">正在生成设计体系</h2>
            <p className="mt-1 break-words text-body text-muted-foreground">{system.name}</p>
          </div>
        </div>
        <ProjectDesignSystemTaskActivity system={system} agents={agents} />
        <p className="mt-4 text-caption text-muted-foreground">
          生成完成后，本页会自动展示 DESIGN.md 章节、Token 与在线 UI Kit。
        </p>
      </div>
    </div>
  );
}

/**
 * No content and no task: the first generation failed or was stopped. Shows
 * the real failure and offers regenerate, which re-runs the frozen creation
 * inputs with the picked agent.
 */
function FailedView({ system, agents }: { system: ProjectDesignSystem; agents: Agent[] }) {
  const wsId = useWorkspaceId();
  const queryClient = useQueryClient();
  const [agentId, setAgentId] = useState(system.current_agent_id ?? "");
  const agentOptions = agents.filter((agent) => !agent.archived_at || agent.id === agentId);
  const selectedAgent = agents.find((agent) => agent.id === agentId);
  const agentAvailable = isDesignSystemAgentAvailable(selectedAgent);

  const regenerate = useMutation({
    mutationFn: () => api.regenerateProjectDesignSystem(system.id, { agent_id: agentId }),
    onSuccess: (updated) => {
      queryClient.setQueryData(designKeys.projectDesignSystem(wsId, system.id), updated);
      toast.success("已重新发起生成");
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : "重新生成失败，请稍后重试。");
    },
  });

  return (
    <div className="min-h-0 flex-1 overflow-auto">
      <div className="mx-auto w-full max-w-xl px-6 py-14">
        <div className="flex items-start gap-3">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-destructive/10 text-destructive">
            <CircleAlert className="h-4 w-4" />
          </span>
          <div className="min-w-0 flex-1">
            <h2 className="text-title-sm font-semibold">生成未完成</h2>
            <p className="mt-1 break-words text-body text-muted-foreground">
              {designSystemErrorMessage(system.last_error) ?? "任务已停止或没有产出内容。创建时填写的品牌与参考资料仍然保留，可直接重新生成。"}
            </p>
          </div>
        </div>
        <div className="mt-6 space-y-3">
          <select
            aria-label="执行智能体"
            value={agentId}
            onChange={(event) => setAgentId(event.target.value)}
            className="h-9 w-full rounded-md border bg-background px-3 text-body"
          >
            <option value="">选择智能体</option>
            {agentOptions.map((agent) => (
              <option key={agent.id} value={agent.id} disabled={!isDesignSystemAgentAvailable(agent)}>
                {agent.name} · {isDesignSystemAgentAvailable(agent) ? agent.status : "不可用"}
              </option>
            ))}
          </select>
          {agentId && !agentAvailable ? (
            <p className="text-caption text-destructive">当前智能体不可用，请选择其他智能体。</p>
          ) : null}
          <Button
            type="button"
            className="w-full"
            disabled={!agentAvailable || regenerate.isPending}
            onClick={() => regenerate.mutate()}
          >
            {regenerate.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCcw className="h-4 w-4" />}
            {regenerate.isPending ? "正在发起…" : "重新生成"}
          </Button>
        </div>
      </div>
    </div>
  );
}

export function ProjectDesignSystemPage({ designSystemId }: { designSystemId: string }) {
  const wsId = useWorkspaceId();
  const systemQuery = useQuery({
    ...projectDesignSystemDetailOptions(wsId, designSystemId),
    // Realtime publishes project_design_system:changed on completion; the
    // poll keeps the in-between states (claimed, running) honest.
    refetchInterval: (query) => (
      query.state.data?.active_task || query.state.data?.status === "generating" ? 4000 : false
    ),
  });
  // A standalone system (empty project_id) owns itself: no project to load.
  const standalone = systemQuery.data?.project_id === "";
  const projectQuery = useQuery({
    ...projectDetailOptions(wsId, systemQuery.data?.project_id ?? ""),
    enabled: Boolean(systemQuery.data?.project_id),
  });
  const agentsQuery = useQuery(agentListOptions(wsId));

  if (systemQuery.isLoading || (!standalone && projectQuery.isLoading) || agentsQuery.isLoading) {
    return <ProjectDesignSystemPageSkeleton />;
  }

  if (systemQuery.error || (!standalone && projectQuery.error) || !systemQuery.data?.id || (!standalone && !projectQuery.data)) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center px-6 text-center">
        <div className="space-y-3">
          <p className="text-body font-medium">无法加载此设计体系</p>
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

  const system = systemQuery.data;
  const agents = agentsQuery.data ?? [];
  // Same branch as the project workbench: content (or a draft/saved state)
  // renders the canvas even while an adjustment runs; a first generation with
  // nothing to show renders the task status instead of an empty canvas.
  const hasContent = Boolean(
    system.content.preview_html.trim()
      || system.content.preview_targets?.length
      || system.content.sections.length
      || system.content.token_groups.length,
  );

  if (hasContent || system.status === "draft" || system.status === "saved") {
    return (
      <ProjectDesignSystemCanvas
        system={system}
        project={standalone ? undefined : projectQuery.data}
        agents={agents}
      />
    );
  }

  if (system.status === "generating" || system.active_task) {
    return <GeneratingView system={system} agents={agents} />;
  }

  return <FailedView system={system} agents={agents} />;
}
