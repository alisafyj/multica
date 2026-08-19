import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Boxes, Globe, LoaderCircle, Plus, Sparkles, X } from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { builtinDesignSystemListOptions } from "@multica/core/designs/queries";
import { designKeys } from "@multica/core/designs/keys";
import { agentListOptions } from "@multica/core/workspace/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import type { Agent, BuiltinDesignSystem, ProjectDesignSystemReferenceInput } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useNavigation } from "../navigation";
import { PLATFORM_OPTIONS, isAgentAvailable, visibleBuiltinSystems } from "./project-design-system-create";

const MAX_BUILTIN_REFERENCES = 3;
const MAX_LINKS = 8;

/**
 * The standalone design-system creation page, laid out the way Open Design's
 * creation flow is: a hero, then one labelled section per input — sources,
 * brand description, reference styles — and a generate action at the end.
 *
 * Unlike the project workbench, the system created here belongs to the
 * workspace itself: no project stands behind it, there can be any number of
 * them, and they live in the library beside the official catalogue. A
 * designer keeps several independent systems here the way Open Design's
 * design-systems page allows.
 */
export function WorkspaceDesignSystemCreate() {
  const wsId = useWorkspaceId();
  const navigation = useNavigation();
  const paths = useWorkspacePaths();
  const queryClient = useQueryClient();
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: builtinSystems = [] } = useQuery(builtinDesignSystemListOptions(wsId));

  const [name, setName] = useState("");
  const [sourceInput, setSourceInput] = useState("");
  const [links, setLinks] = useState<string[]>([]);
  const [brief, setBrief] = useState("");
  const [builtinSearch, setBuiltinSearch] = useState("");
  const [builtinSlugs, setBuiltinSlugs] = useState<string[]>([]);
  const [agentId, setAgentId] = useState("");
  const [platform, setPlatform] = useState("web");

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
          : "";

  const createSystem = useMutation({
    mutationFn: () =>
      api.createProjectDesignSystem({
        project_id: "",
        name: name.trim(),
        agent_id: agentId,
        platform: platform as "web" | "mobile" | "cross_platform",
        brief: brief.trim(),
        references: references(),
      }),
    onSuccess: (created) => {
      // The catalogue and the system's own cache: the new row belongs to both
      // the moment it exists, even before generation finishes.
      queryClient.invalidateQueries({ queryKey: designKeys.projectDesignSystemCatalogue(wsId) });
      queryClient.setQueryData(designKeys.projectDesignSystem(wsId, created.id), created);
      navigation.push(paths.projectDesignSystemDetail(created.id));
    },
    onError: (error: Error) => toast.error(error.message),
  });

  const references = (): ProjectDesignSystemReferenceInput[] => [
    ...links.map((link) => ({ kind: "link" as const, value: link, label: "来源链接" })),
    ...builtinSlugs.flatMap((slug) => {
      const builtin = builtinSystems.find((item) => item.slug === slug);
      return builtin ? [{ kind: "builtin_design_system" as const, value: slug, label: builtin.name }] : [];
    }),
  ];

  const visibleBuiltins = useMemo(
    () => visibleBuiltinSystems(builtinSystems, builtinSearch, builtinSlugs),
    [builtinSystems, builtinSearch, builtinSlugs],
  );

  const addLink = () => {
    if (!validLink || duplicate || links.length >= MAX_LINKS) return;
    setLinks((current) => [...current, trimmedInput]);
    setSourceInput("");
  };

  return (
    <div className="mx-auto w-full max-w-5xl overflow-y-auto px-4 py-6 sm:px-6">
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="mb-4 -ml-2"
        onClick={() => navigation.push(paths.designs())}
      >
        <ArrowLeft className="size-3.5" />
        返回设计中心
      </Button>

      <header className="flex items-start gap-3 border-b pb-5">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
          <Sparkles className="h-4 w-4" />
        </span>
        <div className="min-w-0">
          <h2 className="text-title-sm font-semibold">新建设计体系</h2>
          <p className="mt-1 max-w-2xl text-balance text-body text-muted-foreground">
            把你的品牌、产品、素材和设计参考教给智能体，生成一套独立的设计体系。它属于这个工作区，不绑定任何项目。
          </p>
        </div>
      </header>

      <section className="grid gap-4 border-b py-5 md:grid-cols-[11rem_minmax(0,1fr)]">
        <div>
          <h3 className="text-body font-medium">名称</h3>
          <p className="mt-1 text-caption text-muted-foreground">这套体系在工作区里叫什么</p>
        </div>
        <Input
          value={name}
          onChange={(event) => setName(event.target.value)}
          aria-label="设计体系名称"
          placeholder="例如 · 品牌视觉基线"
          className="h-9 max-w-sm text-body"
        />
      </section>

      <section className="grid gap-4 border-b py-5 md:grid-cols-[11rem_minmax(0,1fr)]">
        <div>
          <h3 className="text-body font-medium">来源</h3>
          <p className="mt-1 text-caption text-muted-foreground">从 GitHub、网站或源素材提取</p>
        </div>
        <div className="space-y-3">
          <div className="flex max-w-md items-center gap-2">
            <Input
              value={sourceInput}
              onChange={(event) => setSourceInput(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  addLink();
                }
              }}
              aria-label="GitHub 或网站"
              placeholder="https:// 你的品牌网站或仓库"
              className="h-9 text-body"
            />
            <Button type="button" size="sm" variant="outline" className="h-9" disabled={!validLink || duplicate || links.length >= MAX_LINKS} onClick={addLink}>
              <Plus className="size-3.5" />
              添加
            </Button>
          </div>
          {trimmedInput && !validLink ? <p className="text-caption text-destructive">请输入 https:// 开头的完整链接。</p> : null}
          {duplicate ? <p className="text-caption text-muted-foreground">这个链接已经添加过了。</p> : null}
          {links.length > 0 ? (
            <div className="flex flex-wrap gap-1.5">
              {links.map((link) => (
                <span key={link} className="inline-flex max-w-full items-center gap-1 rounded-md border bg-muted/40 py-1 pl-2 pr-1 text-caption">
                  <Globe className="size-3 shrink-0 text-muted-foreground" />
                  <a href={link} target="_blank" rel="noreferrer" className="truncate hover:underline">{link}</a>
                  <button
                    type="button"
                    aria-label={`移除 ${link}`}
                    className="rounded p-0.5 text-muted-foreground hover:bg-accent hover:text-foreground"
                    onClick={() => setLinks((current) => current.filter((item) => item !== link))}
                  >
                    <X className="size-3" />
                  </button>
                </span>
              ))}
            </div>
          ) : null}
        </div>
      </section>

      <section className="grid gap-4 border-b py-5 md:grid-cols-[11rem_minmax(0,1fr)]">
        <div>
          <h3 className="text-body font-medium">描述品牌</h3>
          <p className="mt-1 text-caption text-muted-foreground">定位、气质与视觉倾向</p>
        </div>
        <Textarea
          value={brief}
          onChange={(event) => setBrief(event.target.value)}
          aria-label="品牌描述"
          placeholder="公司和产品是做什么的、给谁用；想要的气质（克制 / 热烈 / 专业……）；对颜色、字体、密度的倾向。"
          className="min-h-32 resize-none text-body"
        />
      </section>

      <section className="grid gap-4 border-b py-5 md:grid-cols-[11rem_minmax(0,1fr)]">
        <div>
          <h3 className="text-body font-medium">参考</h3>
          <p className="mt-1 text-caption text-muted-foreground">以官方体系为参考风格，最多 {MAX_BUILTIN_REFERENCES} 个</p>
        </div>
        <div className="space-y-3">
          <Input
            value={builtinSearch}
            onChange={(event) => setBuiltinSearch(event.target.value)}
            aria-label="搜索官方设计体系"
            placeholder="搜索官方设计体系…"
            className="h-9 max-w-sm text-body"
          />
          <div className="max-h-72 overflow-y-auto rounded-lg border">
            {visibleBuiltins.map((builtin) => (
              <BuiltinReferenceRow
                key={builtin.slug}
                builtin={builtin}
                checked={builtinSlugs.includes(builtin.slug)}
                disabled={!builtinSlugs.includes(builtin.slug) && builtinSlugs.length >= MAX_BUILTIN_REFERENCES}
                onToggle={() =>
                  setBuiltinSlugs((current) =>
                    current.includes(builtin.slug)
                      ? current.filter((slug) => slug !== builtin.slug)
                      : [...current, builtin.slug],
                  )
                }
              />
            ))}
            {visibleBuiltins.length === 0 ? (
              <p className="px-3 py-3 text-caption text-muted-foreground">没有匹配的官方体系。</p>
            ) : null}
          </div>
        </div>
      </section>

      <section className="grid gap-4 py-5 md:grid-cols-[11rem_minmax(0,1fr)]">
        <div>
          <h3 className="text-body font-medium">生成设置</h3>
          <p className="mt-1 text-caption text-muted-foreground">智能体与目标平台</p>
        </div>
        <div className="space-y-4">
          <label className="block space-y-1.5">
            <span className="text-caption font-medium">智能体</span>
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
          </label>
          <div className="space-y-1.5">
            <span className="text-caption font-medium">平台</span>
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
          </div>
        </div>
      </section>

      <div className="sticky bottom-0 -mx-4 flex flex-wrap items-center justify-between gap-x-3 gap-y-2 border-t bg-background/95 px-4 py-3 backdrop-blur sm:-mx-6 sm:px-6">
        <p role="status" className="text-caption text-muted-foreground">
          {createSystem.isPending ? "" : missingRequirement}
        </p>
        <div className="flex items-center gap-2">
          <Button type="button" variant="outline" onClick={() => navigation.push(paths.designs())} disabled={createSystem.isPending}>
            取消
          </Button>
          <Button
            type="button"
            disabled={!!missingRequirement || createSystem.isPending}
            onClick={() => createSystem.mutate()}
          >
            {createSystem.isPending ? <LoaderCircle className="size-3.5 animate-spin" /> : <Boxes className="size-3.5" />}
            {createSystem.isPending ? "正在发起生成…" : "生成设计体系"}
          </Button>
        </div>
      </div>
    </div>
  );
}

function BuiltinReferenceRow({
  builtin,
  checked,
  disabled,
  onToggle,
}: {
  builtin: BuiltinDesignSystem;
  checked: boolean;
  disabled: boolean;
  onToggle: () => void;
}) {
  return (
    <label className={`flex min-w-0 cursor-pointer items-center gap-3 border-b py-2.5 pl-3 pr-3 last:border-b-0 ${disabled ? "opacity-50" : ""}`}>
      <input type="checkbox" checked={checked} onChange={onToggle} disabled={disabled} aria-label={builtin.name} className="h-4 w-4 shrink-0 accent-primary" />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-body">{builtin.name}</span>
        <span className="block truncate text-caption text-muted-foreground">{builtin.category}{builtin.description ? ` · ${builtin.description}` : ""}</span>
      </span>
      {builtin.swatches.length > 0 ? (
        <span className="flex shrink-0 gap-0.5">
          {builtin.swatches.slice(0, 4).map((swatch) => (
            <span key={swatch} className="size-3.5 rounded-full border" style={{ backgroundColor: swatch }} />
          ))}
        </span>
      ) : null}
    </label>
  );
}
