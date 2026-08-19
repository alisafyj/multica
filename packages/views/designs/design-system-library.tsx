"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Boxes, ExternalLink, FolderOpen, GitBranch, Moon, Package, Plus, Search, Sun, SwatchBook } from "lucide-react";
import { api } from "@multica/core/api";
import {
  builtinDesignSystemDetailOptions,
  builtinDesignSystemListOptions,
  projectDesignSystemCatalogueOptions,
  projectDesignSystemDetailOptions,
} from "@multica/core/designs/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import type {
  BuiltinDesignSystem,
  ProjectDesignSystem,
  ProjectDesignSystemCatalogueEntry,
  ProjectDesignSystemTokenGroup,
} from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@multica/ui/components/ui/empty";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { ReadonlyContent } from "../editor";
import { DesignFilterPill } from "./design-filter-pill";
import { DesignFilterSelect } from "./design-filter-select";
import { PLATFORM_OPTIONS } from "./design-task-composer";

/**
 * Ownership scope. Only `team` has data: a design system belongs to a project
 * in this workspace, and neither an author nor an official publisher exists on
 * the catalogue payload. The other two scopes therefore say why they are empty
 * instead of borrowing the workspace's systems and calling them something else.
 */
type LibraryScope = "mine" | "team" | "official";

const SCOPE_LABELS: ReadonlyArray<{ value: LibraryScope; label: string }> = [
  { value: "mine", label: "我的" },
  { value: "team", label: "团队" },
  { value: "official", label: "官方" },
];

const SCOPE_EMPTY_COPY: Record<LibraryScope, string> = {
  mine: "设计体系归属项目，目前没有按个人归属的体系。你在项目里创建的体系会出现在「团队」中。",
  team: "工作区还没有已保存的设计体系。在项目的「设计体系」里生成一套，保存后就会出现在这里。",
  official: "没有匹配的官方设计体系。",
};

// Sentinel for "no platform picked". Platform is a closed enum, but an entry
// can carry an empty platform, so the filter needs a value neither can take.
const ALL_PLATFORMS = "__all__";

function platformLabel(platform: string): string {
  return PLATFORM_OPTIONS.find((option) => option.value === platform)?.label ?? "未标注平台";
}

// Every saved system in the catalogue belongs to a project of this workspace,
// and the payload carries no author or publisher. Splitting "mine" or "official"
// out of it would be invention, so both stay empty until the data exists.
function scopeOf(_entry: ProjectDesignSystemCatalogueEntry): LibraryScope {
  return "team";
}

