// @vitest-environment node

import { describe, expect, it } from "vitest";
import type { TaskMessagePayload } from "@multica/core/types/events";
import { taskMessageSnippet } from "./task-message-snippet";

function message(overrides: Partial<TaskMessagePayload>): TaskMessagePayload {
  return { task_id: "task-1", issue_id: "", seq: 1, type: "text", ...overrides };
}

describe("taskMessageSnippet", () => {
  it("names the tool a tool event is about", () => {
    expect(taskMessageSnippet(message({ type: "tool_use", tool: "Read" }))).toBe("正在使用 Read");
    expect(taskMessageSnippet(message({ type: "tool_use" }))).toBe("正在调用工具");
    expect(taskMessageSnippet(message({ type: "tool_result", tool: "Bash" }))).toBe("Bash 已返回结果");
    expect(taskMessageSnippet(message({ type: "tool_result" }))).toBe("工具已返回结果");
  });

  it("collapses text-bearing events to a single trimmed line", () => {
    expect(taskMessageSnippet(message({ content: "  正在梳理\n页面结构  " }))).toBe("正在梳理 页面结构");
    expect(taskMessageSnippet(message({ type: "thinking", content: "先看品牌色" }))).toBe("先看品牌色");
    expect(taskMessageSnippet(message({ type: "error", content: "渲染失败" }))).toBe("渲染失败");
  });

  it("caps the line so the card cannot grow with the message", () => {
    const long = "很".repeat(120);
    const snippet = taskMessageSnippet(message({ content: long }));
    expect(snippet.endsWith("…")).toBe(true);
    expect(snippet.length).toBe(81);
  });

  it("stays empty when there is nothing to say", () => {
    expect(taskMessageSnippet(undefined)).toBe("");
    expect(taskMessageSnippet(message({ content: "   " }))).toBe("");
  });
});
