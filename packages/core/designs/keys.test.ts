import { describe, expect, it } from "vitest";
import { designKeys } from "./keys";
import {
  projectDesignSystemByProjectOptions,
  projectDesignSystemDetailOptions,
} from "./queries";

describe("designKeys", () => {
  it("builds workspace-scoped keys", () => {
    expect(designKeys.files("ws-1")).toEqual(["designs", "ws-1", "files"]);
    expect(designKeys.file("ws-1", "design-1")).toEqual(["designs", "ws-1", "files", "design-1"]);
    expect(designKeys.revisions("ws-1", "design-1")).toEqual(["designs", "ws-1", "files", "design-1", "revisions"]);
    expect(designKeys.projectDesignSystemProjectScopes("ws-1", "project-1")).toEqual([
      "designs",
      "ws-1",
      "project-design-systems",
      "project",
      "project-1",
    ]);
    expect(designKeys.projectDesignSystemByProject("ws-1", "project-1")).toEqual([
      "designs",
      "ws-1",
      "project-design-systems",
      "project",
      "project-1",
      "project-level",
    ]);
    expect(designKeys.projectDesignSystem("ws-1", "system-1")).toEqual([
      "designs",
      "ws-1",
      "project-design-systems",
      "system",
      "system-1",
    ]);
    expect(designKeys.projectDesignSystemPackagePreview("ws-1", "system-1")).toEqual([
      "designs",
      "ws-1",
      "project-design-systems",
      "system",
      "system-1",
      "package-preview",
    ]);
  });

  it("keeps project design system query options workspace-scoped", () => {
    expect(projectDesignSystemByProjectOptions("ws-1", "project-1").queryKey).toEqual(
      designKeys.projectDesignSystemByProject("ws-1", "project-1"),
    );
    expect(projectDesignSystemDetailOptions("ws-2", "system-1").queryKey).toEqual(
      designKeys.projectDesignSystem("ws-2", "system-1"),
    );
    expect(projectDesignSystemByProjectOptions("ws-1", "").enabled).toBe(false);
    expect(projectDesignSystemDetailOptions("ws-1", "").enabled).toBe(false);
  });

  it("separates repository scopes so a repository switch cannot serve another one's system", () => {
    const projectLevel = designKeys.projectDesignSystemByProject("ws-1", "project-1");
    const h5 = designKeys.projectDesignSystemByProject("ws-1", "project-1", "resource-h5");
    const admin = designKeys.projectDesignSystemByProject("ws-1", "project-1", "resource-admin");

    expect(h5).toEqual([...designKeys.projectDesignSystemProjectScopes("ws-1", "project-1"), "resource-h5"]);
    expect(new Set([projectLevel, h5, admin].map((key) => JSON.stringify(key))).size).toBe(3);
    // Every scope stays under the project prefix, so realtime events that only
    // know the project can still invalidate all of them.
    for (const key of [projectLevel, h5, admin]) {
      expect(key.slice(0, 5)).toEqual(designKeys.projectDesignSystemProjectScopes("ws-1", "project-1"));
    }
    // An empty repository id is the project-level scope, not a fourth key.
    expect(designKeys.projectDesignSystemByProject("ws-1", "project-1", "")).toEqual(projectLevel);
    expect(designKeys.projectDesignSystemByProject("ws-1", "project-1", null)).toEqual(projectLevel);
    expect(projectDesignSystemByProjectOptions("ws-1", "project-1", "resource-h5").queryKey).toEqual(h5);
  });
});