function looksLikeColor(value: string): boolean {
  return /^\s*(#[0-9a-f]{3,8}|(rgb|rgba|hsl|hsla|oklch|oklab|lab|lch|color)\()/i.test(value);
}

type TokenSection = "color" | "typography" | "shape" | "other";

const SECTION_KEYWORDS: ReadonlyArray<{ section: TokenSection; pattern: RegExp }> = [
  { section: "color", pattern: /(色|颜色|palette|colou?r|brand|accent|surface|semantic)/i },
  { section: "typography", pattern: /(字|排版|font|typo|text|type\b|lead)/i },
  { section: "shape", pattern: /(圆角|阴影|描边|radius|shadow|elevation|border|stroke)/i },
];

/**
 * Which section of the detail panel a token group belongs to. The package
 * contract does not name these sections, so the label decides first and the
 * token values decide when the label says nothing.
 */
function classifyTokenGroup(group: ProjectDesignSystemTokenGroup): TokenSection {
  const haystack = `${group.label} ${group.id}`;
  for (const { section, pattern } of SECTION_KEYWORDS) {
    if (pattern.test(haystack)) return section;
  }
  const colorTokens = group.tokens.filter((token) => looksLikeColor(token.value)).length;
  if (group.tokens.length > 0 && colorTokens * 2 >= group.tokens.length) return "color";
  return "other";
}

function SectionCard({
  title,
  count,
  children,
}: {
  title: string;
  count?: number;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-xl border bg-card p-4">
      <div className="flex items-center gap-2 text-caption font-medium text-muted-foreground">
        <span>{title}</span>
        {typeof count === "number" ? <span className="font-mono tabular-nums">{count}</span> : null}
      </div>
      <div className="mt-3">{children}</div>
    </section>
  );
}

function SectionEmpty({ children }: { children: React.ReactNode }) {
  return <p className="text-caption text-muted-foreground">{children}</p>;
}

function ColorSection({ groups }: { groups: ProjectDesignSystemTokenGroup[] }) {
  const tokens = groups.flatMap((group) => group.tokens);
  return (
    <SectionCard title="色彩" count={tokens.length || undefined}>
      {tokens.length === 0 ? (
        <SectionEmpty>这套体系还没有色彩令牌。</SectionEmpty>
      ) : (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-5">
          {tokens.slice(0, 20).map((token) => (
            <div key={token.name} className="min-w-0">
              <span
                className="block h-10 rounded-lg border"
                // The swatch is the system's own token value, not a palette of
                // ours — showing it any other colour would be showing a lie.
                style={{ background: token.value }}
                title={`${token.name}: ${token.value}`}
              />
              <span className="mt-1.5 block truncate font-mono text-micro text-muted-foreground">
                {token.name}
              </span>
            </div>
          ))}
        </div>
      )}
    </SectionCard>
  );
}

function TokenRowsSection({
  title,
  groups,
  emptyCopy,
}: {
  title: string;
  groups: ProjectDesignSystemTokenGroup[];
  emptyCopy: string;
}) {
  const tokens = groups.flatMap((group) => group.tokens);
  return (
    <SectionCard title={title} count={tokens.length || undefined}>
      {tokens.length === 0 ? (
        <SectionEmpty>{emptyCopy}</SectionEmpty>
      ) : (
        <dl className="flex flex-col gap-2">
          {tokens.slice(0, 12).map((token) => (
            <div key={token.name} className="flex min-w-0 items-baseline justify-between gap-3">
              <dt className="min-w-0 truncate font-mono text-caption text-foreground">{token.name}</dt>
              <dd className="min-w-0 shrink-0 truncate font-mono text-micro text-muted-foreground">
                {token.value}
              </dd>
            </div>
          ))}
        </dl>
      )}
    </SectionCard>
  );
}

function ComponentSection({ system }: { system: ProjectDesignSystem }) {
  const locators = system.content.locators ?? [];
  return (
    <SectionCard title="组件" count={locators.length || undefined}>
      {locators.length === 0 ? (
        <SectionEmpty>这套体系还没有登记组件或区块。</SectionEmpty>
      ) : (
        <div className="flex flex-wrap gap-1.5">
          {locators.slice(0, 24).map((locator) => (
            <Badge key={locator.id} variant="outline" className="max-w-52 px-1.5 text-micro font-normal">
              <span className="truncate">{locator.label || locator.id}</span>
            </Badge>
          ))}
        </div>
      )}
    </SectionCard>
  );
}

function SystemDetail({
  entry,
  onOpenProject,
}: {
  entry: ProjectDesignSystemCatalogueEntry;
  onOpenProject: (projectId: string) => void;
}) {
  const wsId = useWorkspaceId();
  const { data: system, isLoading, error } = useQuery(projectDesignSystemDetailOptions(wsId, entry.id));

  const groups = useMemo(() => {
    const buckets: Record<TokenSection, ProjectDesignSystemTokenGroup[]> = {
      color: [],
      typography: [],
      shape: [],
      other: [],
    };
    for (const group of system?.content.token_groups ?? []) {
      buckets[classifyTokenGroup(group)].push(group);
    }
    return buckets;
  }, [system]);

  // A saved catalogue entry can be under adjustment by the time it is opened,
  // and only the detail payload knows that.
  const isDraft = system?.status === "draft" || system?.has_unsaved_changes === true;
  const scopedToRepository = Boolean(entry.project_resource_id);

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <header className="flex min-w-0 flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1 basis-60">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <h2 className="min-w-0 break-words text-title font-semibold">
              {entry.name.trim() || "未命名设计体系"}
            </h2>
            {isDraft ? (
              <Badge variant="secondary" className="shrink-0 px-1.5 text-micro font-normal">
                草稿
              </Badge>
            ) : null}
            <Badge variant="outline" className="shrink-0 gap-1 px-1.5 text-micro font-normal">
              {scopedToRepository ? <GitBranch className="size-3" /> : <Package className="size-3" />}
              {scopedToRepository ? "仓库专属" : "项目通用"}
            </Badge>
          </div>
          <p className="mt-1 min-w-0 break-words text-body text-muted-foreground">
            {entry.project_id
              ? `项目绑定 · ${entry.project_title || "未知项目"} · ${platformLabel(entry.platform)}`
              : `独立设计体系 · ${platformLabel(entry.platform)}`}
          </p>
        </div>
        {entry.project_id ? (
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="h-7 shrink-0"
            onClick={() => onOpenProject(entry.project_id)}
          >
            <FolderOpen className="size-3.5" />
            打开项目
          </Button>
        ) : null}
      </header>

      {isLoading ? (
        <div className="grid gap-3 lg:grid-cols-2">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-40 w-full rounded-xl" />
          ))}
        </div>
      ) : error || !system ? (
        <div className="rounded-xl border border-dashed px-4 py-8 text-center text-body text-muted-foreground">
          无法加载这套设计体系的内容。
        </div>
      ) : (
        <div className="grid gap-3 lg:grid-cols-2">
          <ColorSection groups={groups.color} />
          <TokenRowsSection
            title="字体与字号"
            groups={groups.typography}
            emptyCopy="这套体系还没有字体与字号令牌。"
          />
          <ComponentSection system={system} />
          <TokenRowsSection
            title="圆角与阴影"
            groups={groups.shape}
            emptyCopy="这套体系还没有圆角与阴影令牌。"
          />
          {groups.other.length > 0 ? (
            <TokenRowsSection
              title="其他令牌"
              groups={groups.other}
              emptyCopy="没有其他令牌。"
            />
          ) : null}
        </div>
      )}
    </div>
  );
}

