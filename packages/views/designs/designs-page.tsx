"use client";

import { useEffect, useMemo, useState } from "react";
import { ClipboardList, Copy, Eye, FileJson, Folder, Palette, Plus, Search, Sparkles, Trash2, X } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { designKeys } from "@multica/core/designs/keys";
import { designDraftListOptions, designFileListOptions, designFolderListOptions, designRestoreTaskListOptions, designTemplateListOptions } from "@multica/core/designs/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { agentListOptions } from "@multica/core/workspace/queries";
import type { DesignCatalogTemplate, DesignDraft, DesignFile, DesignFolder, DesignRestoreTask, GalleryJsonPatchOperation, Project } from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@multica/ui/components/ui/dialog";
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
import { PageHeader } from "../layout/page-header";
import { AppLink, useNavigation } from "../navigation";

type ToolMenuState = { x: number; y: number; file: DesignFile } | null;
type DraftDialogState = { template: DesignCatalogTemplate; title: string; requirement: string; slotValues: string; patch: string; agentId: string; prompt: string } | null;

function formatDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(date);
}

function sourceLabel(sourceType: string) {
  if (sourceType === "ai_generated") return "AI 生成";
  if (sourceType === "template") return "模板";
  if (sourceType === "import") return "导入";
  return "上传";
}

function sourceText(file: DesignFile, key: string, fallback: string) {
  const value = file.source_ref?.[key];
  return typeof value === "string" && value.trim() ? value : fallback;
}

function projectName(file: DesignFile, projectById: Map<string, Project>) {
  if (file.project_id) return projectById.get(file.project_id)?.title ?? "未知项目";
  return sourceText(file, "project", "未分配");
}

function folderName(file: DesignFile, folderById: Map<string, DesignFolder>) {
  if (file.folder_id) return folderById.get(file.folder_id)?.name ?? "未知文件夹";
  return sourceText(file, "group", sourceText(file, "folder", "无文件夹"));
}

function DesignToolMenu({ state, onClose, onView, onCopyImage, onDelete, deleting }: { state: ToolMenuState; onClose: () => void; onView: (file: DesignFile) => void; onCopyImage: (file: DesignFile) => void; onDelete: (file: DesignFile) => void; deleting: boolean }) {
  if (!state) return null;
  return (
    <div className="fixed inset-0 z-50" onClick={onClose} onContextMenu={(event) => { event.preventDefault(); onClose(); }}>
      <div className="absolute min-w-40 overflow-hidden rounded-xl border bg-popover p-1 text-popover-foreground shadow-xl" style={{ left: state.x, top: state.y }} onClick={(event) => event.stopPropagation()}>
        <button type="button" className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm hover:bg-accent" onClick={() => onView(state.file)}><Eye className="h-4 w-4" />查看详情</button>
        <button type="button" className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm hover:bg-accent" onClick={() => onCopyImage(state.file)}><Copy className="h-4 w-4" />复制图片</button>
        <button type="button" className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm text-destructive hover:bg-destructive/10" disabled={deleting} onClick={() => onDelete(state.file)}><Trash2 className="h-4 w-4" />{deleting ? "删除中…" : "删除"}</button>
      </div>
    </div>
  );
}

function DesignFileCard({ file, projectName, folderName, onContextMenu }: { file: DesignFile; projectName: string; folderName: string; onContextMenu: (event: React.MouseEvent, file: DesignFile) => void }) {
  const paths = useWorkspacePaths();
  return (
    <AppLink
      href={paths.designDetail(file.id)}
      onContextMenu={(event) => onContextMenu(event, file)}
      className="group/card flex min-w-0 flex-col overflow-hidden rounded-lg border bg-card transition-colors hover:border-primary/50"
    >
      <div className="relative aspect-[4/3] overflow-hidden bg-muted/50">
        {file.thumbnail_url ? (
          <img src={file.thumbnail_url} alt="" className="h-full w-full object-contain p-3 transition-transform group-hover/card:scale-[1.02]" loading="lazy" />
        ) : (
          <div className="absolute inset-4 rounded-lg border bg-background shadow-sm transition-transform group-hover/card:scale-[1.02]">
            <div className="h-8 border-b bg-muted/40" />
            <div className="grid grid-cols-3 gap-2 p-3">
              <span className="h-16 rounded-md bg-primary/10" />
              <span className="h-16 rounded-md bg-primary/5" />
              <span className="h-16 rounded-md bg-primary/10" />
            </div>
            <div className="space-y-2 px-3">
              <span className="block h-2 w-3/4 rounded bg-muted" />
              <span className="block h-2 w-1/2 rounded bg-muted" />
            </div>
          </div>
        )}
        <Badge variant="secondary" className="absolute left-3 top-3 bg-background/90">{sourceLabel(file.source_type)}</Badge>
      </div>
      <div className="min-w-0 p-3">
        <div className="truncate text-sm font-medium">{file.title}</div>
        <div className="mt-1 truncate text-xs text-muted-foreground">{file.description ?? "暂无描述"}</div>
        <div className="mt-3 flex items-center justify-between gap-2 text-xs text-muted-foreground">
          <span className="truncate">{projectName} · {folderName}</span>
          <span className="shrink-0">{formatDate(file.updated_at)}</span>
        </div>
      </div>
    </AppLink>
  );
}

