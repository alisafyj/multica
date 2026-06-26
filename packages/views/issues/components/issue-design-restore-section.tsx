"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ExternalLink, FileJson, WandSparkles } from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { agentTasksOptions } from "@multica/core/agents/queries";
import { designKeys } from "@multica/core/designs/keys";
import { designFileDetailOptions, designFileListOptions, designRestoreMappingsOptions, designRestorePlanOptions, designRestoreTaskDetailOptions, designRestoreTaskListOptions } from "@multica/core/designs/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import type { Agent, DesignFrame, DesignRestorePlan, DesignRestoreTask, Issue } from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { useNavigation } from "../../navigation";

interface IssueDesignRestoreSectionProps {
  issue: Issue;
  agents: Agent[];
}

function planNeedsTarget(plan: DesignRestorePlan | undefined) {
  const targets = plan?.plan?.targets;
  if (!targets || typeof targets !== "object" || Array.isArray(targets)) return false;
  const record = targets as Record<string, unknown>;
  return record.needsUserSelection === true || !record.selected;
}

function planTargets(plan: DesignRestorePlan | undefined): Record<string, unknown> {
  const targets = plan?.plan?.targets;
  return targets && typeof targets === "object" && !Array.isArray(targets) ? targets as Record<string, unknown> : {};
}

function targetCandidates(plan: DesignRestorePlan | undefined): Array<Record<string, unknown>> {
  const candidates = planTargets(plan).candidates;
  return Array.isArray(candidates) ? candidates.filter((item): item is Record<string, unknown> => !!item && typeof item === "object" && !Array.isArray(item)) : [];
}

function selectedTarget(plan: DesignRestorePlan | undefined): Record<string, unknown> | null {
  const selected = planTargets(plan).selected;
  return selected && typeof selected === "object" && !Array.isArray(selected) ? selected as Record<string, unknown> : null;
}

function label(value: unknown, fallback = "未设置") {
  return typeof value === "string" && value.trim() ? value : fallback;
}

function resultSummary(task: DesignRestoreTask | null): Record<string, unknown> | null {
  const result = task?.result;
  if (!result || typeof result !== "object" || Array.isArray(result)) return null;
  const summary = (result as Record<string, unknown>).summary;
  return summary && typeof summary === "object" && !Array.isArray(summary) ? summary as Record<string, unknown> : null;
}

function isUiDesignIssue(title: string) {
  return /ui/i.test(title) || title.includes("设计");
}

type DesignRestoreFlowStatus = "design_ready" | "restore_task_created" | "plan_generated" | "target_selected" | "plan_approved" | "running" | "completed" | "failed" | "blocked";

function flowStatus(task: DesignRestoreTask | null, plan: DesignRestorePlan | undefined, agentTaskStatus?: string): DesignRestoreFlowStatus {
  if (!task) return "design_ready";
  if (task.status === "completed") return "completed";
  if (task.status === "failed" || task.status === "cancelled") return "failed";
  if (task.status === "running" || agentTaskStatus === "running") return "running";
  if (task.agent_task_id) return "plan_approved";
  if (!plan) return "restore_task_created";
  if (plan.status === "dispatched") return "running";
  if (plan.status === "approved") return "plan_approved";
  if (!planNeedsTarget(plan)) return "target_selected";
  return "plan_generated";
}

function statusCopy(status: DesignRestoreFlowStatus) {
  switch (status) {
    case "design_ready": return { label: "待交付", hint: "设计稿已上传后，可交给 Agent 还原。" };
    case "restore_task_created": return { label: "待交付", hint: "设计稿已上传后，可交给 Agent 还原。" };
    case "plan_generated": return { label: "待交付", hint: "设计稿已上传后，可交给 Agent 还原。" };
    case "target_selected": return { label: "待交付", hint: "设计稿已上传后，可交给 Agent 还原。" };
    case "plan_approved": return { label: "已派发", hint: "已派发，等待 Agent 领取。" };
    case "running": return { label: "还原中", hint: "Agent 正在还原设计稿。" };
    case "completed": return { label: "已完成", hint: "设计交付完成，正在/已经解锁前端开发。" };
    case "blocked": return { label: "已阻塞", hint: "请打开完整 Restore Plan 查看阻塞原因。" };
    case "failed": return { label: "还原失败", hint: "Agent 还原失败，可重试。" };
  }
}

function isLockedStatus(status: DesignRestoreFlowStatus) {
  return status === "running" || status === "completed";
}

