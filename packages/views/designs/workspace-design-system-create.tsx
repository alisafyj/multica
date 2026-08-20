"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  ArrowRight,
  ChevronRight,
  ExternalLink,
  Globe,
  LoaderCircle,
  Paperclip,
  Search,
  Sparkles,
  UploadCloud,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { designKeys } from "@multica/core/designs/keys";
import { useFileUpload } from "@multica/core/hooks/use-file-upload";
import { agentListOptions } from "@multica/core/workspace/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import type { Agent, ProjectDesignSystemReferenceInput } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { ReadonlyContent } from "../editor";
import { useNavigation } from "../navigation";
import {
  BRAND_CATEGORIES,
  BRAND_REFERENCES,
  QUICK_PICK_BRANDS,
  brandCategoryLabel,
  brandFaviconUrl,
  type BrandReference,
} from "./brand-references";
import { PLATFORM_OPTIONS, isAgentAvailable } from "./project-design-system-create";

const MAX_LINKS = 8;
const MAX_FILES = 20;
/** Open Design's per-file cap on the asset dropzone. */
const MAX_FILE_BYTES = 12 << 20;

/**
 * Open Design's sourceUrlLabel: protocol and www stripped, trailing slash
 * trimmed, GitHub repositories shortened to owner/repo.
 */
export function sourceLinkLabel(url: string): string {
  try {
    const parsed = new URL(url);
    const host = parsed.hostname.replace(/^www\./, "");
    const path = parsed.pathname.replace(/\/+$/, "");
    if (host === "github.com") {
      const segments = path.split("/").filter(Boolean);
      if (segments.length >= 2) return `${segments[0]}/${segments[1]}`;
    }
    return `${host}${path}`;
  } catch {
    return url;
  }
}

interface StagedFile {
  id: string;
  name: string;
  contentType: string;
  previewUrl: string;
}

/**
 * The standalone design-system creation page, replicating Open Design's
 * creation flow: a sticky top bar whose primary action is 继续生成, a sticky
 * hero column on the left, and on the right one bordered card whose
 * hairline-separated rows collect the sources — links and brands, files, the
 * brand description, a pasted DESIGN.md — followed by Multica's own required
 * settings.
 *
 * Deliberate differences from upstream, all grounded in this product:
 * an executing agent and a target platform are required here (P-008), the
 * system needs a name because it is a long-lived library entity, and the
 * repository / local code / Figma advanced sources stay in the project
 * workbench — a standalone system has no project to resolve them against.
 */
