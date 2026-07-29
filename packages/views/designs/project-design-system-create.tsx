"use client";

import { useMemo, useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Bot,
  CircleAlert,
  ExternalLink,
  FileImage,
  Link as LinkIcon,
  LoaderCircle,
  Palette,
  Sparkles,
  Upload,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { designKeys } from "@multica/core/designs/keys";
import { useFileUpload } from "@multica/core/hooks/use-file-upload";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import type {
  Agent,
  CreateProjectDesignSystemRequest,
  DesignFile,
  DesignSystemProfile,
  Project,
  ProjectDesignSystem,
  ProjectDesignSystemPlatform,
  ProjectDesignSystemReferenceInput,
} from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { AppLink } from "../navigation";

type UploadedReference = {
  attachmentId: string;
  label: string;
};

type ProjectDesignSystemForm = {
  agentId: string;
  platform: ProjectDesignSystemPlatform | "";
  brief: string;
  attachments: UploadedReference[];
  brandColor: string;
  link: string;
  designFileIds: string[];
  profileIds: string[];
};

const PLATFORM_OPTIONS: Array<{ value: ProjectDesignSystemPlatform; label: string }> = [
  { value: "web", label: "Web" },
  { value: "mobile", label: "移动端" },
  { value: "cross_platform", label: "跨端" },
];

function objectValue(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function platformValue(value: unknown): ProjectDesignSystemPlatform | "" {
  return value === "web" || value === "mobile" || value === "cross_platform" ? value : "";
}

function initialForm(project: Project, system: ProjectDesignSystem | undefined): ProjectDesignSystemForm {
  const snapshot = objectValue(system?.input_snapshot);
  const references = Array.isArray(snapshot?.references)
    ? snapshot.references.map(objectValue).filter((item): item is Record<string, unknown> => item !== null)
    : [];

  return {
    agentId: stringValue(snapshot?.agent_id),
    platform: platformValue(snapshot?.platform),
    brief: stringValue(snapshot?.brief) || project.description?.trim() || "",
    attachments: references
      .filter((reference) => reference.kind === "attachment" && stringValue(reference.attachment_id))
      .map((reference) => ({
        attachmentId: stringValue(reference.attachment_id),
        label: stringValue(reference.label) || stringValue(reference.filename) || "上传资料",
      })),
    brandColor: references.find((reference) => reference.kind === "brand_color")
      ? stringValue(references.find((reference) => reference.kind === "brand_color")?.value)
      : "",
    link: references.find((reference) => reference.kind === "link")
      ? stringValue(references.find((reference) => reference.kind === "link")?.value)
        || stringValue(references.find((reference) => reference.kind === "link")?.url)
      : "",
    designFileIds: references
      .filter((reference) => reference.kind === "design_file")
      .map((reference) => stringValue(reference.design_file_id))
      .filter(Boolean),
    profileIds: references
      .filter((reference) => reference.kind === "design_system_profile")
      .map((reference) => stringValue(reference.design_system_profile_id))
      .filter(Boolean),
  };
}

function isAgentAvailable(agent: Agent | undefined): boolean {
  return Boolean(agent && !agent.archived_at && agent.runtime_id);
}

function isHttpsLink(value: string): boolean {
  if (!value.trim()) return true;
  try {
    const parsed = new URL(value.trim());
    return parsed.protocol === "https:" && Boolean(parsed.hostname) && !parsed.username && !parsed.password;
  } catch {
    return false;
  }
}

function isHexColor(value: string): boolean {
  return /^#[0-9A-Fa-f]{6}$/.test(value.trim());
}

function errorMessage(value: unknown): string | null {
  if (typeof value === "string" && value.trim()) return value.trim();
  const record = objectValue(value);
  if (!record) return null;
  for (const key of ["message", "error", "reason", "code"]) {
    const text = stringValue(record[key]).trim();
    if (text) return text;
  }
  return null;
}

function generationStage(system: ProjectDesignSystem): string | null {
  const status = system.active_task?.status;
  if (status === "queued" || status === "dispatched") return "准备上下文";
  if (status === "running") return "智能体生成";
  if (status === "completed") return "产物校验";
  return null;
}

function platformLabel(platform: ProjectDesignSystem["platform"]): string {
  return PLATFORM_OPTIONS.find((item) => item.value === platform)?.label ?? "未指定";
}

function formatDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "尚未更新";
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function toggleId(values: string[], id: string): string[] {
  return values.includes(id) ? values.filter((value) => value !== id) : [...values, id];
}

function ReferenceCheckbox({
  checked,
  label,
  meta,
  onChange,
}: {
  checked: boolean;
  label: string;
  meta: string;
  onChange: () => void;
}) {
  return (
    <label className="flex min-w-0 cursor-pointer items-center gap-3 border-b py-2.5 last:border-b-0">
      <input type="checkbox" checked={checked} onChange={onChange} aria-label={label} className="h-4 w-4 shrink-0 accent-primary" />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm">{label}</span>
        <span className="block truncate text-xs text-muted-foreground">{meta}</span>
      </span>
    </label>
  );
}

export function ProjectDesignSystemCreate({
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
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [forms, setForms] = useState<Record<string, ProjectDesignSystemForm>>({});
  const [creationOpen, setCreationOpen] = useState<Record<string, boolean>>({});
  const [submitErrors, setSubmitErrors] = useState<Record<string, string | null>>({});
  const { upload, uploading } = useFileUpload(api, (error) => toast.error(error.message));

  const form = forms[project.id] ?? initialForm(project, system);
  const currentAgent = agents.find((agent) => agent.id === form.agentId);
  const agentAvailable = isAgentAvailable(currentAgent);
  const validLink = isHttpsLink(form.link);
  const validColor = !form.brandColor.trim() || isHexColor(form.brandColor);
  const canSubmit = Boolean(
    form.agentId
      && agentAvailable
      && form.platform
      && form.brief.trim()
      && validLink
      && validColor
      && !uploading,
  );
  const lastError = submitErrors[project.id] ?? errorMessage(system?.last_error);
  const showCreation = creationOpen[project.id] === true || Boolean(lastError);

  const agentOptions = useMemo(() => {
    const active = agents
      .filter((agent) => !agent.archived_at || agent.id === form.agentId)
      .map((agent) => ({
        id: agent.id,
        name: agent.name,
        status: agent.status,
        available: isAgentAvailable(agent),
      }));
    if (form.agentId && !active.some((agent) => agent.id === form.agentId)) {
      return [...active, {
        id: form.agentId,
        name: "之前选择的智能体",
        status: "offline",
        available: false,
      }];
    }
    return active;
  }, [agents, form.agentId]);

  const updateForm = (updater: (current: ProjectDesignSystemForm) => ProjectDesignSystemForm) => {
    const projectId = project.id;
    setForms((current) => ({
      ...current,
      [projectId]: updater(current[projectId] ?? initialForm(project, system)),
    }));
  };

  const createSystem = useMutation({
    mutationFn: (request: CreateProjectDesignSystemRequest) => api.createProjectDesignSystem(request),
    onMutate: (request) => {
      setSubmitErrors((current) => ({ ...current, [request.project_id]: null }));
    },
    onSuccess: (created) => {
      queryClient.setQueryData(
        designKeys.projectDesignSystemByProject(wsId, created.project_id),
        created,
      );
      if (created.id) {
        queryClient.setQueryData(designKeys.projectDesignSystem(wsId, created.id), created);
      }
    },
    onError: (error, request) => {
      const message = error instanceof Error ? error.message : "无法生成设计体系，请检查智能体状态后重试。";
      setSubmitErrors((current) => ({ ...current, [request.project_id]: message }));
      toast.error(message);
    },
  });
  const isSubmittingCurrentProject = createSystem.isPending
    && createSystem.variables?.project_id === project.id;

  const handleUpload = async (file: File) => {
    const projectId = project.id;
    const seed = initialForm(project, system);
    try {
      const result = await upload(file);
      if (!result) return;
      setForms((current) => {
        const currentForm = current[projectId] ?? seed;
        if (currentForm.attachments.some((item) => item.attachmentId === result.id)) return current;
        return {
          ...current,
          [projectId]: {
            ...currentForm,
            attachments: [...currentForm.attachments, { attachmentId: result.id, label: result.filename || file.name }],
          },
        };
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "上传参考资料失败");
    }
  };

  const references = (): ProjectDesignSystemReferenceInput[] => [
    ...form.attachments.map((item) => ({
      kind: "attachment" as const,
      attachment_id: item.attachmentId,
      label: item.label,
    })),
    ...(form.brandColor.trim() ? [{
      kind: "brand_color" as const,
      value: form.brandColor.trim().toUpperCase(),
      label: "品牌色",
    }] : []),
    ...(form.link.trim() ? [{
      kind: "link" as const,
      value: form.link.trim(),
      label: "参考链接",
    }] : []),
    ...form.designFileIds.flatMap((id) => {
      const file = designFiles.find((item) => item.id === id);
      return file ? [{ kind: "design_file" as const, design_file_id: id, label: file.title }] : [];
    }),
    ...form.profileIds.flatMap((id) => {
      const profile = legacyProfiles.find((item) => item.id === id);
      return profile ? [{ kind: "design_system_profile" as const, design_system_profile_id: id, label: profile.name }] : [];
    }),
  ];

  if (isLoading || !system) {
    return (
      <div className="mx-auto w-full max-w-5xl space-y-4 py-2">
        <Skeleton className="h-7 w-48" />
        <Skeleton className="h-20 w-full" />
        <Skeleton className="h-36 w-full" />
      </div>
    );
  }

  if (system.status === "generating" || system.active_task) {
    const stage = generationStage(system);
    const activeAgent = agents.find((agent) => agent.id === system.current_agent_id);
    return (
      <div className="mx-auto w-full max-w-5xl py-2">
        <div className="flex items-start gap-3 border-b pb-5">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
            <LoaderCircle className="h-4 w-4 animate-spin" />
          </span>
          <div className="min-w-0 flex-1">
            <h2 className="text-base font-semibold">正在生成设计体系</h2>
            <p className="mt-1 text-sm text-muted-foreground">{project.title}</p>
          </div>
          {stage ? <Badge variant="secondary">{stage}</Badge> : null}
        </div>
        <dl className="grid gap-4 border-b py-5 sm:grid-cols-3">
          <div><dt className="text-xs text-muted-foreground">智能体</dt><dd className="mt-1 truncate text-sm font-medium">{activeAgent?.name ?? "已选择智能体"}</dd></div>
          <div><dt className="text-xs text-muted-foreground">平台</dt><dd className="mt-1 text-sm font-medium">{platformLabel(system.platform)}</dd></div>
          <div><dt className="text-xs text-muted-foreground">任务状态</dt><dd className="mt-1 text-sm font-medium">{system.active_task?.status || "generating"}</dd></div>
        </dl>
      </div>
    );
  }

  if (system.id && (system.status === "draft" || system.status === "saved")) {
    const currentSystemAgent = agents.find((agent) => agent.id === system.current_agent_id);
    return (
      <div className="mx-auto w-full max-w-5xl py-2">
        <div className="flex flex-col gap-4 border-b pb-5 sm:flex-row sm:items-start sm:justify-between">
          <div className="flex min-w-0 items-start gap-3">
            <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
              <Palette className="h-4 w-4" />
            </span>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="truncate text-base font-semibold">{system.name || `${project.title} 设计体系`}</h2>
                <Badge variant={system.status === "saved" ? "secondary" : "outline"}>{system.status === "saved" ? "已保存" : "草稿"}</Badge>
              </div>
              <p className="mt-1 text-sm text-muted-foreground">{project.title}</p>
            </div>
          </div>
          <AppLink
            href={paths.projectDesignSystemDetail(system.id)}
            className="inline-flex h-8 shrink-0 items-center justify-center gap-1.5 rounded-md bg-primary px-3 text-sm font-medium text-primary-foreground hover:bg-primary/90"
          >
            打开设计体系
            <ExternalLink className="h-3.5 w-3.5" />
          </AppLink>
        </div>
        <dl className="grid gap-4 border-b py-5 sm:grid-cols-3">
          <div><dt className="text-xs text-muted-foreground">平台</dt><dd className="mt-1 text-sm font-medium">{platformLabel(system.platform)}</dd></div>
          <div><dt className="text-xs text-muted-foreground">智能体</dt><dd className="mt-1 truncate text-sm font-medium">{currentSystemAgent?.name ?? "未记录"}</dd></div>
          <div><dt className="text-xs text-muted-foreground">最近更新</dt><dd className="mt-1 text-sm font-medium">{formatDate(system.updated_at)}</dd></div>
        </dl>
      </div>
    );
  }

  if (!showCreation) {
    return (
      <div className="mx-auto flex min-h-64 w-full max-w-5xl flex-col items-center justify-center border-y py-12 text-center">
        <span className="flex h-10 w-10 items-center justify-center rounded-md bg-muted text-muted-foreground">
          <Palette className="h-5 w-5" />
        </span>
        <h2 className="mt-4 text-base font-semibold">尚未建立设计体系</h2>
        <p className="mt-1 text-sm text-muted-foreground">{project.title}</p>
        <Button
          type="button"
          className="mt-5"
          onClick={() => setCreationOpen((current) => ({ ...current, [project.id]: true }))}
        >
          <Sparkles className="h-4 w-4" />
          创建设计体系
        </Button>
      </div>
    );
  }

  return (
    <div className="mx-auto w-full max-w-5xl py-2">
      <div className="flex items-start gap-3 border-b pb-5">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
          <Sparkles className="h-4 w-4" />
        </span>
        <div className="min-w-0">
          <h2 className="text-base font-semibold">创建设计体系</h2>
          <p className="mt-1 text-sm text-muted-foreground">{project.title}</p>
        </div>
      </div>

      {lastError ? (
        <div role="alert" className="mt-4 flex items-start gap-2 border-l-2 border-destructive bg-destructive/5 px-3 py-2 text-sm text-destructive">
          <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" />
          <span>{lastError}</span>
        </div>
      ) : null}

      <section className="grid gap-4 border-b py-5 md:grid-cols-[11rem_minmax(0,1fr)]">
        <div>
          <h3 className="text-sm font-medium">生成设置</h3>
          <p className="mt-1 text-xs text-muted-foreground">项目、智能体与目标平台</p>
        </div>
        <div className="space-y-4">
          <label className="block space-y-1.5">
            <span className="text-xs font-medium">智能体</span>
            <select
              aria-label="智能体"
              value={form.agentId}
              onChange={(event) => updateForm((current) => ({ ...current, agentId: event.target.value }))}
              className="h-9 w-full rounded-md border bg-background px-3 text-sm"
            >
              <option value="">选择智能体</option>
              {agentOptions.map((agent) => (
                <option key={agent.id} value={agent.id} disabled={!agent.available}>
                  {agent.name} · {agent.available ? agent.status : "不可用"}
                </option>
              ))}
            </select>
          </label>
          {form.agentId && !agentAvailable ? <p className="text-xs text-destructive">当前智能体不可用，请选择其他智能体。</p> : null}

          <div className="space-y-1.5">
            <span className="text-xs font-medium">平台</span>
            <div role="radiogroup" aria-label="平台" className="inline-flex max-w-full overflow-hidden rounded-md border bg-muted/30 p-0.5">
              {PLATFORM_OPTIONS.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  role="radio"
                  aria-checked={form.platform === option.value}
                  className={`h-8 min-w-20 px-3 text-sm transition-colors ${form.platform === option.value ? "rounded-sm bg-background font-medium shadow-sm" : "text-muted-foreground hover:text-foreground"}`}
                  onClick={() => updateForm((current) => ({ ...current, platform: option.value }))}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>
        </div>
      </section>

      <section className="grid gap-4 border-b py-5 md:grid-cols-[11rem_minmax(0,1fr)]">
        <div>
          <h3 className="text-sm font-medium">设计目标</h3>
          <p className="mt-1 text-xs text-muted-foreground">最终提交给智能体的项目描述</p>
        </div>
        <Textarea
          aria-label="设计目标"
          value={form.brief}
          onChange={(event) => updateForm((current) => ({ ...current, brief: event.target.value }))}
          className="min-h-28 resize-y"
        />
      </section>

      <section className="grid gap-4 border-b py-5 md:grid-cols-[11rem_minmax(0,1fr)]">
        <div>
          <h3 className="text-sm font-medium">参考资料</h3>
          <p className="mt-1 text-xs text-muted-foreground">可选</p>
        </div>
        <div className="space-y-5">
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-2 text-sm font-medium"><Upload className="h-4 w-4 text-muted-foreground" />上传资料</div>
              <Button type="button" size="sm" variant="outline" disabled={uploading} onClick={() => fileInputRef.current?.click()}>
                <Upload className="h-3.5 w-3.5" />
                {uploading ? "上传中…" : "添加"}
              </Button>
              <input
                ref={fileInputRef}
                type="file"
                aria-label="上传参考资料"
                className="sr-only"
                onChange={(event) => {
                  const file = event.target.files?.[0];
                  if (file) void handleUpload(file);
                  event.target.value = "";
                }}
              />
            </div>
            {form.attachments.map((item) => (
              <div key={item.attachmentId} className="flex min-w-0 items-center gap-2 border-t py-2 text-sm">
                <span className="min-w-0 flex-1 truncate">{item.label}</span>
                <Button
                  type="button"
                  size="icon"
                  variant="ghost"
                  className="h-7 w-7"
                  title={`移除 ${item.label}`}
                  aria-label={`移除 ${item.label}`}
                  onClick={() => updateForm((current) => ({
                    ...current,
                    attachments: current.attachments.filter((reference) => reference.attachmentId !== item.attachmentId),
                  }))}
                >
                  <X className="h-3.5 w-3.5" />
                </Button>
              </div>
            ))}
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <label className="space-y-1.5">
              <span className="flex items-center gap-2 text-sm font-medium"><Palette className="h-4 w-4 text-muted-foreground" />品牌色</span>
              <div className="flex items-center gap-2">
                <input
                  type="color"
                  aria-label="选择品牌色"
                  value={isHexColor(form.brandColor) ? form.brandColor : "#000000"}
                  onChange={(event) => updateForm((current) => ({ ...current, brandColor: event.target.value.toUpperCase() }))}
                  className="h-9 w-10 shrink-0 rounded-md border bg-transparent p-1"
                />
                <Input
                  aria-label="品牌色"
                  value={form.brandColor}
                  placeholder="#2463EB"
                  onChange={(event) => updateForm((current) => ({ ...current, brandColor: event.target.value }))}
                />
              </div>
              {!validColor ? <span className="block text-xs text-destructive">请输入 6 位十六进制颜色。</span> : null}
            </label>
            <label className="space-y-1.5">
              <span className="flex items-center gap-2 text-sm font-medium"><LinkIcon className="h-4 w-4 text-muted-foreground" />参考链接</span>
              <Input
                aria-label="参考链接"
                value={form.link}
                placeholder="https://"
                onChange={(event) => updateForm((current) => ({ ...current, link: event.target.value }))}
              />
              {!validLink ? <span className="block text-xs text-destructive">仅支持 HTTPS 链接。</span> : null}
            </label>
          </div>

          <div className="grid gap-5 lg:grid-cols-2">
            <fieldset>
              <legend className="flex items-center gap-2 text-sm font-medium"><FileImage className="h-4 w-4 text-muted-foreground" />项目设计稿</legend>
              <div className="mt-2 border-y">
                {designFiles.length ? designFiles.map((file) => (
                  <ReferenceCheckbox
                    key={file.id}
                    checked={form.designFileIds.includes(file.id)}
                    label={file.title}
                    meta="项目设计稿"
                    onChange={() => updateForm((current) => ({ ...current, designFileIds: toggleId(current.designFileIds, file.id) }))}
                  />
                )) : <p className="py-3 text-xs text-muted-foreground">暂无可用设计稿</p>}
              </div>
            </fieldset>
            <fieldset>
              <legend className="flex items-center gap-2 text-sm font-medium"><Bot className="h-4 w-4 text-muted-foreground" />Figma UI 规范</legend>
              <div className="mt-2 border-y">
                {legacyProfiles.length ? legacyProfiles.map((profile) => (
                  <ReferenceCheckbox
                    key={profile.id}
                    checked={form.profileIds.includes(profile.id)}
                    label={profile.name}
                    meta="历史 UI 规范参考"
                    onChange={() => updateForm((current) => ({ ...current, profileIds: toggleId(current.profileIds, profile.id) }))}
                  />
                )) : <p className="py-3 text-xs text-muted-foreground">暂无可用 UI 规范</p>}
              </div>
            </fieldset>
          </div>
        </div>
      </section>

      <div className="flex justify-end pt-5">
        <Button
          type="button"
          disabled={!canSubmit || isSubmittingCurrentProject}
          onClick={() => {
            if (!form.platform || !canSubmit) return;
            createSystem.mutate({
              project_id: project.id,
              agent_id: form.agentId,
              platform: form.platform,
              brief: form.brief.trim(),
              references: references(),
            });
          }}
        >
          {isSubmittingCurrentProject ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Sparkles className="h-4 w-4" />}
          {isSubmittingCurrentProject ? "提交中…" : "生成设计体系"}
        </Button>
      </div>
    </div>
  );
}
