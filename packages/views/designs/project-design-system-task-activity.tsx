"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CircleAlert, LoaderCircle, Square } from "lucide-react";
import { api } from "@multica/core/api";
import { taskMessagesOptions } from "@multica/core/chat/queries";
import { designKeys } from "@multica/core/designs/keys";
import { useWorkspaceId } from "@multica/core/hooks";
import type { Agent, ProjectDesignSystem, TaskMessagePayload } from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";

const STALE_AFTER_MS = 3 * 60_000;
const ACTIVE_TASK_STATUSES = new Set(["queued", "dispatched", "running", "waiting_local_directory"]);

export function taskStatusLabel(status: string): string {
  if (status === "queued") return "等待智能体接单";
  if (status === "dispatched") return "已派发，等待执行";
  if (status === "running") return "智能体执行中";
  if (status === "waiting_local_directory") return "等待本地目录";
  if (status === "completed") return "执行已结束，正在校验产物";
  if (status === "failed") return "执行失败";
  if (status === "cancelled") return "任务已停止";
  return "任务状态待确认";
}

export function taskOperationLabel(operation: string): string {
  if (operation === "repository_analysis") return "仓库分析";
  if (operation === "adjust") return "调整";
  if (operation === "regenerate") return "重新生成";
  return "生成";
}

function timestamp(value: string | null | undefined): number | null {
  if (!value) return null;
  const parsed = new Date(value).getTime();
  return Number.isNaN(parsed) ? null : parsed;
}

function formatTime(value: string | null | undefined): string {
  const parsed = timestamp(value);
  if (parsed === null) return "尚未开始";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(parsed);
}

function formatDuration(milliseconds: number): string {
  const totalSeconds = Math.max(0, Math.floor(milliseconds / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) return `${hours} 小时 ${minutes} 分`;
  if (minutes > 0) return `${minutes} 分 ${seconds} 秒`;
  return `${seconds} 秒`;
}

function newestActivityAt(
  messages: TaskMessagePayload[],
  fallbacks: Array<string | null | undefined>,
): string | null {
  const values = [
    ...messages.map((message) => message.created_at),
    ...fallbacks,
  ].filter((value): value is string => timestamp(value) !== null);
  if (!values.length) return null;
  return values.reduce((latest, current) => (
    (timestamp(current) ?? 0) > (timestamp(latest) ?? 0) ? current : latest
  ));
}

export function ProjectDesignSystemTaskActivity({
  system,
  agents,
  compact = false,
}: {
  system: ProjectDesignSystem;
  agents: Agent[];
  compact?: boolean;
}) {
  const task = system.active_task;
  const wsId = useWorkspaceId();
  const queryClient = useQueryClient();
  const [now, setNow] = useState(() => Date.now());
  const [cancelError, setCancelError] = useState<string | null>(null);
  const { data: messages = [] } = useQuery(taskMessagesOptions(task?.id ?? ""));

  useEffect(() => {
    if (!task || !ACTIVE_TASK_STATUSES.has(task.status)) return;
    setNow(Date.now());
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [task]);

  const stopTask = useMutation({
    mutationFn: (taskId: string) => api.cancelTaskById(taskId),
    onMutate: () => setCancelError(null),
    onError: (error) => {
      setCancelError(error instanceof Error ? error.message : "停止任务失败，请稍后重试。");
    },
    onSettled: async () => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: designKeys.projectDesignSystemByProject(wsId, system.project_id),
          exact: true,
        }),
        queryClient.invalidateQueries({
          queryKey: designKeys.projectDesignSystem(wsId, system.id),
          exact: true,
        }),
      ]);
    },
  });

  const evidence = useMemo(() => {
    if (!task) return null;
    const latestActivity = newestActivityAt(messages, [
      task.started_at,
      task.dispatched_at,
      task.created_at,
    ]);
    const startedAt = timestamp(task.started_at);
    const completedAt = timestamp(task.completed_at);
    return {
      latestActivity,
      elapsed: startedAt === null ? null : (completedAt ?? now) - startedAt,
      stale: task.status === "running"
        && timestamp(latestActivity) !== null
        && now - (timestamp(latestActivity) ?? now) >= STALE_AFTER_MS,
    };
  }, [messages, now, task]);

  if (!task || !evidence) return null;
  const agent = agents.find((candidate) => candidate.id === task.agent_id);
  const canStop = ACTIVE_TASK_STATUSES.has(task.status);
  const isRepositoryAnalysis = task.operation === "repository_analysis";

  return (
    <section aria-label="智能体任务活动" className={compact ? "border-t py-5" : "border-b py-5"}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2 text-sm font-medium">
          {canStop ? <LoaderCircle className="h-4 w-4 shrink-0 animate-spin text-muted-foreground" /> : null}
          <span>{taskStatusLabel(task.status)}</span>
        </div>
        <Badge variant="secondary">{taskOperationLabel(task.operation)}</Badge>
      </div>

      <dl className={`mt-4 grid gap-4 ${compact ? "grid-cols-2" : "sm:grid-cols-4"}`}>
        <div className="min-w-0">
          <dt className="text-xs text-muted-foreground">智能体</dt>
          <dd className="mt-1 truncate text-sm font-medium">{agent?.name ?? "已选择智能体"}</dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">开始时间</dt>
          <dd className="mt-1 text-sm font-medium">{formatTime(task.started_at)}</dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">运行时长</dt>
          <dd className="mt-1 text-sm font-medium">{evidence.elapsed === null ? "尚未开始" : formatDuration(evidence.elapsed)}</dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">最后活动</dt>
          <dd className="mt-1 text-sm font-medium">{formatTime(evidence.latestActivity)}</dd>
        </div>
      </dl>

      {task.status === "queued" ? (
        <p className="mt-4 text-xs text-muted-foreground">任务已进入队列，智能体尚未接单。</p>
      ) : null}
      {task.status === "waiting_local_directory" && task.wait_reason ? (
        <p className="mt-4 text-xs text-muted-foreground">{task.wait_reason}</p>
      ) : null}
      {evidence.stale ? (
        <div role="alert" className="mt-4 flex items-start gap-2 border-l-2 border-amber-500 bg-amber-500/5 px-3 py-2 text-xs leading-5">
          <CircleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0 text-amber-600" />
          <span>超过 3 分钟没有新的活动，任务可能已停滞。</span>
        </div>
      ) : null}
      {cancelError ? (
        <div role="alert" className="mt-4 border-l-2 border-destructive bg-destructive/5 px-3 py-2 text-xs text-destructive">
          {cancelError}
        </div>
      ) : null}
      {canStop ? (
        <Button
          type="button"
          size="sm"
          variant="outline"
          className="mt-4"
          disabled={stopTask.isPending}
          onClick={() => stopTask.mutate(task.id)}
        >
          {stopTask.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <Square className="h-3.5 w-3.5" />}
          {stopTask.isPending
            ? (isRepositoryAnalysis ? "正在停止分析" : "正在停止")
            : (isRepositoryAnalysis ? "停止分析" : "停止任务")}
        </Button>
      ) : null}
    </section>
  );
}
