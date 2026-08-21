"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { ChevronDown, TriangleAlert, Wrench } from "lucide-react";
import { cn } from "@multica/ui/lib/utils";
// Straight from the module, not the package barrel: the barrel also carries
// the transcript dialog and button, so surfaces that mock it for those React
// components would otherwise stub this pure function out from under us.
import { buildTimeline, type TimelineItem } from "../common/task-transcript/build-timeline";
import type { TaskMessagePayload } from "@multica/core/types";

/**
 * How much of the run stays on screen. The full record is one click away in
 * the transcript dialog, which is filterable and virtualised; this stream is
 * the recent tail, so a long run cannot turn the sidebar into a scroll trap.
 */
const VISIBLE_ITEMS = 40;

/** A tool call reads as one line, so its arguments are summarised, not dumped. */
function toolSummary(item: TimelineItem): string {
  const input = item.input ?? {};
  for (const key of ["path", "file_path", "pattern", "command", "url", "query"]) {
    const value = input[key];
    if (typeof value === "string" && value.trim()) return value.trim();
  }
  return "";
}

function ConversationRow({ item }: { item: TimelineItem }) {
  if (item.type === "tool_use") {
    const summary = toolSummary(item);
    return (
      <div className="flex min-w-0 items-baseline gap-1.5 text-caption text-muted-foreground">
        <Wrench className="size-3 shrink-0 translate-y-0.5" />
        <span className="shrink-0 font-medium">{item.tool || "工具"}</span>
        {summary ? <span className="min-w-0 truncate font-mono">{summary}</span> : null}
      </div>
    );
  }
  if (item.type === "error") {
    return (
      <div className="flex min-w-0 items-baseline gap-1.5 text-caption text-destructive">
        <TriangleAlert className="size-3 shrink-0 translate-y-0.5" />
        <span className="min-w-0 whitespace-pre-wrap break-words">{item.content}</span>
      </div>
    );
  }
  // `thinking` is the agent reasoning aloud and `text` is what it says; both
  // are prose, so they read as prose. Thinking stays muted so the two are
  // still distinguishable without a label for each line.
  return (
    <p
      className={cn(
        "min-w-0 whitespace-pre-wrap break-words text-caption",
        item.type === "thinking" ? "text-muted-foreground" : "text-foreground",
      )}
    >
      {item.content}
    </p>
  );
}

/**
 * The run, inline in the sidebar — Open Design's chat pane shape, where the
 * agent's work is the column's content rather than something behind a button.
 *
 * We had the same messages already (the ambient line this replaces showed the
 * newest one, truncated) but only ever surfaced one at a time, so a run read
 * as a spinner with a caption. Rendering the tail turns the sidebar into a
 * conversation: what the agent said, what it ran, and what failed, in order.
 *
 * Follows the newest line while the user is at the bottom and stops the moment
 * they scroll up to read something — a stream that yanks the viewport away
 * mid-sentence is unreadable, and a stream that never follows is stale.
 */
export function DesignRunConversation({
  messages,
  live,
  className,
}: {
  messages: TaskMessagePayload[];
  /** The task is still running: follow the tail and keep the region polite. */
  live: boolean;
  className?: string;
}) {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const [following, setFollowing] = useState(true);

  const items = buildTimeline(messages);
  const visible = items.slice(-VISIBLE_ITEMS);
  const newestSeq = visible[visible.length - 1]?.seq ?? -1;

  // Before paint, so a new line never renders at the old offset first.
  useLayoutEffect(() => {
    const node = scrollRef.current;
    if (!node || !following) return;
    node.scrollTop = node.scrollHeight;
  }, [newestSeq, following]);

  // A finished run leaves the tail on screen: re-arm follow so the next run
  // starts attached rather than wherever the last one was left.
  useEffect(() => {
    if (!live) setFollowing(true);
  }, [live]);

  if (visible.length === 0) return null;

  return (
    <div className={cn("relative", className)}>
      <div
        ref={scrollRef}
        // Live output is a log, not an alert: announce politely, and only
        // while it is actually moving.
        aria-live={live ? "polite" : undefined}
        aria-label="智能体执行过程"
        onScroll={(event) => {
          const node = event.currentTarget;
          // A small slack: "at the bottom" has to survive sub-pixel heights
          // and the row that arrives between the scroll and this handler.
          const atBottom = node.scrollHeight - node.scrollTop - node.clientHeight < 24;
          setFollowing(atBottom);
        }}
        className="no-scrollbar flex max-h-56 flex-col gap-2 overflow-y-auto rounded-lg border bg-muted/30 px-3 py-2.5"
      >
        {visible.map((item) => (
          <ConversationRow key={item.seq} item={item} />
        ))}
      </div>
      {live && !following ? (
        <button
          type="button"
          onClick={() => {
            setFollowing(true);
            const node = scrollRef.current;
            if (node) node.scrollTop = node.scrollHeight;
          }}
          className="absolute bottom-2 left-1/2 flex -translate-x-1/2 cursor-pointer items-center gap-1 rounded-full border bg-background px-2 py-0.5 text-caption text-muted-foreground shadow-sm hover:text-foreground"
        >
          <ChevronDown className="size-3" />
          回到最新
        </button>
      ) : null}
    </div>
  );
}