export function WorkspaceDesignSystemCreate() {
  const wsId = useWorkspaceId();
  const navigation = useNavigation();
  const paths = useWorkspacePaths();
  const queryClient = useQueryClient();
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { upload, uploadWithToast, uploading } = useFileUpload(api, (error, file) =>
    toast.error(`${file.name}：${error.message}`),
  );

  const [name, setName] = useState("");
  const [sourceInput, setSourceInput] = useState("");
  const [links, setLinks] = useState<string[]>([]);
  const [files, setFiles] = useState<StagedFile[]>([]);
  const [brief, setBrief] = useState("");
  const [designMd, setDesignMd] = useState("");
  const [designMdMode, setDesignMdMode] = useState<"edit" | "preview">("edit");
  const [notes, setNotes] = useState("");
  const [brandPickerOpen, setBrandPickerOpen] = useState(false);
  const [agentId, setAgentId] = useState("");
  const [platform, setPlatform] = useState("web");
  const [dragActive, setDragActive] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const currentAgent = agents.find((agent) => agent.id === agentId);
  const agentAvailable = isAgentAvailable(currentAgent);
  const trimmedInput = sourceInput.trim();
  const validLink = /^https:\/\/[^\s.]+\.[^\s]+$/.test(trimmedInput);
  const duplicate = links.some((link) => link === trimmedInput);

  const missingRequirement = !name.trim()
    ? "先为这套体系起个名字"
    : !brief.trim()
      ? "描述一下品牌或产品"
      : !agentId
        ? "选择一个智能体"
        : !agentAvailable
          ? "当前智能体不可用，请选择其他智能体"
          : uploading
            ? "素材上传中"
            : "";

  const createSystem = useMutation({
    mutationFn: async () => {
      const references: ProjectDesignSystemReferenceInput[] = [
        ...links.map((link) => ({ kind: "link" as const, value: link, label: "来源链接" })),
        ...files.map((file) => ({ kind: "attachment" as const, attachment_id: file.id, label: file.name })),
      ];
      // A pasted DESIGN.md becomes an attachment at submit time: the server's
      // frozen input then carries the exact bytes the user pasted.
      if (designMd.trim()) {
        const pasted = await upload(new File([designMd], "DESIGN.md", { type: "text/markdown" }));
        if (!pasted) throw new Error("DESIGN.md 上传失败，请重试");
        references.push({ kind: "attachment", attachment_id: pasted.id, label: "粘贴的 DESIGN.md" });
      }
      const composedBrief = notes.trim() ? `${brief.trim()}\n\n备注：${notes.trim()}` : brief.trim();
      return api.createProjectDesignSystem({
        project_id: "",
        name: name.trim(),
        agent_id: agentId,
        platform: platform as "web" | "mobile" | "cross_platform",
        brief: composedBrief,
        references,
      });
    },
    onSuccess: (created) => {
      // The catalogue and the system's own cache: the new row belongs to both
      // the moment it exists, even before generation finishes.
      queryClient.invalidateQueries({ queryKey: designKeys.projectDesignSystemCatalogue(wsId) });
      queryClient.setQueryData(designKeys.projectDesignSystem(wsId, created.id), created);
      navigation.push(paths.projectDesignSystemDetail(created.id));
    },
    onError: (error: Error) => toast.error(error.message),
  });

  const addLink = () => {
    if (!validLink || duplicate || links.length >= MAX_LINKS) return;
    setLinks((current) => [...current, trimmedInput]);
    setSourceInput("");
  };

  const stageFiles = async (incoming: FileList | File[]) => {
    for (const file of Array.from(incoming)) {
      if (files.length >= MAX_FILES) return;
      if (file.size > MAX_FILE_BYTES) {
        toast.error(`${file.name} 超过 12 MB 上限`);
        continue;
      }
      const result = await uploadWithToast(file);
      if (!result) continue;
      const previewUrl = file.type.startsWith("image/") ? URL.createObjectURL(file) : "";
      setFiles((current) => (
        current.some((item) => item.id === result.id) || current.length >= MAX_FILES
          ? current
          : [...current, { id: result.id, name: result.filename || file.name, contentType: file.type, previewUrl }]
      ));
    }
  };

  // Open Design's pick semantics: the brand's website joins the source links,
  // de-duplicated, and the picker closes.
  const addBrandLink = (brand: BrandReference) => {
    const link = `https://${brand.domain}`;
    setBrandPickerOpen(false);
    setLinks((current) => {
      if (current.includes(link)) return current;
      if (current.length >= MAX_LINKS) {
        toast.error(`最多 ${MAX_LINKS} 个来源链接`);
        return current;
      }
      return [...current, link];
    });
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
      {/* Open Design's sticky top bar: back on the left, the generate action
          as the page's primary on the right. */}
      <header className="sticky top-0 z-20 flex h-16 shrink-0 items-center justify-between gap-4 border-b bg-background/90 px-4 backdrop-blur sm:px-7">
        <Button type="button" variant="ghost" size="sm" onClick={() => navigation.push(paths.designs())}>
          <ArrowLeft className="size-3.5" />
          返回
        </Button>
        <div className="flex min-w-0 items-center gap-3">
          <p role="status" className="hidden truncate text-caption text-muted-foreground sm:block">
            {createSystem.isPending ? "" : missingRequirement}
          </p>
          <Button
            type="button"
            disabled={!!missingRequirement || createSystem.isPending}
            onClick={() => createSystem.mutate()}
          >
            {createSystem.isPending ? <LoaderCircle className="size-3.5 animate-spin" /> : null}
            {createSystem.isPending ? "正在发起生成…" : "继续生成"}
            {createSystem.isPending ? null : <ChevronRight className="size-3.5" />}
          </Button>
        </div>
      </header>

      <main className="mx-auto grid w-full max-w-[1280px] gap-6 px-4 py-9 sm:px-7 lg:grid-cols-[minmax(320px,420px)_minmax(0,1fr)] lg:gap-12">
        <aside className="self-start lg:sticky lg:top-[84px]">
          <CreateHero />
        </aside>

        <div className="min-w-0">
          <section aria-label="从 GitHub、网站或源素材提取">
            <h2 className="text-title-lg font-bold leading-tight">从 GitHub、网站或源素材提取</h2>
            <p className="mt-2 text-body text-muted-foreground">
              从 GitHub 仓库、网站、DESIGN.md 或能体现风格的文件开始。所选智能体会据此生成一套可用体系，之后可在库中继续调整。
            </p>

            <div className="mt-3 overflow-hidden rounded-lg border bg-card shadow-sm">
              {/* 名称 — Multica's own row: a standalone system is a long-lived
                  library entity and needs an identity upstream does not ask for. */}
              <FormRow label="名称">
                <Input
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  aria-label="设计体系名称"
                  placeholder="例如 · 品牌视觉基线"
                  className="h-9 max-w-sm text-body"
                />
              </FormRow>

              <FormRow label="GitHub 或网站">
                <div className="space-y-2.5">
                  <div className="flex flex-wrap items-center gap-2.5">
                    <Input
                      value={sourceInput}
                      onChange={(event) => setSourceInput(event.target.value)}
                      onKeyDown={(event) => {
                        if (event.key === "Enter" && !event.nativeEvent.isComposing) {
                          event.preventDefault();
                          addLink();
                        }
                      }}
                      aria-label="GitHub 或网站"
                      placeholder="https://github.com/org/repo"
                      className="h-9 min-w-0 flex-1 basis-56 text-body"
                    />
                    <Button type="button" size="sm" variant="outline" className="h-9" disabled={!validLink || duplicate || links.length >= MAX_LINKS} onClick={addLink}>
                      添加
                    </Button>
                    <Button type="button" size="sm" variant="ghost" className="h-9 whitespace-nowrap text-muted-foreground" aria-haspopup="dialog" onClick={() => setBrandPickerOpen(true)}>
                      <Sparkles className="size-3.5 text-primary" />
                      从品牌开始
                    </Button>
                  </div>
                  {trimmedInput && !validLink ? <p className="text-caption text-destructive">请输入 https:// 开头的完整链接。</p> : null}
                  {duplicate ? <p className="text-caption text-muted-foreground">这个链接已经添加过了。</p> : null}
                  {links.length > 0 ? (
                    <div aria-label="已添加的来源链接" className="flex flex-wrap gap-2">
                      {links.map((link) => (
                        <span key={link} className="inline-flex h-7 max-w-72 items-center gap-1.5 rounded-full border bg-muted/40 py-0.5 pl-2 pr-1 text-caption">
                          <SourceLinkFavicon url={link} />
                          <a href={link} target="_blank" rel="noreferrer" title={`打开 ${sourceLinkLabel(link)}`} className="truncate hover:underline">
                            {sourceLinkLabel(link)}
                          </a>
                          <button
                            type="button"
                            aria-label={`移除 ${sourceLinkLabel(link)}`}
                            className="rounded-full p-0.5 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                            onClick={() => setLinks((current) => current.filter((item) => item !== link))}
                          >
                            <X className="size-3" />
                          </button>
                        </span>
                      ))}
                    </div>
                  ) : null}
                </div>
              </FormRow>

              <FormRow label="添加文件" alignTop>
                <div className="space-y-3">
                  <input
                    ref={fileInputRef}
                    type="file"
                    multiple
                    accept="image/*,.pdf,.txt,.md,.json,.html,.woff,.woff2,.ttf,.otf"
                    className="hidden"
                    aria-label="上传素材文件"
                    onChange={(event) => {
                      if (event.target.files) void stageFiles(event.target.files);
                      event.target.value = "";
                    }}
                  />
                  <button
                    type="button"
                    aria-label="添加文件 — 拖放或点击浏览"
                    className={cn(
                      "flex min-h-[104px] w-full flex-col items-center justify-center gap-1.5 rounded-xl border-[1.5px] border-dashed px-4 py-4 text-center transition-colors",
                      dragActive ? "border-primary bg-primary/10" : "border-border hover:border-primary/50",
                    )}
                    onClick={() => fileInputRef.current?.click()}
                    onDragOver={(event) => {
                      event.preventDefault();
                      setDragActive(true);
                    }}
                    onDragLeave={() => setDragActive(false)}
                    onDrop={(event) => {
                      event.preventDefault();
                      setDragActive(false);
                      if (event.dataTransfer.files.length) void stageFiles(event.dataTransfer.files);
                    }}
                  >
                    <span className="flex size-9 items-center justify-center rounded-xl bg-primary/10 text-primary">
                      {uploading ? <LoaderCircle className="size-4 animate-spin" /> : <UploadCloud className="size-4" />}
                    </span>
                    <span className="text-caption font-semibold">拖放，或<span className="text-primary">点击浏览</span></span>
                    <span className="text-micro text-muted-foreground">图片、字体、Logo、PDF、HTML — 单个不超过 12 MB</span>
                  </button>
                  {files.length > 0 ? (
                    <div aria-label="已暂存的素材" className="grid grid-cols-[repeat(auto-fill,minmax(84px,1fr))] gap-3">
                      {files.map((file) => (
                        <figure key={file.id} className="min-w-0">
                          <span className="relative block aspect-square overflow-hidden rounded-lg border bg-muted/30">
                            {file.previewUrl ? (
                              <img src={file.previewUrl} alt="" className="h-full w-full object-cover" />
                            ) : (
                              <span className="flex h-full w-full items-center justify-center text-muted-foreground">
                                <Paperclip className="size-4" />
                              </span>
                            )}
                            <button
                              type="button"
                              aria-label={`移除 ${file.name}`}
                              className="absolute right-1 top-1 rounded-full border bg-background/90 p-0.5 text-muted-foreground hover:text-destructive"
                              onClick={() => setFiles((current) => current.filter((item) => item.id !== file.id))}
                            >
                              <X className="size-3" />
                            </button>
                          </span>
                          <figcaption className="mt-1 truncate text-micro text-muted-foreground">{file.name}</figcaption>
                        </figure>
                      ))}
                    </div>
                  ) : null}
                </div>
              </FormRow>

              {/* Upstream marks this 可选; here the brief is what the agent
                  generates from, so it is required (P-008). */}
              <FormRow label="描述品牌" hint="品牌语气、简介和产品上下文。会用于生成和后续调整。" alignTop>
                <Textarea
                  value={brief}
                  onChange={(event) => setBrief(event.target.value)}
                  aria-label="品牌描述"
                  rows={3}
                  placeholder="例如：Mission Impastabowl，一个支持自助点餐、移动应用和网站的快休闲意面餐厅"
                  className="min-h-[86px] resize-none text-body"
                />
              </FormRow>

              <FormRow
                label="粘贴 DESIGN.md"
                optional
                hint="粘贴 DESIGN.md，即可直接从 token、设计理由和组件指南创建设计体系。"
                hintAction={
                  <a
                    href="https://github.com/VoltAgent/awesome-design-md/"
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-0.5 text-primary hover:underline"
                  >
                    参考
                    <ExternalLink className="size-3" />
                  </a>
                }
                alignTop
              >
                <div className="space-y-2.5">
                  <div role="group" aria-label="DESIGN.md 查看模式" className="inline-flex gap-0.5 rounded-lg border bg-muted/40 p-0.5">
                    <button
                      type="button"
                      aria-pressed={designMdMode === "edit"}
                      className={cn("rounded-md px-2.5 py-0.5 text-micro font-semibold", designMdMode === "edit" ? "bg-background text-primary shadow-sm" : "text-muted-foreground")}
                      onClick={() => setDesignMdMode("edit")}
                    >
                      编辑
                    </button>
                    <button
                      type="button"
                      aria-pressed={designMdMode === "preview"}
                      disabled={!designMd.trim()}
                      className={cn("rounded-md px-2.5 py-0.5 text-micro font-semibold disabled:opacity-50", designMdMode === "preview" ? "bg-background text-primary shadow-sm" : "text-muted-foreground")}
                      onClick={() => setDesignMdMode("preview")}
                    >
                      预览
                    </button>
                  </div>
                  {designMdMode === "edit" ? (
                    <Textarea
                      value={designMd}
                      onChange={(event) => setDesignMd(event.target.value)}
                      aria-label="粘贴 DESIGN.md"
                      rows={5}
                      placeholder={'---\nname: Heritage\ncolors:\n  primary: "#1A1C1E"\n  tertiary: "#B8422E"\n---\n\n## Overview\n...'}
                      className="min-h-[150px] resize-y font-mono text-caption leading-relaxed"
                    />
                  ) : (
                    <div className="max-h-72 overflow-y-auto rounded-lg border bg-muted/20 p-3">
                      <ReadonlyContent content={designMd} className="max-w-none text-body" />
                    </div>
                  )}
                </div>
              </FormRow>

              <FormRow label="备注" optional alignTop>
                <Textarea
                  value={notes}
                  onChange={(event) => setNotes(event.target.value)}
                  aria-label="备注"
                  rows={3}
                  placeholder="例如：我们使用温暖自然的配色和圆角。品牌语气有趣但专业..."
                  className="min-h-[86px] resize-none text-body"
                />
              </FormRow>

              {/* Multica's own rows: generation runs as the picked agent's
                  task (P-008), and the platform shapes the component forms. */}
              <FormRow label="智能体">
                <select
                  aria-label="智能体"
                  value={agentId}
                  onChange={(event) => setAgentId(event.target.value)}
                  className="h-9 w-full max-w-sm rounded-md border bg-background px-3 text-body"
                >
                  <option value="">选择智能体</option>
                  {agents
                    .filter((agent: Agent) => !agent.archived_at || agent.id === agentId)
                    .map((agent: Agent) => (
                      <option key={agent.id} value={agent.id} disabled={!isAgentAvailable(agent)}>
                        {agent.name} · {isAgentAvailable(agent) ? agent.status : "不可用"}
                      </option>
                    ))}
                </select>
              </FormRow>

              <FormRow label="平台">
                <div role="radiogroup" aria-label="平台" className="inline-flex max-w-full overflow-hidden rounded-md border bg-muted/30 p-0.5">
                  {PLATFORM_OPTIONS.map((option) => (
                    <button
                      key={option.value}
                      type="button"
                      role="radio"
                      aria-checked={platform === option.value}
                      onClick={() => setPlatform(option.value)}
                      className={
                        platform === option.value
                          ? "rounded-[5px] bg-background px-3 py-1 text-caption font-medium text-foreground shadow-sm"
                          : "rounded-[5px] px-3 py-1 text-caption text-muted-foreground hover:text-foreground"
                      }
                    >
                      {option.label}
                    </button>
                  ))}
                </div>
              </FormRow>
            </div>

            <p role="status" className="mt-3 text-caption text-muted-foreground sm:hidden">
              {createSystem.isPending ? "" : missingRequirement}
            </p>
          </section>
        </div>
      </main>

      {brandPickerOpen ? <BrandPickerDialog onClose={() => setBrandPickerOpen(false)} onPick={addBrandLink} /> : null}
    </div>
  );
}

/** Favicon for a source-link chip, with the globe as the offline fallback. */
function SourceLinkFavicon({ url }: { url: string }) {
  const [failed, setFailed] = useState(false);
  let host = "";
  try {
    host = new URL(url).hostname;
  } catch {
    // Not a parseable URL — keep the globe.
  }
  if (!host || failed) return <Globe className="size-3.5 shrink-0 text-muted-foreground" />;
  return (
    <img
      src={brandFaviconUrl(host, 32)}
      alt=""
      loading="lazy"
      referrerPolicy="no-referrer"
      className="size-4 shrink-0 rounded-[3px] object-contain"
      onError={() => setFailed(true)}
    />
  );
}

function FormRow({
  label,
  optional,
  hint,
  hintAction,
  alignTop,
  children,
}: {
  label: string;
  optional?: boolean;
  hint?: string;
  hintAction?: React.ReactNode;
  alignTop?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div
      className={cn(
        "grid grid-cols-1 gap-2 border-t px-4 py-4 first:border-t-0 sm:px-[18px] md:grid-cols-[220px_minmax(0,1fr)] md:gap-4",
        alignTop ? "md:items-start" : "md:items-center",
      )}
    >
      <div className="min-w-0">
        <strong className="block text-caption font-semibold leading-tight">{label}</strong>
        {optional ? <span className="mt-0.5 block text-micro font-medium text-muted-foreground">可选</span> : null}
        {hint ? (
          <span className="mt-1 block text-micro leading-4 text-muted-foreground">
            {hint}
            {hintAction ? <span className="ml-1 inline-flex">{hintAction}</span> : null}
          </span>
        ) : null}
      </div>
      <div className="min-w-0">{children}</div>
    </div>
  );
}

/**
 * Open Design's create hero, in this product's voice: eyebrow, headline,
 * lede, the three steps with the time estimate and deliverables, and the
 * brand-agnostic preview card of what a generated system holds.
 */
function CreateHero() {
  return (
    <section className="flex flex-col gap-5">
      <div>
        <span className="inline-flex items-center gap-1.5 text-caption font-semibold text-primary">
          <Sparkles className="size-3.5" />
          设计体系
        </span>
        <h1 className="mt-2 font-serif text-display font-semibold leading-[1.04]">几分钟，生成一套设计体系</h1>
        <p className="mt-3 text-body leading-6 text-muted-foreground">
          把一个网站或 DESIGN.md——连同你手头已有的上下文——变成一套完整、贴合品牌、马上可用的设计体系。
        </p>
        <div className="mt-3 flex flex-wrap items-center gap-x-2 gap-y-1 text-caption text-muted-foreground">
          <span><strong className="text-foreground">3</strong> 步</span>
          <span aria-hidden="true" className="size-[3px] rounded-full bg-border" />
          <span>约 3 分钟</span>
          <span aria-hidden="true" className="size-[3px] rounded-full bg-border" />
          <span>DESIGN.md · tokens · UI Kit · 预览</span>
        </div>
        <ol className="mt-4 flex flex-col gap-2.5">
          {[
            { n: 1, title: "网站或 DESIGN.md", desc: "粘贴链接、挑一个品牌，或直接贴入 token" },
            { n: 2, title: "补充素材", desc: "图片、字体、参考链接——都可选" },
            { n: 3, title: "生成", desc: "所选智能体生成草稿，之后可继续调整" },
          ].map((step) => (
            <li key={step.n} className="flex items-start gap-2.5">
              <span className="flex size-5 shrink-0 items-center justify-center rounded-full border bg-card text-micro font-semibold">{step.n}</span>
              <span className="min-w-0 text-caption leading-5">
                <strong className="font-semibold">{step.title}</strong>
                <em className="ml-1.5 not-italic text-muted-foreground">{step.desc}</em>
              </span>
            </li>
          ))}
        </ol>
      </div>

      {/* Decorative: the outcome, made tangible. Brand-agnostic values only. */}
      <div aria-hidden="true" className="rounded-xl border bg-card p-4 shadow-sm">
        <div className="flex items-center gap-1.5 border-b pb-3">
          <span className="size-2 rounded-full bg-border" />
          <span className="size-2 rounded-full bg-border" />
          <span className="size-2 rounded-full bg-border" />
          <span className="ml-1 text-micro font-medium text-muted-foreground">你的设计体系</span>
        </div>
        <div className="mt-3">
          <span className="text-micro uppercase tracking-wide text-muted-foreground">Palette</span>
          <div className="mt-1.5 flex gap-1.5">
            {["#4f46e5", "#0ea5e9", "#14b8a6", "#f59e0b", "#f43f5e"].map((color) => (
              <span key={color} className="h-6 flex-1 rounded-md" style={{ background: color }} />
            ))}
          </div>
        </div>
        <div className="mt-3">
          <span className="text-micro uppercase tracking-wide text-muted-foreground">Type scale</span>
          <div className="mt-1 flex items-baseline gap-3">
            <span className="text-display leading-none">Aa</span>
            <span className="text-title leading-none">Aa</span>
            <span className="text-body leading-none">Aa</span>
          </div>
        </div>
        <div className="mt-3">
          <span className="text-micro uppercase tracking-wide text-muted-foreground">Components</span>
          <div className="mt-1.5 flex items-center gap-2">
            <span className="rounded-full bg-primary px-3 py-1 text-caption font-medium text-primary-foreground">Primary</span>
            <span className="rounded-full border px-3 py-1 text-caption text-muted-foreground">Ghost</span>
            <span className="flex min-w-0 flex-1 flex-col gap-1 rounded-md border p-2">
              <span className="h-1.5 w-3/4 rounded-full bg-muted" />
              <span className="h-1.5 w-1/2 rounded-full bg-muted" />
            </span>
          </div>
        </div>
      </div>
    </section>
  );
}

const BRAND_PAGE_SIZE = 24;
const ALL_BRAND_CATEGORIES = "all";

/** Favicon tile with a monogram fallback, as upstream's BrandFavicon. */
function BrandFavicon({ domain, name, className }: { domain: string; name: string; className?: string }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    setFailed(false);
  }, [domain]);
  if (failed) {
    return (
      <span
        aria-hidden="true"
        className={cn("flex items-center justify-center rounded-md bg-muted font-semibold text-muted-foreground", className)}
      >
        {name.slice(0, 1).toUpperCase()}
      </span>
    );
  }
  return (
    <img
      src={brandFaviconUrl(domain, 64)}
      alt=""
      loading="lazy"
      decoding="async"
      referrerPolicy="no-referrer"
      className={cn("object-contain", className)}
      onError={() => setFailed(true)}
    />
  );
}