function SystemListItem({
  entry,
  selected,
  onSelect,
}: {
  entry: ProjectDesignSystemCatalogueEntry;
  selected: boolean;
  onSelect: () => void;
}) {
  const initial = (entry.name.trim() || entry.project_title.trim() || "?").slice(0, 1);
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={onSelect}
      className={cn(
        "flex w-full cursor-pointer items-start gap-2.5 rounded-lg p-2.5 text-left transition-colors",
        selected
          ? "bg-accent font-medium text-foreground hover:bg-accent"
          : "text-foreground hover:bg-accent/50",
      )}
    >
      <span className="flex size-7 shrink-0 items-center justify-center rounded-md bg-primary/10 text-caption font-medium text-primary">
        {initial}
      </span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body">{entry.name.trim() || "未命名设计体系"}</span>
        <span className="mt-0.5 truncate text-caption font-normal text-muted-foreground">
          {entry.project_id
            ? `${entry.project_title || "未知项目"}${entry.project_resource_id ? " · 仓库专属" : " · 项目通用"}`
            : "独立设计体系"}
        </span>
      </span>
    </button>
  );
}

const ALL_CATEGORIES = "__all__";

/**
 * Colour tokens pulled straight out of a package's `tokens.css`.
 *
 * A design system is judged by its palette long before its prose, so the
 * detail view leads with real swatches rather than a file listing. Parsed with
 * a scan for custom properties instead of a CSS parser: the sheets are
 * generated, and a value this misses is one missing swatch, not a broken page.
 */
function colorTokensFromCSS(css: string): Array<{ name: string; value: string }> {
  const found: Array<{ name: string; value: string }> = [];
  const seen = new Set<string>();
  const pattern = /(--[a-z0-9-]+)\s*:\s*([^;{}]+);/gi;
  let match = pattern.exec(css);
  while (match !== null && found.length < 24) {
    const name = match[1] ?? "";
    const value = (match[2] ?? "").trim();
    // A var() alias resolves to another token that is already listed, so it
    // would render as a duplicate swatch with no colour of its own.
    if (looksLikeColor(value) && !seen.has(name) && !value.includes("var(")) {
      seen.add(name);
      found.push({ name, value });
    }
    match = pattern.exec(css);
  }
  return found;
}

/**
 * The design language split the way Open Design's kit view reads it: the text
 * before the first `##` heading is the identity preamble, and every `##`
 * heading after it is one module of the system. Front matter and the document
 * title line are not content and are dropped.
 */
