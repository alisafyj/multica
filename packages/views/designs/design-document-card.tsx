"use client";

import type { DesignDocument } from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
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
 * Placeholder media. Design documents carry no thumbnail today, so this is a
 * composed tile rather than an empty frame that would read as a failed image.
 */
function DocumentPreview({ running }: { running: boolean }) {
  return (
    <div className="relative aspect-[4/3] overflow-hidden bg-muted/50">
      <div className="absolute inset-4 overflow-hidden rounded-lg border bg-background shadow-sm">
        <div className="h-7 border-b bg-muted/40" />
        <div className="grid grid-cols-3 gap-2 p-2.5">
          <span className="h-12 rounded-md bg-primary/10" />
          <span className="h-12 rounded-md bg-primary/5" />
          <span className="h-12 rounded-md bg-primary/10" />
        </div>
        <div className="space-y-1.5 px-2.5">
          <span className="block h-1.5 w-3/4 rounded-full bg-muted" />
          <span className="block h-1.5 w-1/2 rounded-full bg-muted" />
        </div>
      </div>
      {running ? (
        <span className="absolute left-3 top-3 inline-flex h-5 items-center gap-1.5 rounded-full bg-background/90 px-2 text-caption font-medium text-foreground shadow-sm">
          <span className="size-1.5 animate-pulse rounded-full bg-primary" />
          生成中
        </span>
      ) : null}
    </div>
  );
}

/**
 * One design document as a card. There is no document detail route yet, so a
 * card either opens the project that owns it or stays inert — it never links
 * somewhere that does not exist.
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
  const meta = [projectTitle.trim(), designDocumentSourceLabel(document.recipe)]
    .filter((part) => part.length > 0)
    .join(" · ");

  const body = (
    <>
      <DocumentPreview running={running} />
      <div className="min-w-0 p-3">
        <div className="flex min-w-0 items-start justify-between gap-2">
          <span className="min-w-0 flex-1 truncate text-body font-medium">{title}</span>
          {status && !running ? (
            <Badge variant="secondary" className="shrink-0 px-1.5 text-micro font-normal">
              {status}
            </Badge>
          ) : null}
        </div>
        <div className="mt-2.5 flex items-center justify-between gap-2 text-caption text-muted-foreground">
          <span className="truncate">{meta}</span>
          {updatedAt ? <span className="shrink-0">{timeAgo(updatedAt)}</span> : null}
        </div>
      </div>
    </>
  );

  const shell = "flex min-w-0 flex-col overflow-hidden rounded-lg border bg-card text-left";
  if (!onOpen) {
    return <article className={shell}>{body}</article>;
  }
  return (
    <button
      type="button"
      onClick={onOpen}
      title={`在项目「${projectTitle}」的设计稿中查看`}
      className={cn(shell, "cursor-pointer transition-colors hover:border-primary/50")}
    >
      {body}
    </button>
  );
}