function parseJSONObject(value: string, label: string): Record<string, unknown> {
  try {
    const parsed = JSON.parse(value.trim() || "{}");
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) throw new Error("Expected object");
    return parsed as Record<string, unknown>;
  } catch {
    throw new Error(`${label} 必须是 JSON 对象`);
  }
}

function parseJSONPatch(value: string): GalleryJsonPatchOperation[] {
  try {
    const parsed = JSON.parse(value.trim() || "[]");
    if (!Array.isArray(parsed)) throw new Error("Expected array");
    return parsed as GalleryJsonPatchOperation[];
  } catch {
    throw new Error("补丁必须是 JSON 数组");
  }
}

function defaultSlotValues(schema: Record<string, unknown> | undefined): Record<string, unknown> {
  if (!schema) return {};
  const entries = Array.isArray(schema.slots)
    ? schema.slots.flatMap((slot) => slot && typeof slot === "object" ? [[String((slot as Record<string, unknown>).key ?? (slot as Record<string, unknown>).slotKey ?? ""), slot as Record<string, unknown>]] as const : [])
    : Object.entries(schema).map(([key, value]) => [key, value && typeof value === "object" ? value as Record<string, unknown> : {}] as const);
  const out: Record<string, unknown> = {};
  for (const [key, slot] of entries) {
    if (!key) continue;
    if ("default" in slot) out[key] = slot.default;
    else if ("default_value" in slot) out[key] = slot.default_value;
    else if (slot.required) out[key] = defaultValueForSlotType(String(slot.type ?? "text"));
  }
  return out;
}

function defaultValueForSlotType(type: string): unknown {
  if (type === "number") return 0;
  if (type === "boolean" || type === "bool") return false;
  if (type === "list" || type === "array") return [];
  if (type === "object") return {};
  return "";
}

function TemplateCatalogCard({ template, onCreateDraft }: { template: DesignCatalogTemplate; onCreateDraft: (template: DesignCatalogTemplate) => void }) {
  const paths = useWorkspacePaths();
  return (
    <div className="flex min-w-[220px] flex-col gap-3 rounded-lg border bg-card p-3 transition-colors hover:border-primary/50">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate text-sm font-medium">{template.name}</div>
          <div className="mt-1 truncate text-xs text-muted-foreground">{template.description ?? template.design_file_title ?? "已发布设计模板"}</div>
        </div>
        <Badge variant="secondary" className="shrink-0">v{template.template_revision_number ?? 1}</Badge>
      </div>
      <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
        <span className="truncate">{template.category}</span>
        <span className="shrink-0">{formatDate(template.updated_at)}</span>
      </div>
      <div className="flex items-center gap-2">
        <Button type="button" size="sm" className="h-7" onClick={() => onCreateDraft(template)}>创建设计草稿</Button>
        {template.design_file_id ? <AppLink href={paths.designDetail(template.design_file_id)} className="text-xs text-muted-foreground hover:text-foreground">打开来源</AppLink> : null}
      </div>
    </div>
  );
}

