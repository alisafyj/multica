"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Bot,
  Check,
  CircleAlert,
  LoaderCircle,
  MoreHorizontal,
  RefreshCcw,
  Save,
  SlidersHorizontal,
  Target,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { designKeys } from "@multica/core/designs/keys";
import { projectDesignSystemDetailOptions } from "@multica/core/designs/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { projectDetailOptions } from "@multica/core/projects/queries";
import type {
  Agent,
  ProjectDesignSystem,
  ProjectDesignSystemScope,
  ProjectDesignSystemToken,
} from "@multica/core/types";
import { agentListOptions } from "@multica/core/workspace/queries";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@multica/ui/components/ui/sheet";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useIsMobile } from "@multica/ui/hooks/use-mobile";
import { BreadcrumbHeader } from "../layout/breadcrumb-header";
import { ReadonlyContent } from "../editor";
import { ProjectDesignSystemPreview } from "./project-design-system-preview";

const PLATFORM_LABELS: Record<string, string> = {
  web: "Web",
  mobile: "移动端",
  cross_platform: "跨端",
};

function statusLabel(system: ProjectDesignSystem): string {
  if (system.active_task || system.status === "generating") return "生成中";
  if (system.status === "saved") return "已保存";
  if (system.status === "draft") return "草稿";
  return "未建立";
}

function taskStage(system: ProjectDesignSystem): string | null {
  const status = system.active_task?.status;
  if (status === "queued" || status === "dispatched") return "准备上下文";
  if (status === "running") return "智能体生成";
  if (status === "completed") return "产物校验";
  return status || null;
}

function errorMessage(value: unknown): string | null {
  if (typeof value === "string" && value.trim()) return value.trim();
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const record = value as Record<string, unknown>;
  for (const key of ["message", "error", "reason", "code"]) {
    const candidate = record[key];
    if (typeof candidate === "string" && candidate.trim()) return candidate.trim();
  }
  return null;
}

function isAgentAvailable(agent: Agent | undefined): boolean {
  return Boolean(
    agent
      && agent.runtime_id
      && !agent.archived_at
      && agent.status !== "offline",
  );
}

function hasRenderableContent(system: ProjectDesignSystem): boolean {
  return Boolean(
    system.content.preview_html.trim()
      && (system.content.sections.length || system.content.token_groups.length),
  );
}

function scopeLabel(scope: ProjectDesignSystemScope, system: ProjectDesignSystem): string {
  if (scope.kind === "all") return "整个设计体系";
  if (scope.kind === "section") {
    return system.content.sections.find((section) => section.id === scope.id)?.title ?? "内容章节";
  }
  if (scope.kind === "token_group") {
    return system.content.token_groups.find((group) => group.id === scope.id)?.label ?? "Token 分组";
  }
  return system.content.locators.find((locator) => locator.id === scope.id)?.label
    ?? (scope.kind === "component" ? "组件" : "组合区块");
}