export function designMarkdownModules(markdown: string): { preamble: string; sections: Array<{ title: string; body: string }> } {
  const withoutFrontMatter = markdown.replace(/^---\r?\n[\s\S]*?\r?\n---\r?\n?/, "");
  const lines = withoutFrontMatter.split(/\r?\n/);
  const sections: Array<{ title: string; body: string[] }> = [];
  const preamble: string[] = [];
  let current: { title: string; body: string[] } | null = null;
  for (const line of lines) {
    const heading = /^##\s+(.+?)\s*$/.exec(line);
    if (heading) {
      current = { title: heading[1]!.replace(/^\d+[.)]\s*/, ""), body: [] };
      sections.push(current);
      continue;
    }
    if (current) current.body.push(line);
    else if (!/^#\s+/.test(line)) preamble.push(line);
  }
  return {
    preamble: preamble.join("\n").trim(),
    sections: sections
      .map((section) => ({ title: section.title, body: section.body.join("\n").trim() }))
      .filter((section) => section.body.length > 0),
  };
}

/** The dark showcase lives beside the light one; the server composes both paths. */
function showcaseVariantURL(showcaseUrl: string, variant: "light" | "dark"): string {
  if (!showcaseUrl) return "";
  return `${api.getBaseUrl()}${showcaseUrl.replace(/\/(light|dark)$/, "")}/${variant}`;
}

/**
 * The system's cover: Open Design's token-driven showcase framed the way its
 * own tab frames it. No scripts run in it (the document has none and its CSP
 * admits none), so the frame is fully sandboxed.
 */
function BuiltinShowcase({ system }: { system: BuiltinDesignSystem }) {
  const [variant, setVariant] = useState<"light" | "dark">("light");
  const src = showcaseVariantURL(system.showcase_url, variant);
  if (!src) return null;
  return (
    <section className="flex flex-col gap-2" aria-label="展示">
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-body font-medium">展示</h3>
        <div className="flex items-center gap-0.5">
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            aria-label="浅色"
            aria-pressed={variant === "light"}
            className={cn(variant === "light" && "bg-accent text-foreground")}
            onClick={() => setVariant("light")}
          >
            <Sun className="h-3.5 w-3.5" />
          </Button>
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            aria-label="深色"
            aria-pressed={variant === "dark"}
            className={cn(variant === "dark" && "bg-accent text-foreground")}
            onClick={() => setVariant("dark")}
          >
            <Moon className="h-3.5 w-3.5" />
          </Button>
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            aria-label="在新标签页中打开"
            title="在新标签页中打开"
            onClick={() => window.open(src, "_blank", "noopener,noreferrer")}
          >
            <ExternalLink className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
      <iframe
        key={src}
        title={`${system.name || system.slug} 展示`}
        src={src}
        sandbox=""
        referrerPolicy="no-referrer"
        className="h-[380px] w-full rounded-lg border bg-background"
      />
    </section>
  );
}

