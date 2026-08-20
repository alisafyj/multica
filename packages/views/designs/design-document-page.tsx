"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowUp,
  CircleAlert,
  Code2,
  ExternalLink,
  Eye,
  History,
  LoaderCircle,
  Maximize2,
  Minimize2,
  Monitor,
  MoreHorizontal,
  RotateCcw,
  RotateCw,
  Scan,
  Smartphone,
  X,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { designKeys } from "@multica/core/designs/keys";
import {
  designDocumentDetailOptions,
  designDocumentRevisionListOptions,
  designDocumentRevisionOptions,
} from "@multica/core/designs/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { projectDetailOptions } from "@multica/core/projects/queries";
import { agentListOptions } from "@multica/core/workspace/queries";
import type {
  Agent,
  DesignDocument,
  DesignDocumentAdjustmentScope,
  DesignDocumentPage as DesignDocumentPageEntry,
  DesignDocumentRevision,
  DesignDocumentRevisionSummary,
} from "@multica/core/types";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { BreadcrumbHeader } from "../layout/breadcrumb-header";
import { useNavigation } from "../navigation";
import { useTimeAgo } from "../i18n/use-time-ago";
import { designDocumentStatusLabel } from "./design-document-card";
import { DesignDocumentCritique, parseCritique } from "./design-document-critique";
import { DesignDocumentSourceView } from "./design-document-source-view";
import { AgentSetting } from "./design-task-composer";
import { DesignTaskActivity, taskOperationLabel } from "./project-design-system-task-activity";

const INSTRUCTION_MAX_LENGTH = 8000;

/**
 * Ready-made adjustments the workspace offers as chips (DC-050). Each is a
 * plain instruction to the agent — the prompt states the convention the agent
 * follows for it — so a preset never bypasses the normal adjust → Audit →
 * Preview path.
 */
export const ADJUSTMENT_PRESETS: ReadonlyArray<{ id: string; label: string; instruction: string }> = [
  {
    id: "tweaks",
    label: "添加调整面板",
    instruction:
      "为原型添加一个调整面板（tweaks）：把强调色、字号比例、密度、明暗模式和动效通过 CSS 自定义属性 --accent / --scale / --density / --mode / --motion 驱动，附带一个默认收起、可从浮动标签打开的包内侧栏控件（色板与取色、缩放与密度滑杆、明暗切换、动效开关、重置），选择保存在 localStorage 并在加载时恢复。整个包继续离线可运行，其余设计保持不变。",
  },
];

type PreviewViewport = "fit" | "desktop" | "mobile";

const VIEWPORTS: ReadonlyArray<{ id: PreviewViewport; label: string; width: number | null; icon: typeof Monitor }> = [
  { id: "fit", label: "适应", width: null, icon: Scan },
  { id: "desktop", label: "桌面", width: 1280, icon: Monitor },
  { id: "mobile", label: "移动", width: 390, icon: Smartphone },
];

/** Zoom presets for the preview frame; index into ZOOM_LEVELS. */
const ZOOM_LEVELS = [0.5, 0.75, 1, 1.25, 1.5] as const;
const ZOOM_DEFAULT_INDEX = 2;

/** The viewport a document opens in: a mobile design starts on a phone width. */
function defaultViewport(platform: string): PreviewViewport {
  return platform === "mobile" ? "mobile" : "fit";
}

function platformLabel(platform: string): string {
  if (platform === "mobile") return "移动端";
  if (platform === "cross_platform") return "跨端";
  if (platform === "web") return "Web";
  return "";
}

/** The revision the workspace shows by default: the draft, else what was saved, else the newest. */
export function defaultRevisionId(document: DesignDocument | undefined, revisions: DesignDocumentRevisionSummary[]): string {
  if (document?.draft_revision_id) return document.draft_revision_id;
  if (document?.saved_revision_id) return document.saved_revision_id;
  return revisions[0]?.id ?? "";
}

/**
 * The prototype documents a revision can show, in page order: pages first, then
 * any preview target the brief did not list as a page. Never empty for a valid
 * revision because the prototype entry is always a preview target.
 */