/**
 * 从品牌开始 — Open Design's brand reference picker in its compact modal
 * form: search and a vertical category nav on the left, the quick-pick row
 * and the two-up brand wall on the right. Picking a brand hands it to the
 * host, which adds `https://<domain>` to the source links.
 *
 * The host mounts this only while open, so every open starts back at the
 * all-categories first page, matching upstream's unmount-on-close behaviour.
 */
function BrandPickerDialog({ onClose, onPick }: { onClose: () => void; onPick: (brand: BrandReference) => void }) {
  const [category, setCategory] = useState(ALL_BRAND_CATEGORIES);
  const [query, setQuery] = useState("");
  const [limit, setLimit] = useState(BRAND_PAGE_SIZE);
  const scrollRef = useRef<HTMLDivElement>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return BRAND_REFERENCES.filter((brand) => {
      if (category !== ALL_BRAND_CATEGORIES && brand.category !== category) return false;
      if (!q) return true;
      // Match the raw bucket AND its zh label, so typing 汽车 finds Porsche.
      return (
        brand.name.toLowerCase().includes(q) ||
        brand.domain.toLowerCase().includes(q) ||
        brand.category.toLowerCase().includes(q) ||
        brandCategoryLabel(brand.category).toLowerCase().includes(q)
      );
    });
  }, [category, query]);

  // Narrowing the wall (new filter / search) starts over from the top.
  useEffect(() => {
    setLimit(BRAND_PAGE_SIZE);
  }, [category, query]);

  // Infinite scroll with the modal body as the observer root; runtimes
  // without IntersectionObserver (jsdom) keep the 显示更多 button instead.
  useEffect(() => {
    const el = sentinelRef.current;
    if (!el || typeof IntersectionObserver === "undefined") return undefined;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setLimit((current) => Math.min(current + BRAND_PAGE_SIZE, filtered.length));
        }
      },
      { root: scrollRef.current, rootMargin: "600px 0px" },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [filtered.length]);

  const visible = filtered.slice(0, limit);
  const showQuickPicks = category === ALL_BRAND_CATEGORIES && query.trim() === "";

  return (
    <Dialog open onOpenChange={(next) => { if (!next) onClose(); }}>
      <DialogContent className="flex h-[min(680px,84vh)] w-[calc(100%-2rem)] flex-col gap-0 p-0 sm:max-w-[920px]">
        <DialogHeader className="shrink-0 gap-1.5 px-6 pb-3.5 pt-5 text-left">
          <DialogTitle className="text-title-lg font-bold">从品牌开始</DialogTitle>
          <DialogDescription className="text-caption">
            搜索数百个品牌，选择一个后我们会把它的网站作为风格参考加入。
          </DialogDescription>
        </DialogHeader>
        {/* One scrolling surface under the pinned header, as upstream. */}
        <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto overflow-x-hidden border-t px-6 pb-5 pt-4">
          <div className="flex flex-col items-stretch gap-5 sm:flex-row sm:items-start">
            <aside className="flex shrink-0 flex-col gap-3 sm:sticky sm:top-0 sm:w-[200px]">
              <div className="relative flex items-center">
                <Search aria-hidden="true" className="pointer-events-none absolute left-3 size-3.5 text-muted-foreground" />
                <Input
                  type="search"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  aria-label="搜索品牌"
                  placeholder="搜索品牌…"
                  className="h-[38px] rounded-full bg-muted/40 pl-9 text-caption"
                />
              </div>
              <nav aria-label="品牌分类" className="flex flex-row flex-wrap gap-0.5 sm:flex-col sm:flex-nowrap">
                {[ALL_BRAND_CATEGORIES, ...BRAND_CATEGORIES].map((value) => {
                  const active = category === value;
                  return (
                    <button
                      key={value}
                      type="button"
                      aria-pressed={active}
                      onClick={() => setCategory(value)}
                      className={cn(
                        "rounded-md px-2.5 py-[7px] text-left text-caption",
                        active
                          ? "bg-primary/10 font-medium text-primary"
                          : "text-muted-foreground hover:bg-muted/60 hover:text-foreground",
                      )}
                    >
                      {value === ALL_BRAND_CATEGORIES ? "全部" : brandCategoryLabel(value)}
                    </button>
                  );
                })}
              </nav>
            </aside>

            <div className="flex min-w-0 flex-1 flex-col gap-3">
              {showQuickPicks ? (
                <div role="group" aria-label="热门品牌 · 点击添加" className="flex flex-col gap-2">
                  <span className="text-micro font-semibold uppercase tracking-wider text-muted-foreground">
                    热门品牌 · 点击添加
                  </span>
                  <div className="flex flex-wrap gap-2">
                    {QUICK_PICK_BRANDS.map((brand) => (
                      <button
                        key={`quick-${brand.domain}`}
                        type="button"
                        onClick={() => onPick(brand)}
                        className="inline-flex items-center gap-2 rounded-full border bg-card py-1.5 pl-2 pr-3 text-caption font-medium transition-colors hover:border-primary hover:bg-primary/10"
                      >
                        <BrandFavicon domain={brand.domain} name={brand.name} className="size-[22px] rounded-[4px]" />
                        <span className="whitespace-nowrap">{brand.name}</span>
                      </button>
                    ))}
                  </div>
                </div>
              ) : null}

              <div className="grid grid-cols-1 gap-x-6 sm:grid-cols-2">
                {visible.map((brand) => (
                  <button
                    key={brand.domain}
                    type="button"
                    onClick={() => onPick(brand)}
                    className="group relative -mx-2 flex min-w-0 items-center gap-3 rounded-lg px-2 py-4 text-left transition-colors hover:bg-muted/50"
                  >
                    <span className="flex size-[46px] shrink-0 items-center justify-center overflow-hidden">
                      <BrandFavicon domain={brand.domain} name={brand.name} className="size-full rounded-md text-title" />
                    </span>
                    <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                      <span className="truncate text-caption font-semibold" title={brand.name}>{brand.name}</span>
                      <span className="truncate text-micro text-muted-foreground">{brandCategoryLabel(brand.category)}</span>
                    </span>
                    {/* Hover affordance: the 添加 pill slides in from the trailing edge. */}
                    <span
                      aria-hidden="true"
                      className="pointer-events-none absolute right-2 inline-flex translate-y-1.5 items-center gap-1 rounded-full bg-primary px-3.5 py-2 text-micro font-semibold text-primary-foreground opacity-0 transition-all group-hover:translate-y-0 group-hover:opacity-100 group-focus-visible:translate-y-0 group-focus-visible:opacity-100"
                    >
                      添加
                      <ArrowRight className="size-3" />
                    </span>
                  </button>
                ))}
              </div>
              {visible.length === 0 ? (
                <p className="py-2 text-caption text-muted-foreground">没有匹配的品牌。</p>
              ) : null}

              {limit < filtered.length ? (
                <>
                  <div ref={sentinelRef} aria-hidden="true" className="h-px" />
                  <div className="flex justify-center">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="rounded-full"
                      onClick={() => setLimit((current) => Math.min(current + BRAND_PAGE_SIZE, filtered.length))}
                    >
                      显示更多
                    </Button>
                  </div>
                </>
              ) : null}
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