function formatDate(value: string | null | undefined): string {
  if (!value) return "尚未记录";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "尚未记录";
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function isColorValue(value: string): boolean {
  if (typeof CSS !== "undefined" && typeof CSS.supports === "function") {
    return CSS.supports("color", value);
  }
  return /^#[0-9a-f]{3,8}$/i.test(value);
}

function TokenPreview({ token }: { token: ProjectDesignSystemToken }) {
  const normalizedName = token.name.toLowerCase();
  if (isColorValue(token.value)) {
    return (
      <span
        aria-hidden="true"
        className="h-6 w-6 shrink-0 rounded-sm border shadow-sm"
        style={{ backgroundColor: token.value }}
      />
    );
  }
  if (normalizedName.includes("font-family")) {
    return (
      <span aria-hidden="true" className="w-8 shrink-0 text-center text-base" style={{ fontFamily: token.value }}>
        Aa
      </span>
    );
  }
  if (normalizedName.includes("font-weight")) {
    return (
      <span aria-hidden="true" className="w-8 shrink-0 text-center text-base" style={{ fontWeight: token.value }}>
        Aa
      </span>
    );
  }
  return <span aria-hidden="true" className="h-1.5 w-6 shrink-0 rounded-full bg-muted-foreground/30" />;
}

function updateSystemCache(
  queryClient: ReturnType<typeof useQueryClient>,
  wsId: string,
  system: ProjectDesignSystem,
) {
  queryClient.setQueryData(designKeys.projectDesignSystem(wsId, system.id), system);
  queryClient.setQueryData(
    designKeys.projectDesignSystemByProject(wsId, system.project_id),
    system,
  );
}

function AdjustmentPanel({
  system,
  agents,
  selectedAgentId,
  selectedScope,
  instruction,
  actionError,
  isAdjusting,
  isRegenerating,
  regenerateConfirmation,
  onAgentChange,
  onInstructionChange,
  onClearScope,
  onAdjust,
  onCancelRegenerate,
  onConfirmRegenerate,
}: {
  system: ProjectDesignSystem;
  agents: Agent[];
  selectedAgentId: string;
  selectedScope: ProjectDesignSystemScope;
  instruction: string;
  actionError: string | null;
  isAdjusting: boolean;
  isRegenerating: boolean;
  regenerateConfirmation: boolean;
  onAgentChange: (id: string) => void;
  onInstructionChange: (value: string) => void;
  onClearScope: () => void;
  onAdjust: () => void;
  onCancelRegenerate: () => void;
  onConfirmRegenerate: () => void;
}) {
  const selectedAgent = agents.find((agent) => agent.id === selectedAgentId);
  const agentAvailable = isAgentAvailable(selectedAgent);
  const busy = Boolean(system.active_task || system.status === "generating" || isAdjusting || isRegenerating);
  const agentOptions = agents.filter((agent) => !agent.archived_at || agent.id === selectedAgentId);
  const selectedLabel = scopeLabel(selectedScope, system);

  return (
    <div className="min-w-0">
      <section className="border-b pb-5">
        <div className="flex items-center gap-2 text-sm font-medium">
          <Bot className="h-4 w-4 text-muted-foreground" />
          执行智能体
        </div>
        <select
          aria-label="执行智能体"
          value={selectedAgentId}
          onChange={(event) => onAgentChange(event.target.value)}
          className="mt-3 h-9 w-full rounded-md border bg-background px-3 text-sm"
        >
          <option value="">选择智能体</option>
          {agentOptions.map((agent) => (
            <option key={agent.id} value={agent.id} disabled={!isAgentAvailable(agent)}>
              {agent.name} · {isAgentAvailable(agent) ? agent.status : "不可用"}
            </option>
          ))}
          {selectedAgentId && !agentOptions.some((agent) => agent.id === selectedAgentId) ? (
            <option value={selectedAgentId} disabled>之前选择的智能体 · 不可用</option>
          ) : null}
        </select>
        {selectedAgentId && !agentAvailable ? (
          <p className="mt-2 text-xs text-destructive">当前智能体不可用，请明确选择其他智能体。</p>
        ) : null}
      </section>

      <section className="border-b py-5">
        <div className="flex items-center justify-between gap-2">
          <span className="text-sm font-medium">调整范围</span>
          {selectedScope.kind !== "all" ? (
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              aria-label="恢复为整个设计体系"
              title="恢复为整个设计体系"
              onClick={onClearScope}
            >
              <X className="h-3.5 w-3.5" />
            </Button>
          ) : null}
        </div>
        <div className="mt-2 flex min-w-0 items-center gap-2 rounded-md border bg-muted/30 px-2.5 py-2 text-xs">
          <Target className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          <span className="min-w-0 break-words font-medium">{selectedLabel}</span>
        </div>
      </section>

      <section className="py-5">
        <label className="block space-y-2">
          <span className="text-sm font-medium">调整要求</span>
          <Textarea
            aria-label="调整要求"
            value={instruction}
            onChange={(event) => onInstructionChange(event.target.value)}
            placeholder="描述希望调整的视觉方向或组件行为"
            className="min-h-28 resize-y"
          />
        </label>
        <Button
          type="button"
          className="mt-3 w-full"
          disabled={busy || !agentAvailable || !instruction.trim()}
          onClick={onAdjust}
        >
          {isAdjusting ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <SlidersHorizontal className="h-4 w-4" />}
          {isAdjusting ? "提交中…" : "提交调整"}
        </Button>
      </section>

      {regenerateConfirmation ? (
        <section className="border-t py-5">
          <div role="alert" className="border-l-2 border-amber-500 bg-amber-500/5 px-3 py-2 text-xs leading-5">
            已保存内容会继续保留，新的结果将先成为草稿。
          </div>
          <div className="mt-3 flex gap-2">
            <Button type="button" size="sm" variant="outline" className="flex-1" onClick={onCancelRegenerate}>
              取消
            </Button>
            <Button
              type="button"
              size="sm"
              className="flex-1"
              disabled={busy || !agentAvailable}
              onClick={onConfirmRegenerate}
            >
              {isRegenerating ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <RefreshCcw className="h-3.5 w-3.5" />}
              确认重新生成
            </Button>
          </div>
        </section>
      ) : null}

      {system.active_task ? (
        <section className="border-t py-5">
          <div className="flex items-center gap-2 text-sm font-medium">
            <LoaderCircle className="h-4 w-4 animate-spin text-muted-foreground" />
            {taskStage(system) ?? "任务进行中"}
          </div>
          <p className="mt-1 text-xs text-muted-foreground">现有内容会保留到新产物完成校验。</p>
        </section>
      ) : null}

      {actionError ? (
        <div role="alert" className="flex items-start gap-2 border-l-2 border-destructive bg-destructive/5 px-3 py-2 text-xs text-destructive">
          <CircleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          <span>{actionError}</span>
        </div>
      ) : null}

      {system.activity.length ? (
        <section className="mt-5 border-t pt-5">
          <h3 className="text-sm font-medium">最近活动</h3>
          <ol className="mt-2 space-y-3">
            {system.activity.slice(0, 3).map((task) => (
              <li key={task.id} className="text-xs">
                <div className="flex items-center justify-between gap-2">
                  <span className="font-medium">{task.operation}</span>
                  <span className="text-muted-foreground">{task.status}</span>
                </div>
                <p className="mt-0.5 text-muted-foreground">{formatDate(task.completed_at ?? task.created_at)}</p>
              </li>
            ))}
          </ol>
        </section>
      ) : null}
    </div>
  );
}

export function ProjectDesignSystemPage({ designSystemId }: { designSystemId: string }) {
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const queryClient = useQueryClient();
  const isMobile = useIsMobile();
  const [selectedScope, setSelectedScope] = useState<ProjectDesignSystemScope>({ kind: "all" });
  const [instruction, setInstruction] = useState("");
  const [agentOverrides, setAgentOverrides] = useState<Record<string, string>>({});
  const [actionError, setActionError] = useState<string | null>(null);
  const [regenerateConfirmation, setRegenerateConfirmation] = useState(false);
  const [adjustmentOpen, setAdjustmentOpen] = useState(false);

  const {
    data: system,
    isLoading,
    error,
    refetch,
  } = useQuery(projectDesignSystemDetailOptions(wsId, designSystemId));
  const { data: project } = useQuery({
    ...projectDetailOptions(wsId, system?.project_id ?? ""),
    enabled: Boolean(system?.project_id),
  });
  const { data: agents = [] } = useQuery(agentListOptions(wsId));

  const overriddenAgentId = system ? agentOverrides[system.id] : undefined;
  const selectedAgentId = system
    ? (Object.prototype.hasOwnProperty.call(agentOverrides, system.id)
      ? overriddenAgentId ?? ""
      : system.current_agent_id ?? "")
    : "";
  const selectedAgent = agents.find((agent) => agent.id === selectedAgentId);
  const isBusy = Boolean(system?.active_task || system?.status === "generating");

  const adjustSystem = useMutation({
    mutationFn: ({ target, agentId, text, scope }: {
      target: ProjectDesignSystem;
      agentId: string;
      text: string;
      scope: ProjectDesignSystemScope;
    }) => api.adjustProjectDesignSystem(target.id, {
      agent_id: agentId,
      instruction: text,
      scope,
    }),
    onMutate: () => setActionError(null),
    onSuccess: (updated) => {
      updateSystemCache(queryClient, wsId, updated);
      setInstruction("");
      toast.success("调整任务已提交");
    },
    onError: (mutationError) => {
      const message = mutationError instanceof Error ? mutationError.message : "调整失败，原有内容已保留。";
      setActionError(message);
      toast.error(message);
    },
  });

  const regenerateSystem = useMutation({
    mutationFn: ({ target, agentId }: { target: ProjectDesignSystem; agentId: string }) => (
      api.regenerateProjectDesignSystem(target.id, { agent_id: agentId })
    ),
    onMutate: () => setActionError(null),
    onSuccess: (updated) => {
      updateSystemCache(queryClient, wsId, updated);
      setRegenerateConfirmation(false);
      toast.success("重新生成任务已提交");
    },
    onError: (mutationError) => {
      const message = mutationError instanceof Error ? mutationError.message : "重新生成失败，原有内容已保留。";
      setActionError(message);
      toast.error(message);
    },
  });

  const saveSystem = useMutation({
    mutationFn: (target: ProjectDesignSystem) => api.saveProjectDesignSystem(target.id),
    onMutate: () => setActionError(null),
    onSuccess: (updated) => {
      updateSystemCache(queryClient, wsId, updated);
      toast.success("已保存为项目设计体系");
    },
    onError: (mutationError) => {
      const message = mutationError instanceof Error ? mutationError.message : "保存失败，请稍后重试。";
      setActionError(message);
      toast.error(message);
    },
  });

  const selectedScopeLabel = system ? scopeLabel(selectedScope, system) : "整个设计体系";
  const canSave = Boolean(
    system
      && system.status === "draft"
      && system.has_unsaved_changes
      && hasRenderableContent(system)
      && !isBusy
      && !saveSystem.isPending,
  );

  const navigationItems = useMemo(() => {
    if (!system) return [];
    return [
      ...system.content.sections.map((section) => ({
        id: `section-${section.id}`,
        label: section.title,
        scope: { kind: "section" as const, id: section.id },
      })),
      ...system.content.token_groups.map((group) => ({
        id: `tokens-${group.id}`,
        label: group.label,
        scope: { kind: "token_group" as const, id: group.id },
      })),
    ];
  }, [system]);

  const selectAndScroll = (scope: ProjectDesignSystemScope, targetId: string) => {
    setSelectedScope(scope);
    document.getElementById(targetId)?.scrollIntoView?.({ behavior: "smooth", block: "start" });
  };

  if (isLoading) {
    return (
      <div className="flex min-h-0 flex-1 flex-col">
        <Skeleton className="h-12 w-full rounded-none" />
        <div className="grid gap-6 p-6 lg:grid-cols-[180px_minmax(0,1fr)] xl:grid-cols-[180px_minmax(0,1fr)_300px]">
          <Skeleton className="h-64" />
          <Skeleton className="h-[680px]" />
          <Skeleton className="h-96" />
        </div>
      </div>
    );
  }

  if (error || !system?.id) {
    return (
      <div className="flex min-h-0 flex-1 flex-col">
        <BreadcrumbHeader segments={[{ href: paths.designs(), label: "设计中心" }]} leaf="设计体系" />
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
          <p className="text-sm font-medium">无法加载此设计体系</p>
          <Button size="sm" variant="outline" onClick={() => void refetch()}>重试</Button>
        </div>
      </div>
    );
  }

  const panel = (
    <AdjustmentPanel
      system={system}
      agents={agents}
      selectedAgentId={selectedAgentId}
      selectedScope={selectedScope}
      instruction={instruction}
      actionError={actionError ?? errorMessage(system.last_error)}
      isAdjusting={adjustSystem.isPending}
      isRegenerating={regenerateSystem.isPending}
      regenerateConfirmation={regenerateConfirmation}
      onAgentChange={(id) => setAgentOverrides((current) => ({ ...current, [system.id]: id }))}
      onInstructionChange={setInstruction}
      onClearScope={() => setSelectedScope({ kind: "all" })}
      onAdjust={() => {
        if (!instruction.trim() || !isAgentAvailable(selectedAgent)) return;
        adjustSystem.mutate({
          target: system,
          agentId: selectedAgentId,
          text: instruction.trim(),
          scope: selectedScope,
        });
      }}
      onCancelRegenerate={() => setRegenerateConfirmation(false)}
      onConfirmRegenerate={() => {
        if (!isAgentAvailable(selectedAgent)) return;
        regenerateSystem.mutate({ target: system, agentId: selectedAgentId });
      }}
    />
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-background">
      <BreadcrumbHeader
        segments={[{ href: paths.designs(), label: "设计中心" }]}
        leaf={<span className="truncate font-medium">{system.name || "项目设计体系"}</span>}
        actions={(
          <div className="flex items-center gap-2">
            {isMobile ? (
              <Button type="button" size="sm" variant="outline" onClick={() => setAdjustmentOpen(true)}>
                <SlidersHorizontal className="h-3.5 w-3.5" />
                调整
              </Button>
            ) : null}
            <Button
              type="button"
              size={isMobile ? "icon-sm" : "sm"}
              aria-label="保存为项目设计体系"
              title={isMobile ? "保存为项目设计体系" : undefined}
              disabled={!canSave}
              onClick={() => saveSystem.mutate(system)}
            >
              {saveSystem.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
              {isMobile ? null : "保存为项目设计体系"}
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger render={<Button type="button" size="icon-sm" variant="outline" aria-label="更多操作" />}>
                <MoreHorizontal className="h-4 w-4" />
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  disabled={isBusy || regenerateSystem.isPending}
                  onClick={() => {
                    setRegenerateConfirmation(true);
                    if (isMobile) setAdjustmentOpen(true);
                  }}
                >
                  <RefreshCcw className="h-4 w-4" />
                  重新生成设计体系
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        )}
      />

      <header className="border-b px-5 py-5 lg:px-7">
        <div className="mx-auto flex w-full max-w-[1600px] flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0">
            <p className="text-xs text-muted-foreground">{project?.title ?? "项目"}</p>
            <h1 className="mt-1 break-words text-xl font-semibold">{system.name || "项目设计体系"}</h1>
            <p className="mt-1 text-sm text-muted-foreground">最近更新 {formatDate(system.updated_at)}</p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline">{PLATFORM_LABELS[system.platform] ?? "未指定平台"}</Badge>
            <Badge variant={system.status === "saved" ? "secondary" : "outline"}>{statusLabel(system)}</Badge>
            {system.has_unsaved_changes ? <Badge variant="outline">有未保存更改</Badge> : null}
          </div>
        </div>
      </header>

      <div className="min-h-0 flex-1 overflow-auto">
        <div className="mx-auto grid w-full max-w-[1600px] gap-x-7 px-5 py-7 lg:grid-cols-[180px_minmax(0,1fr)] lg:px-7 xl:grid-cols-[180px_minmax(0,1fr)_300px]">
          <nav aria-label="设计体系内容" className="hidden min-w-0 lg:block">
            <div className="sticky top-6 space-y-1 border-l pl-3">
              {navigationItems.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  aria-label={`选择范围：${item.label}`}
                  className={`block w-full rounded-sm px-2 py-1.5 text-left text-xs leading-5 ${selectedScope.kind === item.scope.kind && selectedScope.id === item.scope.id ? "bg-muted font-medium text-foreground" : "text-muted-foreground hover:text-foreground"}`}
                  onClick={() => selectAndScroll(item.scope, item.id)}
                >
                  {item.label}
                </button>
              ))}
              <button
                type="button"
                className="block w-full rounded-sm px-2 py-1.5 text-left text-xs leading-5 text-muted-foreground hover:text-foreground"
                onClick={() => document.getElementById("ui-kit")?.scrollIntoView?.({ behavior: "smooth", block: "start" })}
              >
                在线 UI Kit
              </button>
            </div>
          </nav>

          <main className="min-w-0 space-y-10">
            {system.content.sections.map((section) => (
              <section key={section.id} id={`section-${section.id}`} className="scroll-mt-6 border-b pb-8">
                <div className="mb-4 flex items-start justify-between gap-3">
                  <h2 className="break-words text-lg font-semibold">{section.title}</h2>
                  {selectedScope.kind === "section" && selectedScope.id === section.id ? (
                    <Badge variant="secondary"><Check className="h-3 w-3" />已定位</Badge>
                  ) : null}
                </div>
                <ReadonlyContent content={section.markdown} className="max-w-none text-sm leading-7" />
              </section>
            ))}

            {system.content.token_groups.map((group) => (
              <section key={group.id} id={`tokens-${group.id}`} className="scroll-mt-6 border-b pb-8">
                <div className="mb-4 flex items-start justify-between gap-3">
                  <h2 className="break-words text-lg font-semibold">{group.label}</h2>
                  {selectedScope.kind === "token_group" && selectedScope.id === group.id ? (
                    <Badge variant="secondary"><Check className="h-3 w-3" />已定位</Badge>
                  ) : null}
                </div>
                <div className="grid gap-x-6 sm:grid-cols-2">
                  {group.tokens.map((token) => (
                    <div key={token.name} className="flex min-w-0 items-center gap-3 border-t py-3 first:border-t-0 sm:[&:nth-child(2)]:border-t-0">
                      <TokenPreview token={token} />
                      <div className="min-w-0 flex-1">
                        <div className="break-all text-xs font-medium">{token.name}</div>
                        <div className="mt-0.5 break-all font-mono text-xs text-muted-foreground">{token.value}</div>
                      </div>
                    </div>
                  ))}
                </div>
              </section>
            ))}

            <section id="ui-kit" className="scroll-mt-6">
              <div className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
                <div>
                  <h2 className="text-lg font-semibold">在线 UI Kit</h2>
                  <p className="mt-1 text-xs text-muted-foreground">点击预览中的组件或区块，可将调整范围定位到对应内容。</p>
                </div>
                {selectedScope.kind === "component" || selectedScope.kind === "block" ? (
                  <Badge variant="secondary"><Target className="h-3 w-3" />{selectedScopeLabel}</Badge>
                ) : null}
              </div>
              <div className="overflow-hidden border bg-white">
                <ProjectDesignSystemPreview
                  previewHtml={system.content.preview_html}
                  locators={system.content.locators}
                  onSelect={setSelectedScope}
                />
              </div>
            </section>
          </main>

          {!isMobile ? (
            <aside className="mt-10 min-w-0 border-t pt-6 lg:col-start-2 xl:col-start-auto xl:mt-0 xl:border-l xl:border-t-0 xl:pl-6 xl:pt-0">
              <div className="xl:sticky xl:top-6">{panel}</div>
            </aside>
          ) : null}
        </div>
      </div>

      {isMobile ? (
        <Sheet open={adjustmentOpen} onOpenChange={setAdjustmentOpen}>
          <SheetContent side="right" className="w-[min(92vw,380px)] overflow-y-auto">
            <SheetHeader className="border-b">
              <SheetTitle>调整设计体系</SheetTitle>
            </SheetHeader>
            <div className="px-4 pb-6">{panel}</div>
          </SheetContent>
        </Sheet>
      ) : null}
    </div>
  );
}