export function previewEntries(revision: DesignDocumentRevision | undefined): Array<{ id: string; title: string; entry: string; page: DesignDocumentPageEntry | null }> {
  if (!revision) return [];
  const seen = new Set<string>();
  const entries: Array<{ id: string; title: string; entry: string; page: DesignDocumentPageEntry | null }> = [];
  for (const page of revision.pages) {
    if (!page.entry || seen.has(page.entry)) continue;
    seen.add(page.entry);
    entries.push({ id: page.id || page.entry, title: page.title || page.entry, entry: page.entry, page });
  }
  for (const target of revision.preview_targets) {
    if (!target.path || seen.has(target.path)) continue;
    seen.add(target.path);
    const isEntry = target.path === revision.prototype_entry;
    entries.push({ id: target.id || target.path, title: isEntry ? "首页" : target.path.replace(/^prototype\//, ""), entry: target.path, page: null });
  }
  return entries;
}

/** A readable message out of the server's last_error, whatever shape it took. */
export function documentErrorMessage(value: unknown): string | null {
  if (!value) return null;
  if (typeof value === "string") return value;
  if (typeof value === "object") {
    const record = value as Record<string, unknown>;
    for (const key of ["message", "error", "reason", "code"]) {
      const candidate = record[key];
      if (typeof candidate === "string" && candidate.trim()) return candidate;
    }
  }
  return "任务未能产出可用的设计稿。";
}

function briefOf(document: DesignDocument | undefined): string {
  const snapshot = document?.input_snapshot;
  if (snapshot && typeof snapshot === "object") {
    const brief = (snapshot as Record<string, unknown>).brief;
    if (typeof brief === "string") return brief;
  }
  return "";
}

function scopeLabelOf(scope: unknown, entries: ReturnType<typeof previewEntries>): string {
  if (!scope || typeof scope !== "object") return "整份文档";
  const record = scope as { kind?: unknown; id?: unknown };
  if (record.kind === "page" && typeof record.id === "string") {
    const match = entries.find((entry) => entry.id === record.id || entry.entry === record.id);
    return `页面 · ${match?.title ?? record.id}`;
  }
  if (record.kind === "document" || !record.kind) return "整份文档";
  return typeof record.id === "string" ? `${String(record.kind)} · ${record.id}` : String(record.kind);
}

/**
 * One row of the revision timeline. The newest run sits on top; the row the
 * user is looking at is marked, and rows that are not the current draft can be
 * brought back with 回退.
 */
function RevisionRow({
  revision,
  selected,
  entries,
  agents,
  busy,
  onSelect,
  onRestore,
}: {
  revision: DesignDocumentRevisionSummary;
  selected: boolean;
  entries: ReturnType<typeof previewEntries>;
  agents: Agent[];
  busy: boolean;
  onSelect: () => void;
  onRestore: () => void;
}) {
  const timeAgo = useTimeAgo();
  const agent = agents.find((candidate) => candidate.id === revision.agent_id);
  const isAdjustment = revision.instruction.trim().length > 0 || revision.base_revision_id !== "";
  return (
    <li className={cn("group rounded-lg border bg-card p-3 transition-colors", selected ? "border-primary/60" : "hover:border-primary/30")}>
      <button type="button" className="block w-full text-left" onClick={onSelect} aria-current={selected ? "true" : undefined}>
        <div className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 items-center gap-2 text-body font-medium">
            <span>v{revision.revision_number}</span>
            <span className="text-caption font-normal text-muted-foreground">{isAdjustment ? "调整" : "生成"}</span>
          </div>
          <div className="flex shrink-0 items-center gap-1">
            {revision.is_draft ? <Badge variant="secondary" className="px-1.5 text-micro font-normal">草稿</Badge> : null}
            {revision.is_saved ? <Badge variant="outline" className="px-1.5 text-micro font-normal">已保存</Badge> : null}
          </div>
        </div>
        {revision.instruction ? (
          <p className="mt-1.5 line-clamp-3 text-caption leading-5 text-foreground">{revision.instruction}</p>
        ) : null}
        <div className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-caption text-muted-foreground">
          {isAdjustment ? <span>{scopeLabelOf(revision.scope, entries)}</span> : null}
          {agent ? <span>{agent.name}</span> : null}
          {revision.created_at ? <span>{timeAgo(revision.created_at)}</span> : null}
          {revision.page_count > 0 ? <span>{revision.page_count} 页</span> : null}
        </div>
      </button>
      {!revision.is_draft ? (
        <div className="mt-2 flex justify-end">
          <Button type="button" size="sm" variant="ghost" className="h-7 px-2 text-caption" disabled={busy} onClick={onRestore}>
            <RotateCcw className="h-3.5 w-3.5" />
            回退到此版本
          </Button>
        </div>
      ) : null}
    </li>
  );
}

export function DesignDocumentPage({ documentId }: { documentId: string }) {
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const queryClient = useQueryClient();

  const documentQuery = useQuery(designDocumentDetailOptions(wsId, documentId));
  const document = documentQuery.data;
  const { data: revisions = [] } = useQuery(designDocumentRevisionListOptions(wsId, documentId));
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: project } = useQuery({
    ...projectDetailOptions(wsId, document?.project_id ?? ""),
    enabled: !!document?.project_id,
  });

  // The revision on screen. Unset follows the document (draft, then saved);
  // set means the user pinned a historical version and it stays until they
  // leave it, even if a new draft lands.
  const [pinnedRevisionId, setPinnedRevisionId] = useState("");
  const currentRevisionId = defaultRevisionId(document, revisions);
  const selectedRevisionId = pinnedRevisionId && revisions.some((row) => row.id === pinnedRevisionId)
    ? pinnedRevisionId
    : currentRevisionId;
  const viewingHistory = selectedRevisionId !== "" && selectedRevisionId !== currentRevisionId;

  const revisionQuery = useQuery(designDocumentRevisionOptions(wsId, documentId, selectedRevisionId));
  const revision = revisionQuery.data;
  const entries = useMemo(() => previewEntries(revision), [revision]);
  const critique = useMemo(() => parseCritique(revision?.critique), [revision]);

  const [activeEntry, setActiveEntry] = useState("");
  const shownEntry = entries.some((entry) => entry.entry === activeEntry) ? activeEntry : entries[0]?.entry ?? "";
  const shownPage = entries.find((entry) => entry.entry === shownEntry) ?? null;

  const [viewport, setViewport] = useState<PreviewViewport | null>(null);
  const effectiveViewport = viewport ?? defaultViewport(document?.platform ?? "");
  // Open Design's 预览/代码 toggle: the same revision, rendered or read.
  const [viewMode, setViewMode] = useState<"preview" | "code">("preview");
  const [zoomIndex, setZoomIndex] = useState(ZOOM_DEFAULT_INDEX);
  const zoom = ZOOM_LEVELS[zoomIndex] ?? 1;
  const [reloadKey, setReloadKey] = useState(0);
  const [fullscreen, setFullscreen] = useState(false);
  useEffect(() => {
    if (!fullscreen) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setFullscreen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [fullscreen]);

  const [instruction, setInstruction] = useState("");
  const [scopeToPage, setScopeToPage] = useState(false);
  const [agentOverride, setAgentOverride] = useState("");
  const [discardOpen, setDiscardOpen] = useState(false);

  const status = document?.status ?? "empty";
  const activeTask = document?.active_task ?? null;
  const running = status === "running";
  const latestAgentId = revisions[0]?.agent_id ?? activeTask?.agent_id ?? "";
  const agentId = agentOverride || latestAgentId;
  const canSave = !!document && !running && (status === "draft" || status === "draft_ahead_of_saved") && !!document.draft_revision_id;
  const canDiscard = !!document && !running && !!document.draft_revision_id && document.draft_revision_id !== document.saved_revision_id;
  const canAdjust = !!document && !running && (!!document.draft_revision_id || !!document.saved_revision_id);
  const errorMessage = status === "failed" ? documentErrorMessage(document?.last_error) : null;
  const previewUrl = revision && shownEntry ? api.getDesignDocumentPreviewFileURL(revision.resource_base_path, shownEntry) : "";

  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: designKeys.document(wsId, documentId) }),
      queryClient.invalidateQueries({ queryKey: designKeys.documentRevisions(wsId, documentId) }),
      document ? queryClient.invalidateQueries({ queryKey: designKeys.documents(wsId, document.project_id) }) : Promise.resolve(),
    ]);
  };

  const applyDocument = (next: DesignDocument) => {
    queryClient.setQueryData(designKeys.document(wsId, documentId), next);
  };

  const adjust = useMutation({
    mutationFn: () => {
      const scope: Pick<DesignDocumentAdjustmentScope, "kind" | "id"> = scopeToPage && shownPage
        ? { kind: "page", id: shownPage.page?.id ?? shownPage.entry }
        : { kind: "document" };
      return api.adjustDesignDocument(documentId, {
        instruction: instruction.trim(),
        agent_id: agentId,
        scope,
        base_revision_id: currentRevisionId || undefined,
      });
    },
    onSuccess: async (next) => {
      applyDocument(next);
      setInstruction("");
      setPinnedRevisionId("");
      await refresh();
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "无法发起调整"),
  });

  const save = useMutation({
    mutationFn: () => api.saveDesignDocument(documentId, { draft_revision_id: document?.draft_revision_id ?? "" }),
    onSuccess: async (next) => {
      applyDocument(next);
      toast.success("已保存为当前设计稿");
      await refresh();
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "保存失败"),
  });

  const discard = useMutation({
    mutationFn: () => api.discardDesignDocumentDraft(documentId),
    onSuccess: async (next) => {
      applyDocument(next);
      setPinnedRevisionId("");
      setDiscardOpen(false);
      toast.success("已放弃草稿");
      await refresh();
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "放弃草稿失败"),
  });

  const title = document?.title.trim() || "设计稿";

  const downloadArchive = useMutation({
    mutationFn: async () => {
      if (!revision) throw new Error("没有可下载的版本");
      const blob = await api.downloadDesignDocumentRevisionArchive(documentId, revision.id);
      const href = URL.createObjectURL(blob);
      const anchor = window.document.createElement("a");
      anchor.href = href;
      anchor.download = `${title}-v${revision.revision_number}.zip`;
      anchor.rel = "noopener";
      window.document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      window.setTimeout(() => URL.revokeObjectURL(href), 10_000);
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "下载失败"),
  });

  const restore = useMutation({
    mutationFn: (revisionId: string) => api.restoreDesignDocumentRevision(documentId, revisionId),
    onSuccess: async (next) => {
      applyDocument(next);
      setPinnedRevisionId("");
      toast.success("已回退到所选版本，可继续调整或保存");
      await refresh();
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "回退失败"),
  });

  const busy = adjust.isPending || save.isPending || discard.isPending || restore.isPending;
  const instructionBlocker = !canAdjust
    ? (running ? "任务执行中，完成后可以继续调整" : "还没有可以调整的版本")
    : !instruction.trim()
      ? "描述你想怎么改"
      : !agentId
        ? "选择执行调整的智能体"
        : instruction.length > INSTRUCTION_MAX_LENGTH
          ? "说明太长了"
          : null;

  const statusLabel = document ? designDocumentStatusLabel(document.status) : null;

  if (documentQuery.isLoading) {
    return (
      <div className="flex min-h-0 flex-1 flex-col">
        <BreadcrumbHeader segments={[{ href: paths.designs(), label: "设计库" }]} leaf={<Skeleton className="h-4 w-32" />} />
        <div className="grid min-h-0 flex-1 gap-4 p-4 lg:grid-cols-[320px_1fr]"><Skeleton className="h-full min-h-64" /><Skeleton className="h-full min-h-64" /></div>
      </div>
    );
  }
  if (documentQuery.error || !document) {
    return (
      <div className="flex min-h-0 flex-1 flex-col">
        <BreadcrumbHeader segments={[{ href: paths.designs(), label: "设计库" }]} leaf={<span className="font-medium">设计稿</span>} />
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
          <p className="text-body font-medium">无法加载这份设计稿</p>
          <Button size="sm" variant="outline" onClick={() => void documentQuery.refetch()}>重试</Button>
        </div>
      </div>
    );
  }

  const frameWidth = VIEWPORTS.find((option) => option.id === effectiveViewport)?.width ?? null;
  const previewFrame = (
    <div className={cn("relative flex min-h-0 flex-1 flex-col overflow-hidden", fullscreen ? "fixed inset-0 z-50 bg-background" : "rounded-lg border bg-muted/30")}>
      <div className="flex shrink-0 items-center gap-2 border-b bg-background px-2 py-1.5">
        {/* Open Design's 预览/代码 segmented: the same revision, rendered or read. */}
        <div role="group" aria-label="查看方式" className="flex shrink-0 items-center gap-0.5 rounded-lg border bg-muted/40 p-0.5">
          <button
            type="button"
            aria-pressed={viewMode === "preview"}
            onClick={() => setViewMode("preview")}
            className={cn("flex items-center gap-1 rounded-md px-2 py-0.5 text-caption", viewMode === "preview" ? "bg-background font-medium text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground")}
          >
            <Eye className="h-3.5 w-3.5" />
            预览
          </button>
          <button
            type="button"
            aria-pressed={viewMode === "code"}
            disabled={!revision}
            onClick={() => setViewMode("code")}
            className={cn("flex items-center gap-1 rounded-md px-2 py-0.5 text-caption disabled:opacity-50", viewMode === "code" ? "bg-background font-medium text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground")}
          >
            <Code2 className="h-3.5 w-3.5" />
            代码
          </button>
        </div>
        <div className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto" role="tablist" aria-label="页面">
          {viewMode === "preview" ? entries.map((entry) => (
            <button
              key={entry.entry}
              type="button"
              role="tab"
              aria-selected={entry.entry === shownEntry}
              onClick={() => setActiveEntry(entry.entry)}
              className={cn(
                "shrink-0 rounded-md px-2.5 py-1 text-caption transition-colors",
                entry.entry === shownEntry ? "bg-accent font-medium text-foreground" : "text-muted-foreground hover:text-foreground",
              )}
            >
              {entry.title}
            </button>
          )) : (
            <span className="px-2 text-caption text-muted-foreground">{revision ? `${revision.files.length} 个文件` : ""}</span>
          )}
          {viewMode === "preview" && entries.length === 0 && !revisionQuery.isLoading ? <span className="px-2 text-caption text-muted-foreground">暂无可预览的页面</span> : null}
        </div>
        <div className="flex shrink-0 items-center gap-0.5">
          {viewMode === "preview" ? (
            <>
              {VIEWPORTS.map(({ id, label, icon: Icon }) => (
                <Button
                  key={id}
                  type="button"
                  size="icon-sm"
                  variant="ghost"
                  title={label}
                  aria-label={label}
                  aria-pressed={effectiveViewport === id}
                  className={cn(effectiveViewport === id && "bg-accent text-foreground")}
                  onClick={() => setViewport(id)}
                >
                  <Icon className="h-3.5 w-3.5" />
                </Button>
              ))}
              <span className="mx-1 h-4 w-px bg-border" aria-hidden />
              <Button type="button" size="icon-sm" variant="ghost" title="缩小" aria-label="缩小" disabled={zoomIndex === 0} onClick={() => setZoomIndex((index) => Math.max(0, index - 1))}>
                <ZoomOut className="h-3.5 w-3.5" />
              </Button>
              <button
                type="button"
                title="恢复 100%"
                aria-label={`缩放 ${Math.round(zoom * 100)}%，点击恢复 100%`}
                className="min-w-11 rounded px-1 text-center text-micro tabular-nums text-muted-foreground hover:text-foreground"
                onClick={() => setZoomIndex(ZOOM_DEFAULT_INDEX)}
              >
                {Math.round(zoom * 100)}%
              </button>
              <Button type="button" size="icon-sm" variant="ghost" title="放大" aria-label="放大" disabled={zoomIndex === ZOOM_LEVELS.length - 1} onClick={() => setZoomIndex((index) => Math.min(ZOOM_LEVELS.length - 1, index + 1))}>
                <ZoomIn className="h-3.5 w-3.5" />
              </Button>
              <span className="mx-1 h-4 w-px bg-border" aria-hidden />
              <Button type="button" size="icon-sm" variant="ghost" title="重新加载" aria-label="重新加载" onClick={() => setReloadKey((value) => value + 1)}>
                <RotateCw className="h-3.5 w-3.5" />
              </Button>
              <Button type="button" size="icon-sm" variant="ghost" title="在新标签页中打开" aria-label="在新标签页中打开" disabled={!previewUrl} onClick={() => window.open(previewUrl, "_blank", "noopener,noreferrer")}>
                <ExternalLink className="h-3.5 w-3.5" />
              </Button>
            </>
          ) : null}
          <Button type="button" size="icon-sm" variant="ghost" title={fullscreen ? "退出全屏" : "全屏"} aria-label={fullscreen ? "退出全屏" : "全屏"} onClick={() => setFullscreen((value) => !value)}>
            {fullscreen ? <Minimize2 className="h-3.5 w-3.5" /> : <Maximize2 className="h-3.5 w-3.5" />}
          </Button>
        </div>
      </div>
      {viewingHistory && revision ? (
        <div className="flex shrink-0 items-center justify-between gap-3 border-b bg-muted/40 px-3 py-1.5 text-caption">
          <span className="flex items-center gap-1.5 text-muted-foreground"><History className="h-3.5 w-3.5" />正在查看历史版本 v{revision.revision_number}</span>
          <Button type="button" size="sm" variant="ghost" className="h-6 px-2 text-caption" onClick={() => setPinnedRevisionId("")}>回到当前版本</Button>
        </div>
      ) : null}
      {viewMode === "code" && revision ? (
        <div className="min-h-0 flex-1">
          <DesignDocumentSourceView key={revision.id} revision={revision} />
        </div>
      ) : (
        <div className="flex min-h-0 flex-1 items-start justify-center overflow-auto p-3">
          {revisionQuery.isLoading ? (
            <Skeleton className="h-full min-h-64 w-full" />
          ) : previewUrl ? (
            // Zoom wrapper: the outer box takes the scaled footprint so the
            // scroll area is honest, while the iframe keeps its full CSS width
            // and is transform-scaled down/up inside it.
            <div
              className="h-full min-h-[480px]"
              style={{ width: frameWidth ? frameWidth * zoom : `${100 * zoom}%`, maxWidth: zoom <= 1 ? "100%" : undefined }}
            >
              <iframe
                key={`${selectedRevisionId}:${shownEntry}:${reloadKey}`}
                title={`${title} · ${shownPage?.title ?? "预览"}`}
                src={previewUrl}
                sandbox="allow-scripts"
                referrerPolicy="no-referrer"
                className="rounded-md border bg-background shadow-sm"
                style={{
                  width: frameWidth ?? `${100 / zoom}%`,
                  height: `${100 / zoom}%`,
                  minHeight: 480 / zoom,
                  transform: `scale(${zoom})`,
                  transformOrigin: "top left",
                }}
              />
            </div>
          ) : (
            <div className="flex h-full min-h-64 w-full flex-col items-center justify-center gap-2 text-center text-caption text-muted-foreground">
              {status === "running" ? "智能体正在生成，完成并通过校验后这里会显示原型。" : status === "failed" ? "这次运行没有产出可用的原型。" : "还没有可预览的版本。"}
            </div>
          )}
        </div>
      )}
    </div>
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <BreadcrumbHeader
        segments={[
          { href: paths.designs(), label: "设计库" },
          ...(project ? [{ href: paths.projectDetail(project.id), label: project.title }] : []),
        ]}
        leaf={<span className="flex min-w-0 items-center gap-2"><span className="truncate font-medium">{title}</span>{statusLabel ? <Badge variant="secondary" className="px-1.5 text-micro font-normal">{statusLabel}</Badge> : null}</span>}
        actions={(
          <div className="flex items-center gap-2">
            {canSave ? (
              <Button size="sm" disabled={busy} onClick={() => save.mutate()}>
                {save.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : null}
                {status === "draft_ahead_of_saved" ? "保存调整" : "保存为设计稿"}
              </Button>
            ) : null}
            <DropdownMenu>
              <DropdownMenuTrigger
                render={<Button size="icon-sm" variant="ghost" aria-label="更多操作"><MoreHorizontal className="h-4 w-4" /></Button>}
              />
              <DropdownMenuContent align="end">
                <DropdownMenuItem onClick={() => void refresh()}>刷新</DropdownMenuItem>
                <DropdownMenuItem disabled={!previewUrl} onClick={() => window.open(previewUrl, "_blank", "noopener,noreferrer")}>在新标签页中打开原型</DropdownMenuItem>
                <DropdownMenuItem disabled={!revision || downloadArchive.isPending} onClick={() => downloadArchive.mutate()}>
                  {revision ? `下载 v${revision.revision_number} 原型包 (.zip)` : "下载原型包 (.zip)"}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => navigation.push(paths.designs())}>返回设计库</DropdownMenuItem>
                {canDiscard ? (
                  <>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem variant="destructive" onClick={() => setDiscardOpen(true)}>放弃草稿</DropdownMenuItem>
                  </>
                ) : null}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        )}
      />

      <div className="grid min-h-0 flex-1 gap-4 p-4 lg:grid-cols-[340px_minmax(0,1fr)]">
        <aside className="flex min-h-0 flex-col gap-3 overflow-hidden lg:h-full">
          <div className="min-h-0 flex-1 overflow-y-auto pr-1">
            <div className="rounded-lg border bg-card p-3">
              <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-caption text-muted-foreground">
                {project ? <span>{project.title}</span> : null}
                {platformLabel(document.platform) ? <span>{platformLabel(document.platform)}</span> : null}
                <span>{document.repository_grounded ? "已按仓库取证" : "未做仓库取证"}</span>
              </div>
              {briefOf(document) ? (
                <details className="mt-2 group">
                  <summary className="cursor-pointer list-none text-caption font-medium text-foreground">需求描述</summary>
                  <p className="mt-1.5 whitespace-pre-wrap text-caption leading-5 text-muted-foreground">{briefOf(document)}</p>
                </details>
              ) : null}
            </div>

            {activeTask && running ? (
              <div className="mt-3 rounded-lg border bg-card px-3">
                <DesignTaskActivity task={activeTask} agents={agents} compact onStopped={refresh} />
              </div>
            ) : null}
            {errorMessage ? (
              <div role="alert" className="mt-3 flex items-start gap-2 rounded-lg border border-destructive/40 bg-destructive/5 px-3 py-2 text-caption leading-5">
                <CircleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0 text-destructive" />
                <div className="min-w-0">
                  <div className="font-medium text-destructive">{activeTask ? `${taskOperationLabel(activeTask.operation)}失败` : "运行失败"}</div>
                  <div className="text-muted-foreground">{errorMessage}</div>
                  {revisions.length > 0 ? <div className="mt-1 text-muted-foreground">上一版仍然可用，可以在此基础上继续调整。</div> : null}
                </div>
              </div>
            ) : null}

            {critique ? <div className="mt-3"><DesignDocumentCritique critique={critique} /></div> : null}

            <section className="mt-3" aria-label="版本">
              <div className="mb-2 flex items-center justify-between px-0.5">
                <h2 className="text-caption font-medium text-muted-foreground">版本</h2>
                <span className="text-caption text-muted-foreground">{revisions.length}</span>
              </div>
              {revisions.length === 0 ? (
                <p className="rounded-lg border border-dashed px-3 py-4 text-center text-caption text-muted-foreground">
                  {running ? "第一版正在生成。" : "还没有生成任何版本。"}
                </p>
              ) : (
                <ol className="space-y-2">
                  {revisions.map((row) => (
                    <RevisionRow
                      key={row.id}
                      revision={row}
                      selected={row.id === selectedRevisionId}
                      entries={entries}
                      agents={agents}
                      busy={busy || running}
                      onSelect={() => setPinnedRevisionId(row.id === currentRevisionId ? "" : row.id)}
                      onRestore={() => restore.mutate(row.id)}
                    />
                  ))}
                </ol>
              )}
            </section>
          </div>

          <form
            className="shrink-0 rounded-lg border bg-card p-3"
            onSubmit={(event) => {
              event.preventDefault();
              if (!instructionBlocker && !busy) adjust.mutate();
            }}
            aria-label="调整设计稿"
          >
            <div className="flex flex-wrap items-center gap-1.5">
              <button
                type="button"
                className={cn(
                  "inline-flex h-6 items-center gap-1 rounded-full border px-2 text-caption transition-colors",
                  scopeToPage && shownPage ? "border-primary/50 bg-accent text-foreground" : "text-muted-foreground hover:text-foreground",
                )}
                aria-pressed={scopeToPage && !!shownPage}
                disabled={!shownPage}
                onClick={() => setScopeToPage((value) => !value)}
                title={shownPage ? "只调整当前页面" : "先选择一个页面"}
              >
                {scopeToPage && shownPage ? `仅当前页面 · ${shownPage.title}` : "整份文档"}
                {scopeToPage && shownPage ? <X className="h-3 w-3" /> : null}
              </button>
              <div className="ml-auto">
                <AgentSetting agents={agents} agentId={agentId} onChange={setAgentOverride} />
              </div>
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-1.5">
              {ADJUSTMENT_PRESETS.map((preset) => (
                <button
                  key={preset.id}
                  type="button"
                  className="inline-flex h-6 items-center rounded-full border bg-card px-2 text-caption text-muted-foreground transition-colors hover:text-foreground disabled:opacity-50"
                  disabled={!canAdjust || busy}
                  onClick={() => setInstruction(preset.instruction)}
                >
                  {preset.label}
                </button>
              ))}
            </div>
            <Textarea
              value={instruction}
              onChange={(event) => setInstruction(event.target.value)}
              placeholder={canAdjust ? "描述你想怎么改，例如：把顶部导航收紧，订单列表增加筛选。" : "生成完成后可以在这里继续调整。"}
              rows={3}
              maxLength={INSTRUCTION_MAX_LENGTH}
              disabled={!canAdjust || busy}
              className="mt-2 min-h-[72px] resize-none text-body"
              onKeyDown={(event) => {
                if ((event.metaKey || event.ctrlKey) && event.key === "Enter" && !instructionBlocker && !busy) {
                  event.preventDefault();
                  adjust.mutate();
                }
              }}
            />
            <div className="mt-2 flex items-center justify-between gap-2">
              <span className="min-w-0 truncate text-caption text-muted-foreground">{instructionBlocker ?? "⌘/Ctrl + Enter 发送"}</span>
              <Button type="submit" size="sm" disabled={!!instructionBlocker || busy} aria-label="发起调整">
                {adjust.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <ArrowUp className="h-3.5 w-3.5" />}
                调整
              </Button>
            </div>
          </form>
        </aside>

        <main className="flex min-h-0 flex-col lg:h-full">
          {previewFrame}
        </main>
      </div>

      <AlertDialog open={discardOpen} onOpenChange={setDiscardOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>放弃当前草稿？</AlertDialogTitle>
            <AlertDialogDescription>
              {document.saved_revision_id
                ? "草稿会被丢弃，设计稿回到最近一次保存的版本。已保存的内容不受影响。"
                : "这份设计稿还没有保存过任何版本，放弃后将没有可预览的内容。"}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={discard.isPending}>取消</AlertDialogCancel>
            <AlertDialogAction disabled={discard.isPending} onClick={(event) => { event.preventDefault(); discard.mutate(); }}>
              {discard.isPending ? "正在放弃…" : "放弃草稿"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
