"use client";

import { useMemo, useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Bot,
  CircleAlert,
  FileImage,
  GitBranch,
  Link as LinkIcon,
  LoaderCircle,
  Palette,
  PencilLine,
  Sparkles,
  Upload,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { designKeys } from "@multica/core/designs/keys";
import { useFileUpload } from "@multica/core/hooks/use-file-upload";
import { useWorkspaceId } from "@multica/core/hooks";
import type {
  Agent,
  AnalyzeProjectDesignSystemRepositoryRequest,
  CreateProjectDesignSystemRequest,
  DesignFile,
  DesignSystemProfile,
  Project,
  ProjectDesignSystem,
  ProjectDesignSystemPlatform,
  ProjectDesignSystemReferenceInput,
  ProjectRepositoryDesignContext,
} from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";

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

const ANALYZED_REFERENCES_LABEL = "已用于本次仓库分析";
const RESELECT_REFERENCES_LABEL = "重新选择参考资料";
const REFERENCES_NEED_ANALYSIS_LABEL = "参考资料需要重新分析";

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
  const code = stringValue(record.code).trim();
  if (code === "project_design_system_cancelled") {
    return "任务已停止。你可以修改设置后重新生成。";
  }
  if (code === "project_design_system_task_failed") {
    return "智能体执行失败。请检查智能体状态后重新生成。";
  }
  if (code === "project_design_system_invalid_artifacts") {
    return "智能体没有生成有效的设计体系。请调整设计目标或参考资料后重新生成。";
  }
  if (code === "project_design_system_invalid_repository_analysis") {
    return "智能体没有返回有效的仓库分析结果，请重试或更换智能体。";
  }
  for (const key of ["message", "error", "reason", "code"]) {
    const text = stringValue(record[key]).trim();
    if (text) return text;
  }
  return null;
}

function toggleId(values: string[], id: string): string[] {
  return values.includes(id) ? values.filter((value) => value !== id) : [...values, id];
}

function repositorySourcePaths(analysis: ProjectRepositoryDesignContext): string[] {
  return [...new Set([
    ...analysis.source_files.map((source) => source.path),
    ...analysis.facts.flatMap((fact) => fact.source_paths),
    ...analysis.conflicts.flatMap((conflict) => conflict.source_paths),
  ])];
}