export function IssueDesignRestoreSection({ issue, agents }: IssueDesignRestoreSectionProps) {
  const showDesignDelivery = isUiDesignIssue(issue.title);
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const queryClient = useQueryClient();
  const [fileId, setFileId] = useState("");
  const [frameId, setFrameId] = useState("");
  const [agentId, setAgentId] = useState("");
  const [restoreTask, setRestoreTask] = useState<DesignRestoreTask | null>(null);
  const [isOrchestrating, setIsOrchestrating] = useState(false);
  const { data: designFiles = [] } = useQuery(designFileListOptions(wsId));
  const { data: restoreTasks = [] } = useQuery(designRestoreTaskListOptions(wsId));
  const projectDesignFiles = useMemo(() => designFiles.filter((file) => !issue.project_id || file.project_id === issue.project_id), [designFiles, issue.project_id]);
  const selectedFileId = fileId || projectDesignFiles[0]?.id || "";
  const { data: selectedFileDetail } = useQuery({
    ...designFileDetailOptions(wsId, selectedFileId),
    enabled: !!selectedFileId,
  });
  const frames = selectedFileDetail?.current_revision?.native_json?.frames ?? [];
  const selectedFrameId = frameId || frames[0]?.id || "";
  const selectedFrame = frames.find((frame: DesignFrame) => frame.id === selectedFrameId);
  const availableAgents = useMemo(() => agents.filter((agent) => !agent.archived_at && agent.runtime_id), [agents]);
  const assignedAvailableAgent = issue.assignee_type === "agent" ? availableAgents.find((agent) => agent.id === issue.assignee_id) : undefined;
  const selectedAgent = availableAgents.find((agent) => agent.id === agentId) ?? assignedAvailableAgent ?? availableAgents[0];
  const existingIssueRestoreTask = useMemo(() => {
    const issueTasks = restoreTasks.filter((task) => task.issue_id === issue.id);
    return issueTasks.find((task) => task.status === "running")
      ?? issueTasks.find((task) => task.agent_task_id)
      ?? issueTasks[0]
      ?? null;
  }, [restoreTasks, issue.id]);
  const restoreTaskId = restoreTask?.id || existingIssueRestoreTask?.id || "";
  const { data: restorePlan } = useQuery({
    ...designRestorePlanOptions(wsId, restoreTaskId),
    enabled: !!restoreTaskId,
  });
  const { data: restoreMappings = [] } = useQuery({
    ...designRestoreMappingsOptions(wsId, restoreTaskId),
    enabled: !!restoreTaskId,
  });
  const { data: restoreTaskDetail } = useQuery({
    ...designRestoreTaskDetailOptions(wsId, restoreTaskId),
    enabled: !!restoreTaskId,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status === "running" || status === "queued" ? 3000 : false;
    },
  });
  const activeRestoreTask = restoreTaskDetail ?? restoreTask ?? existingIssueRestoreTask;
  const restoreAgent = availableAgents.find((agent) => agent.id === selectedAgent?.id) ?? selectedAgent;
  const { data: agentTasks = [] } = useQuery({
    ...agentTasksOptions(wsId, restoreAgent?.id ?? ""),
    enabled: !!restoreAgent?.id && !!activeRestoreTask?.agent_task_id,
    refetchInterval: activeRestoreTask?.agent_task_id ? 3000 : false,
  });
  const agentTask = agentTasks.find((item) => item.id === activeRestoreTask?.agent_task_id);
  const planCandidates = targetCandidates(restorePlan);
  const planSelectedTarget = selectedTarget(restorePlan);
  const summary = resultSummary(activeRestoreTask);
  const currentStatus = flowStatus(activeRestoreTask, restorePlan, agentTask?.status);
  const currentStatusCopy = statusCopy(currentStatus);
  const controlsLocked = isLockedStatus(currentStatus);
  const primaryAgent = selectedAgent;
  const primaryActionLabel = currentStatus === "failed" ? "重新交给 Agent" : "交给 Agent 还原";

  useEffect(() => {
    if (!restoreTask && existingIssueRestoreTask) {
      setRestoreTask(existingIssueRestoreTask);
    }
  }, [existingIssueRestoreTask, restoreTask]);

  const createRestoreTask = useMutation({
    mutationFn: async () => {
      if (!selectedFileDetail?.current_revision?.id || !selectedFrame) throw new Error("请选择有效设计稿和画板");
      return api.createDesignRestoreTask({
        file_id: selectedFileId,
        revision_id: selectedFileDetail.current_revision.id,
        issue_id: issue.id,
        input: {
          version: "1.0",
          projectId: issue.project_id ?? undefined,
          sourceIssueId: issue.id,
          purpose: "frontend_restore",
          items: [{
            itemId: `issue-${issue.id.slice(0, 8)}-${selectedFrame.id}`,
            order: 1,
            designFileId: selectedFileId,
            revisionId: selectedFileDetail.current_revision.id,
            frameId: selectedFrame.id,
            frameName: selectedFrame.name,
            source: "frame",
            note: "Issue 内触发：前端 Agent 整页设计稿还原。",
          }],
        },
      });
    },
    onSuccess: async (task) => {
      setRestoreTask(task);
      await queryClient.invalidateQueries({ queryKey: designKeys.restoreTasks(wsId) });
      toast.success("已创建设计还原任务");
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "创建设计还原任务失败"),
  });

  const runRestoreFlow = async () => {
    if (currentStatus === "running" || currentStatus === "completed") return;
    if (!primaryAgent) {
      toast.error("暂无可用前端 Agent");
      return;
    }
    setIsOrchestrating(true);
    try {
      const retryingFailedTask = activeRestoreTask?.status === "failed" || activeRestoreTask?.status === "cancelled";
      let task = retryingFailedTask ? null : activeRestoreTask;
      let plan = retryingFailedTask ? undefined : restorePlan;
      if (!task) {
        if (!selectedFileDetail?.current_revision?.id || !selectedFrame) throw new Error("请选择有效设计稿和画板");
        task = await createRestoreTask.mutateAsync();
      }
      if (!plan) {
        plan = await api.generateDesignRestorePlan(task.id);
        await queryClient.invalidateQueries({ queryKey: designKeys.restorePlan(wsId, task.id) });
      }
      const candidates = targetCandidates(plan);
      if (plan.status === "draft" && planNeedsTarget(plan) && candidates.length) {
        plan = await api.updateDesignRestorePlan(task.id, {
          plan: {
            ...plan.plan,
            targets: {
              ...planTargets(plan),
              selected: selectedTarget(plan) ?? candidates[0],
              needsUserSelection: false,
            },
          },
          review_notes: plan.review_notes ?? undefined,
        });
        await queryClient.invalidateQueries({ queryKey: designKeys.restorePlan(wsId, task.id) });
      }
      if (plan.status === "draft" && !planNeedsTarget(plan)) {
        plan = await api.approveDesignRestorePlan(task.id);
        await queryClient.invalidateQueries({ queryKey: designKeys.restorePlan(wsId, task.id) });
      }
      if (!retryingFailedTask && task.agent_task_id) {
        toast.info("任务已派发，等待 Agent 领取");
        return;
      }
      if (plan.status !== "approved") throw new Error("Restore Plan 尚未准备好，请打开完整 Restore Plan 查看");
      const result = await api.dispatchDesignRestoreTask(task.id, {
        agent_id: primaryAgent.id,
        issue_id: issue.id,
        prompt: "根据 Issue 关联设计稿和 approved Restore Plan 完成整页前端还原；禁止整图 preview，完成后输出 RESTORE_RESULT_JSON。",
      });
      setRestoreTask(result.task);
      await queryClient.invalidateQueries({ queryKey: designKeys.restoreTask(wsId, result.task.id) });
      await queryClient.invalidateQueries({ queryKey: designKeys.restoreTasks(wsId) });
      await queryClient.invalidateQueries({ queryKey: designKeys.restoreMappings(wsId, result.task.id) });
      toast.success(`已交给 Agent：${primaryAgent.name}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "还原流程启动失败");
    } finally {
      setIsOrchestrating(false);
    }
  };
  const primaryActionPending = createRestoreTask.isPending || isOrchestrating;

  if (!showDesignDelivery) return null;

  return (
    <section className="rounded-lg border bg-card p-3 text-xs">
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-sm font-medium"><FileJson className="size-4 text-muted-foreground" />设计交付</div>
        <Badge variant={currentStatus === "completed" ? "secondary" : currentStatus === "failed" || currentStatus === "blocked" ? "destructive" : "outline"}>{currentStatusCopy.label}</Badge>
      </div>
      <p className="mt-1 text-muted-foreground">1 上传设计稿 · 2 Agent 还原设计稿</p>
      <div className="mt-3 space-y-2">
        <div className="rounded-md border bg-background p-3">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <div className="font-medium text-foreground">{currentStatusCopy.hint}</div>
              <div className="mt-1 truncate text-muted-foreground">{selectedFrame?.name ?? "默认画板"} · {primaryAgent?.name ?? "等待可用 Agent"}{agentTask ? ` · ${agentTask.status}` : ""}</div>
            </div>
            {activeRestoreTask ? <span className="shrink-0 font-mono text-muted-foreground">{activeRestoreTask.id.slice(0, 8)}</span> : null}
          </div>
        </div>
        {!controlsLocked ? (
          <details className="rounded-md border bg-background/60">
            <summary className="cursor-pointer list-none px-2 py-1.5 text-muted-foreground hover:text-foreground">调整上传设计稿 / Agent</summary>
            <div className="space-y-2 border-t p-2">
              <select value={selectedFileId} onChange={(event) => { setFileId(event.target.value); setFrameId(""); }} className="h-8 w-full rounded-md border bg-background px-2">
                {projectDesignFiles.length ? projectDesignFiles.map((file) => <option key={file.id} value={file.id}>{file.title}</option>) : <option value="">当前项目暂无设计稿</option>}
              </select>
              <select value={selectedFrameId} onChange={(event) => setFrameId(event.target.value)} className="h-8 w-full rounded-md border bg-background px-2" disabled={!frames.length}>
                {frames.length ? frames.map((frame: DesignFrame) => <option key={frame.id} value={frame.id}>{frame.name}</option>) : <option value="">暂无画板</option>}
              </select>
              <select value={primaryAgent?.id ?? ""} onChange={(event) => setAgentId(event.target.value)} className="h-8 w-full rounded-md border bg-background px-2" disabled={!availableAgents.length}>
                {availableAgents.length ? availableAgents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name} · {agent.status}</option>) : <option value="">暂无可用前端 Agent</option>}
              </select>
            </div>
          </details>
        ) : null}
        {!availableAgents.length ? <div className="rounded-md border border-amber-200 bg-amber-50 p-2 text-amber-900">当前没有绑定 runtime 的可用 Agent。请先创建/恢复 Agent，否则无法派发。</div> : null}
        {!controlsLocked && (!activeRestoreTask?.agent_task_id || currentStatus === "failed") ? <Button size="sm" className="w-full" disabled={!selectedFileId || !selectedFrameId || primaryActionPending || !primaryAgent} onClick={() => void runRestoreFlow()}><WandSparkles className="size-3.5" />{primaryActionPending ? "正在准备…" : primaryActionLabel}</Button> : null}
        {activeRestoreTask ? (
          <>
            {restorePlan ? (
              <details className="rounded-md border bg-background/60">
                <summary className="cursor-pointer list-none px-2 py-1.5 text-muted-foreground hover:text-foreground">高级：Restore Plan</summary>
                <div className="space-y-2 border-t p-2">
                  <div className="text-muted-foreground">状态：{restorePlan.status}{planNeedsTarget(restorePlan) ? " · 等待默认目标" : ""}</div>
                  {planSelectedTarget ? <div className="text-muted-foreground">目标：<span className="font-mono text-foreground">{label(planSelectedTarget.path)}</span></div> : null}
                  {planCandidates.length ? <div className="text-muted-foreground">候选目标：{planCandidates.length} 个，默认使用第一个。</div> : null}
                  {planCandidates.slice(0, 3).map((candidate, index) => {
                    const path = label(candidate.path, `candidate-${index + 1}`);
                    const selected = planSelectedTarget?.path === candidate.path;
                    return (
                      <div key={`${path}-${index}`} className={`rounded-md border p-2 ${selected ? "border-primary bg-muted" : ""}`}>
                        <div className="font-mono text-foreground">{path}</div>
                        <div className="mt-1 text-muted-foreground">{selected ? "当前已选择 · " : ""}{label(candidate.kind)} · {label(candidate.reason, "候选目标")}</div>
                      </div>
                    );
                  })}
                  <Button size="sm" variant="ghost" className="w-full" onClick={() => navigation.push(paths.designRestoreTaskDetail(activeRestoreTask.id))}>
                    <ExternalLink className="size-3.5" />打开完整 Restore Plan
                  </Button>
                </div>
              </details>
            ) : null}
            {currentStatus === "plan_approved" && activeRestoreTask.agent_task_id ? <div className="rounded-md bg-muted p-2 text-muted-foreground">已派发，等待 Agent 领取。</div> : null}
            {currentStatus === "running" ? <div className="rounded-md bg-muted p-2 text-muted-foreground">Agent 正在还原设计稿。</div> : null}
            {summary ? (
              <div className="rounded-md border p-2 text-muted-foreground">
                <div>执行结果：<Badge variant={summary.status === "completed" ? "secondary" : "outline"}>{label(summary.status)}</Badge></div>
                {Array.isArray(summary.files) && summary.files.length ? <div className="mt-1">文件：<span className="font-mono text-foreground">{summary.files.join(", ")}</span></div> : null}
                <div className="mt-1">策略违规：<span className="text-foreground">{label((activeRestoreTask.result as Record<string, unknown>)?.policy_violation, "无")}</span></div>
              </div>
            ) : null}
            {restoreMappings.length ? <div className="rounded-md border p-2 text-muted-foreground">Restore Mapping：{restoreMappings.length} 条</div> : null}
          </>
        ) : null}
      </div>
    </section>
  );
}
