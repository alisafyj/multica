"use client";

import { Zap } from "lucide-react";
import { cn } from "@multica/ui/lib/utils";
import { useCommentComposerStore } from "@multica/core/issues/stores";
import { useT } from "../../i18n";

interface ConciseModeToggleProps {
  disabled?: boolean;
}

/**
 * Compact Zap-labelled concise-mode switch for the comment composers.
 *
 * Reads the persisted member-level preference from useCommentComposerStore
 * (shared by the top-level composer and every reply box, so one conversation
 * keeps one mode — mirrors the chat store's per-session concise selection,
 * SY-326). Rendered only when the trigger preview resolved at least one agent,
 * matching the chat composer's !noAgent gate: never a dead switch.
 */
export function ConciseModeToggle({ disabled }: ConciseModeToggleProps) {
  const { t } = useT("issues");
  const concise = useCommentComposerStore((s) => s.concise);
  const setConcise = useCommentComposerStore((s) => s.setConcise);

  return (
    <label
      className={cn(
        "flex h-6 cursor-pointer items-center gap-1 rounded-full border border-transparent px-1.5 text-caption text-muted-foreground transition-colors",
        // Selected state must stay identifiable on hover, so the active
        // styling sits on the label, not a hover-only rule.
        concise
          ? "border-brand/30 bg-brand/10 text-foreground"
          : "hover:bg-accent",
      )}
      title={t(($) => $.comment.concise_mode_tooltip)}
    >
      <input
        type="checkbox"
        className="h-3 w-3 accent-foreground"
        checked={concise}
        aria-label={t(($) => $.comment.concise_mode_aria)}
        disabled={disabled}
        onChange={(e) => setConcise(e.target.checked)}
      />
      <Zap className="h-3 w-3" aria-hidden="true" />
    </label>
  );
}