function referenceSummary(form: ProjectDesignSystemForm): string {
  const items = [
    form.attachments.length ? `附件 ${form.attachments.length}` : "",
    form.brandColor.trim() ? "品牌色" : "",
    form.link.trim() ? "参考链接" : "",
    form.designFileIds.length ? `项目设计稿 ${form.designFileIds.length}` : "",
    form.profileIds.length ? `UI 规范 ${form.profileIds.length}` : "",
  ].filter(Boolean);
  return items.length ? items.join(" · ") : "未使用额外参考资料";
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
}: {
  project: Project;
  agents: Agent[];
  designFiles: DesignFile[];
  legacyProfiles: DesignSystemProfile[];
  system: ProjectDesignSystem | undefined;
  isLoading?: boolean;
}) {
  const wsId = useWorkspaceId();
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [forms, setForms] = useState<Record<string, ProjectDesignSystemForm>>({});
  const [submitErrors, setSubmitErrors] = useState<Record<string, string | null>>({});
  const [referenceEditing, setReferenceEditing] = useState<Record<string, boolean>>({});
  const { upload, uploading } = useFileUpload(api, (error) => toast.error(error.message));

  const form = forms[project.id] ?? initialForm(project, system);
  const currentAgent = agents.find((agent) => agent.id === form.agentId);
  const agentAvailable = isAgentAvailable(currentAgent);
  const validLink = isHttpsLink(form.link);
  const validColor = !form.brandColor.trim() || isHexColor(form.brandColor);
  const repositoryAnalysis = system?.input_snapshot.repository_analysis;
  const repositorySources = repositoryAnalysis ? repositorySourcePaths(repositoryAnalysis) : [];
  const referencesNeedAnalysis = Boolean(repositoryAnalysis && referenceEditing[project.id]);
  const canAnalyze = Boolean(
    form.agentId
      && agentAvailable
      && form.platform
      && validLink
      && validColor
      && !uploading,
  );
  const canSubmit = Boolean(
    form.agentId
      && agentAvailable
      && form.platform
      && form.brief.trim()
      && validLink
      && validColor
      && !uploading
      && !referencesNeedAnalysis,
  );
  const lastError = submitErrors[project.id] ?? errorMessage(system?.last_error);

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

  const analyzeRepository = useMutation({
    mutationFn: (request: AnalyzeProjectDesignSystemRepositoryRequest) => (
      api.analyzeProjectDesignSystemRepository(request)
    ),
    onMutate: (request) => {
      setSubmitErrors((current) => ({ ...current, [request.project_id]: null }));
    },
    onSuccess: (analyzed) => {
      setReferenceEditing((current) => ({ ...current, [analyzed.project_id]: false }));
      queryClient.setQueryData(
        designKeys.projectDesignSystemByProject(wsId, analyzed.project_id),
        analyzed,
      );
      if (analyzed.id) {
        queryClient.setQueryData(designKeys.projectDesignSystem(wsId, analyzed.id), analyzed);
      }
    },
    onError: (error, request) => {
      const message = error instanceof Error ? error.message : "无法分析项目仓库，请检查智能体与项目资源后重试。";
      setSubmitErrors((current) => ({ ...current, [request.project_id]: message }));
      toast.error(message);
    },
  });
  const isAnalyzingCurrentProject = analyzeRepository.isPending
    && analyzeRepository.variables?.project_id === project.id;

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

          <div className="flex justify-end border-t pt-4">
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={!canAnalyze || isAnalyzingCurrentProject}
              onClick={() => {
                if (!form.platform || !canAnalyze) return;
                analyzeRepository.mutate({
                  project_id: project.id,
                  agent_id: form.agentId,
                  platform: form.platform,
                  brief: form.brief.trim(),
                  references: references(),
                });
              }}
            >
              {isAnalyzingCurrentProject
                ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
                : <GitBranch className="h-3.5 w-3.5" />}
              {isAnalyzingCurrentProject ? "正在发起分析" : "分析项目仓库"}
            </Button>
          </div>
        </div>
      </section>

      {repositoryAnalysis ? (
        <section aria-label="仓库背景" className="grid gap-4 border-b py-5 md:grid-cols-[11rem_minmax(0,1fr)]">
          <div>
            <h3 className="text-sm font-medium">仓库背景</h3>
            <p className="mt-1 text-xs text-muted-foreground">
              {repositoryAnalysis.commit_sha
                ? `Commit ${repositoryAnalysis.commit_sha.slice(0, 12)}`
                : `置信度 ${Math.round(repositoryAnalysis.confidence * 100)}%`}
            </p>
          </div>
          <div className="space-y-5">
            <p className="text-sm leading-6">{repositoryAnalysis.summary}</p>

            {repositoryAnalysis.facts.length ? (
              <dl className="border-y">
                {repositoryAnalysis.facts.map((fact, index) => (
                  <div key={`${fact.kind}-${fact.label}-${index}`} className="grid gap-1 border-b py-3 last:border-b-0 sm:grid-cols-[10rem_minmax(0,1fr)]">
                    <dt className="text-xs font-medium text-muted-foreground">{fact.label}</dt>
                    <dd className="text-sm leading-5">{fact.value}</dd>
                  </div>
                ))}
              </dl>
            ) : null}

            {repositoryAnalysis.conflicts.length ? (
              <div className="space-y-3 border-l-2 border-amber-500 bg-amber-500/5 px-3 py-3">
                {repositoryAnalysis.conflicts.map((conflict, index) => (
                  <div key={`${conflict.label}-${index}`} className="space-y-1 text-sm">
                    <p className="font-medium">{conflict.label}</p>
                    <p className="text-muted-foreground">当前：{conflict.repository_fact}</p>
                    <p>目标：{conflict.user_intent}</p>
                  </div>
                ))}
              </div>
            ) : null}

            {repositorySources.length ? (
              <div>
                <p className="text-xs font-medium text-muted-foreground">来源</p>
                <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1">
                  {repositorySources.map((source) => (
                    <code key={source} className="break-all text-xs text-muted-foreground">{source}</code>
                  ))}
                </div>
              </div>
            ) : null}

            {repositoryAnalysis.suggested_brief ? (
              <div className="flex flex-wrap items-start justify-between gap-3 border-t pt-4">
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-medium text-muted-foreground">建议设计目标</p>
                  <p className="mt-1 text-sm leading-6">{repositoryAnalysis.suggested_brief}</p>
                </div>
                {repositoryAnalysis.suggested_brief.trim() !== form.brief.trim() ? (
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => updateForm((current) => ({
                      ...current,
                      brief: repositoryAnalysis.suggested_brief,
                    }))}
                  >
                    应用到设计目标
                  </Button>
                ) : null}
              </div>
            ) : null}
          </div>
        </section>
      ) : null}

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
        {repositoryAnalysis && !referencesNeedAnalysis ? (
          <div className="flex min-h-14 items-center justify-between gap-4 border-y py-3">
            <div className="min-w-0">
              <p className="text-xs font-medium">{ANALYZED_REFERENCES_LABEL}</p>
              <p className="mt-1 truncate text-sm text-muted-foreground">{referenceSummary(form)}</p>
            </div>
            <Button
              type="button"
              size="sm"
              variant="outline"
              aria-label={RESELECT_REFERENCES_LABEL}
              onClick={() => setReferenceEditing((current) => ({ ...current, [project.id]: true }))}
            >
              <PencilLine className="h-3.5 w-3.5" />
              {RESELECT_REFERENCES_LABEL}
            </Button>
          </div>
        ) : <div className="space-y-5">
          {referencesNeedAnalysis ? (
            <div role="status" className="flex items-center gap-2 border-l-2 border-amber-500 bg-amber-500/5 px-3 py-2 text-sm">
              <CircleAlert className="h-4 w-4 shrink-0 text-amber-600" />
              <span>{REFERENCES_NEED_ANALYSIS_LABEL}</span>
            </div>
          ) : null}
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
        </div>}
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
