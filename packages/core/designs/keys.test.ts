import { describe, expect, it } from "vitest";
import { designKeys } from "./keys";

describe("designKeys", () => {
  it("builds workspace-scoped keys", () => {
    expect(designKeys.files("ws-1")).toEqual(["designs", "ws-1", "files"]);
    expect(designKeys.file("ws-1", "design-1")).toEqual(["designs", "ws-1", "files", "design-1"]);
    expect(designKeys.revisions("ws-1", "design-1")).toEqual(["designs", "ws-1", "files", "design-1", "revisions"]);
  });
});