function DraftReviewCard({ draft, onMaterialize, materializing }: { draft: DesignDraft; onMaterialize: (draft: DesignDraft) => void; materializing: boolean }) {
  const paths = useWorkspacePaths();
  return (
    <div className="rounded-lg border bg-card p-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <AppLink href={paths.designDraftDetail(draft.id)} className="block truncate text-sm font-medium hover:text-primary">{draft.title}</AppLink>
          <div className="mt-1 truncate text-xs text-muted-foreground">{draft.catalog_template_id ? `模板 ${draft.catalog_template_id.slice(0, 8)}` : "设计草稿"}</div>
        </div>
        <Badge variant="outline" className="shrink-0">{draft.status}</Badge>
      </div>
      <div className="mt-3 flex items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>{draft.patch.length} 个 patch 操作</span>
        <span>{formatDate(draft.materialized_at ?? draft.updated_at)}</span>
      </div>
      {draft.generated_file_id ? (
        <AppLink href={paths.designDetail(draft.generated_file_id)} className="mt-3 flex h-7 w-full items-center justify-center rounded-md border px-3 text-xs font-medium hover:bg-accent">打开生成的设计稿</AppLink>
      ) : (
        <Button type="button" size="sm" variant="outline" className="mt-3 h-7 w-full" disabled={materializing} onClick={() => onMaterialize(draft)}>{materializing ? "生成中…" : "生成设计稿"}</Button>
      )}
    </div>
  );
}

function RestoreTaskCard({ task }: { task: DesignRestoreTask }) {
  const paths = useWorkspacePaths();
  const itemCount = Array.isArray(task.input?.items) ? task.input.items.length : 0;
  return (
    <AppLink href={paths.designRestoreTaskDetail(task.id)} className="block rounded-lg border bg-card p-3 transition-colors hover:border-primary/50">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate text-sm font-medium">还原任务 {task.id.slice(0, 8)}</div>
          <div className="mt-1 truncate text-xs text-muted-foreground">设计稿 {task.file_id.slice(0, 8)} · {itemCount} 个任务项</div>
        </div>
        <Badge variant="outline" className="shrink-0">{task.status}</Badge>
      </div>
      <div className="mt-3 flex items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>{task.error ? "有错误" : "等待消费"}</span>
        <span>{formatDate(task.updated_at)}</span>
      </div>
    </AppLink>
  );
}

