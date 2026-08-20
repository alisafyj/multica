"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, ChevronRight } from "lucide-react";
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
 * Example prompts on the create panel — Open Design's 示例提示词 wall. Every
 * card is a published catalogue recipe, the same data the community tab shows,
 * rendered with the community card's own live preview so the wall shows what
 * the recipe actually produces. Picking one seeds the brief exactly as the
 * gallery's "填入首页" does, without a second round trip.
 *
 * The category row is built from the catalogue's own categories rather than a
 * fixed list: a hard-coded taxonomy would render filters that match nothing
 * the moment the catalogue says something else. The first few render as
 * pills; the rest fold into 更多.
 */
export function DesignExamplePrompts({
  onUse,
  onBrowseRecipes,
}: {
  onUse: (recipe: DesignScenarioRecipe) => void;
  /** Absent hides the community entry rather than leaving a dead link. */
  onBrowseRecipes?: () => void;
}) {
  const wsId = useWorkspaceId();
  const { data: recipes = [], isLoading } = useQuery(designScenarioRecipeListOptions(wsId));
  const [category, setCategory] = useState(ALL_CATEGORIES);

  const categories = useMemo(
    () => Array.from(new Set(recipes.map((recipe) => recipe.category).filter((item) => !!item.trim()))),
    [recipes],
  );
  // A category the catalogue no longer offers falls back to "all" by
  // derivation, so a refreshed catalogue cannot strand the rail on an empty
  // filter.
  const activeCategory = categories.includes(category) ? category : ALL_CATEGORIES;
  const visible = activeCategory === ALL_CATEGORIES
    ? recipes
    : recipes.filter((recipe) => recipe.category === activeCategory);

  const pillCategories = categories.slice(0, VISIBLE_CATEGORY_COUNT);
  // The active category always renders as a pill, even when it lives in the
  // folded tail — a filter that hides its own selection reads as "no filter".
  if (activeCategory !== ALL_CATEGORIES && !pillCategories.includes(activeCategory)) {
    pillCategories.push(activeCategory);
  }
  const foldedCategories = categories.filter((item) => !pillCategories.includes(item));

  if (!isLoading && recipes.length === 0) return null;

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

      {isLoading ? (
        <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, index) => (
            <Skeleton key={index} className="h-44 rounded-xl" />
          ))}
        </div>
      ) : (
        <>
          {categories.length > 1 ? (
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

          <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {visible.map((recipe) => (
              <button
                key={recipe.slug}
                type="button"
                onClick={() => onUse(recipe)}
                title={recipe.summary || recipe.title}
                className="group/recipe flex min-w-0 cursor-pointer flex-col overflow-hidden rounded-xl border bg-card text-left transition-colors hover:border-primary/50"
              >
                <RecipePreview recipe={recipe} />
                <span className="min-w-0 px-3 py-2.5">
                  <span className="block truncate text-caption font-medium">
                    {recipe.title || recipe.slug}
                  </span>
                </span>
              </button>
            ))}
          </div>
        </>
      )}
    </section>
  );
}