function BuiltinSystemDetail({ slug }: { slug: string }) {
  const wsId = useWorkspaceId();
  const { data, isLoading } = useQuery(builtinDesignSystemDetailOptions(wsId, slug));

  if (isLoading) {
    return (
      <div className="flex flex-col gap-3">
        <Skeleton className="h-6 w-48" />
        <Skeleton className="h-24 w-full rounded-lg" />
        <Skeleton className="h-40 w-full rounded-lg" />
      </div>
    );
  }
  if (!data) return null;

  // Typed at the source, so grouping is a fact from the package rather than a
  // guess from the value's shape. The CSS sheet is only consulted for the few
  // packages that ship no token file.
  const typed = data.tokens ?? [];
  const colors = typed.filter((token) => token.type === "color" && !token.value.includes("var("));
  // Open Design types its font tokens `fontFamily`; older packages may say
  // `font` or `typography`, so all three count as type.
  const typography = typed.filter((token) => ["fontFamily", "font", "typography"].includes(token.type));
  const fallbackColors = typed.length === 0 ? colorTokensFromCSS(data.tokens_css ?? "") : [];
  const palette = colors.length > 0
    ? colors.map((token) => ({ name: token.name, value: token.value }))
    : fallbackColors;

  const fontFamily = typography.find((token) => /(display|body|sans|font(-family)?)$/.test(token.name))?.value
    ?? typography[0]?.value;
  const modules = designMarkdownModules(data.design_markdown ?? "");

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="text-title font-medium">{data.name || slug}</h2>
          {data.category ? <Badge variant="secondary">{data.category}</Badge> : null}
          <Badge variant="outline">官方</Badge>
        </div>
        {data.description ? <p className="text-body text-muted-foreground">{data.description}</p> : null}
      </div>

      <BuiltinShowcase system={data} />

      {modules.preamble ? (
        <section className="flex flex-col gap-2">
          <h3 className="text-body font-medium">品牌标识</h3>
          <ReadonlyContent content={modules.preamble} className="max-w-none text-body leading-7 text-muted-foreground" />
        </section>
      ) : null}

      {typography.length > 0 ? (
        <section className="flex flex-col gap-2">
          <h3 className="text-body font-medium">字体排版</h3>
          <div className="rounded-lg border p-4">
            {/* Rendered in the system's own family so the sample shows the
                typeface rather than describing it. */}
            <p className="text-display leading-none" style={fontFamily ? { fontFamily } : undefined}>
              Ag 设计
            </p>
          </div>
          <dl className="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
            {typography.slice(0, 8).map((token) => (
              <div key={token.name} className="flex min-w-0 items-baseline gap-2 rounded-md border px-2.5 py-1.5">
                <dt className="shrink-0 font-mono text-micro text-muted-foreground">{token.name}</dt>
                <dd className="min-w-0 flex-1 truncate text-right font-mono text-caption">{token.value}</dd>
              </div>
            ))}
          </dl>
        </section>
      ) : null}

      {palette.length > 0 ? (
        <section className="flex flex-col gap-2">
          <h3 className="text-body font-medium">调色板</h3>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-4">
            {palette.slice(0, 24).map((token) => (
              <div key={token.name} className="flex items-center gap-2 rounded-lg border p-2">
                <span
                  aria-hidden="true"
                  className="size-8 shrink-0 rounded-md border"
                  style={{ background: token.value }}
                />
                <span className="flex min-w-0 flex-col">
                  <span className="truncate font-mono text-caption">{token.name}</span>
                  <span className="truncate font-mono text-micro text-muted-foreground">
                    {token.value}
                  </span>
                </span>
              </div>
            ))}
          </div>
        </section>
      ) : null}

      {modules.sections.map((section) => (
        <section key={section.title} className="flex flex-col gap-2">
          <h3 className="text-body font-medium">{section.title}</h3>
          <ReadonlyContent content={section.body} className="max-w-none text-body leading-7" />
        </section>
      ))}

      <p className="text-caption text-muted-foreground">
        官方体系是只读参考。要在项目里使用，请在项目的「设计体系」中创建，并以它作为参考风格。
      </p>
    </div>
  );
}

/**
 * The 官方 scope. Built-ins carry a slug, a category and no owner, where a
 * saved system carries a UUID, a platform and a project — different enough
 * that sharing one list would mean inventing the fields each is missing.
 */