export function DesignsPage() {
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const queryClient = useQueryClient();
  const { data: files = [], isLoading, error, refetch } = useQuery(designFileListOptions(wsId));
  const { data: folders = [] } = useQuery(designFolderListOptions(wsId));
  const { data: templates = [], isLoading: templatesLoading } = useQuery(designTemplateListOptions(wsId));
  const { data: drafts = [], isLoading: draftsLoading } = useQuery(designDraftListOptions(wsId));
  const { data: restoreTasks = [], isLoading: restoreTasksLoading } = useQuery(designRestoreTaskListOptions(wsId));
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: projectData } = useQuery({ queryKey: ["projects", wsId, "designs"], queryFn: () => api.listProjects() });
  const projects = projectData?.projects ?? [];
  const [search, setSearch] = useState("");
  const [figmaCode, setFigmaCode] = useState<{ code: string; expiresAt: string } | null>(null);
  const [toolMenu, setToolMenu] = useState<ToolMenuState>(null);
  const [deleteTarget, setDeleteTarget] = useState<DesignFile | null>(null);
  const [createFolderOpen, setCreateFolderOpen] = useState(false);
  const [newFolderName, setNewFolderName] = useState("");
  const [deleteFolderTarget, setDeleteFolderTarget] = useState<{ folder: DesignFolder; count: number } | null>(null);
  const [templateDrawerOpen, setTemplateDrawerOpen] = useState(false);
  const [draftDialog, setDraftDialog] = useState<DraftDialogState>(null);
  const [materializingDraftId, setMaterializingDraftId] = useState<string | null>(null);
  const [selectedProjectId, setSelectedProjectId] = useState("");
  const figmaConnection = useMutation({
    mutationFn: () => api.createFigmaImportConnection(),
    onSuccess: (data) => setFigmaCode({ code: data.code, expiresAt: data.expires_at }),
  });

  const projectById = useMemo(() => new Map(projects.map((project) => [project.id, project])), [projects]);
  const folderById = useMemo(() => new Map(folders.map((folder) => [folder.id, folder])), [folders]);
  useEffect(() => {
    if (!selectedProjectId && projects[0]?.id) setSelectedProjectId(projects[0].id);
    else if (selectedProjectId && projects.length && !projects.some((project) => project.id === selectedProjectId)) setSelectedProjectId(projects[0]?.id ?? "");
  }, [projects, selectedProjectId]);
  const availableAgents = useMemo(() => agents.filter((agent) => !agent.archived_at && agent.runtime_id), [agents]);
  const defaultAgentId = availableAgents[0]?.id ?? "";
  const deleteDesign = useMutation({
    mutationFn: (fileId: string) => api.deleteDesignFile(fileId),
    onSuccess: async () => {
      setToolMenu(null);
      setDeleteTarget(null);
      await queryClient.invalidateQueries({ queryKey: designKeys.files(wsId) });
      toast.success("已删除画板及历史版本");
    },
  });

  const createFolder = useMutation({
    mutationFn: () => api.createDesignFolder({ project_id: selectedProjectId, name: newFolderName.trim() }),
    onSuccess: async () => {
      setCreateFolderOpen(false);
      setNewFolderName("");
      await queryClient.invalidateQueries({ queryKey: designKeys.folders(wsId) });
      toast.success("已新增分组");
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "新增分组失败"),
  });

  const deleteFolder = useMutation({
    mutationFn: (folderId: string) => api.deleteDesignFolder(folderId),
    onSuccess: async () => {
      setDeleteFolderTarget(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: designKeys.folders(wsId) }),
        queryClient.invalidateQueries({ queryKey: designKeys.files(wsId) }),
      ]);
      toast.success("已删除分组");
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "删除分组失败"),
  });

  const createDraft = useMutation({
    mutationFn: () => {
      if (!draftDialog) throw new Error("未选择模板");
      return api.createDesignDraft({
        catalog_template_id: draftDialog.template.id,
        title: draftDialog.title,
        requirement_core: parseJSONObject(draftDialog.requirement, "需求"),
        slot_values: parseJSONObject(draftDialog.slotValues, "槽位值"),
        patch: parseJSONPatch(draftDialog.patch),
      });
    },
    onSuccess: async (draft) => {
      setDraftDialog(null);
      await queryClient.invalidateQueries({ queryKey: designKeys.drafts(wsId) });
      toast.success(`已创建草稿 ${draft.title}`);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : "创建草稿失败");
    },
  });

  const createAgentDraftTask = useMutation({
    mutationFn: () => {
      if (!draftDialog) throw new Error("未选择模板");
      if (!draftDialog.agentId.trim()) throw new Error("智能体 ID 为必填项");
      return api.createDesignDraftAgentTask({
        agent_id: draftDialog.agentId.trim(),
        catalog_template_id: draftDialog.template.id,
        title: draftDialog.title,
        prompt: draftDialog.prompt,
        requirement_core: parseJSONObject(draftDialog.requirement, "需求"),
      });
    },
    onSuccess: (task) => {
      setDraftDialog(null);
      toast.success(`已提交 UI 智能体任务 ${task.task_id.slice(0, 8)}`);
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "提交 UI 智能体失败"),
  });

  const materializeDraft = useMutation({
    mutationFn: (draft: DesignDraft) => {
      setMaterializingDraftId(draft.id);
      return api.materializeDesignDraft(draft.id);
    },
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({ queryKey: designKeys.files(wsId) });
      await queryClient.invalidateQueries({ queryKey: designKeys.drafts(wsId) });
      toast.success(`已生成设计 ${result.design_file.file.title}`);
      navigation.push(paths.designDetail(result.design_file.file.id));
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : "生成设计失败");
    },
    onSettled: () => setMaterializingDraftId(null),
  });

  const openDraftDialog = (template: DesignCatalogTemplate) => {
    setDraftDialog({
      template,
      title: `${template.name} 草稿`,
      requirement: JSON.stringify({ version: "1.0", title: template.name }, null, 2),
      slotValues: JSON.stringify(defaultSlotValues(template.slot_schema), null, 2),
      patch: "[]",
      agentId: defaultAgentId,
      prompt: "先生成 slot_values；仅在需要时再生成安全 JSON patch。不要更改布局或树结构。",
    });
  };

  const openToolMenu = (event: React.MouseEvent, file: DesignFile) => {
    event.preventDefault();
    event.stopPropagation();
    setToolMenu({ x: event.clientX, y: event.clientY, file });
  };

  const copyImage = (file: DesignFile) => {
    if (!file.thumbnail_url) {
      toast.error("当前画板没有可复制的图片链接");
      return;
    }
    void navigator.clipboard?.writeText(file.thumbnail_url).then(() => toast.success("已复制图片链接"));
    setToolMenu(null);
  };

  const projectFiles = useMemo(() => files.filter((file) => file.project_id === selectedProjectId), [files, selectedProjectId]);
  const projectFolders = useMemo(() => folders.filter((folder) => folder.project_id === selectedProjectId), [folders, selectedProjectId]);
  const projectTemplates = useMemo(() => templates.filter((template) => template.metadata?.project_id === selectedProjectId), [templates, selectedProjectId]);
  const projectRestoreTasks = useMemo(() => restoreTasks.filter((task) => !selectedProjectId || task.input?.projectId === selectedProjectId), [restoreTasks, selectedProjectId]);
  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    if (!query) return projectFiles;
    return projectFiles.filter((file) => {
      const haystack = [
        file.title,
        file.description ?? "",
        file.source_type,
        projectName(file, projectById),
        folderName(file, folderById),
      ].join(" ").toLowerCase();
      return haystack.includes(query);
    });
  }, [folderById, projectById, projectFiles, search]);

  const grouped = useMemo(() => {
    const folderMap = new Map<string, { folderKey: string; folderName: string; items: DesignFile[] }>();
    folderMap.set("__ungrouped", { folderKey: "__ungrouped", folderName: "未分组", items: [] });
    for (const folder of projectFolders) {
      folderMap.set(folder.id, { folderKey: folder.id, folderName: folder.name, items: [] });
    }
    for (const file of filtered) {
      const fName = folderName(file, folderById);
      const fKey = file.folder_id ?? "__ungrouped";
      const folderGroup = folderMap.get(fKey) ?? { folderKey: fKey, folderName: fName, items: [] };
      folderGroup.items.push(file);
    }
    return Array.from(folderMap.values()).filter((folder) => folder.items.length > 0 || search.trim() === "");
  }, [filtered, folderById, projectFolders, search]);
  const selectedProject = projectById.get(selectedProjectId);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <PageHeader className="justify-between px-5">
        <div className="flex items-center gap-2">
          <Palette className="h-4 w-4 text-muted-foreground" />
          <h1 className="text-sm font-medium">设计库</h1>
          {!isLoading && projectFiles.length > 0 ? <span className="font-mono text-xs text-muted-foreground/70">{projectFiles.length}</span> : null}
        </div>
        <Button size="sm" variant="outline" onClick={() => figmaConnection.mutate()} disabled={figmaConnection.isPending}>
          <FileJson className="h-3.5 w-3.5" />
          {figmaConnection.isPending ? "正在创建代码…" : "连接 Figma"}
        </Button>
      </PageHeader>

      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        {figmaCode ? (
          <div className="border-b bg-muted/30 px-4 py-3 text-sm">
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <div className="font-medium">Figma 连接代码</div>
                <div className="text-xs text-muted-foreground">将此一次性代码粘贴到 Figma 插件中。过期时间：{figmaCode.expiresAt}。</div>
              </div>
              <code className="select-all rounded-md border bg-background px-3 py-1.5 font-mono text-xs">{figmaCode.code}</code>
            </div>
          </div>
        ) : null}
        {figmaConnection.error ? (
          <div className="border-b px-4 py-2 text-xs text-destructive">无法创建 Figma 连接代码。</div>
        ) : null}
        <div className="flex shrink-0 flex-col gap-3 border-b px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 items-center gap-2">
            <span className="shrink-0 text-xs text-muted-foreground">项目</span>
            <select value={selectedProjectId} onChange={(event) => setSelectedProjectId(event.target.value)} className="h-8 max-w-72 rounded-md border bg-background px-3 text-sm">
              {projects.map((project) => <option key={project.id} value={project.id}>{project.title}</option>)}
            </select>
          </div>
          <div className="relative w-full sm:w-72">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索设计稿…" className="h-8 pl-8 text-sm" />
          </div>
          <span className="hidden font-mono text-xs text-muted-foreground/70 sm:block">{filtered.length} / {projectFiles.length}</span>
        </div>

        {isLoading ? (
          <div className="space-y-2 p-4">
            {Array.from({ length: 5 }).map((_, index) => <Skeleton key={index} className="h-14 w-full" />)}
          </div>
        ) : error ? (
          <div className="flex flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
            <p className="text-sm font-medium">无法加载设计库</p>
            <p className="text-sm text-muted-foreground">请检查后端路由后重试。</p>
            <Button size="sm" variant="outline" onClick={() => void refetch()}>重试</Button>
          </div>
        ) : !selectedProjectId ? (
          <div className="flex flex-1 flex-col items-center justify-center px-6 text-center">
            <h2 className="text-base font-semibold">请选择项目</h2>
            <p className="mt-1 max-w-md text-sm text-muted-foreground">设计稿和模板资产都按项目归类展示。</p>
          </div>
        ) : projectFiles.length === 0 && projectFolders.length === 0 ? (
          <div className="flex flex-1 flex-col items-center justify-center px-6 text-center">
            <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
              <Palette className="h-6 w-6 text-muted-foreground" />
            </div>
            <h2 className="mt-4 text-base font-semibold">当前项目暂无设计资产</h2>
            <p className="mt-1 max-w-md text-sm text-muted-foreground">请从 Figma 插件上传业务设计稿，模板、草稿和还原任务可从右侧抽屉查看。</p>
            <div className="mt-4 flex gap-2">
              <Button size="sm" variant="outline" onClick={() => setTemplateDrawerOpen(true)}><Sparkles className="h-3.5 w-3.5" />工作台</Button>
              <Button size="sm" onClick={() => setCreateFolderOpen(true)} disabled={!selectedProjectId}><Plus className="h-3.5 w-3.5" />新增分组</Button>
            </div>
          </div>
        ) : filtered.length === 0 && search.trim() ? (
          <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">没有匹配“{search}”的设计稿。</div>
        ) : (
          <div className="min-h-0 flex-1 overflow-auto p-4">
            <div className="mb-4 rounded-lg border bg-background p-3">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <Folder className="h-4 w-4 text-muted-foreground" />
                    {selectedProject?.title ?? "项目"} 设计库
                  </div>
                  <p className="mt-1 text-xs text-muted-foreground">当前项目下的分组和设计稿项目。模板、草稿和还原任务从右侧抽屉查看。</p>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <Button size="sm" variant="outline" onClick={() => setTemplateDrawerOpen(true)}><Sparkles className="h-3.5 w-3.5" />工作台 <span className="font-mono text-xs">{projectTemplates.length + drafts.length + projectRestoreTasks.length}</span></Button>
                  <Button size="sm" onClick={() => setCreateFolderOpen(true)} disabled={!selectedProjectId}><Plus className="h-3.5 w-3.5" />新增分组</Button>
                </div>
              </div>
            </div>
            <div className="space-y-6">
              {grouped.map((folder) => (
                <section key={folder.folderKey} className="space-y-3">
                  <div className="mb-3 flex items-center justify-between gap-3 border-b pb-2 text-xs text-muted-foreground">
                    <div className="flex min-w-0 items-center gap-2">
                      <Folder className="h-3.5 w-3.5" />
                      <span className="truncate font-medium text-foreground/80">{folder.folderName}</span>
                      <span className="font-mono text-muted-foreground/80">{folder.items.length}</span>
                    </div>
                    {folder.folderKey !== "__ungrouped" ? (
                      <Button size="sm" variant="ghost" className="h-7 px-2 text-xs text-destructive hover:text-destructive" onClick={() => {
                        const targetFolder = folderById.get(folder.folderKey);
                        if (targetFolder) setDeleteFolderTarget({ folder: targetFolder, count: folder.items.length });
                      }}>
                        删除分组
                      </Button>
                    ) : null}
                  </div>
                  {folder.items.length ? (
                      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                        {folder.items.map((file) => <DesignFileCard key={file.id} file={file} projectName={selectedProject?.title ?? "项目"} folderName={folder.folderName} onContextMenu={openToolMenu} />)}
                      </div>
                  ) : <p className="text-xs text-muted-foreground">此文件夹暂无设计稿项目。</p>}
                </section>
              ))}
            </div>
          </div>
        )}
      </div>
      <DesignToolMenu
        state={toolMenu}
        deleting={deleteDesign.isPending}
        onClose={() => setToolMenu(null)}
        onView={(file) => { setToolMenu(null); navigation.push(paths.designDetail(file.id)); }}
        onCopyImage={copyImage}
        onDelete={(file) => { setToolMenu(null); setDeleteTarget(file); }}
      />
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除这个画板？</AlertDialogTitle>
            <AlertDialogDescription>“{deleteTarget?.title ?? "当前画板"}” 及其所有历史版本都会被删除，该操作不可撤销。</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteDesign.isPending}>取消</AlertDialogCancel>
            <AlertDialogAction variant="destructive" disabled={!deleteTarget || deleteDesign.isPending} onClick={() => deleteTarget && deleteDesign.mutate(deleteTarget.id)}>{deleteDesign.isPending ? "删除中…" : "删除"}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
      {templateDrawerOpen ? (
        <div className="fixed inset-0 z-50 flex justify-end bg-background/60 backdrop-blur-sm" onClick={() => setTemplateDrawerOpen(false)}>
          <aside className="flex h-full w-full max-w-xl flex-col border-l bg-background shadow-2xl" onClick={(event) => event.stopPropagation()}>
            <div className="flex items-start justify-between gap-3 border-b p-4">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold"><Sparkles className="h-4 w-4 text-muted-foreground" />设计工作台</div>
                <p className="mt-1 text-xs text-muted-foreground">当前项目：{selectedProject?.title ?? "未选择项目"}。集中查看模板、草稿和还原任务。</p>
              </div>
              <Button size="sm" variant="ghost" onClick={() => setTemplateDrawerOpen(false)}><X className="h-4 w-4" /></Button>
            </div>
            <div className="min-h-0 flex-1 overflow-auto p-4">
              <div className="mb-4 rounded-lg border bg-muted/20 p-3">
                <div className="mb-2 flex items-center justify-between text-sm font-medium">
                  <span>已发布模板</span>
                  <span className="font-mono text-xs text-muted-foreground">{projectTemplates.length}</span>
                </div>
                {templatesLoading ? (
                  <div className="grid gap-3">{Array.from({ length: 3 }).map((_, index) => <Skeleton key={index} className="h-24 w-full" />)}</div>
                ) : projectTemplates.length === 0 ? (
                  <p className="text-xs text-muted-foreground">当前项目暂无模板资产。请从 Figma 插件选择“模板资产”上传。</p>
                ) : (
                  <div className="grid gap-3">{projectTemplates.map((template) => <TemplateCatalogCard key={template.id} template={template} onCreateDraft={openDraftDialog} />)}</div>
                )}
              </div>
              <div className="rounded-lg border bg-muted/20 p-3">
                <div className="mb-2 flex items-center justify-between text-sm font-medium">
                  <span>待审核草稿</span>
                  <span className="font-mono text-xs text-muted-foreground">{drafts.length}</span>
                </div>
                {draftsLoading ? (
                  <div className="grid gap-3">{Array.from({ length: 3 }).map((_, index) => <Skeleton key={index} className="h-24 w-full" />)}</div>
                ) : drafts.length === 0 ? (
                  <p className="text-xs text-muted-foreground">暂无生成的草稿。可从模板卡片创建或询问 UI 智能体。</p>
                ) : (
                  <div className="grid gap-3">{drafts.map((draft) => <DraftReviewCard key={draft.id} draft={draft} materializing={materializingDraftId === draft.id} onMaterialize={(item) => materializeDraft.mutate(item)} />)}</div>
                )}
              </div>
              <div className="mt-4 rounded-lg border bg-muted/20 p-3">
                <div className="mb-2 flex items-center justify-between text-sm font-medium">
                  <span className="flex items-center gap-2"><ClipboardList className="h-4 w-4 text-muted-foreground" />还原任务</span>
                  <span className="font-mono text-xs text-muted-foreground">{projectRestoreTasks.length}</span>
                </div>
                {restoreTasksLoading ? (
                  <div className="grid gap-3">{Array.from({ length: 3 }).map((_, index) => <Skeleton key={index} className="h-24 w-full" />)}</div>
                ) : projectRestoreTasks.length === 0 ? (
                  <p className="text-xs text-muted-foreground">暂无还原任务。打开设计稿后点击“保存全量任务”即可创建。</p>
                ) : (
                  <div className="grid gap-3">{projectRestoreTasks.map((task) => <RestoreTaskCard key={task.id} task={task} />)}</div>
                )}
              </div>
            </div>
          </aside>
        </div>
      ) : null}
      <Dialog open={createFolderOpen} onOpenChange={(open) => { setCreateFolderOpen(open); if (!open) setNewFolderName(""); }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>新增分组</DialogTitle>
            <DialogDescription>分组会创建在当前项目下，用于归类设计稿项目。</DialogDescription>
          </DialogHeader>
          <Input value={newFolderName} onChange={(event) => setNewFolderName(event.target.value)} placeholder="请输入分组名" />
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setCreateFolderOpen(false)} disabled={createFolder.isPending}>取消</Button>
            <Button type="button" onClick={() => createFolder.mutate()} disabled={!selectedProjectId || !newFolderName.trim() || createFolder.isPending}>{createFolder.isPending ? "创建中…" : "创建"}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <AlertDialog open={!!deleteFolderTarget} onOpenChange={(open) => { if (!open) setDeleteFolderTarget(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除这个分组？</AlertDialogTitle>
            <AlertDialogDescription>
              {deleteFolderTarget?.count ? `“${deleteFolderTarget.folder.name}” 下有 ${deleteFolderTarget.count} 个设计稿项目。删除分组后会一并清除文件夹下所有设计稿及历史版本，该操作不可撤销。` : `“${deleteFolderTarget?.folder.name ?? "当前分组"}” 会被删除，该操作不可撤销。`}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteFolder.isPending}>取消</AlertDialogCancel>
            <AlertDialogAction variant="destructive" disabled={!deleteFolderTarget || deleteFolder.isPending} onClick={() => deleteFolderTarget && deleteFolder.mutate(deleteFolderTarget.folder.id)}>{deleteFolder.isPending ? "删除中…" : "确认删除"}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
      <Dialog open={!!draftDialog} onOpenChange={(open) => { if (!open) setDraftDialog(null); }}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>从模板创建设计草稿</DialogTitle>
            <DialogDescription>根据槽位值和可选的安全 JSON patch 生成受控草稿。布局/树路径会被 API 拒绝。</DialogDescription>
          </DialogHeader>
          {draftDialog ? (
            <div className="grid gap-3">
              <div>
                <label className="mb-1 block text-xs font-medium text-muted-foreground">模板</label>
                <Input value={draftDialog.template.name} readOnly className="h-8" />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-muted-foreground">草稿标题</label>
                <Input value={draftDialog.title} onChange={(event) => setDraftDialog({ ...draftDialog, title: event.target.value })} className="h-8" />
              </div>
              <div className="grid gap-3 md:grid-cols-2">
                <div>
                  <label className="mb-1 block text-xs font-medium text-muted-foreground">UI 智能体</label>
                  {availableAgents.length ? (
                    <select value={draftDialog.agentId} onChange={(event) => setDraftDialog({ ...draftDialog, agentId: event.target.value })} className="h-8 w-full rounded-md border bg-background px-2 text-xs">
                      {availableAgents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name}</option>)}
                    </select>
                  ) : (
                    <Input value={draftDialog.agentId} onChange={(event) => setDraftDialog({ ...draftDialog, agentId: event.target.value })} placeholder="未找到在线智能体；请粘贴智能体 UUID" className="h-8" />
                  )}
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-muted-foreground">智能体提示词</label>
                  <Input value={draftDialog.prompt} onChange={(event) => setDraftDialog({ ...draftDialog, prompt: event.target.value })} className="h-8" />
                </div>
              </div>
              <div className="grid gap-3 md:grid-cols-2">
                <div>
                  <label className="mb-1 block text-xs font-medium text-muted-foreground">需求 JSON</label>
                  <Textarea value={draftDialog.requirement} onChange={(event) => setDraftDialog({ ...draftDialog, requirement: event.target.value })} className="min-h-40 font-mono text-xs" />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-muted-foreground">槽位值 JSON</label>
                  <Textarea value={draftDialog.slotValues} onChange={(event) => setDraftDialog({ ...draftDialog, slotValues: event.target.value })} className="min-h-40 font-mono text-xs" />
                </div>
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-muted-foreground">安全 patch JSON</label>
                <Textarea value={draftDialog.patch} onChange={(event) => setDraftDialog({ ...draftDialog, patch: event.target.value })} className="min-h-24 font-mono text-xs" />
              </div>
            </div>
          ) : null}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setDraftDialog(null)} disabled={createDraft.isPending || createAgentDraftTask.isPending}>取消</Button>
            <Button type="button" variant="outline" onClick={() => createAgentDraftTask.mutate()} disabled={!draftDialog || createAgentDraftTask.isPending}>{createAgentDraftTask.isPending ? "提交中…" : "询问 UI 智能体"}</Button>
            <Button type="button" onClick={() => createDraft.mutate()} disabled={!draftDialog || createDraft.isPending}>{createDraft.isPending ? "创建中…" : "创建设计草稿"}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
