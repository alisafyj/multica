import type { TaskMessagePayload } from "@multica/core/types/events";

/**
 * One ambient line for a running design task: what the agent is doing right
 * now, from its newest streamed message. The full history lives in the
 * transcript dialog — this only keeps the wait from being a black box.
 * Canonical matrix in task-message-snippet.test.ts.
 */
export function taskMessageSnippet(message: TaskMessagePayload | undefined): string {
  if (!message) return "";
  if (message.type === "tool_use") {
    return message.tool ? `正在使用 ${message.tool}` : "正在调用工具";
  }
  if (message.type === "tool_result") {
    return message.tool ? `${message.tool} 已返回结果` : "工具已返回结果";
  }
  // text / thinking / error all carry their substance in `content`.
  const content = (message.content ?? "").trim().replace(/\s+/g, " ");
  if (!content) return "";
  return content.length > 80 ? `${content.slice(0, 80)}…` : content;
}
