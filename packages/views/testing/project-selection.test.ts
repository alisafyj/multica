// @vitest-environment node
import { describe, expect, it } from "vitest";
import { resolveSelectedProjectId } from "./project-selection";

describe("resolveSelectedProjectId", () => {
  const projects = [{ id: "p-1" }, { id: "p-2" }];

  it("keeps a persisted project that still exists", () => {
    expect(resolveSelectedProjectId(projects, "p-2")).toBe("p-2");
  });

  it("falls back to the first project when nothing is persisted", () => {
    expect(resolveSelectedProjectId(projects, null)).toBe("p-1");
  });

  // The regression this exists for: the persisted id outlives its project, and
  // the raw value would keep every list querying a project that is gone while
  // the picker shows a live one.
  it("falls back when the persisted project has been deleted", () => {
    expect(resolveSelectedProjectId(projects, "p-gone")).toBe("p-1");
  });

  it("returns an empty id when the workspace has no projects", () => {
    expect(resolveSelectedProjectId([], "p-1")).toBe("");
  });
});
