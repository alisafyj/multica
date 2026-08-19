"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AppWindow,
  AudioLines,
  Bot,
  ChartColumn,
  ChevronDown,
  CircleDashed,
  FileCode,
  FileText,
  Frame,
  GitBranch,
  Globe,
  Image as ImageIcon,
  ListTodo,
  Monitor,
  PanelsTopLeft,
  Presentation,
  Smartphone,
  Sparkles,
  SwatchBook,
  Video,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { isAgentRuntimeBound } from "@multica/core/agents";
import { designKeys } from "@multica/core/designs/keys";
import { useWorkspaceId } from "@multica/core/hooks";
import { projectOpenIssuesOptions } from "@multica/core/issues/queries";
import { projectResourcesOptions } from "@multica/core/projects";
import { projectListOptions } from "@multica/core/projects/queries";
import { agentListOptions } from "@multica/core/workspace/queries";
import type {
  Agent,
  DesignDocument,
  DesignDocumentRecipe,
  DesignScenarioRecipe,
  Issue,
  ProjectDesignSystemPlatform,
  ProjectResource,
} from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { ActorAvatar } from "../common/actor-avatar";
import {
  PickerEmpty,
  PickerItem,
  PropertyPicker,
} from "../issues/components/pickers/property-picker";
import { ProjectPicker } from "../projects/components/project-picker";
import { StatusIcon } from "../issues/components/status-icon";
import { DesignDotGrid } from "./design-dot-grid";
import { DesignExamplePrompts } from "./design-example-prompts";
import { DesignRecentDocuments } from "./design-recent-documents";

/**
 * The creation rail (DC-049). One ordered list of every artifact type the
 * design centre presents, live and not-yet-live together, so the rail reads as
 * the whole product surface rather than two stacked tiers.
 *
 * A chip carrying a `recipe` starts a real page-design task; the recipe is the
 * configuration the agent follows, not a different artifact kind. Picking none
 * leaves the recipe at `default`, the free-form path. Chips without one keep
 * their position but stay inert — nothing in this phase produces a deck,
 * image, video, audio, WebGL or live artifact, and a design system is created
 * inside a project's own scope rather than from here (DC-052 / DC-054).
 */
const CREATE_TYPES: ReadonlyArray<{
  id: string;
  recipe?: Exclude<DesignDocumentRecipe, "default">;
  label: string;
  description: string;
  icon: typeof AppWindow;
}> = [
  { id: "ui-mockup", recipe: "ui-mockup", label: "UI Mockup", description: "可交互的应用界面稿", icon: AppWindow },
  { id: "deck", label: "幻灯片", description: "成套的演示页面", icon: Presentation },
  { id: "wireframe", recipe: "wireframe", label: "线框图", description: "低保真的页面与流程", icon: PanelsTopLeft },
  { id: "mobile-app", recipe: "mobile-app", label: "移动应用", description: "iOS 与 Android 界面", icon: Smartphone },
  { id: "document", label: "文档", description: "长文与报告版式", icon: FileText },
  { id: "figma-migration", recipe: "figma-migration", label: "来自 Figma", description: "把 Figma 稿转成页面设计", icon: Frame },
  { id: "image", label: "图片", description: "单张视觉素材", icon: ImageIcon },
  { id: "webgl", label: "WebGL 体验", description: "三维与实时渲染", icon: Sparkles },
  { id: "live-board", label: "实时看板", description: "接入实时数据的看板", icon: ChartColumn },
  { id: "video", label: "视频", description: "分镜与动态素材", icon: Video },
  { id: "audio", label: "音频", description: "语音与音效素材", icon: AudioLines },
  { id: "web-clone", recipe: "web-clone", label: "网站复刻", description: "按现有网站还原页面", icon: Globe },
  { id: "design-system", label: "创建设计体系", description: "在项目的设计体系里创建", icon: SwatchBook },
];

export const PLATFORM_OPTIONS: ReadonlyArray<{
  value: ProjectDesignSystemPlatform;
  label: string;
}> = [
  { value: "web", label: "Web" },
  { value: "mobile", label: "移动端" },
  { value: "cross_platform", label: "跨端" },
];

// Mirrors `designDocumentMaxBriefBytes` on the server. Counted in characters
// here, so a Chinese brief hits this well before the byte limit — the point is
// to warn early, not to reproduce the server's arithmetic.
export const BRIEF_MAX_LENGTH = 4000;

