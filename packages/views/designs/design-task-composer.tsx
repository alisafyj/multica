"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AppWindow,
  Bot,
  ChevronDown,
  CircleDashed,
  FileCode,
  Frame,
  GitBranch,
  Globe,
  ListTodo,
  Monitor,
  PanelsTopLeft,
  Smartphone,
  SwatchBook,
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
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
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

// Scenario chips (DC-049). Every one of them produces a page design; they
// differ in the recipe the agent follows, not in the artifact kind — which is
// why they carry a `recipe` rather than an artifact type. Picking none leaves
// the recipe at `default`, the free-form path.
//
// Deliberately absent: deck / image / video / audio / live-artifact. Those
// artifact kinds are out of scope (DC-048), so they get no placeholder either.
const SCENARIO_CHIPS: ReadonlyArray<{
  recipe: Exclude<DesignDocumentRecipe, "default">;
  label: string;
  description: string;
  icon: typeof AppWindow;
}> = [
  { recipe: "ui-mockup", label: "UI Mockup", description: "可交互的应用界面稿", icon: AppWindow },
  { recipe: "web-clone", label: "网站复刻", description: "按现有网站还原页面", icon: Globe },
  { recipe: "wireframe", label: "线框图", description: "低保真的页面与流程", icon: PanelsTopLeft },
  { recipe: "mobile-app", label: "移动应用", description: "iOS 与 Android 界面", icon: Smartphone },
  { recipe: "figma-migration", label: "来自 Figma", description: "把 Figma 稿转成页面设计", icon: Frame },
];

