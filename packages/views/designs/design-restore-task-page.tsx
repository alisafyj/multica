"use client";

import { useMemo, useState } from "react";
import { ArrowLeft, Bot, Copy, ExternalLink, FileJson, Layers } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { designKeys } from "@multica/core/designs/keys";
import { designRestoreTaskDetailOptions } from "@multica/core/designs/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { agentListOptions } from "@multica/core/workspace/queries";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { BreadcrumbHeader } from "../layout/breadcrumb-header";
import { useNavigation } from "../navigation";
import type { DesignRestoreTaskInputV1, DesignRestoreTaskItemInput } from "@multica/core/types";

function isRestoreTaskInput(value: unknown): value is DesignRestoreTaskInputV1 {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const input = value as Partial<DesignRestoreTaskInputV1>;
  return input.version === "1.0" && Array.isArray(input.items);
}

function itemKey(item: DesignRestoreTaskItemInput, index: number) {
  return item.itemId || `item-${index + 1}`;
}

function sourceLabel(source: DesignRestoreTaskItemInput["source"]) {
  if (source === "frame") return "画板";
  if (source === "selected_layers") return "选中图层";
  if (source === "selection_bounds") return "选区范围";
  if (source === "template") return "模板";
  if (source === "draft") return "草稿";
  return source;
}

function JsonBlock({ title, value }: { title: string; value: unknown }) {
  return (
    <section className="rounded-lg border bg-background">
      <div className="border-b px-3 py-2 text-sm font-medium">{title}</div>
      <pre className="max-h-96 overflow-auto p-3 text-xs leading-relaxed text-muted-foreground">{JSON.stringify(value, null, 2)}</pre>
    </section>
  );
}

function readRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : null;
}

function stringList(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];
}