function BuiltinSystemsPanel({ search }: { search: string }) {
  const wsId = useWorkspaceId();
  const { data: systems = [], isLoading, error, refetch } = useQuery(
    builtinDesignSystemListOptions(wsId),
  );
  const [category, setCategory] = useState<string>(ALL_CATEGORIES);
  const [selectedSlug, setSelectedSlug] = useState("");

  const categories = useMemo(
    () => Array.from(new Set(systems.map((system) => system.category).filter(Boolean))).sort(),
    [systems],
  );
  const activeCategory = categories.includes(category) ? category : ALL_CATEGORIES;
  const query = search.trim().toLowerCase();
  const visible = useMemo(
    () =>
      systems
        .filter((system) => activeCategory === ALL_CATEGORIES || system.category === activeCategory)
        .filter((system) =>
          !query || `${system.name} ${system.category} ${system.description}`.toLowerCase().includes(query),
        ),
    [activeCategory, query, systems],
  );
  const selected = visible.find((system) => system.slug === selectedSlug) ?? visible[0];

  if (isLoading) {
    return (
      <div className="flex flex-col gap-2 px-4 py-2">
        {Array.from({ length: 4 }).map((_, index) => (
          <Skeleton key={index} className="h-12 w-full rounded-lg" />
        ))}
      </div>
    );
  }
  if (error) {
    return (
      <div className="flex flex-col items-center gap-3 px-6 py-12 text-center">
        <p className="text-body font-medium">无法加载官方设计体系</p>
        <Button size="sm" variant="outline" onClick={() => void refetch()}>
          重试
        </Button>
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden lg:flex-row">
      <aside className="flex shrink-0 flex-col overflow-hidden border-b lg:w-72 lg:border-b-0 lg:border-r">
        {categories.length > 1 ? (
          <div className="shrink-0 px-3 py-2.5">
            <DesignFilterSelect
              label="官方设计体系分类"
              value={activeCategory}
              allValue={ALL_CATEGORIES}
              allLabel="全部分类"
              allCount={systems.length}
              options={categories.map((item) => ({
                value: item,
                label: item,
                count: systems.filter((system) => system.category === item).length,
              }))}
              onChange={setCategory}
            />
          </div>
        ) : null}
        <div className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto p-2">
          {visible.map((system) => (
            <BuiltinListItem
              key={system.slug}
              system={system}
              selected={system.slug === selected?.slug}
              onSelect={() => setSelectedSlug(system.slug)}
            />
          ))}
        </div>
      </aside>

      <section className="min-w-0 flex-1 overflow-y-auto p-4 lg:p-5">
        {selected ? (
          <BuiltinSystemDetail slug={selected.slug} />
        ) : (
          <Empty className="border py-12">
            <EmptyHeader>
              <EmptyMedia variant="icon"><Boxes /></EmptyMedia>
              <EmptyTitle>没有匹配的官方设计体系</EmptyTitle>
              <EmptyDescription>换一个关键词，或者清除分类筛选。</EmptyDescription>
            </EmptyHeader>
          </Empty>
        )}
      </section>
    </div>
  );
}

function BuiltinListItem({
  system,
  selected,
  onSelect,
}: {
  system: BuiltinDesignSystem;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={selected}
      // Selection is carried by weight and text colour, which hover does not
      // touch, so hovering the selected row cannot visually demote it.
      className={cn(
        "flex w-full flex-col items-start gap-0.5 rounded-lg px-2.5 py-2 text-left transition-colors",
        selected
          ? "bg-accent font-medium text-foreground hover:bg-accent"
          : "text-muted-foreground hover:bg-accent/60 hover:text-foreground",
      )}
    >
      <span className="flex w-full min-w-0 items-center gap-2">
        <span className="min-w-0 flex-1 truncate text-body">{system.name || system.slug}</span>
        {system.swatches.length > 0 ? (
          <span className="flex shrink-0 items-center gap-0.5" aria-hidden="true">
            {system.swatches.slice(0, 5).map((value, index) => (
              <span key={`${value}-${index}`} className="size-2.5 rounded-full border border-border/60" style={{ background: value }} />
            ))}
          </span>
        ) : null}
      </span>
      <span className="w-full truncate text-caption text-muted-foreground">
        {system.category || "未分类"}
      </span>
    </button>
  );
}

/**
 * "新建设计体系", at the library's top right as Open Design places its create
 * action on the design-systems page. Both open a dedicated creation flow; here
 * that is the standalone creation page, where the system belongs to the
 * workspace itself and is not bound to a project.
 */
function CreateDesignSystemButton({ onCreate }: { onCreate: () => void }) {
  return (
    <Button size="sm" onClick={onCreate}>
      <Plus className="size-3.5" />
      新建设计体系
    </Button>
  );
}

/**
 * Design system library (DC-054): the workspace-wide view of saved project
 * design systems. It is a reading and pick-up surface only — systems are
 * created inside a project's own scope, and this library never introduces a
 * workspace default that projects would inherit (DC-052 / migration §4.7).
 */
export function DesignSystemLibrary({
  onOpenProject,
  onCreate,
}: {
  /** Opens the project that owns a system, where it can be edited. */
  onOpenProject: (projectId: string) => void;
  /** Opens the standalone creation page. */
  onCreate: () => void;
}) {
  const wsId = useWorkspaceId();
  const { data: entries = [], isLoading, error, refetch } = useQuery(
    projectDesignSystemCatalogueOptions(wsId),
  );
  const [scope, setScope] = useState<LibraryScope>("team");
  const [platform, setPlatform] = useState<string>(ALL_PLATFORMS);
  const [search, setSearch] = useState("");
  const [selectedId, setSelectedId] = useState("");

  // The official scope is served by the bundled catalogue, so its count comes
  // from there rather than from the workspace's saved systems.
  const { data: builtinSystems = [] } = useQuery(builtinDesignSystemListOptions(wsId));
  const scopeCounts = useMemo(() => {
    const counts: Record<LibraryScope, number> = { mine: 0, team: 0, official: builtinSystems.length };
    for (const entry of entries) counts[scopeOf(entry)] += 1;
    return counts;
  }, [builtinSystems.length, entries]);

  const scoped = useMemo(
    () => entries.filter((entry) => scopeOf(entry) === scope),
    [entries, scope],
  );
  const platforms = useMemo<string[]>(
    () => Array.from(new Set(scoped.map((entry) => entry.platform as string))),
    [scoped],
  );
  // A platform the current scope no longer offers falls back to "all" by
  // derivation, so a refreshed catalogue cannot strand the list on an empty
  // filter.
  const activePlatform = platforms.includes(platform) ? platform : ALL_PLATFORMS;
  const query = search.trim().toLowerCase();
  const visible = useMemo(
    () =>
      scoped
        .filter((entry) => activePlatform === ALL_PLATFORMS || entry.platform === activePlatform)
        .filter((entry) =>
          !query || `${entry.name} ${entry.project_title}`.toLowerCase().includes(query),
        ),
    [activePlatform, query, scoped],
  );
  const selected = visible.find((entry) => entry.id === selectedId) ?? visible[0];

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-x-3 gap-y-2 px-4 py-2.5">
        <div role="group" aria-label="设计体系归属" className="flex flex-wrap items-center gap-1.5">
          {SCOPE_LABELS.map((option) => (
            <DesignFilterPill
              key={option.value}
              label={option.label}
              count={scopeCounts[option.value]}
              selected={scope === option.value}
              onClick={() => {
                setScope(option.value);
                setPlatform(ALL_PLATFORMS);
                setSelectedId("");
              }}
            />
          ))}
        </div>
        <div className="flex w-full items-center gap-2 sm:w-auto">
          <div className="relative min-w-0 flex-1 sm:w-56 sm:flex-none">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              aria-label="搜索设计体系"
              placeholder="搜索设计体系…"
              className="h-8 pl-8 text-body"
            />
          </div>
          <CreateDesignSystemButton onCreate={onCreate} />
        </div>
      </div>

      {scope === "official" ? (
        <BuiltinSystemsPanel search={search} />
      ) : isLoading ? (
        <div className="flex flex-col gap-2 px-4 py-2">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-12 w-full rounded-lg" />
          ))}
        </div>
      ) : error ? (
        <div className="flex flex-col items-center gap-3 px-6 py-12 text-center">
          <p className="text-body font-medium">无法加载设计体系目录</p>
          <Button size="sm" variant="outline" onClick={() => void refetch()}>
            重试
          </Button>
        </div>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden lg:flex-row">
          <aside className="flex shrink-0 flex-col overflow-hidden border-b lg:w-72 lg:border-b-0 lg:border-r">
            {platforms.length > 1 ? (
              <div className="shrink-0 px-3 py-2.5">
                <DesignFilterSelect
                  label="设计体系分类"
                  value={activePlatform}
                  allValue={ALL_PLATFORMS}
                  allLabel="全部分类"
                  allCount={scoped.length}
                  options={platforms.map((item) => ({
                    value: item,
                    label: platformLabel(item),
                    count: scoped.filter((entry) => entry.platform === item).length,
                  }))}
                  onChange={setPlatform}
                />
              </div>
            ) : null}
            <div className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto p-2">
              {visible.map((entry) => (
                <SystemListItem
                  key={entry.id}
                  entry={entry}
                  selected={entry.id === selected?.id}
                  onSelect={() => setSelectedId(entry.id)}
                />
              ))}
            </div>
          </aside>

          <section className="min-w-0 flex-1 overflow-y-auto p-4 lg:p-5">
            {selected ? (
              <SystemDetail entry={selected} onOpenProject={onOpenProject} />
            ) : (
              <Empty className="border py-12">
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    {scope === "team" ? <SwatchBook /> : <Boxes />}
                  </EmptyMedia>
                  <EmptyTitle>
                    {query || activePlatform !== ALL_PLATFORMS
                      ? "没有匹配的设计体系"
                      : "这里还没有设计体系"}
                  </EmptyTitle>
                  <EmptyDescription>
                    {query || activePlatform !== ALL_PLATFORMS
                      ? "换一个关键词，或者清除分类筛选。"
                      : SCOPE_EMPTY_COPY[scope]}
                  </EmptyDescription>
                </EmptyHeader>
              </Empty>
            )}
          </section>
        </div>
      )}
    </div>
  );
}