// Still a placeholder (DC-049 / DC-054): the design system catalogue slice has
// not landed, so this one keeps its spot in the rail while staying inert.
// "来自模板" is no longer here — it now opens the community gallery.
const UPCOMING_CHIPS: ReadonlyArray<{
  id: string;
  label: string;
  description: string;
  icon: typeof AppWindow;
}> = [
  { id: "brand-kit", label: "创建品牌套件", description: "提炼一套品牌设计语言", icon: SwatchBook },
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
        "flex min-w-0 max-w-64 cursor-pointer items-center gap-1.5 rounded-full border px-2.5 py-1 text-caption transition-colors",
        "hover:bg-accent/60 data-popup-open:bg-accent data-popup-open:text-accent-foreground",
        "disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:bg-transparent",
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

function ScenarioChip({
  label,
  description,
  icon: Icon,
  selected,
  disabled,
  badge,
  onClick,
}: {
  label: string;
  description: string;
  icon: typeof AppWindow;
  selected?: boolean;
  disabled?: boolean;
  badge?: string;
  onClick?: () => void;
}) {
  return (
    <button
      type="button"
      // Omitted for chips that navigate rather than toggle a recipe — a
      // pressed state they can never enter would only mislead.
      aria-pressed={disabled || selected === undefined ? undefined : selected}
      disabled={disabled}
      onClick={onClick}
      className={cn(
        "flex min-w-0 items-start gap-2.5 rounded-xl border p-3 text-left transition-colors",
        disabled
          ? "cursor-not-allowed border-dashed bg-muted/30 text-muted-foreground"
          : "cursor-pointer hover:border-primary/40 hover:bg-accent/40",
        // Selection has to survive hover, so it lives on colour and weight —
        // dimensions hover never touches — and the hover compound is spelled
        // out so a selected chip cannot visually downgrade to a plain hover.
        !disabled && selected
          ? "border-primary bg-primary/5 hover:border-primary hover:bg-primary/5"
          : undefined,
      )}
    >
      <span
        className={cn(
          "flex size-7 shrink-0 items-center justify-center rounded-lg",
          disabled
            ? "bg-muted text-muted-foreground"
            : selected
              ? "bg-primary/10 text-primary"
              : "bg-muted text-muted-foreground",
        )}
      >
        <Icon className="size-4" />
      </span>
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="flex min-w-0 items-center gap-1.5">
          <span
            className={cn(
              "truncate text-body",
              !disabled && selected ? "font-medium text-foreground" : undefined,
            )}
          >
            {label}
          </span>
          {badge ? (
            <Badge variant="secondary" className="shrink-0 px-1.5 text-micro font-normal">
              {badge}
            </Badge>
          ) : null}
        </span>
        <span className="line-clamp-2 text-caption text-muted-foreground">{description}</span>
      </span>
    </button>
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
  recipeSelection,
}: {
  /** Called after the server has created the document, never before. */
  onCreated: (document: DesignDocument) => void;
  /** Opens the community gallery. Absent leaves the template chip inert. */
  onBrowseRecipes?: () => void;
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

  // Applying a gallery recipe is an event, not derived state: it seeds the
  // brief once and then gets out of the way, so later edits to the words keep
  // the recipe the user chose.
  const appliedToken = useRef<number | null>(null);
  useEffect(() => {
    if (!recipeSelection || appliedToken.current === recipeSelection.token) return;
    appliedToken.current = recipeSelection.token;
    setRecipe(recipeSelection.recipe.slug);
    setAppliedRecipe(recipeSelection.recipe);
    setBrief(recipeSelection.recipe.prompt);
    if (recipeSelection.recipe.platform) setPlatform(recipeSelection.recipe.platform);
  }, [recipeSelection]);

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
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div className="mx-auto flex w-full max-w-3xl flex-col px-4 py-10 sm:px-6 sm:py-14">
        <header className="text-center">
          <h2 className="text-title-lg font-semibold">Multica Design</h2>
          <p className="mx-auto mt-2 max-w-xl text-balance text-body text-muted-foreground">
            描述你想要的页面，选好项目和智能体，交给设计智能体生成可以直接打开的页面设计。
          </p>
        </header>

        <div className="mt-8 rounded-2xl border bg-card shadow-sm transition-colors focus-within:border-primary/60">
          <Textarea
            value={brief}
            onChange={(event) => setBrief(event.target.value)}
            aria-label="页面需求描述"
            placeholder="例如：做一个 CRM 客户列表页，支持筛选、批量操作和客户详情抽屉。"
            className="min-h-36 resize-none border-0 bg-transparent px-4 py-3.5 text-body shadow-none focus-visible:border-0 focus-visible:ring-0 dark:bg-transparent"
          />
          <div className="flex flex-wrap items-center justify-end gap-x-3 gap-y-2 px-4 pb-3.5">
            <p
              className={cn(
                "mr-auto text-caption",
                briefTooLong ? "text-destructive" : "text-muted-foreground",
              )}
            >
              {brief.length} / {BRIEF_MAX_LENGTH}
            </p>
            {/* Says which field is still missing instead of leaving a dead
                button — the pickers sit further down the page, so "disabled"
                on its own does not point anywhere. */}
            <p role="status" className="text-caption text-muted-foreground">
              {createDocument.isPending ? "" : missingRequirement}
            </p>
            <Button
              type="button"
              size="sm"
              disabled={!canSubmit}
              onClick={() => createDocument.mutate()}
            >
              {createDocument.isPending ? "创建中…" : "生成页面设计"}
            </Button>
          </div>
        </div>

        <div
          role="group"
          aria-label="设计场景"
          className="mt-5 grid gap-2 sm:grid-cols-2 lg:grid-cols-3"
        >
          {SCENARIO_CHIPS.map((chip) => (
            <ScenarioChip
              key={chip.recipe}
              label={chip.label}
              description={chip.description}
              icon={chip.icon}
              selected={recipe === chip.recipe}
              onClick={() => {
                // A built-in chip and a gallery recipe are the same field, so
                // picking one has to drop the other.
                setAppliedRecipe(null);
                setRecipe((current) => (current === chip.recipe ? "default" : chip.recipe));
              }}
            />
          ))}
          {/* Not a recipe of its own — it hands the choice to the community
              gallery, which comes back through `recipeSelection`. */}
          <ScenarioChip
            label="来自模板"
            description="从社区配方开始"
            icon={FileCode}
            disabled={!onBrowseRecipes}
            badge={onBrowseRecipes ? undefined : "即将支持"}
            onClick={onBrowseRecipes}
          />
          {UPCOMING_CHIPS.map((chip) => (
            <ScenarioChip
              key={chip.id}
              label={chip.label}
              description={chip.description}
              icon={chip.icon}
              disabled
              badge="即将支持"
            />
          ))}
        </div>

        {/* A gallery recipe is not one of the five chips, so without this row
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

        <div className="mt-6 flex flex-wrap items-center gap-2">
          <ProjectPicker
            projectId={projectId || null}
            onUpdate={(updates) => setProjectId(updates.project_id ?? "")}
            align="start"
            triggerRender={<SettingTrigger filled={!!selectedProject} aria-label="项目" />}
          />
          <AgentSetting agents={agents} agentId={agentId} onChange={setAgentId} />
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
        </div>

        {/* DC-053: no repository is a legitimate way to work, so this reads as
            a statement of what will happen, never as a warning. What it must
            never do is leave the user believing the agent read code. */}
        <p className="mt-3 text-caption text-muted-foreground">
          {activeRepositoryId
            ? "已选择仓库：智能体会在任务内对该仓库做一次有界只读取证，并使用该仓库的设计体系。"
            : "未选择仓库：本次不读取任何代码仓库，智能体只依据你的描述、关联任务和项目级设计体系生成。"}
        </p>
      </div>
    </div>
  );
}
