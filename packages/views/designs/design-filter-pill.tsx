"use client";

import type { LucideIcon } from "lucide-react";
import { cn } from "@multica/ui/lib/utils";

/**
 * The one filter pill of the design centre: community scopes, ownership,
 * categories, the system library and the project's artifact row all use it, so
 * a filter looks and behaves the same wherever it appears.
 *
 * Selection is carried by border, surface, weight and text colour — dimensions
 * hover never touches — and the hover compound is spelled out, so hovering the
 * active pill can never visually downgrade it to a plain hover.
 */
export function DesignFilterPill({
  label,
  icon: Icon,
  count,
  selected,
  disabled,
  title,
  onClick,
}: {
  label: string;
  /** Leading glyph, for rows where the pills name kinds of thing rather than facets. */
  icon?: LucideIcon;
  /** Omitted renders the pill without a number rather than with a zero. */
  count?: number;
  selected: boolean;
  /** For a facet nothing can ever match yet; `title` should say why. */
  disabled?: boolean;
  title?: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-pressed={disabled ? undefined : selected}
      disabled={disabled}
      title={title}
      onClick={onClick}
      className={cn(
        "flex max-w-56 shrink-0 items-center gap-1.5 rounded-full border px-3 py-1 text-caption transition-colors",
        disabled
          ? "cursor-not-allowed border-dashed text-muted-foreground opacity-70"
          : selected
            ? "cursor-pointer border-primary bg-accent font-medium text-primary hover:border-primary hover:bg-accent hover:text-primary"
            : "cursor-pointer bg-card text-muted-foreground hover:bg-accent hover:text-foreground",
      )}
    >
      {Icon ? <Icon className="size-3.5 shrink-0" /> : null}
      <span className="truncate">{label}</span>
      {typeof count === "number" ? (
        <span
          className={cn(
            "shrink-0 tabular-nums",
            !disabled && selected ? "text-primary/70" : "text-muted-foreground",
          )}
        >
          {count}
        </span>
      ) : null}
    </button>
  );
}
