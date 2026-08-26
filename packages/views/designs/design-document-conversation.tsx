"use client";

import { type RefObject, useMemo } from "react";
import { useQueries } from "@tanstack/react-query";
import { FileText, LoaderCircle } from "lucide-react";
import { api } from "@multica/core/api";
import { taskMessagesOptions } from "@multica/core/chat/queries";
import type {
  DesignDocumentRevision,
  DesignDocumentRevisionSummary,
  ProjectDesignSystemTask,
  TaskMessagePayload,
} from "@multica/core/types";
import { type ConversationTurn, conversationTurns, writtenPaths } from "./design-document-conversation-model";
import { cn } from "@multica/ui/lib/utils";
import { DesignRunConversation } from "./design-run-conversation";
import { taskOperationLabel } from "./project-design-system-task-activity";

function turnTitle(turn: ConversationTurn): string {
  if (turn.instruction.trim()) return turn.instruction.trim();
  if (turn.revisionNumber === 1) return "首次生成";
  if (turn.live) return taskOperationLabel(turn.operation);
  return "生成";
}

function turnBadge(turn: ConversationTurn): string {
  return turn.revisionNumber === null ? "进行中" : `第 ${turn.revisionNumber} 版`;
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const kb = bytes / 1024;
  return kb < 1024 ? `${kb.toFixed(1)} KB` : `${(kb / 1024).toFixed(1)} MB`;
}

/**
 * What a turn wrote, with a way in when the bytes are still reachable.
 *
 * Open/download need a capability-scoped URL, which only the loaded revision
 * carries. A turn whose revision is not the one on screen still lists its
 * files — the paths are the honest record — but without links that would 404.
 */
export function ProducedFiles({
  paths,
  revision,
}: {
  paths: string[];
  revision: DesignDocumentRevision | undefined;
}) {
  const index = new Map((revision?.files ?? []).map((file) => [file.path, file]));
  return (
    <div className="mt-2 min-w-0">
      <div className="text-caption font-medium text-muted-foreground">本轮产出的文件</div>
      <ul className="mt-1 divide-y border-y">
        {paths.map((path) => {
          const file = index.get(path);
          const url = file && revision?.resource_base_path
            ? api.getDesignDocumentPreviewFileURL(revision.resource_base_path, file.path)
            : "";
          return (
            <li key={path} className="flex items-baseline gap-2 py-1.5 text-caption">
              <FileText className="size-3 shrink-0 translate-y-0.5 text-muted-foreground" />
              <span className="min-w-0 flex-1 truncate font-mono text-foreground">{path}</span>
              {file ? (
                <span className="shrink-0 text-muted-foreground">{formatBytes(file.size_bytes)}</span>
              ) : null}
              {url ? (
                <>
                  <a
                    href={url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="shrink-0 text-muted-foreground hover:text-foreground hover:underline"
                  >
                    打开
                  </a>
                  <a
                    href={url}
                    download={path.split("/").pop() || path}
                    className="shrink-0 text-muted-foreground hover:text-foreground hover:underline"
                  >
                    下载
                  </a>
                </>
              ) : null}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/**
 * The document's whole agent history as one thread.
 *
 * Replaces the activity panel that only existed while a task was running: the
 * work an agent did stays readable after its task ends, which is what makes
 * the adjustment box below read as "send the next message" rather than "start
 * an unrelated job".
 */
export function DesignDocumentConversation({
  revisions,
  activeTask,
  revision,
  onAnswerForm,
  scrollParentRef,
  className,
}: {
  revisions: DesignDocumentRevisionSummary[];
  activeTask: ProjectDesignSystemTask | null;
  /** The revision currently loaded on the page; the only one whose files are fetchable. */
  revision?: DesignDocumentRevision;
  /**
   * Receives a submitted question-form's answer. Absent renders forms
   * read-only, which is the honest state where a reply has nowhere to go.
   */
  onAnswerForm?: (text: string) => void;
  /** The scrollable ancestor; passing one renders every turn flush. */
  scrollParentRef?: RefObject<HTMLElement | null>;
  className?: string;
}) {
  const turns = useMemo(() => conversationTurns(revisions, activeTask), [revisions, activeTask]);
  const results = useQueries({
    queries: turns.map((turn) => taskMessagesOptions(turn.taskId)),
  });

  if (turns.length === 0) return null;

  return (
    <section aria-label="智能体对话" className={cn("flex flex-col gap-4", className)}>
      {turns.map((turn, index) => {
        const messages: TaskMessagePayload[] = results[index]?.data ?? [];
        const produced = writtenPaths(messages);
        const loading = results[index]?.isPending === true;
        return (
          <article key={turn.taskId} className="min-w-0">
            <header className="flex items-baseline gap-2" title={turn.at}>
              {turn.live ? (
                <LoaderCircle className="size-3 shrink-0 translate-y-0.5 animate-spin text-muted-foreground" />
              ) : null}
              <h3 className="min-w-0 flex-1 truncate text-caption font-medium text-foreground">
                {turnTitle(turn)}
              </h3>
              <span className="shrink-0 text-caption text-muted-foreground">{turnBadge(turn)}</span>
            </header>
            {loading ? (
              <p className="mt-2 text-caption text-muted-foreground">正在读取这一轮的记录…</p>
            ) : messages.length === 0 ? (
              // A finished turn with nothing to show: the run predates message
              // capture, or its transcript was pruned. Say so instead of
              // rendering an empty block that reads as a stalled agent.
              <p className="mt-2 text-caption text-muted-foreground">这一轮没有留下可显示的过程记录。</p>
            ) : (
              <>
                <DesignRunConversation
                  messages={messages}
                  live={turn.live}
                  onAnswerForm={turn.live ? onAnswerForm : undefined}
                  {...(scrollParentRef ? { scrollParentRef } : {})}
                  className="mt-2"
                />
                {produced.length > 0 ? (
                  <ProducedFiles
                    paths={produced}
                    revision={turn.revisionNumber === revision?.revision_number ? revision : undefined}
                  />
                ) : null}
              </>
            )}
          </article>
        );
      })}
    </section>
  );
}
