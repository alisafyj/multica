"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, ChevronRight, PanelsTopLeft, Smartphone } from "lucide-react";
import { designScenarioRecipeListOptions } from "@multica/core/designs/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import type { DesignScenarioRecipe } from "@multica/core/types";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { DesignFilterPill } from "./design-filter-pill";
import { RecipePreview } from "./design-recipe-gallery";

// Sentinel for "no category picked". Recipe categories are free-form server
// strings, so no real value can be reserved for it.
const ALL_CATEGORIES = "__all__";

/** Categories shown as pills before the rest fold into 更多, as Open Design's
 *  home does — the catalogue carries 20+ and a full pill cloud buries the
 *  cards it filters. */
const VISIBLE_CATEGORY_COUNT = 6;

/**
 * The recipes that are 原型 or a refinement of it. The composer's rail shows
 * one 原型 chip for the whole family; which member is armed is decided on this
 * wall's scene row (below). Exported for the rail's selected state.
 */
export const PROTOTYPE_FAMILY: ReadonlySet<string> = new Set(["ui-mockup", "wireframe", "mobile-app"]);

export type PrototypeFamilyRecipe = "ui-mockup" | "wireframe" | "mobile-app";

/**
 * 原型's second-level scenes, at Open Design's fixed prototype sub-chip
 * positions (home-hero/sub-chips.ts PROTOTYPE_SUB_CHIPS): 移动应用 and
 * 线框图 are real recipes spliced into the facet row between 数据看板 and
 * 应用. Their card filters diverge from upstream deliberately: Open Design
 * aliases mobile to the 应用 facet and leaves wireframe unfiltered, which on
 * our catalogue made 移动应用 / 线框图 / 应用 show one identical wall. Our
 * catalogue carries a real platform axis and real wireframe seeds, so each
 * scene shows its own set.
 */
const PROTOTYPE_SCENES: ReadonlyArray<{
  recipe: PrototypeFamilyRecipe;
  label: string;
  icon: typeof Smartphone;
  matches: (recipe: DesignScenarioRecipe) => boolean;
}> = [
  {
    recipe: "mobile-app",
    label: "移动应用",
    icon: Smartphone,
    // The catalogue's own device axis, not the 应用 category — 应用 also
    // holds web apps, and the row's 应用 pill already shows those.
    matches: (item) => item.platform === "mobile",
  },
  {
    recipe: "wireframe",
    label: "线框图",
    icon: PanelsTopLeft,
    // The seeded wireframe templates carry the slug prefix; nothing else in
    // the catalogue marks fidelity.
    matches: (item) => item.slug === "wireframe" || item.slug.startsWith("wireframe-"),
  },
];

/** Open Design's fixed prototype row order around the two scenes: visible
 *  head, scenes, visible tail; the rest folds into 更多. The row is fixed —
 *  it does not collapse when a category has no recipes yet. */
const PROTOTYPE_ROW_HEAD: readonly string[] = ["落地页 / 营销", "数据看板"];
const PROTOTYPE_ROW_TAIL: readonly string[] = ["应用", "开发者工具"];
const PROTOTYPE_ROW_FOLDED: readonly string[] = ["品牌 / 设计", "文档 / 报告"];

/**
 * Example prompts on the create panel — Open Design's 示例提示词 wall. Every
 * card is a published catalogue recipe, the same data the community tab shows,
 * rendered with the community card's own live preview so the wall shows what
 * the recipe actually produces. Picking one seeds the brief exactly as the
 * gallery's "填入首页" does, without a second round trip.
 *
 * The category row has two shapes. While a prototype-family recipe is armed,
 * it is 原型's scene rail (Open Design's red-box row below the composer):
 * the fixed facet order with 移动应用 / 线框图 spliced in, cards scoped to
 * prototype-mode recipes, and picking a scene re-arms the composer's recipe.
 * Otherwise it is built from the catalogue's own categories — a hard-coded
 * taxonomy would render filters that match nothing the moment the catalogue
 * says something else. The first few render as pills; the rest fold into 更多.
 */
