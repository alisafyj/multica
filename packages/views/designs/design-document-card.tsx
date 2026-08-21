"use client";

import type { CSSProperties } from "react";
import type { DesignDocument } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { useTimeAgo } from "../i18n/use-time-ago";

/**
 * The recipes the composer has built in (DC-049). Anything else on a document
 * is a published catalogue slug, which is what "模板" means here — the two are
 * the only origins a document can record, so the badge never guesses.
 */
const BUILTIN_RECIPES: ReadonlySet<string> = new Set([
  "default",
  "ui-mockup",
  "web-clone",
  "wireframe",
  "mobile-app",
  "figma-migration",
]);

/**
 * Status wording for a design document. Server-driven, so an unknown value
 * returns null and the card simply shows no badge — a wrong label would be
 * worse than none.
 */
export function designDocumentStatusLabel(status: string): string | null {
  switch (status) {
    case "empty":
      return "待生成";
    case "running":
      return "生成中";
    case "draft":
    case "draft_ahead_of_saved":
      return "草稿";
    case "saved":
      return "完成";
    case "failed":
      return "失败";
    default:
      return null;
  }
}

export function isDesignDocumentRunning(status: string): boolean {
  return status === "running";
}

export function designDocumentSourceLabel(recipe: string): string {
  const slug = recipe.trim();
  return !slug || BUILTIN_RECIPES.has(slug) ? "AI 生成" : "模板";
}

/**
 * The scenario a document was made for — Open Design's card kind tag
 * (RecentProjectsStrip `ProjectTag` / `projectCategory`), which anchors the
 * bottom-right of every card. Built-in recipes name their own scenario; a
 * catalogue slug has no fixed scenario, so it reads as 模板, the same word
 * {@link designDocumentSourceLabel} uses for that origin.
 */
export function designDocumentKindLabel(recipe: string): string {
  switch (recipe.trim()) {
    case "wireframe":
      return "线框图";
    case "mobile-app":
      return "移动应用";
    case "web-clone":
      return "网站复刻";
    case "figma-migration":
      return "来自 Figma";
    case "":
    case "default":
    case "ui-mockup":
      return "原型";
    default:
      return "模板";
  }
}

/**
 * The cover a document shows, ported from Open Design's `projectCover`
 * fallback branch (RecentProjectsStrip.tsx). A design document carries no
 * thumbnail — the saved package's preview needs a per-revision capability
 * token, far too much for a grid — so every card takes the fallback: a hue
 * derived from the document's own id, plus its first character.
 *
 * Deriving the hue from the id rather than picking one colour keeps a grid of
 * coverless cards distinguishable at a glance and stable across reloads,
 * which a shared neutral placeholder never is: Open Design's cards read as
 * documents, ours read as three identical loading skeletons.
 */
export function designDocumentCover(document: DesignDocument): {
  style: CSSProperties;
  initial: string;
} {
  let hash = 0;
  for (let index = 0; index < document.id.length; index += 1) {
    hash = (hash * 31 + document.id.charCodeAt(index)) >>> 0;
  }
  const hue = hash % 360;
  const secondHue = (hue + 38) % 360;
  const trimmed = document.title.trim();
  return {
    style: {
      background:
        `radial-gradient(circle at 30% 28%, hsl(${hue} 70% 78% / 0.55), transparent 42%),`
        + ` linear-gradient(135deg, hsl(${hue} 65% 88%), hsl(${secondHue} 70% 90%))`,
    },
    initial: (trimmed ? Array.from(trimmed)[0]! : "?").toUpperCase(),
  };
}

/**
 * One design document as a card, in Open Design's recent-projects shape: a
 * 16/9 cover with the caption below it on the page itself. The card carries no
 * surface, border or padding of its own (their `.recent-projects__card` is
 * `background: transparent` with a transparent 1px border) — the cover and the
 * two caption lines are the whole card, and the grid gap is the only spacing
 * between them.
 *
 * `onOpen` takes the user to the document's own workspace; without it the card
 * is plain content rather than a control.
 */
export function DesignDocumentCard({
  document,
  projectTitle,
  onOpen,
}: {
  document: DesignDocument;
  /** Empty renders nothing rather than a placeholder project name. */
  projectTitle: string;
  /** Absent renders the card as plain content instead of a control. */
  onOpen?: () => void;
}) {
  const timeAgo = useTimeAgo();
  const running = isDesignDocumentRunning(document.status);
  const status = designDocumentStatusLabel(document.status);
  const title = document.title.trim() || "未命名设计稿";
  const updatedAt = document.updated_at || document.created_at;
  const cover = designDocumentCover(document);
  const where = [projectTitle.trim(), updatedAt ? timeAgo(updatedAt) : ""]
    .filter((part) => part.length > 0)
    .join(" · ");

  const body = (
    <>
      <div
        style={cover.style}
        aria-hidden
        className="relative flex aspect-[16/9] items-center justify-center overflow-hidden rounded-lg"
      >
        <span className="text-display font-medium text-foreground/25">{cover.initial}</span>
        {/* Hairline ring drawn over the cover, not as a border: a light cover
            would otherwise melt into the page. Open Design raises it to 22%
            for exactly this gradient fallback, where their usual 8% ring
            disappears into the colour variation. */}
        <span
          aria-hidden
          className="pointer-events-none absolute inset-0 rounded-[inherit] border"
          style={{ borderColor: "color-mix(in srgb, var(--foreground) 22%, transparent)" }}
        />
        {status ? (
          <span
            className={cn(
              "absolute left-2 top-2 inline-flex h-5 items-center gap-1.5 rounded-full bg-background/90 px-2 text-caption font-medium shadow-sm",
              document.status === "failed" ? "text-destructive" : "text-foreground",
            )}
          >
            {running ? <span className="size-1.5 animate-pulse rounded-full bg-primary" /> : null}
            {status}
          </span>
        ) : null}
      </div>
      {/* Caption: name, then where·when on the left with the kind tag pushed
          to the bottom-right corner — Open Design's `card-footer`. */}
      <div className="mt-2 flex min-w-0 flex-col gap-1">
        <span className="min-w-0 truncate text-body font-medium group-hover/document:text-primary">
          {title}
        </span>
        <div className="flex min-w-0 items-center justify-between gap-2 text-caption text-muted-foreground">
          <span className="min-w-0 flex-1 truncate">{where}</span>
          <span className="shrink-0">{designDocumentKindLabel(document.recipe)}</span>
        </div>
      </div>
    </>
  );

  const shell = "flex min-w-0 flex-col text-left";
  if (!onOpen) {
    return <article className={shell}>{body}</article>;
  }
  return (
    <button
      type="button"
      onClick={onOpen}
      title={projectTitle ? `打开「${projectTitle}」的这份设计稿` : "打开这份设计稿"}
      className={cn(shell, "group/document cursor-pointer")}
    >
      {body}
    </button>
  );
}