/**
 * A recipe the community gallery handed to the composer (DC-041). Carries its
 * own token so picking the same card twice re-seeds the brief: without it the
 * second click would be a no-op, since the recipe itself has not changed.
 */
export interface DesignRecipeSelection {
  token: number;
  recipe: DesignScenarioRecipe;
}

function repositoryUrl(resource: ProjectResource): string {
  const ref = resource.resource_ref as { url?: unknown } | undefined;
  return typeof ref?.url === "string" ? ref.url.trim() : "";
}

function repositoryLabel(resource: ProjectResource): string {
  const label = resource.label?.trim();
  if (label) return label;
  const url = repositoryUrl(resource);
  if (!url) return "未命名仓库";
  const normalized = url.replace(/\.git$/, "").replace(/\/+$/, "");
  return normalized.split("/").pop() || normalized;
}

export function matchesQuery(haystack: string, query: string): boolean {
  return !query || haystack.toLowerCase().includes(query);
}

/**
 * Shared trigger for the settings row. A field that is set reads as
 * foreground text; an unset one stays muted, so required-but-empty fields are
 * visible without painting them as errors before the user has done anything.
 */
export function SettingTrigger({
  filled,
  className,
  children,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { filled?: boolean }) {
  return (
    <button
      type="button"
      className={cn(
        // Opaque on purpose: the composer sits on the dot grid, and a
        // transparent control lets the pattern read through the button, which
        // both muddies the label and hides where the hit area actually is.
        "flex min-w-0 max-w-64 cursor-pointer items-center gap-1.5 rounded-full border bg-card px-2.5 py-1 text-caption transition-colors",
        "hover:bg-accent data-popup-open:bg-accent data-popup-open:text-accent-foreground",
        "disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:bg-card",
        filled ? "text-foreground" : "text-muted-foreground",
        className,
      )}
      {...props}
    >
      {children}
      <ChevronDown className="size-3 shrink-0 text-muted-foreground" />
    </button>
  );
}

/**
 * One position in the creation rail. Compact by design: the rail has to read
 * as a complete surface at a glance, so the scenario's longer wording lives in
 * the tooltip. A position with nothing behind it is disabled and says so in
 * its accessible name — never a live-looking control that does nothing.
 */
function CreateChip({
  label,
  description,
  icon: Icon,
  selected,
  disabled,
  onClick,
}: {
  label: string;
  description: string;
  icon: typeof AppWindow;
  selected?: boolean;
  disabled?: boolean;
  onClick?: () => void;
}) {
  return (
    <button
      type="button"
      // Omitted for chips that navigate rather than toggle a recipe — a
      // pressed state they can never enter would only mislead.
      aria-pressed={disabled || selected === undefined ? undefined : selected}
      aria-label={disabled ? `${label}（即将支持）` : undefined}
      disabled={disabled}
      title={disabled ? `${description}（即将支持）` : description}
      onClick={onClick}
      className={cn(
        // Same reason as SettingTrigger: opaque so the dot grid cannot show
        // through the chip.
        "flex h-7 min-w-0 shrink-0 items-center gap-1.5 rounded-full border bg-card px-2.5 text-caption transition-colors",
        disabled
          ? "cursor-not-allowed border-dashed bg-muted text-muted-foreground"
          : "cursor-pointer text-muted-foreground hover:bg-accent hover:text-foreground",
        // Selection has to survive hover, so it lives on border, surface,
        // weight and colour — dimensions hover never touches — and the hover
        // compound is spelled out so a selected chip cannot visually downgrade
        // to a plain hover.
        !disabled && selected
          ? "border-primary bg-accent font-medium text-primary hover:border-primary hover:bg-accent hover:text-primary"
          : undefined,
      )}
    >
      <Icon className="size-3.5 shrink-0" />
      <span className="truncate">{label}</span>
    </button>
  );
}

/**
 * The creation rail: one row that shows as many types as fit and folds the
 * rest behind 全部.
 *
 * Measurement, not a scroll container: a rail that scrolls hides the types it
 * cut off behind a gesture, while folding them into a menu keeps every type
 * one click away at any width. Which chips fit is a layout fact, so it is read
 * from the DOM after paint rather than guessed from a character count — chip
 * widths depend on the rendered font.
 *
 * The selected chip is always shown, even when it would measure out: a rail
 * that hides the active选择 reads as if nothing is selected at all.
 */
function CreateTypeRail({
  recipe,
  onPick,
}: {
  // Widened the same way the composer's own state is: a catalogue recipe is a
  // slug, and it has to leave every built-in chip unselected rather than fail
  // to type-check against the union.
  recipe: DesignDocumentRecipe | string;
  onPick: (recipe: Exclude<DesignDocumentRecipe, "default">) => void;
}) {
  const railRef = useRef<HTMLDivElement | null>(null);
  const chipRefs = useRef(new Map<string, HTMLElement>());
  const [hiddenIds, setHiddenIds] = useState<ReadonlySet<string>>(() => new Set());

  const selectedId = recipe === "default" ? null : recipe;

  useEffect(() => {
    const rail = railRef.current;
    if (!rail) return;

    const measure = () => {
      // No layout to read yet (first paint, a hidden ancestor, or a test
      // environment without layout). Folding on a zero width would hide every
      // chip; showing them all is the honest answer until a real width exists.
      if (rail.clientWidth === 0) {
        setHiddenIds((current) => (current.size === 0 ? current : new Set()));
        return;
      }
      // Room the 全部 trigger needs once anything folds. Reserved
      // unconditionally: measuring against the full width would let the last
      // chip fit, then be pushed out by the trigger it caused to appear.
      const reserved = 84;
      const limit = rail.clientWidth - reserved;
      const next = new Set<string>();
      // The selected chip is never folded, so its width is committed before
      // anything competes for the row. Charging it in document order instead
      // would let earlier chips spend the budget and leave the selected one to
      // be clipped by the rail's own overflow.
      const selectedNode = selectedId ? chipRefs.current.get(selectedId) : undefined;
      let used = selectedNode ? selectedNode.offsetWidth + 6 : 0;
      for (const type of CREATE_TYPES) {
        if (type.id === selectedId) continue;
        const node = chipRefs.current.get(type.id);
        if (!node) continue;
        const width = node.offsetWidth + 6; // gap-1.5
        if (used + width > limit) {
          next.add(type.id);
          continue;
        }
        used += width;
      }
      setHiddenIds((current) => {
        if (current.size === next.size && [...next].every((id) => current.has(id))) return current;
        return next;
      });
    };

    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(rail);
    return () => observer.disconnect();
  }, [selectedId]);

  const hiddenTypes = CREATE_TYPES.filter((type) => hiddenIds.has(type.id));

  return (
    <div role="group" aria-label="设计场景" className="relative flex items-center gap-1.5">
      <div ref={railRef} className="flex min-w-0 flex-1 items-center gap-1.5 overflow-hidden">
        {CREATE_TYPES.map((type) => (
          <div
            key={type.id}
            ref={(node) => {
              if (node) chipRefs.current.set(type.id, node);
              else chipRefs.current.delete(type.id);
            }}
            // Folded chips stay mounted so their width remains measurable —
            // remeasuring an unmounted chip is what makes a rail oscillate
            // between two widths.
            className={cn("shrink-0", hiddenIds.has(type.id) && "pointer-events-none absolute -z-10 opacity-0")}
            aria-hidden={hiddenIds.has(type.id) || undefined}
          >
            <CreateChip
              label={type.label}
              description={type.description}
              icon={type.icon}
              selected={type.recipe ? recipe === type.recipe : undefined}
              disabled={!type.recipe}
              onClick={type.recipe ? () => onPick(type.recipe!) : undefined}
            />
          </div>
        ))}
      </div>
      {hiddenTypes.length > 0 ? (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <button
                type="button"
                aria-label={`全部设计场景，另有 ${hiddenTypes.length} 项`}
                className="flex h-7 shrink-0 cursor-pointer items-center gap-1 rounded-full border bg-card px-2.5 text-caption text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
              >
                <span>全部</span>
                <ChevronDown className="size-3.5 shrink-0" />
              </button>
            }
          />
          <DropdownMenuContent align="end" className="w-56">
            {hiddenTypes.map((type) => (
              <DropdownMenuItem
                key={type.id}
                disabled={!type.recipe}
                onClick={type.recipe ? () => onPick(type.recipe!) : undefined}
              >
                <type.icon className="size-4 shrink-0" />
                <span className="flex-1 truncate">{type.label}</span>
                {!type.recipe ? (
                  <span className="shrink-0 text-caption text-muted-foreground">即将支持</span>
                ) : null}
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      ) : null}
    </div>
  );
}

export function AgentSetting({
  agents,
  agentId,
  onChange,
}: {
  agents: Agent[];
  agentId: string;
  onChange: (agentId: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const active = useMemo(() => agents.filter((agent) => !agent.archived_at), [agents]);
  const selected = active.find((agent) => agent.id === agentId);
  const query = filter.trim().toLowerCase();
  const filtered = active.filter((agent) => matchesQuery(agent.name, query));

  return (
    <PropertyPicker
      open={open}
      onOpenChange={setOpen}
      width="w-56"
      align="start"
      searchable
      searchPlaceholder="搜索智能体…"
      onSearchChange={setFilter}
      triggerRender={<SettingTrigger filled={!!selected} aria-label="设计智能体" />}
      trigger={
        selected ? (
          <>
            <ActorAvatar actorType="agent" actorId={selected.id} size="sm" showStatusDot />
            <span className="truncate">{selected.name}</span>
          </>
        ) : (
          <>
            <Bot className="size-3.5 shrink-0" />
            <span className="truncate">选择智能体</span>
          </>
        )
      }
    >
      {filtered.length === 0 ? (
        <PickerEmpty />
      ) : (
        filtered.map((agent) => {
          // An agent without a runtime cannot pick the task up, so offering it
          // would only produce a task that never starts.
          const runtimeBound = isAgentRuntimeBound(agent);
          return (
            <PickerItem
              key={agent.id}
              selected={agent.id === agentId}
              disabled={!runtimeBound}
              tooltip={runtimeBound ? undefined : "该智能体尚未绑定运行时，无法领取设计任务"}
              onClick={() => {
                onChange(agent.id);
                setOpen(false);
              }}
            >
              <ActorAvatar actorType="agent" actorId={agent.id} size="sm" showStatusDot />
              <span className="truncate">{agent.name}</span>
            </PickerItem>
          );
        })
      )}
    </PropertyPicker>
  );
}

/**
 * Repository scope (DC-053). Attaching a repository grounds the task against
 * it; "不指定仓库" is a first-class choice, not an empty state — so it is a
 * named row rather than a clear affordance, and the copy below the row spells
 * out what each choice means for the result.
 */
function RepositorySetting({
  repositories,
  repositoryId,
  disabled,
  onChange,
}: {
  repositories: ProjectResource[];
  repositoryId: string;
  disabled: boolean;
  onChange: (repositoryId: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const selected = repositories.find((repository) => repository.id === repositoryId);
  const query = filter.trim().toLowerCase();
  const filtered = repositories.filter((repository) =>
    matchesQuery(`${repositoryLabel(repository)} ${repositoryUrl(repository)}`, query),
  );

  return (
    <PropertyPicker
      open={open}
      onOpenChange={setOpen}
      width="w-64"
      align="start"
      searchable
      searchPlaceholder="搜索仓库…"
      onSearchChange={setFilter}
      triggerRender={
        <SettingTrigger filled={!!selected} disabled={disabled} aria-label="代码仓库" />
      }
      trigger={
        selected ? (
          <>
            <GitBranch className="size-3.5 shrink-0" />
            <span className="truncate">{repositoryLabel(selected)}</span>
          </>
        ) : (
          <>
            <GitBranch className="size-3.5 shrink-0" />
            <span className="truncate">不指定仓库</span>
          </>
        )
      }
    >
      <PickerItem
        selected={!repositoryId}
        onClick={() => {
          onChange("");
          setOpen(false);
        }}
      >
        <CircleDashed className="size-3.5 shrink-0 text-muted-foreground" />
        <span className="truncate">不指定仓库</span>
      </PickerItem>
      {filtered.map((repository) => (
        <PickerItem
          key={repository.id}
          selected={repository.id === repositoryId}
          tooltip={repositoryUrl(repository) || undefined}
          onClick={() => {
            onChange(repository.id);
            setOpen(false);
          }}
        >
          <GitBranch className="size-3.5 shrink-0 text-muted-foreground" />
          <span className="truncate">{repositoryLabel(repository)}</span>
        </PickerItem>
      ))}
      {repositories.length === 0 ? (
        <div className="px-2 py-1.5 text-caption text-muted-foreground">
          当前项目还没有关联代码仓库。
        </div>
      ) : null}
      {repositories.length > 0 && filtered.length === 0 && query ? <PickerEmpty /> : null}
    </PropertyPicker>
  );
}

/**
 * Optional issue link (DC-045). Linking is traceability only — it never moves
 * the issue's status, assignee or priority — so an unlinked design document is
 * an ordinary exploratory one, not an incomplete one.
 */
function IssueSetting({
  issues,
  issueId,
  disabled,
  onChange,
}: {
  issues: Issue[];
  issueId: string;
  disabled: boolean;
  onChange: (issueId: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const selected = issues.find((issue) => issue.id === issueId);
  const query = filter.trim().toLowerCase();
  const filtered = issues.filter((issue) =>
    matchesQuery(`${issue.identifier} ${issue.title}`, query),
  );

  return (
    <PropertyPicker
      open={open}
      onOpenChange={setOpen}
      width="w-72"
      align="start"
      searchable
      searchPlaceholder="搜索任务…"
      onSearchChange={setFilter}
      triggerRender={
        <SettingTrigger filled={!!selected} disabled={disabled} aria-label="关联任务" />
      }
      trigger={
        selected ? (
          <>
            <StatusIcon status={selected.status} className="size-3.5 shrink-0" />
            <span className="truncate">{selected.identifier}</span>
          </>
        ) : (
          <>
            <ListTodo className="size-3.5 shrink-0" />
            <span className="truncate">不关联任务</span>
          </>
        )
      }
    >
      <PickerItem
        emptyValue
        selected={!issueId}
        onClick={() => {
          onChange("");
          setOpen(false);
        }}
      >
        <CircleDashed className="size-3.5 shrink-0 text-muted-foreground" />
        <span className="truncate text-muted-foreground">不关联任务</span>
      </PickerItem>
      {filtered.map((issue) => (
        <PickerItem
          key={issue.id}
          selected={issue.id === issueId}
          tooltip={issue.title}
          onClick={() => {
            onChange(issue.id);
            setOpen(false);
          }}
        >
          <StatusIcon status={issue.status} className="size-3.5 shrink-0" />
          <span className="shrink-0 font-mono text-caption text-muted-foreground">
            {issue.identifier}
          </span>
          <span className="truncate">{issue.title}</span>
        </PickerItem>
      ))}
      {issues.length === 0 ? (
        <div className="px-2 py-1.5 text-caption text-muted-foreground">
          当前项目没有进行中的任务。
        </div>
      ) : null}
      {issues.length > 0 && filtered.length === 0 && query ? <PickerEmpty /> : null}
    </PropertyPicker>
  );
}

export function PlatformSetting({
  platform,
  onChange,
}: {
  platform: ProjectDesignSystemPlatform;
  onChange: (platform: ProjectDesignSystemPlatform) => void;
}) {
  const [open, setOpen] = useState(false);
  const selected = PLATFORM_OPTIONS.find((option) => option.value === platform);

  return (
    <PropertyPicker
      open={open}
      onOpenChange={setOpen}
      width="w-40"
      align="start"
      triggerRender={<SettingTrigger filled aria-label="目标平台" />}
      trigger={
        <>
          <Monitor className="size-3.5 shrink-0" />
          <span className="truncate">{selected?.label ?? "Web"}</span>
        </>
      }
    >
      {PLATFORM_OPTIONS.map((option) => (
        <PickerItem
          key={option.value}
          selected={option.value === platform}
          onClick={() => {
            onChange(option.value);
            setOpen(false);
          }}
        >
          <span className="truncate">{option.label}</span>
        </PickerItem>
      ))}
    </PropertyPicker>
  );
}

/**
 * Design centre home composer: the cross-project entry point for page-design
 * tasks (DC-042 / DC-049). Project and agent are required; repository, issue
 * and title are not (DC-053 / DC-045).
 */
export function DesignTaskComposer({
  onCreated,
  onBrowseRecipes,
  onOpenDocument,
  recipeSelection,
}: {
  /** Called after the server has created the document, never before. */
  onCreated: (document: DesignDocument) => void;
  /** Opens the community gallery. Absent hides the community entry. */
  onBrowseRecipes?: () => void;
  /** Opens a project tab, where that project's design files live. */
  onOpenDocument?: (document: DesignDocument) => void;
  /** A recipe picked in the community gallery, waiting to be applied. */
  recipeSelection?: DesignRecipeSelection | null;
}) {
  const wsId = useWorkspaceId();
  const queryClient = useQueryClient();
  const [brief, setBrief] = useState("");
  // Widened past the built-in chips on purpose: a community recipe contributes
  // its slug here, and the server validates it against the catalogue.
  const [recipe, setRecipe] = useState<DesignDocumentRecipe | string>("default");
  const [appliedRecipe, setAppliedRecipe] = useState<DesignScenarioRecipe | null>(null);
  const [projectId, setProjectId] = useState("");
  const [agentId, setAgentId] = useState("");
  const [repositoryId, setRepositoryId] = useState("");
  const [issueId, setIssueId] = useState("");
  const [platform, setPlatform] = useState<ProjectDesignSystemPlatform>("web");

  // Applying a catalogue recipe is an event, not derived state: it seeds the
  // brief once and then gets out of the way, so later edits to the words keep
  // the recipe the user chose. The gallery and the example rail hand over the
  // same shape, so they share this path.
  const applyRecipe = useCallback((picked: DesignScenarioRecipe) => {
    setRecipe(picked.slug);
    setAppliedRecipe(picked);
    setBrief(picked.prompt);
    if (picked.platform) setPlatform(picked.platform);
  }, []);
  const appliedToken = useRef<number | null>(null);
  useEffect(() => {
    if (!recipeSelection || appliedToken.current === recipeSelection.token) return;
    appliedToken.current = recipeSelection.token;
    applyRecipe(recipeSelection.recipe);
  }, [applyRecipe, recipeSelection]);

  const { data: projects = [] } = useQuery(projectListOptions(wsId));
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: projectResources = [] } = useQuery({
    ...projectResourcesOptions(wsId, projectId),
    enabled: !!projectId,
  });
  const { data: issues = [] } = useQuery(projectOpenIssuesOptions(wsId, projectId));

  const selectedProject = projects.find((project) => project.id === projectId);
  const repositories = useMemo(
    () => projectResources.filter((resource) => resource.resource_type === "github_repo"),
    [projectResources],
  );
  // A repository or issue chosen before the project changed no longer belongs
  // to it; the server would reject them, so drop them for rendering too.
  const activeRepositoryId = repositories.some((repository) => repository.id === repositoryId)
    ? repositoryId
    : "";
  const activeIssueId = issues.some((issue) => issue.id === issueId) ? issueId : "";

  const trimmedBrief = brief.trim();
  const briefTooLong = brief.length > BRIEF_MAX_LENGTH;

  const createDocument = useMutation({
    mutationFn: () =>
      api.createDesignDocument({
        project_id: projectId,
        agent_id: agentId,
        ...(activeRepositoryId ? { project_resource_id: activeRepositoryId } : {}),
        ...(activeIssueId ? { issue_id: activeIssueId } : {}),
        platform,
        recipe,
        brief: trimmedBrief,
      }),
    onSuccess: async (created) => {
      await queryClient.invalidateQueries({
        queryKey: designKeys.documents(wsId, created.project_id || projectId),
      });
      // DC-053: never let the result read as if the agent inspected code when
      // it did not. The server's own flag decides, not what was submitted.
      toast.success(
        created.repository_grounded === true
          ? "已创建页面设计任务，智能体将对所选仓库做只读取证"
          : "已创建页面设计任务，本次未做仓库取证",
      );
      setBrief("");
      onCreated(created);
    },
    onError: (error) =>
      toast.error(error instanceof Error ? error.message : "创建页面设计任务失败"),
  });

  const missingRequirement = !projectId
    ? "请选择项目"
    : !agentId
      ? "请选择智能体"
      : !trimmedBrief
        ? "请描述你想要的页面"
        : briefTooLong
          ? `需求描述超出 ${BRIEF_MAX_LENGTH} 字上限`
          : "";
  const canSubmit = !missingRequirement && !createDocument.isPending;

  return (
    <div className="relative min-h-0 flex-1 overflow-y-auto">
      <DesignDotGrid />
      <div className="relative z-10 mx-auto flex w-full max-w-4xl flex-col px-4 py-8 sm:px-6 sm:py-10">
        {/* The whole creation surface on one line. Only the scenarios with a
            real producer are live; the rest keep their position so the rail
            reads as complete without promising anything. */}
        <CreateTypeRail
          recipe={recipe}
          onPick={(picked) => {
            // A built-in chip and a catalogue recipe are the same field, so
            // picking one has to drop the other.
            setAppliedRecipe(null);
            setRecipe((current) => (current === picked ? "default" : picked));
          }}
        />

        <div className="mt-3 rounded-2xl border bg-card shadow-sm transition-colors focus-within:border-primary/60">
          <Textarea
            value={brief}
            onChange={(event) => setBrief(event.target.value)}
            aria-label="页面需求描述"
            placeholder="例如：做一个 CRM 客户列表页，支持筛选、批量操作和客户详情抽屉。"
            className="min-h-32 resize-none border-0 bg-transparent px-4 py-3.5 text-body shadow-none focus-visible:border-0 focus-visible:ring-0 dark:bg-transparent"
          />
          {/* Settings live in the card so choosing a project never means
              leaving the sentence being written. */}
          <div className="flex flex-wrap items-center gap-x-2 gap-y-2 px-3 pb-3">
            <ProjectPicker
              projectId={projectId || null}
              onUpdate={(updates) => setProjectId(updates.project_id ?? "")}
              align="start"
              triggerRender={<SettingTrigger filled={!!selectedProject} aria-label="项目" />}
            />
            <RepositorySetting
              repositories={repositories}
              repositoryId={activeRepositoryId}
              disabled={!projectId}
              onChange={setRepositoryId}
            />
            <IssueSetting
              issues={issues}
              issueId={activeIssueId}
              disabled={!projectId}
              onChange={setIssueId}
            />
            <PlatformSetting platform={platform} onChange={setPlatform} />
            <div className="ml-auto flex min-w-0 items-center gap-2">
              <AgentSetting agents={agents} agentId={agentId} onChange={setAgentId} />
              <Button
                type="button"
                size="sm"
                className="shrink-0"
                disabled={!canSubmit}
                onClick={() => createDocument.mutate()}
              >
                {createDocument.isPending ? "创建中…" : "生成页面设计"}
              </Button>
            </div>
          </div>
        </div>

        <div className="mt-2 flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
          {/* Says which field is still missing instead of leaving a dead
              button — "disabled" on its own does not point anywhere. */}
          <p role="status" className="text-caption text-muted-foreground">
            {createDocument.isPending ? "" : missingRequirement}
          </p>
          <p
            className={cn(
              "ml-auto text-caption",
              briefTooLong ? "text-destructive" : "text-muted-foreground",
            )}
          >
            {brief.length} / {BRIEF_MAX_LENGTH}
          </p>
        </div>

        {/* A catalogue recipe is not one of the five chips, so without this row
            the user would have no sign of which scenario is armed. */}
        {appliedRecipe ? (
          <div className="mt-3 flex min-w-0 items-center gap-2 rounded-lg border border-primary/40 bg-primary/5 px-3 py-2">
            <FileCode className="size-3.5 shrink-0 text-primary" />
            <p className="min-w-0 flex-1 truncate text-caption">
              <span className="text-muted-foreground">已套用社区配方：</span>
              <span className="font-medium text-foreground">
                {appliedRecipe.title || appliedRecipe.slug}
              </span>
            </p>
            <button
              type="button"
              aria-label="不使用该社区配方"
              title="不使用该社区配方"
              className="flex size-5 shrink-0 cursor-pointer items-center justify-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
              onClick={() => {
                setAppliedRecipe(null);
                setRecipe("default");
              }}
            >
              <X className="size-3" />
            </button>
          </div>
        ) : null}

        {/* DC-053: no repository is a legitimate way to work, so this reads as
            a statement of what will happen, never as a warning. What it must
            never do is leave the user believing the agent read code. */}
        <p className="mt-3 text-caption text-muted-foreground">
          {activeRepositoryId
            ? "已选择仓库：智能体会在任务内对该仓库做一次有界只读取证，并使用该仓库的设计体系。"
            : "未选择仓库：本次不读取任何代码仓库，智能体只依据你的描述、关联任务和项目级设计体系生成。"}
        </p>

        <DesignExamplePrompts onUse={applyRecipe} onBrowseRecipes={onBrowseRecipes} />
        <DesignRecentDocuments onOpenDocument={onOpenDocument} />
      </div>
    </div>
  );
}
