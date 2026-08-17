"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronRight } from "lucide-react";
import { designScenarioRecipeListOptions } from "@multica/core/designs/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import type { DesignScenarioRecipe } from "@multica/core/types";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { DesignFilterPill } from "./design-filter-pill";

// Sentinel for "no category picked". Recipe categories are free-form server
// strings, so no real value can be reserved for it.
const ALL_CATEGORIES = "__all__";

/**
 * Card media. Recipes rarely ship a preview image, so the fallback is a
 * composed tile rather than an empty frame that would read as a failed load.
 */
function ExampleThumb({ recipe }: { recipe: DesignScenarioRecipe }) {
  if (recipe.preview_path) {
    return (
      <div className="aspect-[16/10] overflow-hidden bg-muted/40">
        <img src={recipe.preview_path} alt="" loading="lazy" className="size-full object-cover" />
      </div>
    );
  }
  return (
    <div className="relative aspect-[16/10] overflow-hidden bg-muted/40">
      <div className="absolute inset-3 rounded-lg border bg-background p-2.5 shadow-sm">
        <span className="block h-1.5 w-2/5 rounded-full bg-primary/30" />
        <span className="mt-1.5 block h-1.5 w-3/4 rounded-full bg-muted" />
        <span className="mt-2.5 block h-8 rounded-md bg-muted/80" />
      </div>
    </div>
  );
}

/**
 * Example prompts on the create panel. Every card is a published catalogue
 * recipe — the same data the community tab shows — so picking one seeds the
 * brief exactly as the gallery's "填入首页" does, without a second round trip.
 *
 * The category row is built from the catalogue's own categories rather than a
 * fixed list: a hard-coded taxonomy would render filters that match nothing
 * the moment the catalogue says something else.
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
        <div className="mt-3 flex gap-3 overflow-hidden">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-40 w-52 shrink-0 rounded-xl" />
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
              {categories.map((item) => (
                <DesignFilterPill
                  key={item}
                  label={item}
                  selected={activeCategory === item}
                  onClick={() => setCategory(item)}
                />
              ))}
            </div>
          ) : null}

          <div className="-mx-1 mt-3 flex snap-x gap-3 overflow-x-auto px-1 pb-2">
            {visible.map((recipe) => (
              <button
                key={recipe.slug}
                type="button"
                onClick={() => onUse(recipe)}
                title={recipe.summary || recipe.title}
                className="flex w-52 shrink-0 snap-start cursor-pointer flex-col overflow-hidden rounded-xl border bg-card text-left transition-colors hover:border-primary/50"
              >
                <ExampleThumb recipe={recipe} />
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