function unknownList(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

export function DesignRestoreTaskPage({ taskId }: { taskId: string }) {
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const queryClient = useQueryClient();
  const [copyingItemId, setCopyingItemId] = useState<string | null>(null);
  const [selectedAgentId, setSelectedAgentId] = useState("");
  const [issueId, setIssueId] = useState("");
  const [prompt, setPrompt] = useState("根据这个 restore task 完成最小安全前端还原；优先复用现有组件，完成后运行相关 typecheck，并回写变更文件、检查项、阻塞项和 restore mapping。");
  const { data: task, isLoading, error, refetch } = useQuery(designRestoreTaskDetailOptions(wsId, taskId));
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const input = useMemo(() => isRestoreTaskInput(task?.input) ? task.input : null, [task?.input]);
  const items = input?.items ?? [];
  const result = readRecord(task?.result);
  const resultSummary = readRecord(result?.summary);
  const policyViolation = typeof result?.policy_violation === "string" ? result.policy_violation : typeof resultSummary?.policyViolation === "string" ? resultSummary.policyViolation : "";
  const usedFullFramePreview = resultSummary?.usedFullFramePreview === true;
  const usedLayerIds = stringList(resultSummary?.usedLayerIds);
  const usedAssetIds = stringList(resultSummary?.usedAssetIds);
  const resultFiles = stringList(resultSummary?.files);
  const resultChecks = stringList(resultSummary?.checks);
  const resultBlockers = stringList(resultSummary?.blockers);
  const restoreMapping = unknownList(resultSummary?.restoreMapping);
  const resultStatus = typeof resultSummary?.status === "string" ? resultSummary.status : task?.status;
  const resultText = typeof resultSummary?.summary === "string" ? resultSummary.summary : "";
  const availableAgents = useMemo(() => agents.filter((agent) => !agent.archived_at && agent.runtime_id), [agents]);
  const dispatchAgentId = selectedAgentId || availableAgents[0]?.id || "";

  const dispatchTask = useMutation({
    mutationFn: () => api.dispatchDesignRestoreTask(taskId, { agent_id: dispatchAgentId, issue_id: issueId.trim() || undefined, prompt }),
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({ queryKey: designKeys.restoreTask(wsId, taskId) });
      await queryClient.invalidateQueries({ queryKey: designKeys.restoreTasks(wsId) });
      toast.success(`已派发给 Agent：${result.agent_task_id.slice(0, 8)}`);
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : "派发还原任务失败"),
  });

  const copyTaskJSON = async () => {
    if (!task) return;
    await navigator.clipboard?.writeText(JSON.stringify(task, null, 2));
    toast.success("已复制任务 JSON");
  };

  const copyItemContext = async (item: DesignRestoreTaskItemInput) => {
    const key = item.itemId;
    if (!key) {
      toast.error("任务项缺少 itemId，无法获取上下文");
      return;
    }
    setCopyingItemId(key);
    try {
      const context = await api.getDesignRestoreTaskItemContext(taskId, key);
      await navigator.clipboard?.writeText(JSON.stringify(context, null, 2));
      toast.success("已复制任务项上下文 JSON");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "复制任务项上下文失败");
    } finally {
      setCopyingItemId(null);
    }
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-muted/20">
      <BreadcrumbHeader
        segments={[{ href: paths.designs(), label: "设计库" }]}
        leaf={<span className="truncate font-medium">还原任务 {taskId.slice(0, 8)}</span>}
        actions={(
          <>
            <Button size="sm" variant="outline" onClick={() => navigation.push(paths.designs())}><ArrowLeft className="h-3.5 w-3.5" />返回</Button>
            <Button size="sm" variant="outline" disabled={!task} onClick={() => void copyTaskJSON()}><Copy className="h-3.5 w-3.5" />复制任务 JSON</Button>
          </>
        )}
      />
      {isLoading ? (
        <div className="grid gap-4 p-4 lg:grid-cols-[1fr_340px]"><Skeleton className="h-96" /><Skeleton className="h-96" /></div>
      ) : error || !task ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
          <p className="text-sm font-medium">无法加载此还原任务</p>
          <Button size="sm" variant="outline" onClick={() => void refetch()}>重试</Button>
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-auto p-4">
          <div className="grid gap-4 lg:grid-cols-[1fr_340px]">
            <div className="space-y-4">
              <section className="rounded-lg border bg-background p-4">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="flex items-center gap-2 text-sm font-medium"><FileJson className="h-4 w-4 text-muted-foreground" />设计还原任务</div>
                    <p className="mt-1 text-xs text-muted-foreground">按任务项复制上下文，供人或 Agent 逐个画板消费。</p>
                  </div>
                  <Badge variant="outline">{task.status}</Badge>
                </div>
                <div className="mt-4 grid gap-2 text-xs text-muted-foreground sm:grid-cols-2">
                  <div>任务 ID：<span className="font-mono">{task.id}</span></div>
                  <div>设计稿：<span className="font-mono">{task.file_id}</span></div>
                  <div>版本：<span className="font-mono">{task.revision_id}</span></div>
                  {task.issue_id ? <div>需求：<span className="font-mono">{task.issue_id}</span></div> : null}
                  {task.agent_task_id ? <div>Agent 任务：<span className="font-mono">{task.agent_task_id}</span></div> : null}
                  <div>创建时间：{task.created_at}</div>
                </div>
              </section>

              <section className="rounded-lg border bg-background">
                <div className="flex items-center justify-between border-b px-3 py-2">
                  <div className="flex items-center gap-2 text-sm font-medium"><Layers className="h-4 w-4 text-muted-foreground" />任务项</div>
                  <Badge variant="secondary">{items.length} 项</Badge>
                </div>
                <div className="divide-y">
                  {items.length ? items.map((item, index) => {
                    const key = itemKey(item, index);
                    return (
                      <div key={key} className="p-3">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <div className="flex items-center gap-2 text-sm font-medium"><Badge variant="secondary">#{item.order}</Badge><span className="truncate">{item.frameName || item.frameId}</span></div>
                            <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                              <span>{sourceLabel(item.source)}</span>
                              <span className="font-mono">{item.frameId}</span>
                              {item.layerIds?.length ? <span>{item.layerIds.length} 个图层</span> : null}
                            </div>
                            {item.note ? <p className="mt-2 text-xs text-muted-foreground">{item.note}</p> : null}
                          </div>
                          <div className="flex shrink-0 items-center gap-2">
                            <Button size="sm" variant="outline" onClick={() => navigation.push(paths.designFrameDetail(item.designFileId, item.frameId))}><ExternalLink className="h-3.5 w-3.5" />打开画板</Button>
                            <Button size="sm" onClick={() => void copyItemContext(item)} disabled={!item.itemId || copyingItemId === key}><Copy className="h-3.5 w-3.5" />{copyingItemId === key ? "复制中…" : "复制上下文"}</Button>
                          </div>
                        </div>
                      </div>
                    );
                  }) : <div className="p-6 text-center text-sm text-muted-foreground">暂无任务项</div>}
                </div>
              </section>
            </div>
            <aside className="space-y-4">
              <section className="rounded-lg border bg-background p-3">
                <div className="flex items-center gap-2 text-sm font-medium"><Bot className="h-4 w-4 text-muted-foreground" />交给 Agent</div>
                <p className="mt-1 text-xs text-muted-foreground">选择本地 Agent 消费 restore task context，进入执行队列。</p>
                <div className="mt-3 rounded-md border border-amber-200 bg-amber-50 p-2 text-xs leading-relaxed text-amber-900">
                  <div className="font-medium">结构化还原策略</div>
                  <div>模式：strict-structure</div>
                  <div>整图 preview / thumbnail / full-frame slice：禁止作为还原结果。</div>
                  <div>无法结构化时：标记阻塞，或输出“缺少可结构化 UI 稿”的占位说明。</div>
                </div>
                <div className="mt-3 space-y-3">
                  <div>
                    <label className="mb-1 block text-xs font-medium text-muted-foreground">Agent</label>
                    <select value={dispatchAgentId} onChange={(event) => setSelectedAgentId(event.target.value)} className="h-8 w-full rounded-md border bg-background px-2 text-xs" disabled={!availableAgents.length}>
                      {availableAgents.length ? availableAgents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name} · {agent.status}</option>) : <option value="">暂无可用 Agent</option>}
                    </select>
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium text-muted-foreground">关联 Issue ID（可选）</label>
                    <Input value={issueId || task.issue_id || ""} onChange={(event) => setIssueId(event.target.value)} placeholder="粘贴需求 Issue UUID" className="h-8 font-mono text-xs" />
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium text-muted-foreground">执行提示</label>
                    <Textarea value={prompt} onChange={(event) => setPrompt(event.target.value)} className="min-h-28 text-xs" />
                  </div>
                  <Button className="w-full" disabled={!dispatchAgentId || dispatchTask.isPending || task.status === "running"} onClick={() => dispatchTask.mutate()}>
                    <Bot className="h-3.5 w-3.5" />{task.status === "running" ? "执行中…" : task.agent_task_id ? "重新派发" : dispatchTask.isPending ? "派发中…" : "交给 Agent"}
                  </Button>
                </div>
              </section>
              {result ? (
                <>
                  <section className="rounded-lg border bg-background p-3">
                    <div className="flex items-center justify-between gap-2">
                      <div className="text-sm font-medium">执行摘要</div>
                      <Badge variant={resultStatus === "completed" ? "secondary" : resultStatus === "failed" || resultStatus === "blocked" ? "destructive" : "outline"}>{resultStatus}</Badge>
                    </div>
                    {resultText ? <p className="mt-2 text-xs leading-relaxed text-muted-foreground">{resultText}</p> : null}
                    <div className="mt-3 space-y-3 text-xs">
                      {resultFiles.length ? <div><div className="mb-1 font-medium">变更文件</div><ul className="space-y-1 text-muted-foreground">{resultFiles.map((file) => <li key={file} className="font-mono">{file}</li>)}</ul></div> : null}
                      {resultChecks.length ? <div><div className="mb-1 font-medium">检查命令</div><ul className="space-y-1 text-muted-foreground">{resultChecks.map((check) => <li key={check} className="font-mono">{check}</li>)}</ul></div> : null}
                      {resultBlockers.length ? <div><div className="mb-1 font-medium text-destructive">阻塞项</div><ul className="space-y-1 text-destructive">{resultBlockers.map((blocker) => <li key={blocker}>{blocker}</li>)}</ul></div> : null}
                      {restoreMapping.length ? <JsonBlock title="Restore Mapping" value={restoreMapping} /> : null}
                    </div>
                  </section>
                  <section className="rounded-lg border bg-background p-3">
                    <div className="text-sm font-medium">策略校验</div>
                    <div className="mt-3 space-y-2 text-xs text-muted-foreground">
                      <div className="flex items-center justify-between gap-2"><span>整图预览</span><Badge variant={usedFullFramePreview ? "destructive" : "secondary"}>{usedFullFramePreview ? "已使用" : "未使用"}</Badge></div>
                      <div className="flex items-center justify-between gap-2"><span>策略违规</span><Badge variant={policyViolation ? "destructive" : "secondary"}>{policyViolation || "无"}</Badge></div>
                      {usedLayerIds.length ? <div>使用图层：<span className="font-mono">{usedLayerIds.join(", ")}</span></div> : null}
                      {usedAssetIds.length ? <div>使用资产：<span className="font-mono">{usedAssetIds.join(", ")}</span></div> : null}
                    </div>
                  </section>
                </>
              ) : null}
              <JsonBlock title="任务输入" value={task.input} />
              {task.result && typeof task.result === "object" && Object.keys(task.result).length ? <JsonBlock title="执行结果" value={task.result} /> : null}
              {task.error ? <JsonBlock title="错误" value={task.error} /> : null}
            </aside>
          </div>
        </div>
      )}
    </div>
  );
}
