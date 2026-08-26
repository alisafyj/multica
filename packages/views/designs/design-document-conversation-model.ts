import type { DesignDocumentRevisionSummary, ProjectDesignSystemTask, TaskMessagePayload } from "@multica/core/types";

/**
 * One agent turn on a document: the task that produced (or is producing) a
 * revision, plus what to call it in the thread.
 */
export interface ConversationTurn {
  taskId: string;
  /** What the user asked for. Empty on a first generation. */
  instruction: string;
  /** Set once the turn produced a revision; null while it is still running. */
  revisionNumber: number | null;
  at: string;
  live: boolean;
  /** Only set for the live turn, where the operation is all we know yet. */
  operation: string;
}

/**
 * Orders a document's turns oldest-first.
 *
 * A design run is a one-shot task, so the document's history lives in its
 * revisions — each carries the `source_task_id` that produced it. Replaying
 * them in order and appending the task still running reconstructs a single
 * continuous conversation out of what the server stores as separate tasks.
 *
 * Deduped by task id: a task contributes one turn even if it were ever to
 * write more than one revision, and the running task is appended only when it
 * has not already been credited with a revision — otherwise the turn it just
 * finished would appear twice, once as history and once as live.
 */
export function conversationTurns(
  revisions: DesignDocumentRevisionSummary[],
  activeTask: ProjectDesignSystemTask | null,
): ConversationTurn[] {
  const turns: ConversationTurn[] = [];
  const seen = new Set<string>();
  // `revisions` arrives newest-first; the thread reads oldest-first.
  for (const revision of [...revisions].reverse()) {
    const taskId = revision.source_task_id;
    if (!taskId || seen.has(taskId)) continue;
    seen.add(taskId);
    turns.push({
      taskId,
      instruction: revision.instruction ?? "",
      revisionNumber: revision.revision_number,
      at: revision.created_at,
      live: false,
      operation: "",
    });
  }
  if (activeTask?.id && !seen.has(activeTask.id)) {
    turns.push({
      taskId: activeTask.id,
      instruction: "",
      revisionNumber: null,
      at: activeTask.created_at,
      live: true,
      operation: activeTask.operation,
    });
  }
  return turns;
}

/**
 * The package paths a turn actually wrote, in the order it wrote them.
 *
 * Read off the run's own `patch_apply` calls rather than the revision, so a
 * turn that failed before producing a revision still shows what it managed to
 * write. Deduped: an agent that edits one file three times produced one file.
 */
export function writtenPaths(messages: TaskMessagePayload[]): string[] {
  const paths: string[] = [];
  const seen = new Set<string>();
  for (const message of messages) {
    if (message.type !== "tool_use" || message.tool !== "patch_apply") continue;
    const changes = message.input?.["changes"];
    if (!Array.isArray(changes)) continue;
    for (const change of changes) {
      if (!change || typeof change !== "object") continue;
      const raw = (change as Record<string, unknown>).path;
      const path = typeof raw === "string" ? raw.trim() : "";
      if (!path || seen.has(path)) continue;
      seen.add(path);
      paths.push(path);
    }
  }
  return paths;
}