export function DesignExamplePrompts({
  onUse,
  onBrowseRecipes,
  recipe = "default",
  onPickPrototypeScene,
}: {
  onUse: (recipe: DesignScenarioRecipe) => void;
  /** Absent hides the community entry rather than leaving a dead link. */
  onBrowseRecipes?: () => void;
  /** The composer's armed recipe; a prototype-family value turns the category
   *  row into 原型's scene rail. */
  recipe?: string;
  /** Arms a prototype-family recipe. Plain assignment — the row decides the
   *  target, including falling back to bare 原型. */
  onPickPrototypeScene?: (recipe: PrototypeFamilyRecipe) => void;
}) {
  const wsId = useWorkspaceId();
  const { data: allRecipes = [], isLoading } = useQuery(designScenarioRecipeListOptions(wsId));
  const [category, setCategory] = useState(ALL_CATEGORIES);

  const sceneMode = !!onPickPrototypeScene && PROTOTYPE_FAMILY.has(recipe);
  // While 原型 is armed the wall shows what 原型 produces, as Open Design
  // scopes its example wall to the active chip.
  const recipes = useMemo(
    () => (sceneMode ? allRecipes.filter((item) => item.mode === "prototype") : allRecipes),
    [allRecipes, sceneMode],
  );

  const categories = useMemo(
    () => Array.from(new Set(recipes.map((item) => item.category).filter((item) => !!item.trim()))),
    [recipes],
  );
  // A category the catalogue no longer offers falls back to "all" by
  // derivation, so a refreshed catalogue cannot strand the rail on an empty
  // filter.
  const activeCategory = categories.includes(category) ? category : ALL_CATEGORIES;
  const activeScene = sceneMode
    ? PROTOTYPE_SCENES.find((scene) => scene.recipe === recipe) ?? null
    : null;
  // A scene carries its own card filter, overriding the category pick while
  // it is armed.
  const visible = activeScene
    ? recipes.filter(activeScene.matches)
    : activeCategory === ALL_CATEGORIES
      ? recipes
      : recipes.filter((item) => item.category === activeCategory);

  const pillCategories = categories.slice(0, VISIBLE_CATEGORY_COUNT);
  // The active category always renders as a pill, even when it lives in the
  // folded tail — a filter that hides its own selection reads as "no filter".
  if (activeCategory !== ALL_CATEGORIES && !pillCategories.includes(activeCategory)) {
    pillCategories.push(activeCategory);
  }
  const foldedCategories = categories.filter((item) => !pillCategories.includes(item));

  // The scene rail is part of 原型's fixed information architecture, so it
  // stays even when the catalogue has nothing to show under it.
  if (!isLoading && recipes.length === 0 && !sceneMode) return null;

  const pickCategory = (item: string) => {
    setCategory(item);
    // Picking a facet re-arms bare 原型: the facet filters what 原型
    // produces, it is not a refinement of 移动应用 or 线框图.
    if (activeScene) onPickPrototypeScene?.("ui-mockup");
  };

  // 原型 scene rail: fixed head, the two scenes at their upstream slots,
  // fixed tail; the remaining fixed entries plus any extra catalogue
  // categories fold into 更多.
  const sceneRowFixed = [...PROTOTYPE_ROW_HEAD, ...PROTOTYPE_ROW_TAIL, ...PROTOTYPE_ROW_FOLDED];
  const sceneRowFolded = [
    ...PROTOTYPE_ROW_FOLDED,
    ...categories.filter((item) => !sceneRowFixed.includes(item)),
  ];
  const sceneCategoryPill = (item: string) => (
    <DesignFilterPill
      key={item}
      label={item}
      selected={!activeScene && activeCategory === item}
      onClick={() => pickCategory(item)}
    />
  );

  return (
    <section className="mt-8">
      <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
        <h3 className="text-body font-medium">示例提示词</h3>
        {onBrowseRecipes ? (
          <button
            type="button"
            onClick={onBrowseRecipes}
            className="inline-flex cursor-pointer items-center gap-0.5 text-caption text-primary transition-colors hover:text-primary/80"
          >
            从社区模板开始
            <ChevronRight className="size-3" />
          </button>
        ) : null}
      </div>

      {sceneMode ? (
        <div role="group" aria-label="原型场景" className="mt-3 flex flex-wrap items-center gap-1.5">
          <DesignFilterPill
            label="全部"
            selected={!activeScene && activeCategory === ALL_CATEGORIES}
            onClick={() => pickCategory(ALL_CATEGORIES)}
          />
          {PROTOTYPE_ROW_HEAD.map(sceneCategoryPill)}
          {PROTOTYPE_SCENES.map((scene) => (
            <DesignFilterPill
              key={scene.recipe}
              label={scene.label}
              icon={scene.icon}
              selected={recipe === scene.recipe}
              // The row is one selection slot, as upstream's: arming a scene
              // replaces any facet pick rather than layering on top of it, so
              // stepping back out cannot land on a stale filter. Re-picking
              // the armed scene steps back to bare 原型, not out of the
              // family — the rail chip owns leaving it.
              onClick={() => {
                setCategory(ALL_CATEGORIES);
                onPickPrototypeScene?.(recipe === scene.recipe ? "ui-mockup" : scene.recipe);
              }}
            />
          ))}
          {PROTOTYPE_ROW_TAIL.map(sceneCategoryPill)}
          {!activeScene && activeCategory !== ALL_CATEGORIES && !sceneRowFixed.slice(0, 4).includes(activeCategory)
            ? sceneCategoryPill(activeCategory)
            : null}
          {sceneRowFolded.length > 0 ? (
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <button
                    type="button"
                    aria-label={`更多分类，另有 ${sceneRowFolded.length} 项`}
                    className="flex h-7 shrink-0 cursor-pointer items-center gap-1 rounded-full border bg-card px-2.5 text-caption text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                  >
                    <span>更多</span>
                    <ChevronDown className="size-3.5 shrink-0" />
                  </button>
                }
              />
              <DropdownMenuContent align="start" className="max-h-72 w-48 overflow-y-auto">
                {sceneRowFolded.map((item) => (
                  <DropdownMenuItem key={item} onClick={() => pickCategory(item)}>
                    <span className="truncate">{item}</span>
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}
        </div>
      ) : null}

      {isLoading ? (
        <div className="mt-3 flex gap-3 overflow-hidden">
          {Array.from({ length: 5 }).map((_, index) => (
            <Skeleton key={index} className="h-48 w-64 shrink-0 rounded-xl" />
          ))}
        </div>
      ) : (
        <>
          {!sceneMode && categories.length > 1 ? (
            <div role="group" aria-label="示例分类" className="mt-3 flex flex-wrap items-center gap-1.5">
              <DesignFilterPill
                label="全部"
                selected={activeCategory === ALL_CATEGORIES}
                onClick={() => setCategory(ALL_CATEGORIES)}
              />
              {pillCategories.map((item) => (
                <DesignFilterPill
                  key={item}
                  label={item}
                  selected={activeCategory === item}
                  onClick={() => setCategory(item)}
                />
              ))}
              {foldedCategories.length > 0 ? (
                <DropdownMenu>
                  <DropdownMenuTrigger
                    render={
                      <button
                        type="button"
                        aria-label={`更多分类，另有 ${foldedCategories.length} 项`}
                        className="flex h-7 shrink-0 cursor-pointer items-center gap-1 rounded-full border bg-card px-2.5 text-caption text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                      >
                        <span>更多</span>
                        <ChevronDown className="size-3.5 shrink-0" />
                      </button>
                    }
                  />
                  <DropdownMenuContent align="start" className="max-h-72 w-48 overflow-y-auto">
                    {foldedCategories.map((item) => (
                      <DropdownMenuItem key={item} onClick={() => setCategory(item)}>
                        <span className="truncate">{item}</span>
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              ) : null}
            </div>
          ) : null}

          {visible.length === 0 ? (
            <p className="mt-3 text-caption text-muted-foreground">该场景还没有示例。</p>
          ) : (
            // One row, scrolled sideways — Open Design's home wall. The -mx/px
            // pair lets the row bleed to the panel edge so a cut-off card
            // signals there is more to scroll; the scrollbar itself stays
            // invisible (their .home-hero__plugin-presets does the same) —
            // wheel/trackpad still scroll it.
            <div className="no-scrollbar -mx-1 mt-3 flex snap-x gap-3 overflow-x-auto px-1 pb-2">
              {visible.map((item) => (
                <button
                  key={item.slug}
                  type="button"
                  onClick={() => onUse(item)}
                  title={item.summary || item.title}
                  className="group/recipe flex w-64 shrink-0 snap-start cursor-pointer flex-col overflow-hidden rounded-xl border bg-card text-left transition-colors hover:border-primary/50"
                >
                  <RecipePreview recipe={item} />
                  <span className="min-w-0 px-3 py-2.5">
                    <span className="block truncate text-caption font-medium">
                      {item.title || item.slug}
                    </span>
                  </span>
                </button>
              ))}
            </div>
          )}
        </>
      )}
    </section>
  );
}
